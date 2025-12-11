//go:build goexperiment.jsonv2

package butterflymx

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"iter"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// API URL constants.
const (
	APIBaseURL             = "https://api.butterflymx.com"
	DenizenGraphQLEndpoint = APIBaseURL + "/denizen/v1/graphql"
)

// APIStaticToken represents a static ButterflyMX API token.
type APIStaticToken string

// APIToken returns the token as a string.
func (t APIStaticToken) APIToken(ctx context.Context) (APIStaticToken, error) {
	return t, nil
}

// APITokenSource is an interface for acquiring a ButterflyMX API token.
type APITokenSource interface {
	// APIToken should return a valid API token or an error.
	APIToken(ctx context.Context) (APIStaticToken, error)
}

// APIClient is a client for interacting with the main ButterflyMX API.
type APIClient struct {
	tokenSource APITokenSource
	opts        APIClientOpts
}

// APIClientOpts holds optional parameters for configuring the API client.
type APIClientOpts struct {
	HTTPClient *http.Client
	Logger     *slog.Logger
}

// NewAPIClient creates a new API client.
// It requires an APITokenSource to dynamically fetch the Rails API token.
func NewAPIClient(tokenSource APITokenSource, opts *APIClientOpts) *APIClient {
	opts = use(opts, &APIClientOpts{})
	opts.HTTPClient = use(opts.HTTPClient, http.DefaultClient)
	opts.Logger = use(opts.Logger, slog.Default())

	return &APIClient{
		tokenSource: tokenSource,
		opts:        *opts,
	}
}

func use[T any](v, otherwise *T) *T {
	if v != nil {
		return v
	}
	return otherwise
}

// CollectResults collects all results from the given iterator into a slice,
// returning an error if any occurred during iteration.
func CollectResults[T any](seq iter.Seq2[T, error]) ([]T, error) {
	var results []T
	for v, err := range seq {
		if err != nil {
			return results, err
		}
		results = append(results, v)
	}
	return results, nil
}

// Tenants retrieves a list of tenants associated with the current user.
// It calls the POST /denizen/v1/graphql endpoint with the "Tenants" operation.
// This method automatically handles pagination and returns an iterator.
func (c *APIClient) Tenants(ctx context.Context) iter.Seq2[Tenant, error] {
	return func(yield func(Tenant, error) bool) {
		var after *string
		for {
			variables := map[string]any{"after": after}
			var resp tenantsGraphQLResponse
			if err := c.doDenizenGraphQL(ctx, "Tenants", tenantsQuery, variables, &resp); err != nil {
				yield(Tenant{}, err)
				return
			}

			for _, tenant := range resp.Data.Tenants.Nodes {
				if !yield(tenant, nil) {
					return
				}
			}

			if !resp.Data.Tenants.PageInfo.HasNextPage {
				return
			}
			after = &resp.Data.Tenants.PageInfo.EndCursor
		}
	}
}

// TenantAccessPoints retrieves a list of access points (doors) for a given tenant.
// It calls the POST /denizen/v1/graphql endpoint with the "TenantAccessPoints" operation.
// This method automatically handles pagination and returns an iterator.
func (c *APIClient) TenantAccessPoints(ctx context.Context, tenantID TaggedID) iter.Seq2[AccessPoint, error] {
	return func(yield func(AccessPoint, error) bool) {
		var after *string
		for {
			variables := map[string]any{
				"ids":   []TaggedID{tenantID},
				"after": after,
			}
			var resp tenantAccessPointsGraphQLResponse
			if err := c.doDenizenGraphQL(ctx, "TenantAccessPoints", tenantAccessPointsQuery, variables, &resp); err != nil {
				yield(AccessPoint{}, err)
				return
			}
			if len(resp.Data.Nodes) == 0 {
				return
			}
			if len(resp.Data.Nodes) > 1 {
				yield(AccessPoint{}, fmt.Errorf("more than 1 tenant returned"))
				return
			}

			accessPoints := resp.Data.Nodes[0].AccessPoints
			for _, ap := range accessPoints.Nodes {
				if !yield(ap, nil) {
					return
				}
			}

			if !accessPoints.PageInfo.HasNextPage {
				return
			}
			after = &accessPoints.PageInfo.EndCursor
		}
	}
}

