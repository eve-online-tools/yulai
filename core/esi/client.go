// Package esi builds the shared ESI HTTP client on top of lib-esi-go.
package esi

import "net/http"

const Version = "0.0.1"

// NewClient builds an ESI client. Planned: lib-esi-go transport with the
// ratelimiting and on-disk cache middleware, plus Check/Fetch/ExpiresAt helpers
// for turning responses into errors and next-run times.
func NewClient(appName, contact, cachePath string) *http.Client {
	return &http.Client{}
}
