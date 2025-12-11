package butterflymx

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"
	"github.com/neilotoole/slogt"
	"libdb.so/go-butterflymx/internal/httptest"
)

var mockToken APIStaticToken = "meowmeow"

func TestAPIClient_Keychains(t *testing.T) {
	accessCodesResponse := readFileAsResponseBody(t, "testdata/api-get-v3-access-codes.json")

	mockrt := httptest.NewMockRoundTripper([]httptest.MockResponse{
		{
			Status: 200,
			Body:   accessCodesResponse,
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

func readFileAsRequestAndResponseBodies(t *testing.T, path string) (jsontext.Value, jsontext.Value) {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read test file %q: %v", path, err)
	}

	var v1, v2 jsontext.Value
	if err := errors.Join(
		json.Unmarshal(b, &v1),
		json.Unmarshal(b, &v2),
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
