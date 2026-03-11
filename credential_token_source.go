package butterflymx

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/oauth2"
)

// AccountsBaseURL is the base URL for ButterflyMX account authentication.
const AccountsBaseURL = "https://accounts.butterflymx.com"

// SpoofUserAgent is a user-agent string that mimics a mobile browser, used
// during the OAuth2 login flow.
const SpoofUserAgent = "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36"

// RedirectURI is the OAuth2 redirect URI used by the ButterflyMX mobile app.
const RedirectURI = "com.butterflymx.oauth://oauth"

// CredentialTokenSource implements [oauth2.TokenSource] by performing the full
// ButterflyMX browser-based OAuth2 login flow. It produces OAuth2 tokens that
// can be fed into [OAuth2Client] to obtain API tokens.
//
// The login flow is reverse-engineered from the ButterflyMX mobile app and
// involves HTML form scraping, CSRF token extraction, and cookie-based session
// management.
type CredentialTokenSource struct {
	Email        string
	Password     string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
	Logger       *slog.Logger

	mu           sync.Mutex
	refreshToken string
}

var _ oauth2.TokenSource = (*CredentialTokenSource)(nil)

// Token returns an OAuth2 token. It first attempts to use a cached refresh
// token. If that fails or no refresh token is available, it performs the full
// browser-based login flow.
func (s *CredentialTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.refreshToken != "" {
		token, err := s.refresh()
		if err == nil {
			return token, nil
		}
		s.logger().Warn("refresh token failed, performing full login", "error", err)
		s.refreshToken = ""
	}

	return s.login()
}

func (s *CredentialTokenSource) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s *CredentialTokenSource) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return http.DefaultClient
}

// login performs the full browser-based OAuth2 authorization code flow:
//  1. GET /oauth/authorize - start OAuth flow (sets cookies)
//  2. GET /login/new - fetch login page, parse authenticity_token
//  3. POST /login - submit credentials
//  4. Follow redirect - extract authorization code
//  5. POST /oauth/token - exchange code for tokens
func (s *CredentialTokenSource) login() (*oauth2.Token, error) {
	slog := s.logger()

	// Create a cookie jar for session continuity across requests.
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating cookie jar: %w", err)
	}

	// Clone the underlying transport but use our own cookie jar,
	// and disable redirect following for steps that need manual handling.
	client := &http.Client{
		Transport: s.httpClient().Transport,
		Jar:       jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Step 1: Start OAuth flow to get session cookies.
	slog.Debug("starting OAuth authorize request")

	authorizeURL := AccountsBaseURL + "/oauth/authorize?" + url.Values{
		"client_id":     {s.ClientID},
		"redirect_uri":  {RedirectURI},
		"response_type": {"code"},
	}.Encode()

	req, err := http.NewRequest(http.MethodGet, authorizeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating authorize request: %w", err)
	}
	req.Header.Set("User-Agent", SpoofUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("authorize request: %w", err)
	}
	resp.Body.Close()

	// Step 2: Fetch login page and extract authenticity token.
	slog.Debug("fetching login page")

	req, err = http.NewRequest(http.MethodGet, AccountsBaseURL+"/login/new", nil)
	if err != nil {
		return nil, fmt.Errorf("creating login page request: %w", err)
	}
	req.Header.Set("User-Agent", SpoofUserAgent)

	resp, err = client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("login page request: %w", err)
	}
	defer resp.Body.Close()

	authenticityToken, err := parseAuthenticityToken(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing authenticity token: %w", err)
	}

	slog.Debug("extracted authenticity token")

	// Step 3: Submit login credentials.
	slog.Debug("submitting login credentials")

	formData := url.Values{
		"utf8":               {"✓"},
		"authenticity_token": {authenticityToken},
		"account[email]":     {s.Email},
		"account[password]":  {s.Password},
		"button":             {""},
	}

	req, err = http.NewRequest(http.MethodPost, AccountsBaseURL+"/login", strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", SpoofUserAgent)
	req.Header.Set("Referer", AccountsBaseURL+"/login/new")
	req.Header.Set("Origin", AccountsBaseURL)

	resp, err = client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("login request: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		return nil, fmt.Errorf("login request returned status %d, expected 302", resp.StatusCode)
	}

	// Step 4: Follow redirect to extract authorization code.
	redirectLocation := resp.Header.Get("Location")
	if redirectLocation == "" {
		return nil, fmt.Errorf("login response missing Location header")
	}

	slog.Debug("following login redirect", "location", redirectLocation)

	req, err = http.NewRequest(http.MethodGet, redirectLocation, nil)
	if err != nil {
		return nil, fmt.Errorf("creating redirect request: %w", err)
	}
	req.Header.Set("User-Agent", SpoofUserAgent)
	req.Header.Set("Referer", AccountsBaseURL+"/login/new")

	resp, err = client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("redirect request: %w", err)
	}
	resp.Body.Close()

	// The response should redirect to the custom scheme URL with the code.
	codeLocation := resp.Header.Get("Location")
	if codeLocation == "" {
		return nil, fmt.Errorf("redirect response missing Location header")
	}

	codeURL, err := url.Parse(codeLocation)
	if err != nil {
		return nil, fmt.Errorf("parsing code redirect URL: %w", err)
	}

	code := codeURL.Query().Get("code")
	if code == "" {
		return nil, fmt.Errorf("authorization code not found in redirect URL")
	}

	slog.Debug("extracted authorization code")

	// Step 5: Exchange authorization code for tokens.
	return s.exchangeCode(code)
}

