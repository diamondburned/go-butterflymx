package butterflymx

import (
	"bytes"
	"context"
	"encoding/json/v2"
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

// DenizenLoginClient is a client that performs the OAuth2 to API token exchange
// using the /denizen/v1/login endpoint. It is designed to be used with an
// [oauth2.TokenSource] to obtain an [APITokenSource] that provides API tokens
// for authenticating with the ButterflyMX API.
//
// It implements the [APITokenSource] interface.
type DenizenLoginClient struct {
	tokenSource oauth2.TokenSource
	lastToken   atomic.Pointer[APIStaticToken]
}

var _ APITokenSource = (*DenizenLoginClient)(nil)

// NewDenizenLoginClient creates a new client for handling the OAuth2 to API token
// exchange. It takes an [oauth2.TokenSource], which is expected to be fully
// configured and capable of providing valid OAuth2 access tokens for the
// ButterflyMX service.
func NewDenizenLoginClient(tokenSource oauth2.TokenSource) *DenizenLoginClient {
	return &DenizenLoginClient{
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
func (c *DenizenLoginClient) APIToken(ctx context.Context, renew bool) (APIStaticToken, error) {
	return c.APITokenSource().APIToken(ctx, renew)
}

// APITokenSource returns an [APITokenSource] that provides an API token until it
// needs to be renewed (once [renew] is true).
func (c *DenizenLoginClient) APITokenSource() APITokenSource {
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
	if err := json.UnmarshalRead(resp.Body, &responseBody); err != nil {
		return "", err
	}

	return APIStaticToken(responseBody.Token), nil
}
