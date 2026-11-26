package syncengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// OAuthTokens holds an OAuth2 access/refresh token pair persisted securely via mygo.Secrets.
type OAuthTokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	TokenType    string `json:"tokenType,omitempty"`
	ExpiryUTC    string `json:"expiryUtc,omitempty"`
	AccountEmail string `json:"accountEmail,omitempty"`
}

// DeviceCodeChallenge describes an active RFC 8628 OAuth2 Device Authorization prompt.
type DeviceCodeChallenge struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	Message         string `json:"message,omitempty"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// OAuthEndpointConfig parameterizes Microsoft Identity / Google OAuth2 endpoints.
type OAuthEndpointConfig struct {
	DeviceCodeURL string
	TokenURL      string
	ClientID      string
	ClientSecret  string
	Scopes        []string
}

// DefaultOneDriveOAuthConfig returns Microsoft Identity v2.0 endpoints for OneDrive AppFolder sync.
func DefaultOneDriveOAuthConfig(clientID string) OAuthEndpointConfig {
	return OAuthEndpointConfig{
		DeviceCodeURL: "https://login.microsoftonline.com/common/oauth2/v2.0/devicecode",
		TokenURL:      "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		ClientID:      strings.TrimSpace(clientID),
		Scopes:        []string{"Files.ReadWrite.AppFolder", "Files.ReadWrite", "offline_access", "User.Read"},
	}
}

// DefaultGoogleDriveOAuthConfig returns Google OAuth2 endpoints for Drive appDataFolder sync.
func DefaultGoogleDriveOAuthConfig(clientID, clientSecret string) OAuthEndpointConfig {
	return OAuthEndpointConfig{
		DeviceCodeURL: "https://oauth2.googleapis.com/device/code",
		TokenURL:      "https://oauth2.googleapis.com/token",
		ClientID:      strings.TrimSpace(clientID),
		ClientSecret:  strings.TrimSpace(clientSecret),
		Scopes:        []string{"https://www.googleapis.com/auth/drive.appdata"},
	}
}

// OAuthClient manages OAuth2 Device Code authorization and thread-safe token refresh.
type OAuthClient struct {
	cfg             OAuthEndpointConfig
	httpClient      *http.Client
	mu              sync.Mutex
	tokens          OAuthTokens
	onTokensUpdated func(OAuthTokens)
	nowFn           func() time.Time
}

// NewOAuthClient creates a thread-safe OAuthClient initialized with existing tokens.
func NewOAuthClient(cfg OAuthEndpointConfig, initial OAuthTokens, httpClient *http.Client, onUpdated func(OAuthTokens)) *OAuthClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &OAuthClient{
		cfg:             cfg,
		httpClient:      httpClient,
		tokens:          initial,
		onTokensUpdated: onUpdated,
		nowFn:           time.Now,
	}
}

// Tokens returns a snapshot of the current token pair.
func (c *OAuthClient) Tokens() OAuthTokens {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tokens
}

// StartDeviceCode initiates an RFC 8628 Device Authorization Grant.
func (c *OAuthClient) StartDeviceCode(ctx context.Context) (DeviceCodeChallenge, error) {
	if strings.TrimSpace(c.cfg.ClientID) == "" {
		return DeviceCodeChallenge{}, errors.New("oauth: ClientID is required")
	}
	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	if len(c.cfg.Scopes) > 0 {
		form.Set("scope", strings.Join(c.cfg.Scopes, " "))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.DeviceCodeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return DeviceCodeChallenge{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DeviceCodeChallenge{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DeviceCodeChallenge{}, fmt.Errorf("oauth device code HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var wire struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURL         string `json:"verification_url"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		Message                 string `json:"message"`
		ExpiresIn               int    `json:"expires_in"`
		Interval                int    `json:"interval"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return DeviceCodeChallenge{}, err
	}
	uri := wire.VerificationURI
	if uri == "" {
		uri = wire.VerificationURL
	}
	if uri == "" {
		uri = wire.VerificationURIComplete
	}
	if wire.Interval <= 0 {
		wire.Interval = 5
	}
	return DeviceCodeChallenge{
		DeviceCode:      wire.DeviceCode,
		UserCode:        wire.UserCode,
		VerificationURI: uri,
		Message:         wire.Message,
		ExpiresIn:       wire.ExpiresIn,
		Interval:        wire.Interval,
	}, nil
}

// PollDeviceCode polls the token endpoint once for completion, or until ctx cancels when block=true.
func (c *OAuthClient) PollDeviceCodeOnce(ctx context.Context, deviceCode string) (tokens OAuthTokens, pending bool, slowDown bool, err error) {
	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	if strings.TrimSpace(c.cfg.ClientSecret) != "" {
		form.Set("client_secret", c.cfg.ClientSecret)
	}
	form.Set("device_code", deviceCode)
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return OAuthTokens{}, false, false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return OAuthTokens{}, false, false, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		toks, parseErr := c.parseAndStoreTokenResponse(raw, "")
		return toks, false, false, parseErr
	}
	var errBody struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(raw, &errBody)
	switch errBody.Error {
	case "authorization_pending":
		return OAuthTokens{}, true, false, nil
	case "slow_down":
		return OAuthTokens{}, true, true, nil
	case "expired_token", "access_denied":
		return OAuthTokens{}, false, false, fmt.Errorf("%w: %s", ErrAuthExpired, firstNonEmptySync(errBody.ErrorDescription, errBody.Error))
	default:
		return OAuthTokens{}, false, false, fmt.Errorf("oauth token HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
}

// AccessToken returns a non-expired access token, refreshing automatically if near expiry.
func (c *OAuthClient) AccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	tok := c.tokens
	c.mu.Unlock()

	if tok.AccessToken != "" && !c.isExpiredSoon(tok.ExpiryUTC) {
		return tok.AccessToken, nil
	}
	refreshed, err := c.Refresh(ctx)
	if err != nil {
		return "", err
	}
	return refreshed.AccessToken, nil
}

// Refresh forces a refresh_token exchange and persists updated tokens.
func (c *OAuthClient) Refresh(ctx context.Context) (OAuthTokens, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if strings.TrimSpace(c.tokens.RefreshToken) == "" {
		if c.tokens.AccessToken != "" && !c.isExpiredSoon(c.tokens.ExpiryUTC) {
			return c.tokens, nil
		}
		return OAuthTokens{}, fmt.Errorf("%w: missing refresh token", ErrAuthExpired)
	}
	if strings.TrimSpace(c.cfg.ClientID) == "" || strings.TrimSpace(c.cfg.TokenURL) == "" {
		return OAuthTokens{}, fmt.Errorf("%w: missing OAuth client config", ErrAuthExpired)
	}

	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	if strings.TrimSpace(c.cfg.ClientSecret) != "" {
		form.Set("client_secret", c.cfg.ClientSecret)
	}
	form.Set("refresh_token", c.tokens.RefreshToken)
	form.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return OAuthTokens{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return OAuthTokens{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusBadRequest {
		var eb struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &eb)
		if eb.Error == "invalid_grant" || resp.StatusCode == http.StatusUnauthorized {
			return OAuthTokens{}, fmt.Errorf("%w: %s", ErrAuthExpired, strings.TrimSpace(string(raw)))
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return OAuthTokens{}, fmt.Errorf("oauth refresh HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return c.parseAndStoreTokenResponseLocked(raw, c.tokens.RefreshToken)
}

func (c *OAuthClient) parseAndStoreTokenResponse(raw []byte, fallbackRefresh string) (OAuthTokens, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.parseAndStoreTokenResponseLocked(raw, fallbackRefresh)
}

func (c *OAuthClient) parseAndStoreTokenResponseLocked(raw []byte, fallbackRefresh string) (OAuthTokens, error) {
	var wire struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return OAuthTokens{}, err
	}
	if strings.TrimSpace(wire.AccessToken) == "" {
		return OAuthTokens{}, fmt.Errorf("%w: empty access_token", ErrAuthExpired)
	}
	refresh := strings.TrimSpace(wire.RefreshToken)
	if refresh == "" {
		refresh = fallbackRefresh
	}
	exp := ""
	if wire.ExpiresIn > 0 {
		exp = c.nowFn().UTC().Add(time.Duration(wire.ExpiresIn) * time.Second).Format(time.RFC3339)
	}
	next := OAuthTokens{
		AccessToken:  wire.AccessToken,
		RefreshToken: refresh,
		TokenType:    firstNonEmptySync(wire.TokenType, "Bearer"),
		ExpiryUTC:    exp,
		AccountEmail: c.tokens.AccountEmail,
	}
	c.tokens = next
	if c.onTokensUpdated != nil {
		c.onTokensUpdated(next)
	}
	return next, nil
}

func (c *OAuthClient) isExpiredSoon(expiryUTC string) bool {
	if strings.TrimSpace(expiryUTC) == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, expiryUTC)
	if err != nil {
		return false
	}
	return !t.After(c.nowFn().UTC().Add(45 * time.Second))
}

func firstNonEmptySync(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
