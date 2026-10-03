// Package esi builds the shared ESI HTTP client on top of lib-esi-go.
package esi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	defaults "github.com/eve-online-tools/lib-esi-go"
	"github.com/eve-online-tools/lib-esi-go/middleware/cache"
	"github.com/eve-online-tools/lib-esi-go/middleware/ratelimiting"
	"github.com/eve-online-tools/lib-esi-go/middleware/ratelimiting/memory"
	"github.com/eve-online-tools/lib-esi-go/request"
	"github.com/eve-online-tools/lib-esi-go/transport"
)

const Version = "0.0.1"

// NewClient builds an ESI client with rate limiting and an on-disk ETag/Expires cache.
func NewClient(appName, contact, cachePath string) *http.Client {
	return &http.Client{
		Transport: transport.New(appName, Version, []string{contact}, defaults.CompatibilityDate,
			transport.WithMiddleware(ratelimiting.Middleware(memory.New())),
			transport.WithMiddleware(cache.Middleware(cachePath)),
		),
	}
}

type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("esi: %d %s", e.Status, e.Message) }

// Check turns a non-2xx response into an error. lib-esi-go does not do this itself.
func Check[T any](resp *request.Response[T], err error) (T, error) {
	var zero T
	if err != nil {
		return zero, err
	}
	if resp.StatusCode >= 400 {
		msg := resp.Status
		if resp.ErrorData != nil {
			msg = resp.ErrorData.ErrorMessage
		}
		return zero, &Error{Status: resp.StatusCode, Message: msg}
	}
	return resp.Data, nil
}

// ExpiresAt reads when ESI says the data changes next. Cache-Control max-age wins,
// then Expires, then now+fallback.
func ExpiresAt(resp *http.Response, fallback time.Duration) time.Time {
	if resp == nil {
		return time.Now().Add(fallback)
	}
	if maxAge, ok := maxAge(resp.Header.Get("Cache-Control")); ok {
		base := time.Now()
		if d, err := http.ParseTime(resp.Header.Get("Date")); err == nil {
			base = d
		}
		return base.Add(maxAge)
	}
	if t, err := http.ParseTime(resp.Header.Get("Expires")); err == nil {
		return t
	}
	return time.Now().Add(fallback)
}

func maxAge(cacheControl string) (time.Duration, bool) {
	for _, part := range strings.Split(cacheControl, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || !strings.EqualFold(k, "max-age") {
			continue
		}
		secs, err := strconv.Atoi(strings.Trim(v, `"`))
		if err != nil {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}
	return 0, false
}

// Fetch is Check but also hands back the raw response, for cache headers.
func Fetch[T any](resp *request.Response[T], err error) (T, *http.Response, error) {
	data, err := Check(resp, err)
	if err != nil {
		return data, nil, err
	}
	return data, resp.Response, nil
}
