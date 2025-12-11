package httptest

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// MockResponse defines the behavior for a single HTTP response in the sequence.
type MockResponse struct {
	Status  int
	Headers map[string]string
	Body    []byte

	// Error allows simulating a network error (RoundTrip returns error)
	Error error
}

// MockRoundTripper is a simplistic http.RoundTripper that serves a pre-defined
// sequence of responses.
type MockRoundTripper struct {
	resps []MockResponse
	index int
}

// NewMockRoundTripper creates a new [MockRoundTripper].
func NewMockRoundTripper(resps []MockResponse) *MockRoundTripper {
	return &MockRoundTripper{
		resps: resps,
		index: 0,
	}
}

// RoundTrip implements the http.RoundTripper interface.
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.index >= len(m.resps) {
		return nil, fmt.Errorf("MockRoundTripper: no more responses configured")
	}

	respCfg := m.resps[m.index]
	m.index++

	if respCfg.Error != nil {
		return nil, respCfg.Error
	}

	statusCode := respCfg.Status
	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	header := make(http.Header, len(respCfg.Headers))
	for k, v := range respCfg.Headers {
		header.Add(k, v)
	}

	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader(respCfg.Body)),
		Header:     header,
		Request:    req,
	}, nil
}
