package butterflymx

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"

	"golang.org/x/oauth2"
)

// AccountAuthConfig is an [oauth2.Config] for the ButterflyMX accounts service
// with the appropriate configuration.
var AccountAuthConfig = &oauth2.Config{
	ClientID: "0e3aeeb7cec2782b9fb21352a4349a44405ed5d7674072416b6481d51abfd6b6",
	Endpoint: oauth2.Endpoint{
		AuthURL:   "https://accounts.butterflymx.com/oauth/authorize",
		TokenURL:  "https://accounts.butterflymx.com/oauth/token",
		AuthStyle: oauth2.AuthStyleInParams,
	},
	// RedirectURI is the redirect URI that is used by the ButterflyMX app to
	// finish the OAuth2 flow. We're not using this URL for anything, but we
	// give it to the server to satisfy its requirements.
	RedirectURL: "com.butterflymx.oauth://oauth",
}

// AuthFlowClient handles the flow of exchanging user credentials for an OAuth2
// token. It is built with the assumption that the user manually visits the
// authorization URL and pastes the redirected URL back into the program.
//
// The workaround is needed because we use the ButterflyMX app's fake redirect
// URL to finish the OAuth2 flow, which would trip up normal web browsers, since
// they would attempt to navigate to the URL and fail. By having the user
// manually feed back the redirected URL, we can extract the authorization code
// and state from it without needing to handle the redirection ourselves.
//
// TODO: write a light browser wrapper that interjects the redirection request
// with this URL and finishes the handshake automatically. We can't use a normal
// browser because the server will likely flag all HTTP redirect URLs.
type AuthFlowClient struct {
	config *oauth2.Config
}

// NewAuthFlowClient creates a new [AuthFlowClient] with the default configuration.
func NewAuthFlowClient() *AuthFlowClient {
	return &AuthFlowClient{config: AccountAuthConfig}
}

// AuthFlowStart contains the information about the starting URL of the
// authorization flow.
type AuthFlowStart struct {
	url      string
	state    string
	verifier string
}

// URL returns the URL that a user would need to visit to authorize the
// application and obtain an authorization code.
func (u AuthFlowStart) URL() string {
	return u.url
}

// Start returns the URL that a user would need to visit to authorize the
// application and obtain an authorization code. The user is expected to visit
// this URL, complete the authorization process, and then paste the redirected
// URL back into [AuthFlowClient.Finish] to complete the flow
// and obtain an OAuth2 token.
func (f *AuthFlowClient) Start() AuthFlowStart {
	state := generateState()
	verifier := oauth2.GenerateVerifier()
	return AuthFlowStart{
		url:      f.config.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)),
		state:    state,
		verifier: verifier,
	}
}

// Finish finishes the flow using the URL that the user has received from the
// server. This URL is then used to exchange for an OAuth2 token that can be fed
// into [DenizenLoginClient] to obtain API tokens.
//
// The redirected URL is expected to have the same scheme and host as the
// redirect URL in [AccountAuthConfig], and contain the authorization code and
// state as query parameters.
func (f *AuthFlowClient) Finish(ctx context.Context, start AuthFlowStart, redirectedURL string) (*oauth2.Token, error) {
	originalRedirectURL, err := url.Parse(f.config.RedirectURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redirect URL in config: %w", err)
	}

	finalRedirectURL, err := url.Parse(redirectedURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redirected URL: %w", err)
	}

	if originalRedirectURL.Scheme != finalRedirectURL.Scheme || originalRedirectURL.Host != finalRedirectURL.Host {
		finalShort := url.URL{
			Scheme: finalRedirectURL.Scheme,
			Host:   finalRedirectURL.Host,
		}
		return nil, fmt.Errorf("redirected URL (at %q) does not match expected", finalShort.String())
	}

	finalQuery := finalRedirectURL.Query()

	state := finalQuery.Get("state")
	if state != start.state {
		return nil, fmt.Errorf("state mismatch: expected %q, got %q", start.state, state)
	}

	code := finalQuery.Get("code")
	token, err := f.config.Exchange(ctx, code, oauth2.VerifierOption(start.verifier))
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	return token, nil
}

func generateState() string {
	var r [16]byte
	if _, err := rand.Read(r[:]); err != nil {
		panic("failed to generate random state: " + err.Error())
	}
	return base64.URLEncoding.EncodeToString(r[:])
}
