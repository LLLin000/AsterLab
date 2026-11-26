package syncengine

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrNotFound is returned when a remote WebDAV resource does not exist (HTTP 404).
	ErrNotFound = errors.New("webdav: resource not found")
	// ErrPreconditionFailed is returned when MOVE Overwrite:F or conditional check gets 412.
	ErrPreconditionFailed = errors.New("webdav: precondition failed")
)

const (
	defaultWebDAVConcurrency    = 2
	nutstoreDefaultRatePerMin   = 20.0
	nutstoreDefaultBurst        = 10
	defaultMaxRetries           = 5
	defaultInitialBackoff       = 200 * time.Millisecond
	defaultMaxBackoff           = 30 * time.Second
)

// WebDAVConfig configures the Pure-Go WebDAV HTTP client.
type WebDAVConfig struct {
	BaseURL            string
	Username           string
	Password           string
	Concurrency        int
	ForceNutstoreMode  bool
	RateLimitPerMinute float64
	RateBurst          int
	MaxRetries         int
	InitialBackoff     time.Duration
	MaxBackoff         time.Duration
	HTTPClient         *http.Client
	NowFn              func() time.Time
	SleepFn            func(ctx context.Context, d time.Duration) error
}

// ClientStats exposes observable request counts and backoff telemetry.
type ClientStats struct {
	TotalHTTPRequests int64
	RateLimitHits     int64
	TotalBackoffTime  time.Duration
	LastRetryAfter    time.Duration
}

// DAVResource represents one entry returned by PROPFIND.
type DAVResource struct {
	Href          string
	IsCollection  bool
	ContentLength int64
}

// WebDAVClient is a Pure-Go, rate-limit-safe WebDAV client supporting PROPFIND,
// MKCOL, GET, PUT, DELETE, and MOVE with Nutstore auto-throttling and 429/503 backoff.
type WebDAVClient struct {
	baseURL            *url.URL
	username           string
	password           string
	httpClient         *http.Client
	concurrency        int
	isNutstore         bool
	maxRetries         int
	initialBackoff     time.Duration
	maxBackoff         time.Duration
	nowFn              func() time.Time
	sleepFn            func(ctx context.Context, d time.Duration) error
	sem                chan struct{}
	limiter            *tokenBucket
	ensuredMu          sync.Mutex
	ensuredDirs        map[string]bool
	totalRequests      atomic.Int64
	rateLimitHits      atomic.Int64
	totalBackoffNanos  atomic.Int64
	lastRetryAfterNano atomic.Int64
}

