package butterflymx

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"golang.org/x/oauth2"
)

// OAuth2Client is responsible for the final step of the ButterflyMX authentication process.
// It uses a standard OAuth2 token to acquire a session-specific API token (a Rails token)
// from the ButterflyMX API. This client does not interact with the main ButterflyMX API endpoints
// for actions like opening doors or creating keys.
type OAuth2Client struct {
	tokenSource oauth2.TokenSource
}

var _ APITokenSource = (*OAuth2Client)(nil)

// NewOAuth2Client creates a new client for handling the OAuth2 to API token exchange.
// It takes an oauth2.TokenSource, which is expected to be fully configured and capable
// of providing valid OAuth2 access tokens for the ButterflyMX service.
func NewOAuth2Client(tokenSource oauth2.TokenSource) *OAuth2Client {
	return &OAuth2Client{
		tokenSource: tokenSource,
	}
}

// APIToken performs the token exchange. It retrieves an OAuth2 access token from the
// client's token source, then sends it to the /denizen/v1/login endpoint.
// The ButterflyMX API validates the OAuth2 token and returns a Rails session token,
// which is required for all subsequent API interactions. This process effectively bridges
// the gap between the standard OAuth2 authentication and ButterflyMX's session-based API authentication.
func (c *OAuth2Client) APIToken(ctx context.Context) (APIStaticToken, error) {
	token, err := c.tokenSource.Token()
	if err != nil {
		return "", err
	}

	requestBody, err := json.Marshal(map[string]interface{}{
		"access_token": token.AccessToken,
		"device": map[string]interface{}{
			"locales":  []string{"en"},
			"platform": "android",
			"version":  "1.56.0",
		},
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