// exchangeCode exchanges an authorization code for OAuth2 tokens.
func (s *CredentialTokenSource) exchangeCode(code string) (*oauth2.Token, error) {
	s.logger().Debug("exchanging authorization code for tokens")

	formData := url.Values{
		"client_secret": {s.ClientSecret},
		"grant_type":    {"authorization_code"},
		"client_id":     {s.ClientID},
		"redirect_uri":  {RedirectURI},
		"code":          {code},
	}

	return s.tokenRequest(formData)
}

// refresh exchanges a refresh token for a new OAuth2 token.
func (s *CredentialTokenSource) refresh() (*oauth2.Token, error) {
	s.logger().Debug("refreshing OAuth2 token")

	formData := url.Values{
		"refresh_token": {s.refreshToken},
		"client_id":     {s.ClientID},
		"grant_type":    {"refresh_token"},
	}

	return s.tokenRequest(formData)
}

// tokenRequest performs a POST to the token endpoint and parses the response.
func (s *CredentialTokenSource) tokenRequest(formData url.Values) (*oauth2.Token, error) {
	req, err := http.NewRequest(
		http.MethodPost,
		AccountsBaseURL+"/oauth/token",
		strings.NewReader(formData.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", SpoofUserAgent)

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token request returned status %d: %s", resp.StatusCode, body)
	}

	var tokenResp oauth2TokenResponse
	if err := parseJSONResponse(resp.Body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}

	// Cache the refresh token for next time.
	if tokenResp.RefreshToken != "" {
		s.refreshToken = tokenResp.RefreshToken
	}

	token := tokenResp.oauth2Token()
	return token, nil
}

// parseAuthenticityToken extracts the CSRF authenticity_token from the login
// page HTML. It finds <input name="authenticity_token" value="..."> using the
// stdlib-adjacent golang.org/x/net/html package.
func parseAuthenticityToken(r io.Reader) (string, error) {
	tokenizer := html.NewTokenizer(r)

	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			err := tokenizer.Err()
			if err == io.EOF {
				return "", fmt.Errorf("authenticity_token input not found in HTML")
			}
			return "", fmt.Errorf("HTML tokenizer error: %w", err)
		case html.SelfClosingTagToken, html.StartTagToken:
			tn, hasAttr := tokenizer.TagName()
			if string(tn) != "input" || !hasAttr {
				continue
			}

			var name, value string
			for {
				key, val, more := tokenizer.TagAttr()
				switch string(key) {
				case "name":
					name = string(val)
				case "value":
					value = string(val)
				}
				if !more {
					break
				}
			}

			if name == "authenticity_token" && value != "" {
				return value, nil
			}
		}
	}
}

// oauth2TokenResponse represents the JSON response from the ButterflyMX
// OAuth2 token endpoint.
type oauth2TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	CreatedAt    int64  `json:"created_at"`
}

func (r *oauth2TokenResponse) oauth2Token() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
		TokenType:    r.TokenType,
		Expiry:       time.Unix(r.CreatedAt+r.ExpiresIn, 0),
	}
}

func parseJSONResponse(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}