// Keychains retrieves a rich list of keychains, with all related entities
// resolved into a convenient structure. It calls the GET /v3/access_codes REST
// endpoint. This method automatically handles pagination and accumulates all
// results before resolving relationships.
func (c *APIClient) Keychains(ctx context.Context, tenantID TaggedID, status AccessCodeStatus) (*ResultsWithReferences[Keychain], error) {
	slog := c.opts.Logger
	slog.Debug(
		"fetching keychains",
		"tenant_id", tenantID.Number,
		"status", status)

	type accessCodesResponse struct {
		Data     []RawReference `json:"data"`
		Included []RawReference `json:"included"`
		Links    struct {
			Next *string `json:"next"`
		} `json:"links"`
	}

	var allData []RawReference
	var allIncluded []RawReference

	hasNext := true
	for page := 1; hasNext; page++ {
		path := "/v3/access_codes?" + url.Values{
			"include":        {"virtual_keys.door_releases.panel,devices"},
			"filter[tenant]": {fmt.Sprintf("%d", tenantID.Number)},
			"filter[status]": {string(status)},
			"page[size]":     {"100"},
			"page[number]":   {strconv.Itoa(page)},
		}.Encode()

		slog.Debug(
			"fetching keychains page",
			"page", page,
			"path", path)

		var resp accessCodesResponse
		if err := c.getAPI(ctx, path, &resp); err != nil {
			return nil, err
		}

		allData = append(allData, resp.Data...)
		allIncluded = append(allIncluded, resp.Included...)

		slog.Debug(
			"fetched keychains page",
			"page", page,
			"data_count", len(resp.Data),
			"data_count_total", len(allData),
			"included_count", len(resp.Included),
			"included_count_total", len(allIncluded),
			"has_next", resp.Links.Next != nil)

		hasNext = resp.Links.Next != nil
	}

	return unmarshalResultsWithReferences[Keychain](allData, allIncluded, slog)
}

// CustomKeychainArgs holds arguments for creating a new keychain.
type CustomKeychainArgs struct {
	// Name is the name of the keychain.
	Name string `json:"name"`
	// StartsAt is the start time of the keychain.
	StartsAt time.Time `json:"starts_at,format:'2006-01-02T15:04:05-0700'"`
	// EndsAt is the end time of the keychain.
	EndsAt time.Time `json:"ends_at,format:'2006-01-02T15:04:05-0700'"`
	// AllowUnitAccess indicates whether unit access is allowed.
	AllowUnitAccess bool `json:"allow_unit_access"`
}

// CreateCustomKeychain creates a new custom keychain.
// It calls the POST /v3/keychains/custom endpoint.
func (c *APIClient) CreateCustomKeychain(
	ctx context.Context,
	tenantID ID, accessPointIDs []ID, args CustomKeychainArgs,
) (*ResultWithReferences[Keychain], error) {
	slog := c.opts.Logger

	type RequestBody struct {
		Data struct {
			Type       string `json:"type"`
			Attributes struct {
				Kind string `json:"kind"`
				CustomKeychainArgs
			} `json:"attributes"`
			Relationships struct {
				AccessPoints struct {
					Data []RawReference `json:"data"`
				} `json:"access_points"`
				Devices struct {
					Data []RawReference `json:"data"` // unsupported
				} `json:"devices"`
				Tenant struct {
					Data RawReference `json:"data"`
				} `json:"tenant"`
			} `json:"relationships"`
		} `json:"data"`
	}

	var body RequestBody
	body.Data.Type = "keychains"
	body.Data.Attributes.Kind = "custom"
	body.Data.Attributes.CustomKeychainArgs = args
	body.Data.Relationships.Tenant.Data = RawReference{
		ID:   tenantID,
		Type: "tenants",
	}
	body.Data.Relationships.AccessPoints.Data = make([]RawReference, len(accessPointIDs))
	for i, apID := range accessPointIDs {
		body.Data.Relationships.AccessPoints.Data[i] = RawReference{
			ID:   apID,
			Type: "access_points",
		}
	}
	// Since devices are unsupported, we set an empty list.
	body.Data.Relationships.Devices.Data = []RawReference{}

	slog.Debug(
		"creating custom keychain",
		"tenant_id", tenantID,
		"access_point_ids", accessPointIDs,
		"args", args)

	var resp struct {
		Data     RawReference   `json:"data"`
		Included []RawReference `json:"included"`
	}

	if err := c.doAPIWithBody(ctx, http.MethodPost, "/v3/keychains/custom", body, &resp); err != nil {
		return nil, err
	}

	return unmarshalResultWithReferences[Keychain](resp.Data, resp.Included, slog)
}

func (c *APIClient) doDenizenGraphQL(ctx context.Context, operationName, query string, variables map[string]any, v any) error {
	token, err := c.tokenSource.APIToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get API token: %w", err)
	}

	reqBody, err := json.Marshal(map[string]any{
		"operationName": operationName,
		"variables":     variables,
		"query":         query,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, DenizenGraphQLEndpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+string(token))

	return c.doJSONRequest(req, v)
}

func (c *APIClient) getAPI(ctx context.Context, path string, v any) error {
	token, err := c.tokenSource.APIToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get API token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, APIBaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+string(token))

	return c.doJSONRequest(req, v)
}

func (c *APIClient) doAPIWithBody(ctx context.Context, method, path string, body any, v any) error {
	token, err := c.tokenSource.APIToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get API token: %w", err)
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, APIBaseURL+path, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+string(token))

	return c.doJSONRequest(req, v)
}

func (c *APIClient) doJSONRequest(req *http.Request, dst any) error {
	resp, err := c.opts.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}

	if err := json.UnmarshalRead(resp.Body, dst); err != nil {
		return fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}

	return nil
}

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(fmt.Sprintf("BUG: failed to parse URL %q: %v", rawURL, err))
	}
	return u
}
