package appleauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// DefaultBaseURL is the Sign in with Apple token host (/auth/token, /auth/revoke).
const DefaultBaseURL = "https://appleid.apple.com"

// TokenClientSecretTTL is the lifetime of the client_secret generated for one
// token call (exchange or revoke). Short: it is used once, right away.
const TokenClientSecretTTL = 5 * time.Minute

// maxTokenResponseBytes bounds what is read from Apple.
const maxTokenResponseBytes = 64 << 10

// ErrNoRefreshToken: Apple answered the code exchange without a refresh token.
var ErrNoRefreshToken = errors.New("appleauth: token response has no refresh_token")

// appleErrorCode matches Apple's documented error codes (invalid_grant, invalid_client, ...).
// Anything else in an error body is dropped, so an error never carries response text.
var appleErrorCode = regexp.MustCompile(`^[a-z_]{1,64}$`)

// TokenClient calls Apple's token endpoints: exchanging an authorization code
// for a refresh token and revoking a refresh token (TEC-1181). Errors carry the
// endpoint, the HTTP status and Apple's error code only, never a code, token or
// client secret. Safe for concurrent use.
type TokenClient struct {
	baseURL string
	http    *http.Client
}

// NewTokenClient builds a client for baseURL ("" = DefaultBaseURL; tests pass an
// httptest server). A nil httpClient gets a 10 s timeout. Redirects are not followed.
func NewTokenClient(baseURL string, httpClient *http.Client) *TokenClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	c := *httpClient
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &TokenClient{baseURL: baseURL, http: &c}
}

// ExchangeCode trades an authorization code from the app for a refresh token.
// clientID is the client the code was issued to (the iOS bundle id) and
// clientSecret a client_secret JWT for it (SigningKey.ClientSecret). The access
// and id tokens in the response are discarded.
func (c *TokenClient) ExchangeCode(ctx context.Context, clientID, clientSecret, code string) (string, error) {
	code = strings.TrimSpace(code)
	if strings.TrimSpace(clientID) == "" || clientSecret == "" || code == "" {
		return "", errors.New("appleauth: exchange needs a client id, client secret and code")
	}
	body, err := c.post(ctx, "/auth/token", url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		return "", err
	}
	var out struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", errors.New("appleauth: /auth/token: response is not JSON")
	}
	if strings.TrimSpace(out.RefreshToken) == "" {
		return "", ErrNoRefreshToken
	}
	return out.RefreshToken, nil
}

// Revoke invalidates a refresh token. clientID must be the client the token was
// issued to, clientSecret a client_secret JWT for it.
func (c *TokenClient) Revoke(ctx context.Context, clientID, clientSecret, refreshToken string) error {
	if strings.TrimSpace(clientID) == "" || clientSecret == "" || refreshToken == "" {
		return errors.New("appleauth: revoke needs a client id, client secret and token")
	}
	_, err := c.post(ctx, "/auth/revoke", url.Values{
		"client_id":       {clientID},
		"client_secret":   {clientSecret},
		"token":           {refreshToken},
		"token_type_hint": {"refresh_token"},
	})
	return err
}

func (c *TokenClient) post(ctx context.Context, path string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("appleauth: %s: build request", path)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		// The *url.Error names the endpoint only; the form is in the body, never in the URL.
		return nil, fmt.Errorf("appleauth: %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxTokenResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("appleauth: %s: read response", path)
	}
	if res.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &e)
		if !appleErrorCode.MatchString(e.Error) {
			e.Error = "unknown"
		}
		return nil, fmt.Errorf("appleauth: %s: status %d (%s)", path, res.StatusCode, e.Error)
	}
	return body, nil
}
