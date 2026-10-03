package db

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

type row struct {
	ID        int64
	Name      *string
	FetchedAt time.Time
}

func TestFound(t *testing.T) {
	if ok, err := Found(sql.ErrNoRows); ok || err != nil {
		t.Fatalf("no rows = %v, %v", ok, err)
	}
	if ok, err := Found(nil); !ok || err != nil {
		t.Fatalf("row = %v, %v", ok, err)
	}
	boom := errors.New("boom")
	if _, err := Found(boom); err != boom {
		t.Fatalf("err = %v", err)
	}
}

func TestChanged(t *testing.T) {
	a, b, c := "a", "a", "c"
	prev := row{ID: 1, Name: &a, FetchedAt: time.Unix(1, 0)}
	if Changed(prev, row{ID: 1, Name: &b, FetchedAt: time.Unix(2, 0)}) {
		t.Fatal("equal values behind different pointers and a new FetchedAt should not count")
	}
	if !Changed(prev, row{ID: 1, Name: &c}) {
		t.Fatal("different name should count as changed")
	}
}
