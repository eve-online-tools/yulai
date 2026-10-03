// Package sde keeps CCP's Static Data Export in its own sqlite database, one typed
// table per export file. See docs/SDE.md.
package sde

//go:generate go run ./gen

// table is one SDE table: a file's records, or the elements of an array within them.
type table struct {
	name   string
	file   string
	parent int      // index in tables; -1 for a file's top table
	path   []string // from the parent's row to the array
	keyed  bool     // child rows have a key column instead of idx
	id     bool     // child rows get an id their own children refer to
	cols   []column
}

// column reads path within a row's JSON value. kind is i, f, s or b.
type column struct {
	name string
	path []string
	kind byte
}
