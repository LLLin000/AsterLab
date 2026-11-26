package syncengine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// OneDriveConfig configures the Microsoft Graph v1.0 RemoteStore client.
type OneDriveConfig struct {
	BaseURL        string // Defaults to https://graph.microsoft.com/v1.0
	RemoteRoot     string // Optional subpath inside drive root/approot (e.g. "AsterLab")
	UseAppFolder   bool   // True uses special/approot (recommended least-privilege); false uses root:
	OAuth          *OAuthClient
	HTTPClient     *http.Client
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	SleepFn        func(ctx context.Context, d time.Duration) error
}

// OneDriveStore implements RemoteStore over Microsoft Graph v1.0 OneDrive API.
type OneDriveStore struct {
	baseURL        string
	remoteRoot     string
	useAppFolder   bool
	oauth          *OAuthClient
	httpClient     *http.Client
	maxRetries     int
	initialBackoff time.Duration
	maxBackoff     time.Duration
	sleepFn        func(ctx context.Context, d time.Duration) error
	requests       atomic.Int64
}

// NewOneDriveStore constructs an OneDriveStore backed by Microsoft Graph API.
func NewOneDriveStore(cfg OneDriveConfig) (*OneDriveStore, error) {
	if cfg.OAuth == nil {
		return nil, errors.New("onedrive: OAuthClient is required")
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = "https://graph.microsoft.com/v1.0"
	}
	root := strings.Trim(strings.ReplaceAll(strings.TrimSpace(cfg.RemoteRoot), "\\", "/"), "/")
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
	return &OneDriveStore{
		baseURL:        base,
		remoteRoot:     root,
		useAppFolder:   cfg.UseAppFolder,
		oauth:          cfg.OAuth,
		httpClient:     hc,
		maxRetries:     maxRetries,
		initialBackoff: initBackoff,
		maxBackoff:     maxBackoff,
		sleepFn:        sleepFn,
	}, nil
}

func (s *OneDriveStore) RequestCount() int64 { return s.requests.Load() }

func (s *OneDriveStore) drivePrefix() string {
	if s.useAppFolder {
		return "/me/drive/special/approot"
	}
	return "/me/drive/root"
}

func (s *OneDriveStore) resolveItemPath(relPath string) string {
	trimmed := strings.Trim(strings.ReplaceAll(relPath, "\\", "/"), "/")
	if s.remoteRoot != "" {
		if trimmed != "" {
			trimmed = s.remoteRoot + "/" + trimmed
		} else {
			trimmed = s.remoteRoot
		}
	}
	if trimmed == "" {
		return s.baseURL + s.drivePrefix()
	}
	parts := strings.Split(trimmed, "/")
	escaped := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			escaped = append(escaped, url.PathEscape(p))
		}
	}
	return s.baseURL + s.drivePrefix() + ":/" + strings.Join(escaped, "/") + ":"
}

func (s *OneDriveStore) TestConnection(ctx context.Context) error {
	target := s.baseURL + s.drivePrefix()
	resp, err := s.doWithRetry(ctx, http.MethodGet, target, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrAuthExpired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("onedrive test HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}

func (s *OneDriveStore) Get(ctx context.Context, relPath string) ([]byte, error) {
	data, _, err := s.GetWithETag(ctx, relPath)
	return data, err
}

func (s *OneDriveStore) GetWithETag(ctx context.Context, relPath string) ([]byte, string, error) {
	target := s.resolveItemPath(relPath) + "/content"
	resp, err := s.doWithRetry(ctx, http.MethodGet, target, nil, nil)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", ErrNotFound
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, "", ErrAuthExpired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, "", fmt.Errorf("onedrive GET %s HTTP %d: %s", relPath, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return data, strings.TrimSpace(resp.Header.Get("ETag")), nil
}

func (s *OneDriveStore) GetVerified(ctx context.Context, relPath, wantSHA256 string) ([]byte, error) {
	data, err := s.Get(ctx, relPath)
	if err != nil {
		return nil, err
	}
	if wantSHA256 != "" {
		got := SHA256Bytes(data)
		if !strings.EqualFold(got, wantSHA256) {
			return nil, fmt.Errorf("onedrive GET %s sha256 mismatch: got %s want %s", relPath, got, wantSHA256)
		}
	}
	return data, nil
}

func (s *OneDriveStore) Put(ctx context.Context, relPath string, data []byte, contentType string) error {
	_, err := s.PutIfMatch(ctx, relPath, data, contentType, "")
	return err
}

func (s *OneDriveStore) PutIfMatch(ctx context.Context, relPath string, data []byte, contentType, ifMatchETag string) (string, error) {
	target := s.resolveItemPath(relPath) + "/content"
	headers := map[string]string{}
	if contentType != "" {
		headers["Content-Type"] = contentType
	} else {
		headers["Content-Type"] = "application/octet-stream"
	}
	if strings.TrimSpace(ifMatchETag) != "" {
		headers["If-Match"] = strings.TrimSpace(ifMatchETag)
	}
	resp, err := s.doWithRetry(ctx, http.MethodPut, target, data, headers)
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
		return "", fmt.Errorf("onedrive PUT %s HTTP %d: %s", relPath, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return strings.TrimSpace(resp.Header.Get("ETag")), nil
}

func (s *OneDriveStore) Delete(ctx context.Context, relPath string) error {
	target := s.resolveItemPath(relPath)
	resp, err := s.doWithRetry(ctx, http.MethodDelete, target, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrAuthExpired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("onedrive DELETE %s HTTP %d", relPath, resp.StatusCode)
	}
	return nil
}

func (s *OneDriveStore) doWithRetry(ctx context.Context, method, targetURL string, body []byte, headers map[string]string) (*http.Response, error) {
	refreshedOnce := false
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		tok, err := s.oauth.AccessToken(ctx)
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
		s.requests.Add(1)
		resp, err := s.httpClient.Do(req)
		if err != nil {
			if attempt == s.maxRetries {
				return nil, err
			}
			wait := s.expBackoff(attempt)
			if sErr := s.sleepFn(ctx, wait); sErr != nil {
				return nil, sErr
			}
			continue
		}
		if resp.StatusCode == http.StatusUnauthorized && !refreshedOnce {
			_ = resp.Body.Close()
			refreshedOnce = true
			if _, rErr := s.oauth.Refresh(ctx); rErr != nil {
				return nil, rErr
			}
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
			_ = resp.Body.Close()
			if attempt == s.maxRetries {
				return nil, fmt.Errorf("onedrive rate-limited HTTP %d", resp.StatusCode)
			}
			wait := s.expBackoff(attempt)
			if retryAfter > wait {
				wait = retryAfter
			}
			if wait > s.maxBackoff {
				wait = s.maxBackoff
			}
			if sErr := s.sleepFn(ctx, wait); sErr != nil {
				return nil, sErr
			}
			continue
		}
		return resp, nil
	}
	return nil, errors.New("onedrive exhausted retries")
}

func (s *OneDriveStore) expBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	d := s.initialBackoff << attempt
	if d <= 0 || d > s.maxBackoff {
		d = s.maxBackoff
	}
	return d
}
