package esi

import (
	"net/http"
	"testing"
	"time"
)

func TestExpiresAt(t *testing.T) {
	date := time.Date(2026, 9, 15, 21, 58, 29, 0, time.UTC)
	expires := date.Add(30 * time.Second)

	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Date", date.Format(http.TimeFormat))
	resp.Header.Set("Expires", expires.Format(http.TimeFormat))

	// Cache-Control disagrees with Expires on purpose; it must win.
	resp.Header.Set("Cache-Control", "public, max-age=60, must-revalidate, stale-if-error=900")
	if got := ExpiresAt(resp, time.Minute); !got.Equal(date.Add(60 * time.Second)) {
		t.Fatalf("max-age: got %v", got)
	}

	resp.Header.Del("Cache-Control")
	if got := ExpiresAt(resp, time.Minute); !got.Equal(expires) {
		t.Fatalf("expires: got %v", got)
	}

	resp.Header.Del("Expires")
	if got := ExpiresAt(resp, time.Minute); time.Until(got) < 55*time.Second {
		t.Fatalf("fallback: got %v", got)
	}
}
