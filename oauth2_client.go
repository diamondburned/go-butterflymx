package butterflymx

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"golang.org/x/oauth2"
)

// AssumedAPITokenValidity is the assumed validity duration for ButterflyMX API
// tokens obtained via OAuth2 exchange, as the actual validity period is
// unknown.
const AssumedAPITokenValidity = 5 * time.Minute

// APIDeviceInfo represents the device information sent during the OAuth2 to
// API token exchange.
var APIDeviceInfo = map[string]any{
	"locales":  []string{"en"},
	"platform": "android",
	"version":  "1.56.0",
}

// OAuth2Client consumes an OAuth2 token to exchange it for a ButterflyMX API
// token. This client does not interact with the main ButterflyMX API endpoints
// for actions like opening doors or creating keys.
//
// It implements the [APITokenSource] interface.
type OAuth2Client struct {
	tokenSource oauth2.TokenSource
	lastToken   atomic.Pointer[APIStaticToken]
}

var _ APITokenSource = (*OAuth2Client)(nil)

// NewOAuth2Client creates a new client for handling the OAuth2 to API token
// exchange. It takes an [oauth2.TokenSource], which is expected to be fully
// configured and capable of providing valid OAuth2 access tokens for the
// ButterflyMX service.
func NewOAuth2Client(tokenSource oauth2.TokenSource) *OAuth2Client {
	return &OAuth2Client{
		tokenSource: tokenSource,
	}
}

// APIToken performs the token exchange for a new token. It always returns a new
// token regardless of [renew].
//
// It first retrieves an OAuth2 access token from the client's token source,
// then sends it to the /denizen/v1/login endpoint. The ButterflyMX API
// validates the OAuth2 token and returns a Rails session token, which is
// required for all subsequent API interactions.
func (c *OAuth2Client) APIToken(ctx context.Context, renew bool) (APIStaticToken, error) {
	return c.APITokenSource().APIToken(ctx, renew)
}

// APITokenSource returns an [APITokenSource] that provides an API token until it
// needs to be renewed (once [renew] is true).
func (c *OAuth2Client) APITokenSource() APITokenSource {
	return ReuseAPITokenSource(oauth2APITokenSource{
		oauth2TokenSource: c.tokenSource,
	})
}

type oauth2APITokenSource struct {
	oauth2TokenSource oauth2.TokenSource
}

func (s oauth2APITokenSource) APIToken(ctx context.Context, renew bool) (APIStaticToken, error) {
	token, err := s.oauth2TokenSource.Token()
	if err != nil {
		return "", err
	}

	requestBody, err := json.Marshal(map[string]any{
		"access_token": token.AccessToken,
		"device":       APIDeviceInfo,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, APIBaseURL+"/denizen/v1/login", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var responseBody struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		return "", err
	}

	return APIStaticToken(responseBody.Token), nil
}
