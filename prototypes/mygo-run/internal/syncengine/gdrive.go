package syncengine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// GoogleDriveConfig configures the Google Drive v3 RemoteStore client.
type GoogleDriveConfig struct {
	BaseURL          string // Defaults to https://www.googleapis.com
	RemoteRoot       string // Optional top folder inside appDataFolder/MyDrive
	UseAppDataFolder bool   // True isolates inside appDataFolder (drive.appdata scope)
	OAuth            *OAuthClient
	HTTPClient       *http.Client
	MaxRetries       int
	InitialBackoff   time.Duration
	MaxBackoff       time.Duration
	SleepFn          func(ctx context.Context, d time.Duration) error
}

// GoogleDriveStore implements RemoteStore over Google Drive API v3.
type GoogleDriveStore struct {
	baseURL        string
	remoteRoot     string
	space          string
	rootParentID   string
	oauth          *OAuthClient
	httpClient     *http.Client
	maxRetries     int
	initialBackoff time.Duration
	maxBackoff     time.Duration
	sleepFn        func(ctx context.Context, d time.Duration) error

	cacheMu   sync.RWMutex
	folderIDs map[string]string
	fileMeta  map[string]gdriveEntry
	requests  atomic.Int64
}

type gdriveEntry struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	MimeType      string `json:"mimeType,omitempty"`
	HeadRevision  string `json:"headRevisionId,omitempty"`
	Version       string `json:"version,omitempty"`
}

// NewGoogleDriveStore constructs a GoogleDriveStore backed by Google Drive API v3.
func NewGoogleDriveStore(cfg GoogleDriveConfig) (*GoogleDriveStore, error) {
	if cfg.OAuth == nil {
		return nil, errors.New("gdrive: OAuthClient is required")
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = "https://www.googleapis.com"
	}
	root := strings.Trim(strings.ReplaceAll(strings.TrimSpace(cfg.RemoteRoot), "\\", "/"), "/")
	space := "drive"
	parent := "root"
	if cfg.UseAppDataFolder {
		space = "appDataFolder"
		parent = "appDataFolder"
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}
	initBackoff := cfg.InitialBackoff
	if initBackoff <= 0 {
		initBackoff = defaultInitialBackoff
	}
	maxBackoff := cfg.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = defaultMaxBackoff
	}
	sleepFn := cfg.SleepFn
	if sleepFn == nil {
		sleepFn = defaultContextSleep
	}
	return &GoogleDriveStore{
		baseURL:        base,
		remoteRoot:     root,
		space:          space,
		rootParentID:   parent,
		oauth:          cfg.OAuth,
		httpClient:     hc,
		maxRetries:     maxRetries,
		initialBackoff: initBackoff,
		maxBackoff:     maxBackoff,
		sleepFn:        sleepFn,
		folderIDs:      map[string]string{"": parent},
		fileMeta:       map[string]gdriveEntry{},
	}, nil
}

func (g *GoogleDriveStore) RequestCount() int64 { return g.requests.Load() }

func (g *GoogleDriveStore) fullRel(relPath string) string {
	trimmed := strings.Trim(strings.ReplaceAll(relPath, "\\", "/"), "/")
	if g.remoteRoot == "" {
		return trimmed
	}
	if trimmed == "" {
		return g.remoteRoot
	}
	return g.remoteRoot + "/" + trimmed
}

