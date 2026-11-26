package syncengine

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockWebDAVServer implements an in-memory WebDAV server with request counting,
// concurrency tracking, programmable 429/503 Retry-After injection, and
// PROPFIND/MKCOL/GET/PUT/DELETE/MOVE handlers.
type mockWebDAVServer struct {
	mu          sync.Mutex
	files       map[string][]byte
	dirs        map[string]bool
	reqLog      []string
	propfindCnt atomic.Int64
	reqCnt      atomic.Int64
	inFlight    atomic.Int64
	maxInFlight atomic.Int64

	// Optional fault injection queue: each status code consumed once before normal handling.
	faultMu sync.Mutex
	faults  []faultSpec
}

type faultSpec struct {
	status     int
	retryAfter string
}

func newMockWebDAVServer() *mockWebDAVServer {
	return &mockWebDAVServer{
		files: map[string][]byte{},
		dirs:  map[string]bool{"": true},
	}
}

func (m *mockWebDAVServer) enqueueFault(status int, retryAfter string) {
	m.faultMu.Lock()
	m.faults = append(m.faults, faultSpec{status: status, retryAfter: retryAfter})
	m.faultMu.Unlock()
}

func (m *mockWebDAVServer) popFault() (faultSpec, bool) {
	m.faultMu.Lock()
	defer m.faultMu.Unlock()
	if len(m.faults) == 0 {
		return faultSpec{}, false
	}
	f := m.faults[0]
	m.faults = m.faults[1:]
	return f, true
}

func (m *mockWebDAVServer) resetCounts() {
	m.reqCnt.Store(0)
	m.propfindCnt.Store(0)
	m.mu.Lock()
	m.reqLog = nil
	m.mu.Unlock()
}

