package butterflymx

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"
	"github.com/neilotoole/slogt"
	"libdb.so/go-butterflymx/internal/httpmock"
)

var mockToken APIStaticToken = "meowmeow"

func requestCheckAuthorizationBearer(t *testing.T, req *http.Request) {
	assert.Equal(t, "Bearer meowmeow", req.Header.Get("Authorization"))
}

func TestAPIClient_Keychains(t *testing.T) {
	accessCodesResponse := readFileAsResponseBody(t, "testdata/api-get-v3-access-codes.json")

	mockrt := httpmock.NewRoundTripper(t, []httpmock.RoundTrip{
		{
			RequestCheck: requestCheckAuthorizationBearer,
			Response: httpmock.RoundTripResponse{
				Status: http.StatusOK,
				Body:   accessCodesResponse,
			},
		},
	})

	apiClient := NewAPIClient(mockToken, &APIClientOpts{
		HTTPClient: &http.Client{Transport: mockrt},
		Logger:     slogt.New(t),
	})

	results, err := apiClient.Keychains(t.Context(), TaggedID{}, "")
	assert.NoError(t, err)

	keychains := results.Data

	// Assert keychain objects.
	assert.Equal(t, 4, len(keychains), "expected 4 keychains")
	assert.Equal(t, ID(20001), keychains[0].ID)
	assert.Equal(t, ID(20003), keychains[1].ID)
	assert.Equal(t, ID(20005), keychains[2].ID)
	assert.Equal(t, ID(20007), keychains[3].ID)

	// Assert virtual key references of the first keychain.
	assert.Equal(t, 1, len(keychains[0].Relationships.VirtualKeys))
	assert.Equal(t, ID(20002), keychains[0].Relationships.VirtualKeys[0].ID)
	assert.Equal(t, TypeVirtualKey, keychains[0].Relationships.VirtualKeys[0].Type)
	// The Data field should be empty in the reference.
	assert.Zero(t, keychains[0].Relationships.VirtualKeys[0].Data)

	// Assert resolving of virtual key of the first keychain.
	virtualKey, err := keychains[0].Relationships.VirtualKeys[0].Resolve(results.Refs)
	assert.NoError(t, err)
	assert.Equal(t, ID(20002), virtualKey.ID)
	assert.Equal(t, "user+delivery@example.com", virtualKey.Attributes.Name)
	assert.Equal(t, "user+delivery@example.com", virtualKey.Attributes.Email)
	assert.Equal(t, "012345", virtualKey.Attributes.PINCode)
	assert.Equal(t, "<REDACTED>", virtualKey.Attributes.QRCodeImageURL)
	assert.Equal(t, "<REDACTED>", virtualKey.Attributes.InstructionsURL)

	// Assert door release references of the virtual key.
	assert.Equal(t, 9, len(virtualKey.Relationships.DoorReleases))
	assert.Equal(t, ID(30001), virtualKey.Relationships.DoorReleases[0].ID)
	assert.Equal(t, TypeDoorRelease, virtualKey.Relationships.DoorReleases[0].Type)
	// The Data field should be empty in the reference.
	assert.Zero(t, virtualKey.Relationships.DoorReleases[0].Data)

	// Assert resolving of door release of the first virtual key.
	doorRelease, err := virtualKey.Relationships.DoorReleases[0].Resolve(results.Refs)
	assert.NoError(t, err)
	assert.Equal(t, ID(30001), doorRelease.ID)
	assert.Equal(t, "Jane Doe", doorRelease.Attributes.Name)
	assert.Equal(t, "2023-01-01T00:00:00Z", doorRelease.Attributes.CreatedAt.Format(time.RFC3339))
	assert.Equal(t, "2023-01-01T00:00:00Z", doorRelease.Attributes.LoggedAt.Format(time.RFC3339))
	assert.Equal(t, "<REDACTED>", doorRelease.Attributes.ThumbURL)
	assert.Equal(t, "<REDACTED>", doorRelease.Attributes.MediumURL)
}

