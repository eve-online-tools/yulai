// Command gen infers the SDE schema from an export zip and writes schema.sql and
// tables_gen.go for package sde. Without -zip it downloads the latest export.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"log"
	"maps"
	"net/http"
	"os"
	"path"
	"slices"
	"strings"
)

const latestURL = "https://developers.eveonline.com/static-data/eve-online-static-data-latest-jsonl.zip"

// languages are the keys of localized objects.
var languages = map[string]bool{"de": true, "en": true, "es": true, "fr": true, "ja": true, "ko": true, "ru": true, "zh": true}

// domains maps id fields to the column types sqlc.yaml turns into lib-esi-go identifiers.
var domains = map[string]string{
	"typeID":        "TYPE_ID",
	"solarSystemID": "SOLAR_SYSTEM_ID",
	"stationID":     "STATION_ID",
	"characterID":   "CHARACTER_ID",
	"corporationID": "CORPORATION_ID",
	"allianceID":    "ALLIANCE_ID",
	"factionID":     "FACTION_ID",
	"bloodlineID":   "BLOODLINE_ID",
	"raceID":        "RACE_ID",
}

// keyDomains types the key of the files whose rows are those ids.
var keyDomains = map[string]string{
	"types":           "TYPE_ID",
	"mapSolarSystems": "SOLAR_SYSTEM_ID",
	"npcStations":     "STATION_ID",
	"npcCorporations": "CORPORATION_ID",
	"factions":        "FACTION_ID",
	"bloodlines":      "BLOODLINE_ID",
	"races":           "RACE_ID",
	"typeBonus":       "TYPE_ID",
	"typeDogma":       "TYPE_ID",
	"typeMaterials":   "TYPE_ID",
}

// skip holds files that are not data tables. _sde is the build record, read into meta.
var skip = map[string]bool{"_sde": true}

func main() {
	zipPath := flag.String("zip", "", "export zip; downloads the latest when empty")
	out := flag.String("out", ".", "package sde directory")
	flag.Parse()
	if err := run(*zipPath, *out); err != nil {
		log.Fatal(err)
	}
}