func (m *mockWebDAVServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cur := m.inFlight.Add(1)
	for {
		prev := m.maxInFlight.Load()
		if cur <= prev || m.maxInFlight.CompareAndSwap(prev, cur) {
			break
		}
	}
	defer m.inFlight.Add(-1)

	m.reqCnt.Add(1)
	if r.Method == "PROPFIND" {
		m.propfindCnt.Add(1)
	}

	rel := strings.TrimPrefix(r.URL.Path, "/")
	if decoded, err := url.PathUnescape(r.URL.EscapedPath()); err == nil {
		rel = strings.TrimPrefix(decoded, "/")
	}

	m.mu.Lock()
	m.reqLog = append(m.reqLog, r.Method+" "+rel)
	m.mu.Unlock()

	if f, ok := m.popFault(); ok {
		if f.retryAfter != "" {
			w.Header().Set("Retry-After", f.retryAfter)
		}
		w.WriteHeader(f.status)
		return
	}

	switch r.Method {
	case http.MethodGet:
		m.mu.Lock()
		data, ok := m.files[rel]
		m.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// Intentionally emit weak/unstable ETag to prove engine ignores fragile ETags.
		w.Header().Set("ETag", fmt.Sprintf(`W/"unstable-%d"`, time.Now().UnixNano()))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)

	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		m.mu.Lock()
		m.files[rel] = body
		m.mu.Unlock()
		w.WriteHeader(http.StatusCreated)

	case http.MethodDelete:
		m.mu.Lock()
		_, ok := m.files[rel]
		delete(m.files, rel)
		m.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	case "MKCOL":
		dir := strings.Trim(rel, "/")
		m.mu.Lock()
		exists := m.dirs[dir]
		m.dirs[dir] = true
		m.mu.Unlock()
		if exists {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusCreated)

	case "MOVE":
		dstRaw := r.Header.Get("Destination")
		dstURL, err := url.Parse(dstRaw)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		dstRel := strings.TrimPrefix(dstURL.Path, "/")
		overwrite := strings.ToUpper(r.Header.Get("Overwrite")) != "F"
		m.mu.Lock()
		srcData, ok := m.files[rel]
		if !ok {
			m.mu.Unlock()
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if _, exists := m.files[dstRel]; exists && !overwrite {
			m.mu.Unlock()
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		m.files[dstRel] = append([]byte(nil), srcData...)
		delete(m.files, rel)
		m.mu.Unlock()
		w.WriteHeader(http.StatusCreated)

	case "PROPFIND":
		dir := strings.Trim(rel, "/")
		m.mu.Lock()
		var children []string
		for k := range m.files {
			if dir == "" || strings.HasPrefix(k, dir+"/") {
				children = append(children, k)
			}
		}
		sort.Strings(children)
		var buf bytes.Buffer
		buf.WriteString(`<?xml version="1.0" encoding="utf-8"?><D:multistatus xmlns:D="DAV:">`)
		buf.WriteString(fmt.Sprintf(`<D:response><D:href>/%s</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`, dir))
		for _, c := range children {
			buf.WriteString(fmt.Sprintf(`<D:response><D:href>/%s</D:href><D:propstat><D:prop><D:resourcetype/><D:getcontentlength>%d</D:getcontentlength></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`, c, len(m.files[c])))
		}
		buf.WriteString(`</D:multistatus>`)
		m.mu.Unlock()
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write(buf.Bytes())

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func TestNutstoreAutoThrottleAndRetryAfterBackoff(t *testing.T) {
	// 1. Verify jianguoyun.com URL automatically locks concurrency=1 and Nutstore mode.
	nutClient, err := NewWebDAVClient(WebDAVConfig{
		BaseURL:     "https://dav.jianguoyun.com/dav/AsterLab",
		Concurrency: 8, // must be overridden to 1
	})
	if err != nil {
		t.Fatalf("NewWebDAVClient nutstore: %v", err)
	}
	if !nutClient.IsNutstoreMode() || nutClient.EffectiveConcurrency() != 1 {
		t.Fatalf("expected Nutstore mode with concurrency=1, got isNutstore=%v conc=%d",
			nutClient.IsNutstoreMode(), nutClient.EffectiveConcurrency())
	}

	// 2. Test 503 + 429 Retry-After exponential backoff against httptest server.
	srv := newMockWebDAVServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	srv.enqueueFault(http.StatusServiceUnavailable, "1")
	srv.enqueueFault(http.StatusTooManyRequests, "2")

	var clockMu sync.Mutex
	simNow := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	var recordedSleeps []time.Duration
	client, err := NewWebDAVClient(WebDAVConfig{
		BaseURL:           ts.URL,
		ForceNutstoreMode: true,
		InitialBackoff:    50 * time.Millisecond,
		MaxBackoff:        10 * time.Second,
		MaxRetries:        4,
		NowFn: func() time.Time {
			clockMu.Lock()
			defer clockMu.Unlock()
			return simNow
		},
		SleepFn: func(_ context.Context, d time.Duration) error {
			clockMu.Lock()
			simNow = simNow.Add(d)
			recordedSleeps = append(recordedSleeps, d)
			clockMu.Unlock()
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewWebDAVClient: %v", err)
	}

	ctx := context.Background()
	payload := []byte("CCK-8 assay data payload")
	if err := client.Put(ctx, "exp/cck8.asterml", payload, "text/markdown"); err != nil {
		t.Fatalf("Put after 503/429 backoff failed: %v", err)
	}

	clockMu.Lock()
	sleeps := append([]time.Duration(nil), recordedSleeps...)
	clockMu.Unlock()
	if len(sleeps) != 2 || sleeps[0] < 1*time.Second || sleeps[1] < 2*time.Second {
		t.Fatalf("expected Retry-After sleeps >= [1s, 2s], got %v", sleeps)
	}
	if stats := client.Stats(); stats.RateLimitHits != 2 {
		t.Fatalf("RateLimitHits = %d, want 2", stats.RateLimitHits)
	}

	// Verify GET with SHA-256 verification, MOVE, PROPFIND, and DELETE.
	got, err := client.GetVerified(ctx, "exp/cck8.asterml", SHA256Bytes(payload))
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("GetVerified mismatch: err=%v len=%d", err, len(got))
	}
	if err := client.Move(ctx, "exp/cck8.asterml", "exp/cck8-moved.asterml", false); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
	resList, err := client.Propfind(ctx, "exp", "1")
	if err != nil || len(resList) < 2 {
		t.Fatalf("Propfind failed: err=%v list=%+v", err, resList)
	}
	if err := client.Delete(ctx, "exp/cck8-moved.asterml"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify Nutstore concurrency=1 holds even under concurrent goroutines.
	srv.maxInFlight.Store(0)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = client.Put(ctx, fmt.Sprintf("batch/f-%d.asterml", idx), []byte("x"), "text/plain")
		}(i)
	}
	wg.Wait()
	if srv.maxInFlight.Load() > 1 {
		t.Fatalf("Nutstore concurrency violated: maxInFlight=%d", srv.maxInFlight.Load())
	}
}

func TestManifestSync50UnchangedFilesUsesAtMostTwoHTTPRequests(t *testing.T) {
	srv := newMockWebDAVServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	vaultA := t.TempDir()
	for i := 0; i < 50; i++ {
		p := filepath.Join(vaultA, "2026", fmt.Sprintf("exp-%02d.asterml", i))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(fmt.Sprintf("---\ntitle: Exp %02d\n---\n# Exp %02d\n", i, i)), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	clientA, err := NewWebDAVClient(WebDAVConfig{BaseURL: ts.URL, ForceNutstoreMode: true, RateBurst: 100, RateLimitPerMinute: 6000})
	if err != nil {
		t.Fatal(err)
	}
	engA, err := NewEngine(vaultA, clientA, DefaultDesktopDownloadPolicy(), "desktop-mac")
	if err != nil {
		t.Fatal(err)
	}

	rep1, err := engA.Sync(context.Background())
	if err != nil {
		t.Fatalf("initial Sync failed: %v", err)
	}
	if len(rep1.UploadedFiles) != 50 {
		t.Fatalf("expected 50 uploaded files, got %d", len(rep1.UploadedFiles))
	}

	// Second sync with 50 unchanged files MUST use <= 2 HTTP requests and ZERO PROPFINDs!
	srv.resetCounts()
	rep2, err := engA.Sync(context.Background())
	if err != nil {
		t.Fatalf("unchanged Sync failed: %v", err)
	}
	if rep2.HTTPRequests > 2 {
		t.Fatalf("unchanged 50-file sync sent %d HTTP requests (want <= 2); log=%v", rep2.HTTPRequests, srv.reqLog)
	}
	if srv.propfindCnt.Load() != 0 {
		t.Fatalf("expected 0 PROPFIND requests during manifest sync, got %d", srv.propfindCnt.Load())
	}
	if rep2.ManifestUpdated {
		t.Fatalf("expected ManifestUpdated=false on unchanged sync")
	}

	// Also verify a fresh device that already has the same 50 files adopts them in <= 2 requests.
	vaultB := t.TempDir()
	for i := 0; i < 50; i++ {
		p := filepath.Join(vaultB, "2026", fmt.Sprintf("exp-%02d.asterml", i))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(fmt.Sprintf("---\ntitle: Exp %02d\n---\n# Exp %02d\n", i, i)), 0o644)
	}
	clientB, _ := NewWebDAVClient(WebDAVConfig{BaseURL: ts.URL})
	engB, _ := NewEngine(vaultB, clientB, DefaultDesktopDownloadPolicy(), "desktop-win")
	srv.resetCounts()
	repB, err := engB.Sync(context.Background())
	if err != nil {
		t.Fatalf("fresh device adopt Sync failed: %v", err)
	}
	if repB.HTTPRequests > 2 {
		t.Fatalf("fresh device adopt sync sent %d HTTP requests (want <= 2)", repB.HTTPRequests)
	}
}

func TestScientificOriginalUploadAndMobileThresholdPreviewOnlyAndOnDemandFetch(t *testing.T) {
	srv := newMockWebDAVServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Build a real high-resolution >2MB noisy fluorescence PNG (1600x1200) so PNG stays >2MB
	// while its 1200px JPEG preview sidecar is compact (~150-350KB).
	largePNG := buildLargeNoisyPNG(t, 1600, 1200)
	if int64(len(largePNG)) <= Threshold2MB {
		t.Fatalf("expected test PNG > 2MB, got %d bytes", len(largePNG))
	}
	smallPNG := buildSolidPNG(t, 400, 300, color.RGBA{R: 40, G: 200, B: 120, A: 255})

	desktopVault := t.TempDir()
	largeRel := "assets/fluorescence_dapi.png"
	smallRel := "assets/plate_thumb.png"
	_ = os.MkdirAll(filepath.Join(desktopVault, "assets"), 0o755)
	_ = os.WriteFile(filepath.Join(desktopVault, filepath.FromSlash(largeRel)), largePNG, 0o644)
	_ = os.WriteFile(filepath.Join(desktopVault, filepath.FromSlash(smallRel)), smallPNG, 0o644)

	desktopClient, _ := NewWebDAVClient(WebDAVConfig{BaseURL: ts.URL})
	desktopEng, _ := NewEngine(desktopVault, desktopClient, DefaultDesktopDownloadPolicy(), "lab-pc")
	if _, err := desktopEng.Sync(context.Background()); err != nil {
		t.Fatalf("desktop Sync failed: %v", err)
	}

	// Assert lossless original stored under assets/originals/<sha256>.png AND preview under .asterlab/previews/<sha256>.jpg.
	largeSHA := SHA256Bytes(largePNG)
	origRemoteKey := CanonicalOriginalPath(largeSHA, largeRel)
	prevRemoteKey := CanonicalPreviewPath(largeSHA)

	srv.mu.Lock()
	storedOrig := srv.files[origRemoteKey]
	storedPrev := srv.files[prevRemoteKey]
	srv.mu.Unlock()
	if !bytes.Equal(storedOrig, largePNG) {
		t.Fatalf("remote original %s is not 100%% lossless identical (got %d want %d bytes)", origRemoteKey, len(storedOrig), len(largePNG))
	}
	if len(storedPrev) == 0 || len(storedPrev) >= len(largePNG)/4 {
		t.Fatalf("expected compact preview sidecar at %s, got %d bytes", prevRemoteKey, len(storedPrev))
	}
	prevCfg, err := jpeg.DecodeConfig(bytes.NewReader(storedPrev))
	if err != nil {
		t.Fatalf("decode preview JPEG config: %v", err)
	}
	if prevCfg.Width > DefaultPreviewMaxDim || prevCfg.Height > DefaultPreviewMaxDim {
		t.Fatalf("preview exceeds maxDim=%d: %dx%d", DefaultPreviewMaxDim, prevCfg.Width, prevCfg.Height)
	}

	// Mobile pulls with default 2MB threshold -> large image downloads ONLY preview; small image downloads original.
	mobileVault := t.TempDir()
	mobileClient, _ := NewWebDAVClient(WebDAVConfig{BaseURL: ts.URL})
	mobilePolicy := DefaultMobileDownloadPolicy()
	mobilePolicy.IsMeteredNetwork = false // Wi-Fi
	mobileEng, _ := NewEngine(mobileVault, mobileClient, mobilePolicy, "iphone-16")

	repMob, err := mobileEng.Sync(context.Background())
	if err != nil {
		t.Fatalf("mobile Sync failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(mobileVault, filepath.FromSlash(largeRel))); !os.IsNotExist(err) {
		t.Fatalf("expected large original %s NOT downloaded on mobile", largeRel)
	}
	if _, err := os.Stat(filepath.Join(mobileVault, filepath.FromSlash(prevRemoteKey))); err != nil {
		t.Fatalf("expected preview sidecar %s downloaded on mobile: %v", prevRemoteKey, err)
	}
	if _, err := os.Stat(filepath.Join(mobileVault, filepath.FromSlash(smallRel))); err != nil {
		t.Fatalf("expected small asset %s (<2MB) downloaded on mobile: %v", smallRel, err)
	}
	if len(repMob.NeedsOriginalDownload) != 1 || repMob.NeedsOriginalDownload[0] != largeRel {
		t.Fatalf("NeedsOriginalDownload = %v, want [%s]", repMob.NeedsOriginalDownload, largeRel)
	}

	// Subsequent mobile sync must NOT treat deferred large original as local deletion and uses <=2 HTTP requests.
	srv.resetCounts()
	repMob2, err := mobileEng.Sync(context.Background())
	if err != nil {
		t.Fatalf("second mobile Sync failed: %v", err)
	}
	if repMob2.HTTPRequests > 2 {
		t.Fatalf("expected <=2 HTTP requests on unchanged mobile sync, got %d", repMob2.HTTPRequests)
	}

	// On-demand original fetch pulls 100% lossless original and clears NeedsOriginalDownload.
	if err := mobileEng.FetchOriginalOnDemand(context.Background(), largeRel); err != nil {
		t.Fatalf("FetchOriginalOnDemand failed: %v", err)
	}
	fetchedBytes, err := os.ReadFile(filepath.Join(mobileVault, filepath.FromSlash(largeRel)))
	if err != nil || !bytes.Equal(fetchedBytes, largePNG) {
		t.Fatalf("on-demand original mismatch: err=%v", err)
	}
	stAfterFetch, _ := LoadLocalSyncState(mobileVault, "iphone-16")
	if stAfterFetch.Files[largeRel].NeedsOriginalDownload {
		t.Fatalf("expected NeedsOriginalDownload=false after FetchOriginalOnDemand")
	}

	// ClearLocalOriginalsCache frees local originals while leaving previews & remote originals intact.
	usedBytes, usedCount, err := mobileEng.LocalOriginalsCacheUsage()
	if err != nil || usedCount != 2 || usedBytes < int64(len(largePNG)) {
		t.Fatalf("unexpected LocalOriginalsCacheUsage: bytes=%d count=%d err=%v", usedBytes, usedCount, err)
	}
	cleared, err := mobileEng.ClearLocalOriginalsCache()
	if err != nil {
		t.Fatalf("ClearLocalOriginalsCache failed: %v", err)
	}
	if cleared.FilesCleared != 2 || cleared.BytesFreed < int64(len(largePNG)) {
		t.Fatalf("unexpected CacheClearResult: %+v", cleared)
	}
	if _, err := os.Stat(filepath.Join(mobileVault, filepath.FromSlash(prevRemoteKey))); err != nil {
		t.Fatalf("preview sidecar must survive ClearLocalOriginalsCache: %v", err)
	}
	srv.mu.Lock()
	stillOnRemote := len(srv.files[origRemoteKey]) == len(largePNG)
	srv.mu.Unlock()
	if !stillOnRemote {
		t.Fatalf("remote original must never be deleted by ClearLocalOriginalsCache")
	}

	// Verify cellular WiFiOnlyForOriginals defers even small assets.
	cellularVault := t.TempDir()
	cellClient, _ := NewWebDAVClient(WebDAVConfig{BaseURL: ts.URL})
	cellPolicy := DefaultMobileDownloadPolicy()
	cellPolicy.IsMeteredNetwork = true
	cellPolicy.WiFiOnlyForOriginals = true
	cellEng, _ := NewEngine(cellularVault, cellClient, cellPolicy, "iphone-5g")
	repCell, err := cellEng.Sync(context.Background())
	if err != nil {
		t.Fatalf("cellular Sync failed: %v", err)
	}
	if len(repCell.NeedsOriginalDownload) != 2 {
		t.Fatalf("expected both assets deferred under WiFiOnlyForOriginals on cellular, got %v", repCell.NeedsOriginalDownload)
	}
}

func TestAsterMLConflictCopyPreservationAndPantryAppendMerge(t *testing.T) {
	srv := newMockWebDAVServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	vaultA := t.TempDir()
	vaultB := t.TempDir()
	expRel := "2026/qpcr.asterml"
	_ = os.MkdirAll(filepath.Join(vaultA, "2026"), 0o755)
	_ = os.MkdirAll(filepath.Join(vaultA, ".asterlab"), 0o755)

	initialExp := "---\ntitle: qPCR\n---\n# Initial\n"
	initialPantry := strings.Join([]string{
		`{"type":"schema","version":1}`,
		`{"type":"reagent","id":"r-1","name":"SYBR Mix","revision":1,"updatedAt":"2026-11-25T10:00:00Z"}`,
		`{"type":"tx","tx_id":"tx-base","id":"r-1","delta":100,"timestamp":"2026-11-25T10:00:00Z"}`,
	}, "\n") + "\n"

	_ = os.WriteFile(filepath.Join(vaultA, filepath.FromSlash(expRel)), []byte(initialExp), 0o644)
	_ = os.WriteFile(filepath.Join(vaultA, filepath.FromSlash(PantryLedgerRelPath)), []byte(initialPantry), 0o644)

	cA, _ := NewWebDAVClient(WebDAVConfig{BaseURL: ts.URL})
	cB, _ := NewWebDAVClient(WebDAVConfig{BaseURL: ts.URL})
	engA, _ := NewEngine(vaultA, cA, DefaultDesktopDownloadPolicy(), "win-lab")
	engB, _ := NewEngine(vaultB, cB, DefaultMobileDownloadPolicy(), "iphone-bench")
	fixedNow := time.Date(2026, 11, 26, 9, 30, 0, 0, time.UTC)
	engB.NowFn = func() time.Time { return fixedNow }

	if _, err := engA.Sync(context.Background()); err != nil {
		t.Fatalf("initial A sync: %v", err)
	}
	if _, err := engB.Sync(context.Background()); err != nil {
		t.Fatalf("initial B sync: %v", err)
	}

	// Concurrent edits on both devices:
	// Device A edits qpcr.asterml and appends tx-a-1 + tx-shared to pantry.jsonl, then pushes.
	expA := "---\ntitle: qPCR\n---\n# Edited on Lab PC\n"
	pantryA := initialPantry +
		`{"type":"tx","tx_id":"tx-shared","id":"r-1","delta":-5,"timestamp":"2026-11-26T09:10:00Z"}` + "\n" +
		`{"type":"tx","tx_id":"tx-a-1","id":"r-1","delta":-10,"timestamp":"2026-11-26T09:15:00Z"}` + "\n"
	_ = os.WriteFile(filepath.Join(vaultA, filepath.FromSlash(expRel)), []byte(expA), 0o644)
	_ = os.WriteFile(filepath.Join(vaultA, filepath.FromSlash(PantryLedgerRelPath)), []byte(pantryA), 0o644)
	if _, err := engA.Sync(context.Background()); err != nil {
		t.Fatalf("A concurrent push: %v", err)
	}

	// Device B concurrently edits qpcr.asterml AND appends tx-b-1 + duplicate tx-shared to pantry.jsonl.
	expB := "---\ntitle: qPCR\n---\n# Edited on Bench iPhone\n"
	pantryB := initialPantry +
		`{"type":"tx","tx_id":"tx-shared","id":"r-1","delta":-5,"timestamp":"2026-11-26T09:10:00Z"}` + "\n" +
		`{"type":"tx","tx_id":"tx-b-1","id":"r-1","delta":-20,"timestamp":"2026-11-26T09:20:00Z"}` + "\n"
	_ = os.WriteFile(filepath.Join(vaultB, filepath.FromSlash(expRel)), []byte(expB), 0o644)
	_ = os.WriteFile(filepath.Join(vaultB, filepath.FromSlash(PantryLedgerRelPath)), []byte(pantryB), 0o644)

	repB, err := engB.Sync(context.Background())
	if err != nil {
		t.Fatalf("B concurrent sync failed: %v", err)
	}

	// 1. Verify .asterml local edit preserved AND conflict copy created + ConflictEntry emitted.
	gotLocalB, _ := os.ReadFile(filepath.Join(vaultB, filepath.FromSlash(expRel)))
	if string(gotLocalB) != expB {
		t.Fatalf("local .asterml was overwritten! got %q want %q", string(gotLocalB), expB)
	}
	if len(repB.Conflicts) != 1 {
		t.Fatalf("expected 1 ConflictEntry, got %+v", repB.Conflicts)
	}
	ce := repB.Conflicts[0]
	wantConflictPath := "2026/qpcr.sync-conflict-win-lab-20261126-093000.asterml"
	if ce.RelPath != expRel || ce.ConflictCopyPath != wantConflictPath {
		t.Fatalf("unexpected ConflictEntry: %+v", ce)
	}
	gotConflictBytes, err := os.ReadFile(filepath.Join(vaultB, filepath.FromSlash(ce.ConflictCopyPath)))
	if err != nil || string(gotConflictBytes) != expA {
		t.Fatalf("conflict copy missing or mismatched: err=%v got=%q", err, string(gotConflictBytes))
	}

	// Re-syncing Device B must not create a duplicate conflict copy for the same remote SHA.
	repB2, err := engB.Sync(context.Background())
	if err != nil {
		t.Fatalf("repeat B sync: %v", err)
	}
	if len(repB2.Conflicts) != 0 {
		t.Fatalf("expected 0 duplicate conflicts on repeat sync, got %+v", repB2.Conflicts)
	}

	// 2. Verify pantry.jsonl merged conflict-free with both tx-a-1 and tx-b-1 and deduped tx-shared!
	if !repB.MergedPantry {
		t.Fatalf("expected MergedPantry=true")
	}
	mergedRaw, err := os.ReadFile(filepath.Join(vaultB, filepath.FromSlash(PantryLedgerRelPath)))
	if err != nil {
		t.Fatalf("read merged pantry.jsonl: %v", err)
	}
	mergedStr := string(mergedRaw)
	for _, want := range []string{"tx-base", "tx-shared", "tx-a-1", "tx-b-1"} {
		if !strings.Contains(mergedStr, want) {
			t.Fatalf("merged pantry.jsonl missing %q:\n%s", want, mergedStr)
		}
	}
	if strings.Count(mergedStr, "tx-shared") != 1 {
		t.Fatalf("expected tx-shared deduplicated to 1 occurrence, got:\n%s", mergedStr)
	}

	// Commutativity check: Merge(A, B) == Merge(B, A).
	mAB, _, err := MergePantryJSONL([]byte(pantryA), []byte(pantryB))
	if err != nil {
		t.Fatal(err)
	}
	mBA, _, err := MergePantryJSONL([]byte(pantryB), []byte(pantryA))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mAB, mBA) {
		t.Fatalf("MergePantryJSONL is not commutative:\nAB:\n%s\nBA:\n%s", mAB, mBA)
	}

	// Corrupt JSONL must return error.
	if _, _, err := MergePantryJSONL([]byte("{not-json}\n"), []byte(pantryB)); err == nil {
		t.Fatalf("expected error on corrupt pantry.jsonl line")
	}
}

func TestCrossPlatformPathNormalizationAndCaseCollisions(t *testing.T) {
	// NFD -> NFC normalization (e + combining acute accent -> single NFC codepoint)
	nfd := "2026/e\u0301tude.asterml"
	nfc, err := NormalizeVaultPath(nfd)
	if err != nil {
		t.Fatalf("NormalizeVaultPath NFD failed: %v", err)
	}
	if nfc != "2026/\u00e9tude.asterml" {
		t.Fatalf("expected NFC path, got %q (% x)", nfc, nfc)
	}

	invalidCases := []string{
		"",
		"../escape.asterml",
		"folder/CON.asterml",
		"folder/bad<name>.asterml",
		"folder/trailing.",
		"folder/trailing ",
		`C:\abs\path.asterml`,
	}
	for _, bad := range invalidCases {
		if _, err := NormalizeVaultPath(bad); err == nil {
			t.Fatalf("expected error for invalid path %q", bad)
		}
	}

	cols := CheckCaseCollisions([]string{"Exp/Plate.asterml", "exp/plate.asterml", "other.asterml"})
	if len(cols) != 1 || cols[0].LowerPath != "exp/plate.asterml" || len(cols[0].Paths) != 2 {
		t.Fatalf("expected 1 case collision, got %+v", cols)
	}
}

func buildLargeNoisyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	// Deterministic pseudo-random pixel fill so PNG zlib cannot compress it below 2MB.
	var state uint32 = 0x9e3779b9
	pix := img.Pix
	for i := 0; i < len(pix); i += 4 {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		pix[i] = byte(state)
		pix[i+1] = byte(state >> 8)
		pix[i+2] = byte(state >> 16)
		pix[i+3] = 255
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.NoCompression}
	if err := enc.Encode(&buf, img); err != nil {
		t.Fatalf("encode test PNG: %v", err)
	}
	sum := sha256.Sum256(buf.Bytes())
	_ = hex.EncodeToString(sum[:])
	return buf.Bytes()
}

func buildSolidPNG(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode solid PNG: %v", err)
	}
	return buf.Bytes()
}