// NewWebDAVClient creates a WebDAVClient. When BaseURL host contains `jianguoyun.com`
// (or ForceNutstoreMode is set), concurrency is locked to 1 and a token-bucket rate
// limiter is automatically engaged.
func NewWebDAVClient(cfg WebDAVConfig) (*WebDAVClient, error) {
	rawURL := strings.TrimSpace(cfg.BaseURL)
	if rawURL == "" {
		return nil, errors.New("webdav: BaseURL is required")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("webdav: invalid BaseURL %q: %w", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("webdav: unsupported scheme %q", u.Scheme)
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}

	isNutstore := cfg.ForceNutstoreMode || strings.Contains(strings.ToLower(u.Hostname()), "jianguoyun.com")
	conc := cfg.Concurrency
	if conc <= 0 {
		conc = defaultWebDAVConcurrency
	}
	if isNutstore {
		conc = 1
	}

	ratePerMin := cfg.RateLimitPerMinute
	burst := cfg.RateBurst
	if isNutstore && ratePerMin <= 0 {
		ratePerMin = nutstoreDefaultRatePerMin
		if burst <= 0 {
			burst = nutstoreDefaultBurst
		}
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
	nowFn := cfg.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	sleepFn := cfg.SleepFn
	if sleepFn == nil {
		sleepFn = defaultContextSleep
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}

	var tb *tokenBucket
	if ratePerMin > 0 {
		if burst <= 0 {
			burst = 1
		}
		tb = newTokenBucket(ratePerMin/60.0, burst, nowFn)
	}

	return &WebDAVClient{
		baseURL:        u,
		username:       cfg.Username,
		password:       cfg.Password,
		httpClient:     hc,
		concurrency:    conc,
		isNutstore:     isNutstore,
		maxRetries:     maxRetries,
		initialBackoff: initBackoff,
		maxBackoff:     maxBackoff,
		nowFn:          nowFn,
		sleepFn:        sleepFn,
		sem:            make(chan struct{}, conc),
		limiter:        tb,
		ensuredDirs:    map[string]bool{},
	}, nil
}

// EffectiveConcurrency returns the active concurrency limit (1 for Nutstore).
func (c *WebDAVClient) EffectiveConcurrency() int { return c.concurrency }

// IsNutstoreMode reports whether Nutstore throttling safeguards are active.
func (c *WebDAVClient) IsNutstoreMode() bool { return c.isNutstore }

// RequestCount returns the total number of HTTP requests sent (including retries).
func (c *WebDAVClient) RequestCount() int64 { return c.totalRequests.Load() }

// Stats returns a snapshot of HTTP request and rate-limit metrics.
func (c *WebDAVClient) Stats() ClientStats {
	return ClientStats{
		TotalHTTPRequests: c.totalRequests.Load(),
		RateLimitHits:     c.rateLimitHits.Load(),
		TotalBackoffTime:  time.Duration(c.totalBackoffNanos.Load()),
		LastRetryAfter:    time.Duration(c.lastRetryAfterNano.Load()),
	}
}

// ResetDirCache clears the cached MKCOL directory existence map.
func (c *WebDAVClient) ResetDirCache() {
	c.ensuredMu.Lock()
	c.ensuredDirs = map[string]bool{}
	c.ensuredMu.Unlock()
}

// Get downloads a remote resource and verifies its HTTP status.
func (c *WebDAVClient) Get(ctx context.Context, relPath string) ([]byte, error) {
	resp, err := c.doWithRetry(ctx, http.MethodGet, relPath, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("webdav GET %s failed: HTTP %d (%s)", relPath, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return io.ReadAll(resp.Body)
}

// GetVerified downloads a remote resource and verifies its SHA-256 against wantSHA256
// when wantSHA256 is non-empty. Never trusts ETag or Last-Modified.
func (c *WebDAVClient) GetVerified(ctx context.Context, relPath, wantSHA256 string) ([]byte, error) {
	data, err := c.Get(ctx, relPath)
	if err != nil {
		return nil, err
	}
	if wantSHA256 != "" {
		got := SHA256Bytes(data)
		if !strings.EqualFold(got, wantSHA256) {
			return nil, fmt.Errorf("webdav GET %s sha256 mismatch: got %s want %s", relPath, got, wantSHA256)
		}
	}
	return data, nil
}

// Put ensures parent collections exist and uploads payload atomically to relPath.
func (c *WebDAVClient) Put(ctx context.Context, relPath string, data []byte, contentType string) error {
	if err := c.EnsureParentDirs(ctx, relPath); err != nil {
		return err
	}
	headers := map[string]string{}
	if contentType != "" {
		headers["Content-Type"] = contentType
	} else {
		headers["Content-Type"] = "application/octet-stream"
	}
	resp, err := c.doWithRetry(ctx, http.MethodPut, relPath, data, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("webdav PUT %s failed: HTTP %d (%s)", relPath, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}

// Delete removes a remote resource (ignoring 404 Not Found).
func (c *WebDAVClient) Delete(ctx context.Context, relPath string) error {
	resp, err := c.doWithRetry(ctx, http.MethodDelete, relPath, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webdav DELETE %s failed: HTTP %d", relPath, resp.StatusCode)
	}
	return nil
}

// Move renames/moves a remote resource from srcRel to dstRel.
func (c *WebDAVClient) Move(ctx context.Context, srcRel, dstRel string, overwrite bool) error {
	if err := c.EnsureParentDirs(ctx, dstRel); err != nil {
		return err
	}
	dstURL := c.resolveURL(dstRel)
	ov := "F"
	if overwrite {
		ov = "T"
	}
	headers := map[string]string{
		"Destination": dstURL,
		"Overwrite":   ov,
	}
	resp, err := c.doWithRetry(ctx, "MOVE", srcRel, nil, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode == http.StatusPreconditionFailed {
		return ErrPreconditionFailed
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webdav MOVE %s -> %s failed: HTTP %d", srcRel, dstRel, resp.StatusCode)
	}
	return nil
}

// Mkcol creates a single collection directory (treating 405 Method Not Allowed as already-exists).
func (c *WebDAVClient) Mkcol(ctx context.Context, dirRel string) error {
	dirRel = strings.Trim(strings.ReplaceAll(dirRel, "\\", "/"), "/")
	if dirRel == "" {
		return nil
	}
	resp, err := c.doWithRetry(ctx, "MKCOL", dirRel+"/", nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// 201 Created, 200 OK, 405 Method Not Allowed (already exists), 301 Moved Permanently (already exists)
	if resp.StatusCode == http.StatusCreated ||
		resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusNoContent ||
		resp.StatusCode == http.StatusMethodNotAllowed ||
		resp.StatusCode == http.StatusMovedPermanently {
		return nil
	}
	return fmt.Errorf("webdav MKCOL %s failed: HTTP %d", dirRel, resp.StatusCode)
}

// EnsureParentDirs creates all missing parent directories for fileRel top-down,
// caching ensured directories so repeated uploads in the same folder do not
// re-issue MKCOL requests.
func (c *WebDAVClient) EnsureParentDirs(ctx context.Context, fileRel string) error {
	dir := path.Dir(strings.Trim(strings.ReplaceAll(fileRel, "\\", "/"), "/"))
	if dir == "." || dir == "" || dir == "/" {
		return nil
	}
	parts := strings.Split(dir, "/")
	cur := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if cur == "" {
			cur = p
		} else {
			cur = cur + "/" + p
		}
		c.ensuredMu.Lock()
		already := c.ensuredDirs[cur]
		c.ensuredMu.Unlock()
		if already {
			continue
		}
		if err := c.Mkcol(ctx, cur); err != nil {
			return err
		}
		c.ensuredMu.Lock()
		c.ensuredDirs[cur] = true
		c.ensuredMu.Unlock()
	}
	return nil
}

// Propfind issues a bounded-depth PROPFIND (Depth "0" or "1") and parses the DAV multistatus response.
func (c *WebDAVClient) Propfind(ctx context.Context, relPath, depth string) ([]DAVResource, error) {
	if depth != "0" && depth != "1" {
		depth = "1"
	}
	const propfindBody = `<?xml version="1.0" encoding="utf-8"?><D:propfind xmlns:D="DAV:"><D:prop><D:resourcetype/><D:getcontentlength/></D:prop></D:propfind>`
	headers := map[string]string{
		"Depth":        depth,
		"Content-Type": "application/xml; charset=utf-8",
	}
	resp, err := c.doWithRetry(ctx, "PROPFIND", relPath, []byte(propfindBody), headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusMultiStatus && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return nil, fmt.Errorf("webdav PROPFIND %s failed: HTTP %d", relPath, resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parsePropfindXML(raw)
}

func (c *WebDAVClient) doWithRetry(ctx context.Context, method, relPath string, body []byte, headers map[string]string) (*http.Response, error) {
	targetURL := c.resolveURL(relPath)

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if c.limiter != nil {
			if err := c.limiter.wait(ctx, c.sleepFn); err != nil {
				return nil, err
			}
		}

		// Acquire concurrency semaphore for this single HTTP attempt.
		select {
		case c.sem <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, targetURL, bodyReader)
		if err != nil {
			<-c.sem
			return nil, err
		}
		if c.username != "" || c.password != "" {
			req.SetBasicAuth(c.username, c.password)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if body != nil {
			req.ContentLength = int64(len(body))
		}

		c.totalRequests.Add(1)
		resp, err := c.httpClient.Do(req)
		<-c.sem
		if err != nil {
			if attempt == c.maxRetries {
				return nil, err
			}
			wait := c.expBackoff(attempt)
			c.recordBackoff(wait, 0)
			if sErr := c.sleepFn(ctx, wait); sErr != nil {
				return nil, sErr
			}
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			c.rateLimitHits.Add(1)
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), c.nowFn())
			_ = resp.Body.Close()
			if attempt == c.maxRetries {
				return nil, fmt.Errorf("webdav %s %s rate-limited after %d retries (HTTP %d)", method, relPath, c.maxRetries, resp.StatusCode)
			}
			wait := c.expBackoff(attempt)
			if retryAfter > wait {
				wait = retryAfter
			}
			if wait > c.maxBackoff {
				wait = c.maxBackoff
			}
			c.recordBackoff(wait, retryAfter)
			if sErr := c.sleepFn(ctx, wait); sErr != nil {
				return nil, sErr
			}
			continue
		}

		return resp, nil
	}
	return nil, fmt.Errorf("webdav %s %s exhausted retries", method, relPath)
}

func (c *WebDAVClient) expBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	d := c.initialBackoff << attempt
	if d <= 0 || d > c.maxBackoff {
		d = c.maxBackoff
	}
	return d
}

func (c *WebDAVClient) recordBackoff(wait, retryAfter time.Duration) {
	c.totalBackoffNanos.Add(int64(wait))
	if retryAfter > 0 {
		c.lastRetryAfterNano.Store(int64(retryAfter))
	}
}

func (c *WebDAVClient) resolveURL(relPath string) string {
	trimmed := strings.TrimPrefix(strings.ReplaceAll(relPath, "\\", "/"), "/")
	if trimmed == "" {
		return c.baseURL.String()
	}
	trailingSlash := strings.HasSuffix(trimmed, "/")
	parts := strings.Split(strings.TrimSuffix(trimmed, "/"), "/")
	escaped := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			escaped = append(escaped, url.PathEscape(p))
		}
	}
	joined := strings.Join(escaped, "/")
	if trailingSlash && joined != "" {
		joined += "/"
	}
	u := *c.baseURL
	u.Path = strings.TrimRight(c.baseURL.Path, "/") + "/" + joined
	u.RawPath = ""
	return u.String()
}

func parseRetryAfter(raw string, now time.Time) time.Duration {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0
	}
	if secs, err := strconv.Atoi(s); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(s); err == nil {
		d := t.Sub(now)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}

func defaultContextSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type tokenBucket struct {
	mu         sync.Mutex
	ratePerSec float64
	burst      float64
	tokens     float64
	last       time.Time
	nowFn      func() time.Time
}

func newTokenBucket(ratePerSec float64, burst int, nowFn func() time.Time) *tokenBucket {
	return &tokenBucket{
		ratePerSec: ratePerSec,
		burst:      float64(burst),
		tokens:     float64(burst),
		last:       nowFn(),
		nowFn:      nowFn,
	}
}

func (tb *tokenBucket) wait(ctx context.Context, sleepFn func(context.Context, time.Duration) error) error {
	for {
		tb.mu.Lock()
		now := tb.nowFn()
		elapsed := now.Sub(tb.last).Seconds()
		if elapsed > 0 {
			tb.tokens += elapsed * tb.ratePerSec
			if tb.tokens > tb.burst {
				tb.tokens = tb.burst
			}
			tb.last = now
		}
		if tb.tokens >= 1.0 {
			tb.tokens -= 1.0
			tb.mu.Unlock()
			return nil
		}
		need := (1.0 - tb.tokens) / tb.ratePerSec
		waitDur := time.Duration(need * float64(time.Second))
		if waitDur < time.Millisecond {
			waitDur = time.Millisecond
		}
		tb.mu.Unlock()

		if err := sleepFn(ctx, waitDur); err != nil {
			return err
		}
	}
}

type xmlMultistatus struct {
	Responses []xmlResponse `xml:"response"`
}

type xmlResponse struct {
	Href     string        `xml:"href"`
	Propstat []xmlPropstat `xml:"propstat"`
}

type xmlPropstat struct {
	Prop   xmlProp `xml:"prop"`
	Status string  `xml:"status"`
}

type xmlProp struct {
	ResourceType struct {
		Collection *struct{} `xml:"collection"`
	} `xml:"resourcetype"`
	ContentLength int64 `xml:"getcontentlength"`
}

func parsePropfindXML(raw []byte) ([]DAVResource, error) {
	var ms xmlMultistatus
	if err := xml.Unmarshal(raw, &ms); err != nil {
		return nil, fmt.Errorf("webdav: parse PROPFIND XML: %w", err)
	}
	out := make([]DAVResource, 0, len(ms.Responses))
	for _, r := range ms.Responses {
		res := DAVResource{Href: r.Href}
		for _, ps := range r.Propstat {
			if strings.Contains(ps.Status, "200") || ps.Status == "" {
				if ps.Prop.ResourceType.Collection != nil {
					res.IsCollection = true
				}
				if ps.Prop.ContentLength > 0 {
					res.ContentLength = ps.Prop.ContentLength
				}
			}
		}
		out = append(out, res)
	}
	return out, nil
}
