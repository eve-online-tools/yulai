package sde

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/eve-online-tools/yulai/core/task"
)

const EventChanged = "sde:changed"

const (
	baseURL   = "https://developers.eveonline.com/static-data/tranquility/"
	userAgent = "yulai (+https://github.com/eve-online-tools/yulai)"
)

type Emitter interface {
	Emit(name string)
}

// Updater keeps the store at the latest SDE build.
type Updater struct {
	store  *Store
	http   *http.Client
	events Emitter
	base   string

	mu     sync.Mutex
	etag   string
	latest *Build
}

func NewUpdater(store *Store, client *http.Client, events Emitter) *Updater {
	return &Updater{
		store:  store,
		http:   client,
		events: events,
		base:   baseURL,
	}
}

func (u *Updater) Tasks() []task.Binding {
	return []task.Binding{
		Check.Bind(u),
		Update.Bind(u),
	}
}

// Latest is the build the last check saw, nil before the first check.
func (u *Updater) Latest() *Build {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.latest
}

var Check = task.New((*Updater).Check,
	task.WithStartup(),
	task.WithInterval(time.Hour),
	task.WithTimeout(time.Minute),
	task.Pausable("sde.check"),
)

// Check reads the latest build and queues Update when it is not installed.
func (u *Updater) Check(ctx context.Context, _ struct{}) (*Build, error) {
	b, err := u.fetchLatest(ctx)
	if err != nil {
		return nil, err
	}
	if !u.store.Outdated(*b) {
		return b, nil
	}
	if err := Update.Queue(ctx, *b); err != nil {
		return nil, err
	}
	return b, nil
}

func (u *Updater) fetchLatest(ctx context.Context) (*Build, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.base+"latest.jsonl", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	u.mu.Lock()
	if u.latest != nil && u.etag != "" {
		req.Header.Set("If-None-Match", u.etag)
	}
	u.mu.Unlock()

	resp, err := u.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return u.Latest(), nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sde: latest: %s", resp.Status)
	}
	b, err := parseLatest(resp.Body)
	if err != nil {
		return nil, err
	}
	u.mu.Lock()
	u.etag, u.latest = resp.Header.Get("ETag"), b
	u.mu.Unlock()
	return b, nil
}

// parseLatest finds the "sde" record in latest.jsonl.
func parseLatest(r io.Reader) (*Build, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		var rec struct {
			Key string `json:"_key"`
			Build
		}
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			return nil, fmt.Errorf("sde: latest: %w", err)
		}
		if rec.Key == "sde" {
			return &rec.Build, nil
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("sde: latest: no sde record")
}

var Update = task.New((*Updater).Update,
	task.WithTimeout(30*time.Minute),
)

// Update downloads build b and installs it. An interrupted download resumes on
// the next run.
func (u *Updater) Update(ctx context.Context, b Build) (*Meta, error) {
	if !u.store.Outdated(b) {
		return u.store.Meta(), nil
	}
	zipPath := filepath.Join(u.store.dir, fmt.Sprintf("sde-%d.zip", b.Number))
	if err := u.removeZips(filepath.Base(zipPath)); err != nil {
		return nil, err
	}
	if _, err := os.Stat(zipPath); errors.Is(err, os.ErrNotExist) {
		if err := u.download(ctx, b, zipPath); err != nil {
			return nil, err
		}
	} else {
		report(ctx, "download", "", 0, downloaded, 1, 1)
	}
	if err := u.store.Install(ctx, zipPath); err != nil {
		return nil, err
	}
	if err := os.Remove(zipPath); err != nil {
		return nil, err
	}
	u.events.Emit(EventChanged)
	return u.store.Meta(), nil
}

func (u *Updater) download(ctx context.Context, b Build, dst string) error {
	part := dst + ".part"
	f, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	offset, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%seve-online-static-data-%d-jsonl.zip", u.base, b.Number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	// A build's zip never changes, so resuming needs no If-Range.
	if offset > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
	}
	resp, err := u.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusPartialContent:
	case http.StatusOK:
		offset = 0
		if err := f.Truncate(0); err != nil {
			return err
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
	default:
		// E.g. 416 for a part that does not fit the file: the next run starts over.
		if offset > 0 {
			if err := f.Truncate(0); err != nil {
				return err
			}
		}
		return fmt.Errorf("sde: download: %s", resp.Status)
	}

	size := offset + resp.ContentLength
	if resp.StatusCode == http.StatusPartialContent {
		size = rangeSize(resp)
	}
	done := offset
	report(ctx, "download", "", 0, downloaded, done, size)
	buf := make([]byte, 256<<10)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return err
			}
			done += int64(n)
			report(ctx, "download", "", 0, downloaded, done, size)
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
	}
	if size > 0 && done != size {
		return fmt.Errorf("sde: download: got %d of %d bytes", done, size)
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(part, dst)
}

// rangeSize reads the full size from a "bytes a-b/size" Content-Range, -1 if absent.
func rangeSize(resp *http.Response) int64 {
	_, size, ok := strings.Cut(resp.Header.Get("Content-Range"), "/")
	if !ok {
		return -1
	}
	n, err := strconv.ParseInt(size, 10, 64)
	if err != nil {
		return -1
	}
	return n
}

// removeZips deletes downloads other than keep and its .part.
func (u *Updater) removeZips(keep string) error {
	entries, err := os.ReadDir(u.store.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		n := e.Name()
		if n == keep || n == keep+".part" || !strings.HasPrefix(n, "sde-") || !strings.Contains(n, ".zip") {
			continue
		}
		if err := os.Remove(filepath.Join(u.store.dir, n)); err != nil {
			return err
		}
	}
	return nil
}
