package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testKey = "test-api-key-do-not-use-0000"

func TestGetJSONSendsKeyAsHeaderNotURL(t *testing.T) {
	var gotHeader, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Goog-Api-Key")
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := &youtubeClient{apiKey: testKey, http: srv.Client()}
	var dst map[string]any
	if err := c.getJSON(context.Background(), srv.URL+"/search?q=go", &dst); err != nil {
		t.Fatalf("getJSON: %v", err)
	}
	if gotHeader != testKey {
		t.Errorf("X-Goog-Api-Key = %q, want the API key", gotHeader)
	}
	if strings.Contains(gotQuery, testKey) || strings.Contains(gotQuery, "key=") {
		t.Errorf("API key leaked into query string: %q", gotQuery)
	}
}

func TestGetJSONTransportErrorDoesNotLeakKey(t *testing.T) {
	// A handler that never answers in time forces a client timeout, the same
	// failure that previously printed the full ?key=... URL into run logs.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	c := &youtubeClient{apiKey: testKey, http: &http.Client{Timeout: 20 * time.Millisecond}}
	var dst map[string]any
	err := c.getJSON(context.Background(), srv.URL+"/search?q=go", &dst)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if strings.Contains(err.Error(), testKey) {
		t.Fatalf("error leaks API key: %v", err)
	}
}

func TestRedactErr(t *testing.T) {
	err := redactErr(errString("Get \"https://x/?key="+testKey+"\": timeout"), testKey)
	if strings.Contains(err.Error(), testKey) || !strings.Contains(err.Error(), "REDACTED") {
		t.Fatalf("redactErr did not scrub key: %v", err)
	}
	if redactErr(nil, testKey) != nil {
		t.Fatal("redactErr(nil) should be nil")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