func (g *GoogleDriveStore) TestConnection(ctx context.Context) error {
	u := fmt.Sprintf("%s/drive/v3/about?fields=user", g.baseURL)
	resp, err := g.doWithRetry(ctx, http.MethodGet, u, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrAuthExpired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("gdrive test HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}

func (g *GoogleDriveStore) Get(ctx context.Context, relPath string) ([]byte, error) {
	data, _, err := g.GetWithETag(ctx, relPath)
	return data, err
}

func (g *GoogleDriveStore) GetWithETag(ctx context.Context, relPath string) ([]byte, string, error) {
	entry, found, err := g.lookupFile(ctx, g.fullRel(relPath), false)
	if err != nil {
		return nil, "", err
	}
	if !found {
		return nil, "", ErrNotFound
	}
	dlURL := fmt.Sprintf("%s/drive/v3/files/%s?alt=media", g.baseURL, url.PathEscape(entry.ID))
	resp, err := g.doWithRetry(ctx, http.MethodGet, dlURL, nil, nil)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		g.evictFile(g.fullRel(relPath))
		return nil, "", ErrNotFound
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, "", ErrAuthExpired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, "", fmt.Errorf("gdrive GET %s HTTP %d: %s", relPath, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	etag := firstNonEmptySync(entry.Version, entry.HeadRevision, strings.TrimSpace(resp.Header.Get("ETag")))
	return data, etag, nil
}

func (g *GoogleDriveStore) GetVerified(ctx context.Context, relPath, wantSHA256 string) ([]byte, error) {
	data, err := g.Get(ctx, relPath)
	if err != nil {
		return nil, err
	}
	if wantSHA256 != "" {
		got := SHA256Bytes(data)
		if !strings.EqualFold(got, wantSHA256) {
			return nil, fmt.Errorf("gdrive GET %s sha256 mismatch: got %s want %s", relPath, got, wantSHA256)
		}
	}
	return data, nil
}

func (g *GoogleDriveStore) Put(ctx context.Context, relPath string, data []byte, contentType string) error {
	_, err := g.PutIfMatch(ctx, relPath, data, contentType, "")
	return err
}

func (g *GoogleDriveStore) PutIfMatch(ctx context.Context, relPath string, data []byte, contentType, ifMatchETag string) (string, error) {
	full := g.fullRel(relPath)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	entry, found, err := g.lookupFile(ctx, full, strings.TrimSpace(ifMatchETag) != "")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(ifMatchETag) != "" {
		if !found {
			return "", ErrPreconditionFailed
		}
		curETag := firstNonEmptySync(entry.Version, entry.HeadRevision)
		if curETag != "" && curETag != strings.TrimSpace(ifMatchETag) {
			return "", ErrPreconditionFailed
		}
	}

	if found {
		patchURL := fmt.Sprintf("%s/upload/drive/v3/files/%s?uploadType=media&fields=id,name,version,headRevisionId", g.baseURL, url.PathEscape(entry.ID))
		headers := map[string]string{"Content-Type": contentType}
		if strings.TrimSpace(ifMatchETag) != "" {
			headers["If-Match"] = strings.TrimSpace(ifMatchETag)
		}
		resp, err := g.doWithRetry(ctx, http.MethodPatch, patchURL, data, headers)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusPreconditionFailed {
			return "", ErrPreconditionFailed
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return "", ErrAuthExpired
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			return "", fmt.Errorf("gdrive PATCH %s HTTP %d: %s", relPath, resp.StatusCode, strings.TrimSpace(string(msg)))
		}
		var updated gdriveEntry
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		_ = json.Unmarshal(raw, &updated)
		if updated.ID == "" {
			updated = entry
		}
		g.storeFile(full, updated)
		return firstNonEmptySync(updated.Version, updated.HeadRevision), nil
	}

	slashIdx := strings.LastIndexByte(full, '/')
	parentPath, fileName := "", full
	if slashIdx >= 0 {
		parentPath = full[:slashIdx]
		fileName = full[slashIdx+1:]
	}
	parentID, err := g.ensureFolderPath(ctx, parentPath)
	if err != nil {
		return "", err
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	metaPart, _ := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type": {"application/json; charset=UTF-8"},
	})
	_ = json.NewEncoder(metaPart).Encode(map[string]any{
		"name":    fileName,
		"parents": []string{parentID},
	})
	dataPart, _ := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type": {contentType},
	})
	_, _ = dataPart.Write(data)
	_ = mw.Close()

	createURL := fmt.Sprintf("%s/upload/drive/v3/files?uploadType=multipart&fields=id,name,version,headRevisionId", g.baseURL)
	resp, err := g.doWithRetry(ctx, http.MethodPost, createURL, body.Bytes(), map[string]string{
		"Content-Type": "multipart/related; boundary=" + mw.Boundary(),
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return "", ErrAuthExpired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("gdrive CREATE %s HTTP %d: %s", relPath, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	var created gdriveEntry
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	_ = json.Unmarshal(raw, &created)
	if created.ID != "" {
		g.storeFile(full, created)
	}
	return firstNonEmptySync(created.Version, created.HeadRevision), nil
}

func (g *GoogleDriveStore) Delete(ctx context.Context, relPath string) error {
	full := g.fullRel(relPath)
	entry, found, err := g.lookupFile(ctx, full, true)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	delURL := fmt.Sprintf("%s/drive/v3/files/%s", g.baseURL, url.PathEscape(entry.ID))
	resp, err := g.doWithRetry(ctx, http.MethodDelete, delURL, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	g.evictFile(full)
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrAuthExpired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gdrive DELETE %s HTTP %d", relPath, resp.StatusCode)
	}
	return nil
}

func (g *GoogleDriveStore) storeFile(full string, e gdriveEntry) {
	g.cacheMu.Lock()
	g.fileMeta[full] = e
	g.cacheMu.Unlock()
}

func (g *GoogleDriveStore) evictFile(full string) {
	g.cacheMu.Lock()
	delete(g.fileMeta, full)
	g.cacheMu.Unlock()
}

func (g *GoogleDriveStore) ensureFolderPath(ctx context.Context, dirRel string) (string, error) {
	dirRel = strings.Trim(dirRel, "/")
	if dirRel == "" {
		return g.rootParentID, nil
	}
	g.cacheMu.RLock()
	if id, ok := g.folderIDs[dirRel]; ok {
		g.cacheMu.RUnlock()
		return id, nil
	}
	g.cacheMu.RUnlock()

	parts := strings.Split(dirRel, "/")
	curPath := ""
	parentID := g.rootParentID
	for _, seg := range parts {
		if seg == "" {
			continue
		}
		if curPath == "" {
			curPath = seg
		} else {
			curPath = curPath + "/" + seg
		}
		g.cacheMu.RLock()
		cachedID, ok := g.folderIDs[curPath]
		g.cacheMu.RUnlock()
		if ok {
			parentID = cachedID
			continue
		}
		entry, found, err := g.findChild(ctx, parentID, seg, true)
		if err != nil {
			return "", err
		}
		if !found {
			created, cErr := g.createFolder(ctx, parentID, seg)
			if cErr != nil {
				return "", cErr
			}
			entry = created
		}
		g.cacheMu.Lock()
		g.folderIDs[curPath] = entry.ID
		g.cacheMu.Unlock()
		parentID = entry.ID
	}
	return parentID, nil
}

func (g *GoogleDriveStore) createFolder(ctx context.Context, parentID, name string) (gdriveEntry, error) {
	payload, _ := json.Marshal(map[string]any{
		"name":     name,
		"mimeType": "application/vnd.google-apps.folder",
		"parents":  []string{parentID},
	})
	u := fmt.Sprintf("%s/drive/v3/files?fields=id,name,mimeType", g.baseURL)
	resp, err := g.doWithRetry(ctx, http.MethodPost, u, payload, map[string]string{
		"Content-Type": "application/json; charset=utf-8",
	})
	if err != nil {
		return gdriveEntry{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return gdriveEntry{}, ErrAuthExpired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return gdriveEntry{}, fmt.Errorf("gdrive MKDIR %s HTTP %d: %s", name, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	var e gdriveEntry
	_ = json.NewDecoder(resp.Body).Decode(&e)
	return e, nil
}

func (g *GoogleDriveStore) lookupFile(ctx context.Context, full string, bypassCache bool) (gdriveEntry, bool, error) {
	if !bypassCache {
		g.cacheMu.RLock()
		if e, ok := g.fileMeta[full]; ok && e.ID != "" {
			g.cacheMu.RUnlock()
			return e, true, nil
		}
		g.cacheMu.RUnlock()
	}
	slashIdx := strings.LastIndexByte(full, '/')
	parentPath, fileName := "", full
	if slashIdx >= 0 {
		parentPath = full[:slashIdx]
		fileName = full[slashIdx+1:]
	}
	parentID := g.rootParentID
	if parentPath != "" {
		// Walk parent folders without creating missing ones on read.
		parts := strings.Split(parentPath, "/")
		curPath := ""
		for _, seg := range parts {
			if seg == "" {
				continue
			}
			if curPath == "" {
				curPath = seg
			} else {
				curPath = curPath + "/" + seg
			}
			g.cacheMu.RLock()
			cid, ok := g.folderIDs[curPath]
			g.cacheMu.RUnlock()
			if ok {
				parentID = cid
				continue
			}
			fe, found, err := g.findChild(ctx, parentID, seg, true)
			if err != nil {
				return gdriveEntry{}, false, err
			}
			if !found {
				return gdriveEntry{}, false, nil
			}
			g.cacheMu.Lock()
			g.folderIDs[curPath] = fe.ID
			g.cacheMu.Unlock()
			parentID = fe.ID
		}
	}
	entry, found, err := g.findChild(ctx, parentID, fileName, false)
	if err != nil || !found {
		return gdriveEntry{}, false, err
	}
	g.storeFile(full, entry)
	return entry, true, nil
}

func (g *GoogleDriveStore) findChild(ctx context.Context, parentID, name string, wantFolder bool) (gdriveEntry, bool, error) {
	escapedName := strings.ReplaceAll(name, `'`, `\'`)
	op := "!="
	if wantFolder {
		op = "="
	}
	q := fmt.Sprintf("name = '%s' and '%s' in parents and mimeType %s 'application/vnd.google-apps.folder' and trashed = false", escapedName, parentID, op)
	u := fmt.Sprintf("%s/drive/v3/files?spaces=%s&q=%s&fields=files(id,name,mimeType,version,headRevisionId)&pageSize=2",
		g.baseURL, url.QueryEscape(g.space), url.QueryEscape(q))
	resp, err := g.doWithRetry(ctx, http.MethodGet, u, nil, nil)
	if err != nil {
		return gdriveEntry{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return gdriveEntry{}, false, ErrAuthExpired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return gdriveEntry{}, false, fmt.Errorf("gdrive list %s HTTP %d: %s", name, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	var listResp struct {
		Files []gdriveEntry `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return gdriveEntry{}, false, err
	}
	if len(listResp.Files) == 0 {
		return gdriveEntry{}, false, nil
	}
	return listResp.Files[0], true, nil
}

func (g *GoogleDriveStore) doWithRetry(ctx context.Context, method, targetURL string, body []byte, headers map[string]string) (*http.Response, error) {
	refreshedOnce := false
	for attempt := 0; attempt <= g.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		tok, err := g.oauth.AccessToken(ctx)
		if err != nil {
			return nil, err
		}
		var br io.Reader
		if body != nil {
			br = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, targetURL, br)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if body != nil {
			req.ContentLength = int64(len(body))
		}
		g.requests.Add(1)
		resp, err := g.httpClient.Do(req)
		if err != nil {
			if attempt == g.maxRetries {
				return nil, err
			}
			wait := g.expBackoff(attempt)
			if sErr := g.sleepFn(ctx, wait); sErr != nil {
				return nil, sErr
			}
			continue
		}
		if resp.StatusCode == http.StatusUnauthorized && !refreshedOnce {
			_ = resp.Body.Close()
			refreshedOnce = true
			if _, rErr := g.oauth.Refresh(ctx); rErr != nil {
				return nil, rErr
			}
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
			_ = resp.Body.Close()
			if attempt == g.maxRetries {
				return nil, fmt.Errorf("gdrive rate-limited HTTP %d", resp.StatusCode)
			}
			wait := g.expBackoff(attempt)
			if retryAfter > wait {
				wait = retryAfter
			}
			if wait > g.maxBackoff {
				wait = g.maxBackoff
			}
			if sErr := g.sleepFn(ctx, wait); sErr != nil {
				return nil, sErr
			}
			continue
		}
		return resp, nil
	}
	return nil, errors.New("gdrive exhausted retries")
}

func (g *GoogleDriveStore) expBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	d := g.initialBackoff << attempt
	if d <= 0 || d > g.maxBackoff {
		d = g.maxBackoff
	}
	return d
}
