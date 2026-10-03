package sde

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// writeZip writes an export zip with the given files to dir.
func writeZip(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	p := filepath.Join(dir, "export.zip")
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
		if _, err := io.WriteString(w, body); err != nil {
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

func fixture(build int) map[string]string {
	return map[string]string{
		"_sde.jsonl": `{"_key": "sde", "buildNumber": ` + strconv.Itoa(build) + `, "releaseDate": "2026-10-02T11:08:57Z"}` + "\n",
		"types.jsonl": `{"_key": 34, "groupID": 18, "name": {"en": "Tritanium", "de": "Tritanium"}, "published": true, "volume": 0.01, "shiny": 1}` + "\n" +
			`{"_key": 35, "groupID": 18, "name": {"en": "Pyerite"}, "volume": 0.01}` + "\n",
		"typeBonus.jsonl":       `{"_key": 587, "types": [{"_key": 3330, "_value": [{"bonus": 5, "bonusText": {"en": "a"}}, {"bonus": 7.5}]}], "roleBonuses": [{"bonus": 1, "unitID": 105}]}` + "\n",
		"mapSolarSystems.jsonl": `{"_key": 30000142, "name": {"en": "Jita"}, "planetIDs": [40009077, 40009078], "position": {"x": 1.5, "y": 2, "z": 3}, "securityStatus": 0.9}` + "\n",
		"notAFile.jsonl":        `{"_key": 1}` + "\n",
	}
}

func query[T any](t *testing.T, s *Store, q string, args ...any) T {
	t.Helper()
	var out T
	err := s.Read(t.Context(), func(*Queries) error {
		return s.conn.QueryRowContext(t.Context(), q, args...).Scan(&out)
	})
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return out
}

func TestInstall(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(t.Context(), dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.Read(t.Context(), func(*Queries) error { return nil }); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("Read before install: %v", err)
	}
	if v := s.Installed().Check(t.Context()); v.OK {
		t.Fatal("Installed before install")
	}
	if !s.Outdated(Build{Number: 1}) {
		t.Fatal("not outdated before install")
	}

	var logs bytes.Buffer
	s.log = slog.New(slog.NewTextHandler(&logs, nil))
	if err := s.Install(t.Context(), writeZip(t, t.TempDir(), fixture(1))); err != nil {
		t.Fatal(err)
	}

	m := s.Meta()
	if m == nil || m.Build != 1 || m.Schema != schemaHash || !m.ReleaseDate.Equal(time.Date(2026, 10, 2, 11, 8, 57, 0, time.UTC)) {
		t.Fatalf("meta = %+v", m)
	}
	if s.Outdated(Build{Number: 1}) || !s.Outdated(Build{Number: 2}) {
		t.Fatal("Outdated does not compare the build")
	}
	if v := s.Installed().Check(t.Context()); !v.OK {
		t.Fatal("not Installed after install")
	}

	for q, want := range map[string]any{
		`SELECT name_en FROM types WHERE key = 34`:                     "Tritanium",
		`SELECT name_de FROM types WHERE key = 34`:                     "Tritanium",
		`SELECT count(*) FROM types WHERE name_de IS NULL`:             int64(1),
		`SELECT published FROM types WHERE key = 34`:                   true,
		`SELECT group_concat(value) FROM map_solar_systems_planet_ids`: "40009077,40009078",
		`SELECT position_y FROM map_solar_systems`:                     2.0,
		`SELECT key FROM type_bonus_types WHERE parent = 587`:          int64(3330),
		`SELECT group_concat(v.bonus) FROM type_bonus_types_value v JOIN type_bonus_types t ON t.id = v.parent WHERE t.key = 3330`: "5.0,7.5",
		`SELECT bonus_text_en FROM type_bonus_types_value WHERE idx = 0`:                                                           "a",
		`SELECT unit_id FROM type_bonus_role_bonuses WHERE parent = 587`:                                                           int64(105),
	} {
		switch want := want.(type) {
		case string:
			if got := query[string](t, s, q); got != want {
				t.Errorf("%s = %q, want %q", q, got, want)
			}
		case int64:
			if got := query[int64](t, s, q); got != want {
				t.Errorf("%s = %d, want %d", q, got, want)
			}
		case float64:
			if got := query[float64](t, s, q); got != want {
				t.Errorf("%s = %v, want %v", q, got, want)
			}
		case bool:
			if got := query[bool](t, s, q); got != want {
				t.Errorf("%s = %v, want %v", q, got, want)
			}
		}
	}
	for _, want := range []string{"types.shiny", "notAFile.jsonl"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("no warning for %s in\n%s", want, logs.String())
		}
	}

	// A second install swaps the database under the open store.
	if err := s.Install(t.Context(), writeZip(t, t.TempDir(), fixture(2))); err != nil {
		t.Fatal(err)
	}
	if m := s.Meta(); m.Build != 2 {
		t.Fatalf("build after update = %d", m.Build)
	}
	if n := query[int64](t, s, `SELECT count(*) FROM types`); n != 2 {
		t.Fatalf("types after update = %d", n)
	}
}

func TestOpenCleansUp(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(t.Context(), dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Install(t.Context(), writeZip(t, t.TempDir(), fixture(3))); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if err := os.WriteFile(filepath.Join(dir, fileName+".tmp"), []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err = Open(t.Context(), dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if m := s.Meta(); m == nil || m.Build != 3 {
		t.Fatalf("meta after reopen = %+v", m)
	}
	if _, err := os.Stat(filepath.Join(dir, fileName+".tmp")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("tmp not removed: %v", err)
	}
}

func TestOpenRemovesBrokenDatabase(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte("not sqlite"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(t.Context(), dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Meta() != nil {
		t.Fatal("broken database opened")
	}
	if _, err := os.Stat(filepath.Join(dir, fileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("broken database not removed: %v", err)
	}
}

// TestInstallExport builds a real export: SDE_ZIP=/path/to/export.zip go test -run Export -v
func TestInstallExport(t *testing.T) {
	src := os.Getenv("SDE_ZIP")
	if src == "" {
		t.Skip("SDE_ZIP not set")
	}
	dir := t.TempDir()
	s, err := Open(t.Context(), dir, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	start := time.Now()
	if err := s.Install(t.Context(), src); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("build %d in %s, %d MB", s.Meta().Build, time.Since(start).Round(time.Second), fi.Size()>>20)
}