func TestAPIClient_Keychain(t *testing.T) {
	customKeychainResponse := readFileAsResponseBody(t, "testdata/api-get-v3-keychains-id.json")

	mockrt := httpmock.NewRoundTripper(t, []httpmock.RoundTrip{
		{
			RequestCheck: httpmock.ChainRoundTripRequestChecks(
				requestCheckAuthorizationBearer,
				func(t *testing.T, req *http.Request) {
					assert.Contains(t, req.URL.Path, "/10001")
				},
			),
			Response: httpmock.RoundTripResponse{
				Status: http.StatusOK,
				Body:   customKeychainResponse,
			},
		},
	})

	apiClient := newTestAPIClient(t, mockrt)

	result, err := apiClient.Keychain(t.Context(), 10001)
	assert.NoError(t, err)

	keychain := result.Data

	// Assert keychain object.
	assert.Equal(t, ID(10001), keychain.ID)
	assert.Equal(t, "Jane Doe", keychain.Attributes.Name)
	assert.Equal(t, CustomKeychain, keychain.Attributes.Kind)
	assert.Equal(t, "2023-01-01T00:00:00Z", keychain.Attributes.StartsAt.Format(time.RFC3339))
	assert.Equal(t, "2023-01-02T00:00:00Z", keychain.Attributes.EndsAt.Format(time.RFC3339))
	assert.Equal(t, Timestamp{Hour: 16, Minute: 58}, keychain.Attributes.TimeFrom)
	assert.Equal(t, Timestamp{Hour: 17, Minute: 58}, keychain.Attributes.TimeTo)
	assert.Equal(t, Datestamp{Year: 2023, Month: time.January, Day: 1}, keychain.Attributes.StartDate)
	assert.Equal(t, Datestamp{Year: 2023, Month: time.January, Day: 2}, keychain.Attributes.EndDate)
	assert.False(t, keychain.Attributes.AllowUnitAccess)
	assert.Zero(t, keychain.Attributes.Weekdays)

	// Assert virtual key references.
	assert.Equal(t, 1, len(keychain.Relationships.VirtualKeys))
	assert.Equal(t, ID(10002), keychain.Relationships.VirtualKeys[0].ID)
	assert.Equal(t, TypeVirtualKey, keychain.Relationships.VirtualKeys[0].Type)
	assert.Zero(t, keychain.Relationships.VirtualKeys[0].Data)

	// Assert devices references.
	assert.Equal(t, 1, len(keychain.Relationships.Devices))
	assert.Equal(t, ID(10003), keychain.Relationships.Devices[0].ID)
	assert.Equal(t, TypePanel, keychain.Relationships.Devices[0].Type)
	assert.Zero(t, keychain.Relationships.Devices[0].Data)

	// Assert resolving of virtual key.
	virtualKey, err := keychain.Relationships.VirtualKeys[0].Resolve(result.Refs)
	assert.NoError(t, err)
	assert.Equal(t, ID(10002), virtualKey.ID)
	assert.Equal(t, "john.doe@example.com", virtualKey.Attributes.Name)
	assert.Equal(t, "john.doe@example.com", virtualKey.Attributes.Email)
	assert.Equal(t, PINCode("012345"), virtualKey.Attributes.PINCode)
	assert.Equal(t, "<REDACTED>", virtualKey.Attributes.QRCodeImageURL)
	assert.Equal(t, "<REDACTED>", virtualKey.Attributes.InstructionsURL)
	assert.True(t, virtualKey.Attributes.SentAt.IsZero())
}

func TestAPIClient_CreateCustomKeychain(t *testing.T) {
	customKeychainRequest, customKeychainResponse := readFileAsRequestAndResponseBodies(t, "testdata/api-post-v3-keychains-custom.json")
	assert.NoError(t, customKeychainRequest.Canonicalize())
	assert.NoError(t, customKeychainResponse.Canonicalize())

	mockrt := httpmock.NewRoundTripper(t, []httpmock.RoundTrip{
		{
			RequestCheck: httpmock.ChainRoundTripRequestChecks(
				requestCheckAuthorizationBearer,
				httpmock.RoundTripRequestCheckJSON(func(t *testing.T, data jsontext.Value) {
					assert.NoError(t, data.Canonicalize())
					// Ensure request body matches expected exactly.
					assert.Equal(t, string(customKeychainRequest), string(data))
				}),
			),
			Response: httpmock.RoundTripResponse{
				Status: http.StatusOK,
				Body:   customKeychainResponse,
			},
		},
	})

	apiClient := newTestAPIClient(t, mockrt)

	result, err := apiClient.CreateCustomKeychain(t.Context(), 10001, []ID{50001}, CustomKeychainArgs{
		Name:            "Jane Doe",
		StartsAt:        mustRFC3339(t, "2023-01-01T00:00:00-0800"),
		EndsAt:          mustRFC3339(t, "2023-01-02T00:00:00-0800"),
		AllowUnitAccess: false,
	})
	assert.NoError(t, err)

	keychain := result.Data

	// Assert keychain object.
	assert.Equal(t, ID(10001), keychain.ID)
	assert.Equal(t, "Jane Doe", keychain.Attributes.Name)
	assert.Equal(t, CustomKeychain, keychain.Attributes.Kind)
	assert.Equal(t, "2023-01-01T00:00:00Z", keychain.Attributes.StartsAt.Format(time.RFC3339))
	assert.Equal(t, "2023-01-02T00:00:00Z", keychain.Attributes.EndsAt.Format(time.RFC3339))

	// Assert devices references.
	assert.Zero(t, keychain.Relationships.VirtualKeys)
	assert.Equal(t, 1, len(keychain.Relationships.Devices))
	assert.Equal(t, ID(10003), keychain.Relationships.Devices[0].ID)
	assert.Equal(t, TypePanel, keychain.Relationships.Devices[0].Type)
	assert.Zero(t, keychain.Relationships.Devices[0].Data)

	// Assert resolving of device.
	device, err := keychain.Relationships.Devices[0].Resolve(result.Refs)
	assert.NoError(t, err)
	assert.Equal(t, ID(10003), device.ID)
	assert.Equal(t, "Front Door", device.Attributes.Name)
}

