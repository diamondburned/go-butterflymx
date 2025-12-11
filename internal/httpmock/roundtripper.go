// Package httpmock provides a mock HTTP RoundTripper for testing purposes.
package httpmock

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"testing"
)

// RoundTrip defines the behavior for a single HTTP response in the sequence.
type RoundTrip struct {
	RequestCheck RoundTripRequestCheck
	Response     RoundTripResponse
}

// RoundTripRequestCheck defines a function type for checking HTTP requests.
type RoundTripRequestCheck func(t *testing.T, req *http.Request)

// RoundTripRequestCheckJSON creates a RoundTripRequestCheck that parses the
// request body as JSON into the specified type T and applies the provided check
// function.
func RoundTripRequestCheckJSON[T any](req *http.Request, checkFn func(t *testing.T, data T)) RoundTripRequestCheck {
	return func(t *testing.T, req *http.Request) {
		var data T
		if err := json.UnmarshalRead(req.Body, &data); err != nil {
			t.Fatalf("roundtrip request check: failed to unmarshal request body as JSON: %v", err)
		}
		checkFn(t, data)
	}
}

// ChainRoundTripRequestChecks chains multiple RoundTripRequestCheck functions
// into a single RoundTripRequestCheck.
func ChainRoundTripRequestChecks(checks ...RoundTripRequestCheck) RoundTripRequestCheck {
	return func(t *testing.T, req *http.Request) {
		for _, check := range checks {
			check(t, req)
		}
	}
}

// RoundTripResponse defines the components of an HTTP response.
type RoundTripResponse struct {
	Status  int
	Headers map[string]string
	Body    []byte
	// Error allows simulating a network error (RoundTrip returns error)
	Error error
}

// RoundTripper is a simplistic http.RoundTripper that serves a pre-defined
// sequence of responses.
type RoundTripper struct {
	t     *testing.T
	resps []RoundTrip
	index int
}

// NewRoundTripper creates a new [RoundTripper].
func NewRoundTripper(t *testing.T, resps []RoundTrip) *RoundTripper {
	return &RoundTripper{
		t:     t,
		resps: resps,
		index: 0,
	}
}

// RoundTrip implements the http.RoundTripper interface.
func (m *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.index >= len(m.resps) {
		m.t.Errorf("MockRoundTripper: no more responses configured (index %d out of %d)", m.index, len(m.resps))
		return nil, errors.New("no more responses configured in MockRoundTripper")
	}

	rt := m.resps[m.index]
	m.index++

	if rt.RequestCheck != nil {
		m.t.Run("request_check", func(t *testing.T) {
			rt.RequestCheck(m.t, req)
		})
	}

	if rt.Response.Error != nil {
		return nil, rt.Response.Error
	}

	statusCode := rt.Response.Status
	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	header := make(http.Header, len(rt.Response.Headers))
	for k, v := range rt.Response.Headers {
		header.Add(k, v)
	}

	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader(rt.Response.Body)),
		Header:     header,
		Request:    req,
	}, nil
}
