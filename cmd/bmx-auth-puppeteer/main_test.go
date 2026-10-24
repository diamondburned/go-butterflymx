package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"
)

func TestRunAuthBrowserInterception(t *testing.T) {
	execPath, err := findBrowserExec("")
	if err != nil {
		t.Skipf("skipping test: %v", err)
	}

	expectedRedirect := "com.butterflymx.oauth://oauth?code=mock_auth_code_123&state=mock_state_456"

	// Mock server simulating immediate 302 redirect after visiting auth page
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, expectedRedirect, http.StatusFound)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	redirectedURL, err := runAuthBrowser(ctx, ts.URL, browserOptions{
		BrowserExec: execPath,
		Headless:    true,
	})
	assert.NoError(t, err)
	assert.Equal(t, expectedRedirect, redirectedURL)
}

func TestRunAuthBrowserJSInterception(t *testing.T) {
	execPath, err := findBrowserExec("")
	if err != nil {
		t.Skipf("skipping test: %v", err)
	}

	expectedRedirect := "com.butterflymx.oauth://oauth?code=js_code_789&state=js_state_012"

	// Mock server simulating page that redirects via JavaScript after loading
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html><html><body>
			<script>window.location.href = %q;</script>
		</body></html>`, expectedRedirect)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	redirectedURL, err := runAuthBrowser(ctx, ts.URL, browserOptions{
		BrowserExec: execPath,
		Headless:    true,
	})
	assert.NoError(t, err)
	assert.Equal(t, expectedRedirect, redirectedURL)
}