func TestAPIClient_CreateVirtualKeys(t *testing.T) {
	virtualKeyRequest, virtualKeyResponse := readFileAsRequestAndResponseBodies(t, "testdata/api-post-v3-keychains-id.json")
	assert.NoError(t, virtualKeyRequest.Canonicalize())
	assert.NoError(t, virtualKeyResponse.Canonicalize())

	mockrt := httpmock.NewRoundTripper(t, []httpmock.RoundTrip{
		{
			RequestCheck: httpmock.ChainRoundTripRequestChecks(
				requestCheckAuthorizationBearer,
				func(t *testing.T, req *http.Request) {
					assert.Contains(t, req.URL.Path, "/v3/keychains/10001/virtual_keys")
				},
				httpmock.RoundTripRequestCheckJSON(func(t *testing.T, data jsontext.Value) {
					assert.NoError(t, data.Canonicalize())
					// Ensure request body matches expected exactly.
					assert.Equal(t, string(virtualKeyRequest), string(data))
				}),
			),
			Response: httpmock.RoundTripResponse{
				Status: http.StatusOK,
				Body:   virtualKeyResponse,
			},
		},
	})

	apiClient := newTestAPIClient(t, mockrt)

	results, err := apiClient.CreateVirtualKeys(t.Context(), 10001, VirtualKeyArgs{
		Recipients: []VirtualKeyRecipient{
			{DeliverTo: "john.doe@example.com", Name: "john.doe@example.com"},
		},
	})
	assert.NoError(t, err)

	virtualKeys := results.Data
	assert.Equal(t, 1, len(virtualKeys))
	assert.Equal(t, ID(10002), virtualKeys[0].ID)
	assert.Equal(t, "john.doe@example.com", virtualKeys[0].Attributes.Name)
	assert.Equal(t, "john.doe@example.com", virtualKeys[0].Attributes.Email)
	assert.Equal(t, PINCode("012345"), virtualKeys[0].Attributes.PINCode)
	assert.Equal(t, "<REDACTED>", virtualKeys[0].Attributes.QRCodeImageURL)
	assert.Equal(t, "<REDACTED>", virtualKeys[0].Attributes.InstructionsURL)
	assert.True(t, virtualKeys[0].Attributes.SentAt.IsZero())
}

func newTestAPIClient(t *testing.T, mockrt http.RoundTripper) *APIClient {
	return NewAPIClient(mockToken, &APIClientOpts{
		HTTPClient: &http.Client{Transport: mockrt},
		Logger:     slogt.New(t),
	})
}

func mustRFC3339(t *testing.T, s string) time.Time {
	tm, err := time.Parse("2006-01-02T15:04:05-0700", s)
	assert.NoError(t, err, "BUG: failed to parse RFC3339 time constant")
	return tm
}

func readFileAsRequestAndResponseBodies(t *testing.T, path string) (jsontext.Value, jsontext.Value) {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read test file %q: %v", path, err)
	}

	dec := jsontext.NewDecoder(bytes.NewReader(b))
	var v1, v2 jsontext.Value
	if err := errors.Join(
		json.UnmarshalDecode(dec, &v1),
		json.UnmarshalDecode(dec, &v2),
	); err != nil {
		t.Fatalf("failed to unmarshal JSON from test file %q: %v", path, err)
	}

	return v1, v2
}

func readFileAsResponseBody(t *testing.T, path string) jsontext.Value {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read test file %q: %v", path, err)
	}

	var v jsontext.Value
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("failed to unmarshal JSON from test file %q: %v", path, err)
	}

	return v
}
