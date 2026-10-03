package sde

import (
	"archive/zip"
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	"github.com/eve-online-tools/yulai/core/task"
)

//go:embed schema.sql
var schemaSQL string

// reportEvery throttles build progress to one report per this many bytes read.
const reportEvery = 1 << 20

// Build is an SDE release as latest.jsonl and _sde.jsonl describe it.
type Build struct {
	Number      int64     `json:"buildNumber"`
	ReleaseDate time.Time `json:"releaseDate"`
}

// build fills a new database at dst from the export zip at src.
func build(ctx context.Context, src, dst string, log *slog.Logger) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()

	if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// A crash leaves a broken file behind, which Open deletes, so durability is not needed.
	conn, err := sql.Open("sqlite", dst+"?_pragma=journal_mode(OFF)&_pragma=synchronous(OFF)")
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetMaxOpenConns(1)

	creates, indexes := statements(schemaSQL)
	for _, stmt := range creates {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}

	b := &builder{log: log, warned: map[string]bool{}}
	for _, f := range zr.File {
		b.total += int64(f.UncompressedSize64)
	}
	var release *Build
	for _, f := range zr.File {
		file, ok := strings.CutSuffix(path.Base(f.Name), ".jsonl")
		switch {
		case !ok:
		case file == "_sde":
			release, err = readRelease(f)
		case len(files[file]) == 0:
			log.Warn("sde: file not in schema, skipped", "file", f.Name)
		default:
			err = b.file(ctx, conn, f, file)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		b.done += int64(f.UncompressedSize64)
		b.read = 0
	}
	if release == nil {
		return errors.New("sde: export has no _sde.jsonl")
	}

	for i, stmt := range indexes {
		task.Report(ctx, task.Progress{Phase: "index", Done: int64(i), Total: int64(len(indexes))})
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	_, err = conn.ExecContext(ctx, `INSERT INTO meta (build, release_date, schema) VALUES (?, ?, ?)`,
		release.Number, release.ReleaseDate.UTC(), schemaHash)
	if err != nil {
		return err
	}
	return conn.Close()
}

// statements splits schema.sql into table and index statements.
func statements(schema string) (creates, indexes []string) {
	for stmt := range strings.SplitSeq(schema, ";\n") {
		var lines []string
		for line := range strings.SplitSeq(stmt, "\n") {
			if !strings.HasPrefix(line, "--") {
				lines = append(lines, line)
			}
		}
		stmt = strings.TrimSpace(strings.Join(lines, "\n"))
		switch {
		case stmt == "":
		case strings.HasPrefix(stmt, "CREATE INDEX"):
			indexes = append(indexes, stmt)
		default:
			creates = append(creates, stmt)
		}
	}
	return creates, indexes
}

func readRelease(f *zip.File) (*Build, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var b Build
	if err := json.NewDecoder(r).Decode(&b); err != nil {
		return nil, err
	}
	return &b, nil
}

type builder struct {
	log *slog.Logger
	// done counts bytes of finished files, read those of the current one.
	done, read, total int64
	reported          int64
	warned            map[string]bool
}

// count adds the bytes of the current file and reports progress.
func (b *builder) count(ctx context.Context, item string, n int) {
	b.read += int64(n)
	if b.read-b.reported < reportEvery {
		return
	}
	b.reported = b.read
	task.Report(ctx, task.Progress{Phase: "build", Item: item, Done: b.done + b.read, Total: b.total})
}

type countingReader struct {
	r    io.Reader
	read func(n int)
}

func (c countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.read(n)
	return n, err
}

// insert holds the statement and next id of one table while a file is loaded.
type insert struct {
	t      *table
	stmt   *sql.Stmt
	nextID int64
	kids   []*insert
	known  *known
}

func (b *builder) file(ctx context.Context, conn *sql.DB, f *zip.File, file string) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	byTable := map[int]*insert{}
	var top *insert
	for _, i := range files[file] {
		t := &tables[i]
		stmt, err := tx.PrepareContext(ctx, insertSQL(t))
		if err != nil {
			return err
		}
		defer stmt.Close()
		in := &insert{t: t, stmt: stmt, known: knownPaths(i)}
		byTable[i] = in
		if t.parent < 0 {
			top = in
		} else {
			byTable[t.parent].kids = append(byTable[t.parent].kids, in)
		}
	}

	b.reported = 0
	dec := json.NewDecoder(countingReader{r, func(n int) { b.count(ctx, f.Name, n) }})
	dec.UseNumber()
	for {
		var rec map[string]any
		err := dec.Decode(&rec)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		b.unknown(top, rec, nil)
		if err := b.row(ctx, top, rec, rec["_key"], nil); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// row inserts v into in's table and recurses into its child tables.
// parent and pos are nil for a top table row.
func (b *builder) row(ctx context.Context, in *insert, v any, parent any, pos any) error {
	args := make([]any, 0, len(in.t.cols)+3)
	ref := parent
	if in.t.parent >= 0 {
		if in.t.id {
			in.nextID++
			ref = in.nextID
			args = append(args, ref)
		}
		args = append(args, parent, pos)
	} else {
		args = append(args, number(parent))
	}
	for _, c := range in.t.cols {
		args = append(args, b.value(in.t, c, lookup(v, c.path)))
	}
	if _, err := in.stmt.ExecContext(ctx, args...); err != nil {
		return fmt.Errorf("%s: %w", in.t.name, err)
	}
	for _, kid := range in.kids {
		arr, _ := lookup(v, kid.t.path).([]any)
		for i, el := range arr {
			pos := any(i)
			if kid.t.keyed {
				m, _ := el.(map[string]any)
				pos = number(m["_key"])
			}
			if err := b.row(ctx, kid, el, ref, pos); err != nil {
				return err
			}
		}
	}
	return nil
}

func insertSQL(t *table) string {
	var cols []string
	if t.parent < 0 {
		cols = append(cols, "key")
	} else {
		if t.id {
			cols = append(cols, "id")
		}
		cols = append(cols, "parent")
		if t.keyed {
			cols = append(cols, "key")
		} else {
			cols = append(cols, "idx")
		}
	}
	for _, c := range t.cols {
		cols = append(cols, c.name)
	}
	return fmt.Sprintf(`INSERT INTO %q ("%s") VALUES (?%s)`,
		t.name, strings.Join(cols, `", "`), strings.Repeat(", ?", len(cols)-1))
}

func lookup(v any, p []string) any {
	for _, k := range p {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

// number turns a json.Number key into an int64 when it is one.
func number(v any) any {
	if n, ok := v.(json.Number); ok {
		if i, err := n.Int64(); err == nil {
			return i
		}
	}
	return v
}

// value converts v to the column's kind. A value of another kind is stored as NULL.
func (b *builder) value(t *table, c column, v any) any {
	if v == nil {
		return nil
	}
	var out any
	switch n, _ := v.(json.Number); c.kind {
	case 'i':
		if i, err := n.Int64(); err == nil {
			out = i
		}
	case 'f':
		if f, err := n.Float64(); err == nil {
			out = f
		}
	case 's':
		out, _ = v.(string)
	case 'b':
		out, _ = v.(bool)
	}
	if out == nil {
		b.warn(t.name+"."+c.name, "sde: value does not match the column type, stored as NULL", "value", v)
	}
	return out
}

func (b *builder) warn(key, msg string, args ...any) {
	if b.warned[key] {
		return
	}
	b.warned[key] = true
	b.log.Warn(msg, append([]any{"field", key}, args...)...)
}

// known is the tree of JSON paths a table reads. Leaves are columns; array
// nodes are read by a child table.
type known struct {
	keys  map[string]*known
	array bool
}

func knownPaths(i int) *known {
	t := &tables[i]
	root := &known{keys: map[string]*known{"_key": nil}}
	for _, c := range t.cols {
		root.add(c.path)
	}
	for j := range tables {
		if tables[j].parent == i {
			root.add(tables[j].path).array = true
		}
	}
	return root
}

func (k *known) add(p []string) *known {
	for _, s := range p {
		if k.keys == nil {
			k.keys = map[string]*known{}
		}
		next := k.keys[s]
		if next == nil {
			next = &known{}
			k.keys[s] = next
		}
		k = next
	}
	return k
}

// unknown logs, once per path, fields of v that no column or child table reads.
func (b *builder) unknown(in *insert, v any, prefix []string) {
	b.walk(in, in.known, v, prefix)
}

func (b *builder) walk(in *insert, k *known, v any, p []string) {
	m, ok := v.(map[string]any)
	if !ok || k == nil || k.keys == nil {
		return
	}
	for key, x := range m {
		next, ok := k.keys[key]
		if !ok {
			b.warn(in.t.name+"."+strings.Join(append(p, key), "."), "sde: field not in schema, dropped")
			continue
		}
		if next == nil {
			continue // _key
		}
		if next.array {
			kid := in.kidAt(append(p, key))
			arr, _ := x.([]any)
			for _, el := range arr {
				if kid != nil {
					b.walk(kid, kid.known, el, nil)
				}
			}
			continue
		}
		b.walk(in, next, x, append(p, key))
	}
}

func (in *insert) kidAt(p []string) *insert {
	for _, kid := range in.kids {
		if strings.Join(kid.t.path, ".") == strings.Join(p, ".") {
			return kid
		}
	}
	return nil
}

// files maps an export file name to its tables, top table first.
var files = func() map[string][]int {
	m := map[string][]int{}
	for i, t := range tables {
		m[t.file] = append(m[t.file], i)
	}
	return m
}()
