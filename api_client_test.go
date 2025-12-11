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
	assert.Equal(t, 31427903, keychains[0].ID)
	assert.Equal(t, 31353789, keychains[1].ID)
	assert.Equal(t, 31353474, keychains[2].ID)
	assert.Equal(t, 31299156, keychains[3].ID)

	// Assert virtual key references of the first keychain.
	assert.Equal(t, 1, len(keychains[0].Relationships.VirtualKeys))
	assert.Equal(t, 32553889, keychains[0].Relationships.VirtualKeys[0].ID)
	assert.Equal(t, TypeVirtualKey, keychains[0].Relationships.VirtualKeys[0].Type)
	// The Data field should be empty in the reference.
	assert.Zero(t, keychains[0].Relationships.VirtualKeys[0].Data)

	// Assert resolving of virtual key of the first keychain.
	virtualKey, err := keychains[0].Relationships.VirtualKeys[0].Resolve(results.Refs)
	assert.NoError(t, err)
	assert.Equal(t, 32553889, virtualKey.ID)
	assert.Equal(t, "user+delivery@example.com", virtualKey.Attributes.Name)
	assert.Equal(t, "user+delivery@example.com", virtualKey.Attributes.Email)
	assert.Equal(t, "012345", virtualKey.Attributes.PINCode)
	assert.Equal(t, "<REDACTED>", virtualKey.Attributes.QRCodeImageURL)
	assert.Equal(t, "<REDACTED>", virtualKey.Attributes.InstructionsURL)

	// Assert door release references of the virtual key.
	assert.Equal(t, 9, len(virtualKey.Relationships.DoorReleases))
	assert.Equal(t, 1536605877, virtualKey.Relationships.DoorReleases[0].ID)
	assert.Equal(t, TypeDoorRelease, virtualKey.Relationships.DoorReleases[0].Type)
	// The Data field should be empty in the reference.
	assert.Zero(t, virtualKey.Relationships.DoorReleases[0].Data)

	// Assert resolving of door release of the first virtual key.
	doorRelease, err := virtualKey.Relationships.DoorReleases[0].Resolve(results.Refs)
	assert.NoError(t, err)
	assert.Equal(t, 1536605877, doorRelease.ID)
	assert.Equal(t, "Jane Doe", doorRelease.Attributes.Name)
	assert.Equal(t, "2025-12-06T22:01:34Z", doorRelease.Attributes.CreatedAt.Format(time.RFC3339))
	assert.Equal(t, "2025-12-06T22:01:33Z", doorRelease.Attributes.LoggedAt.Format(time.RFC3339))
	assert.Equal(t, "<REDACTED>", doorRelease.Attributes.ThumbURL)
	assert.Equal(t, "<REDACTED>", doorRelease.Attributes.MediumURL)
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

	apiClient := NewAPIClient(mockToken, &APIClientOpts{
		HTTPClient: &http.Client{Transport: mockrt},
		Logger:     slogt.New(t),
	})

	result, err := apiClient.CreateCustomKeychain(t.Context(), 7648837, []ID{53449}, CustomKeychainArgs{
		Name:            "Jane Test",
		StartsAt:        mustRFC3339(t, "2025-12-09T16:58:00-0800"),
		EndsAt:          mustRFC3339(t, "2025-12-10T17:58:00-0800"),
		AllowUnitAccess: false,
	})
	assert.NoError(t, err)

	keychain := result.Data

	// Assert keychain object.
	assert.Equal(t, 31629991, keychain.ID)
	assert.Equal(t, "Jane Test", keychain.Attributes.Name)
	assert.Equal(t, CustomKeychain, keychain.Attributes.Kind)
	assert.Equal(t, "2025-12-10T00:58:00Z", keychain.Attributes.StartsAt.Format(time.RFC3339))
	assert.Equal(t, "2025-12-11T01:58:00Z", keychain.Attributes.EndsAt.Format(time.RFC3339))

	// Assert devices references.
	assert.Zero(t, keychain.Relationships.VirtualKeys)
	assert.Equal(t, 1, len(keychain.Relationships.Devices))
	assert.Equal(t, 27861, keychain.Relationships.Devices[0].ID)
	assert.Equal(t, TypePanel, keychain.Relationships.Devices[0].Type)
	assert.Zero(t, keychain.Relationships.Devices[0].Data)

	// Assert resolving of device.
	device, err := keychain.Relationships.Devices[0].Resolve(result.Refs)
	assert.NoError(t, err)
	assert.Equal(t, 27861, device.ID)
	assert.Equal(t, "Front Door", device.Attributes.Name)
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
