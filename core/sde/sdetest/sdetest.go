// Package sdetest installs small SDE exports for tests of packages that read the SDE.
package sdetest

import (
	"archive/zip"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/eve-online-tools/yulai/core/sde"
)

// Build is the build number Install gives an export without _sde.jsonl.
const Build = 1

// Open returns an empty store, closed when the test ends.
func Open(t *testing.T) *sde.Store {
	t.Helper()
	s, err := sde.Open(t.Context(), t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// Install returns a store with files installed, keyed by file name in CCP's
// JSONL export format.
func Install(t *testing.T, files map[string]string) *sde.Store {
	t.Helper()
	s := Open(t)
	if err := s.Install(t.Context(), writeZip(t, files)); err != nil {
		t.Fatal(err)
	}
	return s
}

func writeZip(t *testing.T, files map[string]string) string {
	t.Helper()
	if _, ok := files["_sde.jsonl"]; !ok {
		files["_sde.jsonl"] = `{"_key": "sde", "buildNumber": 1, "releaseDate": "2026-10-02T11:08:57Z"}`
	}
	p := filepath.Join(t.TempDir(), "export.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, body+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}
