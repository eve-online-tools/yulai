// Package todo marks skeleton code. Every stub returns ErrNotImplemented so the app
// boots and the UI renders while the real implementation is still planned.
// Delete this package once nothing imports it.
package todo

import "errors"

var ErrNotImplemented = errors.New("not implemented yet")