func run(zipPath, out string) error {
	if zipPath == "" {
		p, err := download()
		if err != nil {
			return err
		}
		defer os.Remove(p)
		zipPath = p
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	var tables []*table
	for _, f := range zr.File {
		file, ok := strings.CutSuffix(path.Base(f.Name), ".jsonl")
		if !ok || skip[file] {
			continue
		}
		root, err := infer(f)
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		ts, err := build(file, root)
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		tables = append(tables, ts...)
	}
	slices.SortStableFunc(tables, func(a, b *table) int { return strings.Compare(a.file, b.file) })
	if err := unique(tables); err != nil {
		return err
	}

	schema := schemaSQL(tables)
	sum := sha256.Sum256(schema)
	src, err := format.Source(goSource(tables, hex.EncodeToString(sum[:8])))
	if err != nil {
		return err
	}
	if err := os.WriteFile(path.Join(out, "schema.sql"), schema, 0o644); err != nil {
		return err
	}
	return os.WriteFile(path.Join(out, "tables_gen.go"), src, 0o644)
}

func download() (string, error) {
	resp, err := http.Get(latestURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: %s", resp.Status)
	}
	f, err := os.CreateTemp("", "sde-*.zip")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// node is what a JSON path held across all records of a file.
type node struct {
	ints, floats, strs, bools bool
	langs                     map[string]bool
	fields                    map[string]*node // objects
	elem                      *node            // arrays
}

func (n *node) scalar() bool { return n.ints || n.floats || n.strs || n.bools }

func infer(f *zip.File) (*node, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	dec := json.NewDecoder(r)
	dec.UseNumber()
	root := &node{}
	for {
		var rec map[string]any
		err := dec.Decode(&rec)
		if errors.Is(err, io.EOF) {
			return root, nil
		}
		if err != nil {
			return nil, err
		}
		if err := root.add(rec); err != nil {
			return nil, err
		}
	}
}

func (n *node) add(v any) error {
	switch v := v.(type) {
	case nil:
	case bool:
		n.bools = true
	case string:
		n.strs = true
	case json.Number:
		if strings.ContainsAny(string(v), ".eE") {
			n.floats = true
		} else {
			n.ints = true
		}
	case map[string]any:
		if localized(v) {
			if n.langs == nil {
				n.langs = map[string]bool{}
			}
			for l := range v {
				n.langs[l] = true
			}
			return nil
		}
		if n.fields == nil {
			n.fields = map[string]*node{}
		}
		for k, x := range v {
			c := n.fields[k]
			if c == nil {
				c = &node{}
				n.fields[k] = c
			}
			if err := c.add(x); err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
		}
	case []any:
		if n.elem == nil {
			n.elem = &node{}
		}
		for _, x := range v {
			if err := n.elem.add(x); err != nil {
				return fmt.Errorf("[]: %w", err)
			}
		}
	default:
		return fmt.Errorf("unexpected %T", v)
	}
	return nil
}

func localized(m map[string]any) bool {
	if len(m) == 0 {
		return false
	}
	for k, v := range m {
		if _, ok := v.(string); !ok || !languages[k] {
			return false
		}
	}
	return true
}

type table struct {
	name   string
	file   string
	parent *table
	path   []string // from the parent's record or element to the array
	key    string   // SQL type of key; "" for child tables keyed by position
	id     bool     // has child tables
	cols   []column
}

type column struct {
	name string
	path []string
	typ  string
	kind byte // i, f, s, b
}

func build(file string, root *node) ([]*table, error) {
	k := root.fields["_key"]
	if k == nil {
		return nil, errors.New("no _key")
	}
	t := &table{name: snake(file), file: file}
	switch {
	case k.ints && !k.strs && !k.floats:
		t.key = "INTEGER"
		if d := keyDomains[file]; d != "" {
			t.key = d
		}
	case k.strs && !k.ints && !k.floats:
		t.key = "TEXT"
	default:
		return nil, errors.New("_key is neither integer nor string")
	}
	tables := []*table{t}
	err := flatten(t, root, nil, &tables, map[string]bool{"_key": true})
	return tables, err
}

// flatten adds the columns and child tables of object n, found at prefix within t's rows.
func flatten(t *table, n *node, prefix []string, tables *[]*table, skip map[string]bool) error {
	for _, k := range slices.Sorted(maps.Keys(n.fields)) {
		if skip[k] {
			continue
		}
		if err := field(t, n.fields[k], append(slices.Clone(prefix), k), tables); err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
	}
	return nil
}

func field(t *table, n *node, p []string, tables *[]*table) error {
	kinds := 0
	for _, b := range []bool{n.scalar(), n.langs != nil, n.fields != nil, n.elem != nil} {
		if b {
			kinds++
		}
	}
	if kinds > 1 {
		return errors.New("mixes scalars, localized text, objects and arrays")
	}
	switch {
	case n.langs != nil:
		for _, l := range slices.Sorted(maps.Keys(n.langs)) {
			t.cols = append(t.cols, column{name: colName(p) + "_" + l, path: append(slices.Clone(p), l), typ: "TEXT", kind: 's'})
		}
	case n.fields != nil:
		return flatten(t, n, p, tables, nil)
	case n.elem != nil:
		return child(t, n.elem, p, tables)
	case n.scalar():
		c, err := scalar(n, p)
		if err != nil {
			return err
		}
		t.cols = append(t.cols, c)
	}
	// Only nulls seen: no column.
	return nil
}

func scalar(n *node, p []string) (column, error) {
	c := column{name: colName(p), path: p}
	switch {
	case n.strs && !n.ints && !n.floats && !n.bools:
		c.typ, c.kind = "TEXT", 's'
	case n.bools && !n.ints && !n.floats && !n.strs:
		c.typ, c.kind = "BOOLEAN", 'b'
	case n.floats && !n.strs && !n.bools:
		c.typ, c.kind = "REAL", 'f'
	case n.ints && !n.strs && !n.bools:
		c.typ, c.kind = "INTEGER", 'i'
		if len(p) > 0 && domains[p[len(p)-1]] != "" {
			c.typ = domains[p[len(p)-1]]
		}
	default:
		return c, errors.New("mixed scalar types")
	}
	return c, nil
}

// child adds a table for the array at p in t's rows, with elements like elem.
func child(t *table, elem *node, p []string, tables *[]*table) error {
	t.id = t.parent != nil || t.id
	c := &table{name: t.name + "_" + colName(p), file: t.file, parent: t, path: p}
	*tables = append(*tables, c)
	if elem.elem != nil {
		return errors.New("array of arrays")
	}
	if k := elem.fields["_key"]; k != nil {
		col, err := scalar(k, []string{"_key"})
		if err != nil {
			return fmt.Errorf("_key: %w", err)
		}
		c.key = col.typ
		// A {_key, _value} pair is a map entry; its value fields are columns of the entry.
		if v := elem.fields["_value"]; v != nil && len(elem.fields) == 2 {
			if v.fields != nil {
				return flatten(c, v, []string{"_value"}, tables, nil)
			}
			return valueField(c, v, []string{"_value"}, tables)
		}
		return flatten(c, elem, nil, tables, map[string]bool{"_key": true})
	}
	if elem.fields != nil {
		return flatten(c, elem, nil, tables, nil)
	}
	return valueField(c, elem, nil, tables)
}

// valueField adds a non-object element or map value as the "value" column(s).
func valueField(c *table, n *node, p []string, tables *[]*table) error {
	switch {
	case n.elem != nil:
		return child(c, n.elem, p, tables)
	case n.langs != nil:
		for _, l := range slices.Sorted(maps.Keys(n.langs)) {
			c.cols = append(c.cols, column{name: "value_" + l, path: append(slices.Clone(p), l), typ: "TEXT", kind: 's'})
		}
	default:
		col, err := scalar(n, p)
		if err != nil {
			return err
		}
		col.name = "value"
		c.cols = append(c.cols, col)
	}
	return nil
}

func colName(p []string) string {
	parts := make([]string, 0, len(p))
	for _, s := range p {
		if s == "_value" {
			s = "value"
		}
		parts = append(parts, snake(s))
	}
	return strings.Join(parts, "_")
}

// snake turns camelCase into snake_case, keeping ID and IDs as words.
func snake(s string) string {
	s = strings.ReplaceAll(s, "IDs", "Ids")
	s = strings.ReplaceAll(s, "ID", "Id")
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		upper := r >= 'A' && r <= 'Z'
		if upper && i > 0 {
			prev := rs[i-1]
			nextLower := i+1 < len(rs) && rs[i+1] >= 'a' && rs[i+1] <= 'z'
			if prev >= 'a' && prev <= 'z' || prev >= 'A' && prev <= 'Z' && nextLower {
				b.WriteByte('_')
			}
		}
		if upper {
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// fixed are the columns the converter adds before the data columns.
func fixed(t *table) []string {
	var out []string
	if t.parent == nil {
		return []string{"key"}
	}
	if t.id {
		out = append(out, "id")
	}
	out = append(out, "parent")
	if t.key != "" {
		return append(out, "key")
	}
	return append(out, "idx")
}

func unique(tables []*table) error {
	names := map[string]bool{"meta": true}
	for _, t := range tables {
		if names[t.name] {
			return fmt.Errorf("duplicate table %s", t.name)
		}
		names[t.name] = true
		cols := map[string]bool{}
		for _, c := range fixed(t) {
			cols[c] = true
		}
		for _, c := range t.cols {
			if cols[c.name] {
				return fmt.Errorf("%s: duplicate column %s", t.name, c.name)
			}
			cols[c.name] = true
		}
	}
	return nil
}

func parentType(t *table) string {
	p := t.parent
	if p.parent == nil {
		return p.key
	}
	return "INTEGER"
}

func schemaSQL(tables []*table) []byte {
	var b bytes.Buffer
	b.WriteString(`-- Code generated by core/sde/gen. DO NOT EDIT.
-- The SDE database: one table per export file, child tables for arrays. See docs/SDE.md.

CREATE TABLE meta (
    build        INTEGER   NOT NULL,
    release_date TIMESTAMP NOT NULL,
    schema       TEXT      NOT NULL
);
`)
	for _, t := range tables {
		var lines []string
		if t.parent == nil {
			lines = append(lines, fmt.Sprintf("%q %s PRIMARY KEY", "key", t.key))
		} else {
			if t.id {
				lines = append(lines, `"id" INTEGER PRIMARY KEY`)
			}
			lines = append(lines, fmt.Sprintf(`"parent" %s NOT NULL`, parentType(t)))
			if t.key != "" {
				lines = append(lines, fmt.Sprintf(`"key" %s NOT NULL`, t.key))
			} else {
				lines = append(lines, `"idx" INTEGER NOT NULL`)
			}
		}
		for _, c := range t.cols {
			lines = append(lines, fmt.Sprintf("%q %s", c.name, c.typ))
		}
		fmt.Fprintf(&b, "\nCREATE TABLE %q (\n    %s\n);\n", t.name, strings.Join(lines, ",\n    "))
	}
	// The converter creates indexes after the inserts.
	b.WriteString("\n")
	for _, t := range tables {
		if t.parent != nil {
			fmt.Fprintf(&b, "CREATE INDEX %q ON %q (\"parent\");\n", t.name+"_parent", t.name)
		}
	}
	return b.Bytes()
}

func goSource(tables []*table, hash string) []byte {
	index := map[*table]int{}
	for i, t := range tables {
		index[t] = i
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, `// Code generated by core/sde/gen. DO NOT EDIT.

package sde

// schemaHash identifies schema.sql. A database built with another schema is rebuilt.
const schemaHash = %q

var tables = []table{
`, hash)
	for _, t := range tables {
		parent := -1
		if t.parent != nil {
			parent = index[t.parent]
		}
		fmt.Fprintf(&b, "{name: %q, file: %q, parent: %d, path: %#v, keyed: %t, id: %t, cols: []column{\n",
			t.name, t.file, parent, t.path, t.key != "", t.id)
		for _, c := range t.cols {
			fmt.Fprintf(&b, "{name: %q, path: %#v, kind: %q},\n", c.name, c.path, c.kind)
		}
		b.WriteString("}},\n")
	}
	b.WriteString("}\n")
	return b.Bytes()
}
