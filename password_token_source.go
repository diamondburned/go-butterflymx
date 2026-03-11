package butterflymx

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/oauth2"
)

// PasswordTokenSource implements [oauth2.TokenSource] using the OAuth2
// resource owner password credentials grant. This is the same flow used by the
// ButterflyMX mobile app and does not require a client secret.
//
// The produced OAuth2 tokens can be fed into [OAuth2Client] to obtain API
// tokens, just like [CredentialTokenSource].
type PasswordTokenSource struct {
	Email    string
	Password string
	ClientID string
	// HTTPClient is the HTTP client to use for requests.
	// If nil, [http.DefaultClient] is used.
	HTTPClient *http.Client
	Logger     *slog.Logger

	mu           sync.Mutex
	refreshToken string
}

var _ oauth2.TokenSource = (*PasswordTokenSource)(nil)

// Token returns an OAuth2 token. It first attempts to use a cached refresh
// token. If that fails or no refresh token is available, it performs the
// password grant flow.
func (s *PasswordTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.refreshToken != "" {
		token, err := s.refresh()
		if err == nil {
			return token, nil
		}
		s.logger().Warn("refresh token failed, performing password grant", "error", err)
		s.refreshToken = ""
	}

	return s.login()
}

func (s *PasswordTokenSource) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s *PasswordTokenSource) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return http.DefaultClient
}

// login performs the password grant: a single POST to the token endpoint.
func (s *PasswordTokenSource) login() (*oauth2.Token, error) {
	s.logger().Debug("performing password grant")

	formData := url.Values{
		"grant_type": {"password"},
		"client_id":  {s.ClientID},
		"username":   {s.Email},
		"password":   {s.Password},
	}

	return s.tokenRequest(formData)
}

// refresh exchanges a refresh token for a new OAuth2 token.
func (s *PasswordTokenSource) refresh() (*oauth2.Token, error) {
	s.logger().Debug("refreshing OAuth2 token")

	formData := url.Values{
		"refresh_token": {s.refreshToken},
		"client_id":     {s.ClientID},
		"grant_type":    {"refresh_token"},
	}

	return s.tokenRequest(formData)
}

// tokenRequest performs a POST to the token endpoint and parses the response.
func (s *PasswordTokenSource) tokenRequest(formData url.Values) (*oauth2.Token, error) {
	req, err := http.NewRequest(
		http.MethodPost,
		AccountsBaseURL+"/oauth/token",
		strings.NewReader(formData.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("X-Client-Platform", "mobile")

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
