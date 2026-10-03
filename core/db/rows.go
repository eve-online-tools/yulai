package db

import (
	"database/sql"
	"errors"
	"reflect"
)

// Found reads the error of a :one query: false without error when there is no row.
func Found(err error) (bool, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// Changed reports whether row differs from prev, ignoring a FetchedAt field.
// Pointer fields are compared by value.
func Changed[T any](prev, row T) bool {
	a, b := reflect.ValueOf(prev), reflect.ValueOf(row)
	for i := range a.NumField() {
		if a.Type().Field(i).Name == "FetchedAt" {
			continue
		}
		if !reflect.DeepEqual(a.Field(i).Interface(), b.Field(i).Interface()) {
			return true
		}
	}
	return false
}
