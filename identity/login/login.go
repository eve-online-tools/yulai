// Package login runs the interactive login in the system browser. A loopback HTTP
// server lives for the whole app lifetime: it shows the feature picker, sends the
// browser to the SSO and finishes the PKCE flow on the callback. Pages are the
// apps/webserver build; each response is its index.html with the page data inlined.
package login

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/eve-online-tools/yulai/identity/sso"
)

// maxAttempts bounds memory. Attempts have no timeout: the user may take as long as
// they like at the SSO.
const maxAttempts = 64

// finishTimeout bounds exchange, verify and the handler. It is detached from the
// request so a closed tab does not abort a half-finished login.
const finishTimeout = 30 * time.Second

// The empty data element in apps/webserver/index.html.
const (
	pageDataOpen  = `<script type="application/json" id="page-data">`
	pageDataClose = `</script>`
)

var pageDataSlot = []byte(pageDataOpen + pageDataClose)

type Result struct {
	sso.Identity
	sso.Tokens
}

// Feature is one choice on the picker.
type Feature struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

// Page data, matching PageData in apps/webserver/src/page-data.ts.
type pickerPage struct {
	Page     string    `json:"page"`
	CSRF     string    `json:"csrf"`
	Features []Feature `json:"features"`
}

// donePage carries the previous choice so "add another" can post straight to /start.
type donePage struct {
	Page     string   `json:"page"`
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	CSRF     string   `json:"csrf"`
	Features []string `json:"features"`
}

type errorPage struct {
	Page    string `json:"page"`
	Message string `json:"message"`
}

// Handler receives each completed login. An error is shown to the user.
type Handler func(ctx context.Context, r *Result) error

type attempt struct {
	pkce     *sso.PKCE
	scopes   []string
	features []string
	busy     bool
	// name is set once the login completed.
	name string
	id   int64
}

type Server struct {
	client   *sso.Client
	verifier *sso.Verifier
	features []Feature
	onLogin  Handler

	callback *url.URL
	hosts    []string
	csrf     string
	index    []byte
	handler  http.Handler

	mu       sync.Mutex
	attempts map[string]*attempt
	order    []string
	servers  []*http.Server
}

// New validates the callback URL. It must be http on a loopback host with an explicit port.
// web is the apps/webserver build.
func New(client *sso.Client, verifier *sso.Verifier, web fs.FS, features []Feature, onLogin Handler) (*Server, error) {
	cb, err := url.Parse(client.Config().CallbackURL)
	if err != nil {
		return nil, fmt.Errorf("login: bad callback url: %w", err)
	}
	host, port := cb.Hostname(), cb.Port()
	if cb.Scheme != "http" || port == "" || !isLoopback(host) {
		return nil, fmt.Errorf("login: callback url %q must be http://localhost:<port>/...", cb)
	}
	if cb.Path == "" || cb.Path == "/" || cb.Path == "/start" || cb.Path == "/done" {
		return nil, fmt.Errorf("login: callback url %q needs its own path", cb)
	}
	index, err := fs.ReadFile(web, "index.html")
	if err != nil {
		return nil, fmt.Errorf("login: webserver build: %w", err)
	}
	if !bytes.Contains(index, pageDataSlot) {
		return nil, errors.New("login: webserver index.html has no page-data element")
	}
	csrf, err := randomToken()
	if err != nil {
		return nil, err
	}

	s := &Server{
		client:   client,
		verifier: verifier,
		features: features,
		onLogin:  onLogin,
		callback: cb,
		csrf:     csrf,
		index:    index,
		attempts: map[string]*attempt{},
	}
	for _, h := range []string{"localhost", "127.0.0.1", "::1"} {
		s.hosts = append(s.hosts, net.JoinHostPort(h, port))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.picker)
	mux.HandleFunc("POST /start", s.start)
	mux.HandleFunc("GET "+cb.Path, s.finish)
	mux.HandleFunc("GET /done", s.done)
	mux.Handle("GET /assets/", http.FileServerFS(web))
	s.handler = s.guard(mux)
	return s, nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// URL is the picker page, the entry point the app opens in the browser.
func (s *Server) URL() string {
	return (&url.URL{Scheme: "http", Host: s.callback.Host, Path: "/"}).String()
}

func (s *Server) Handler() http.Handler { return s.handler }

// Listen binds the callback port and serves until Close. "localhost" binds both
// loopback families because browsers may resolve it to either. A busy port is an
// error; a missing IPv6 stack is not.
func (s *Server) Listen() error {
	host, port := s.callback.Hostname(), s.callback.Port()
	addrs := []string{net.JoinHostPort(host, port)}
	if host == "localhost" {
		addrs = []string{net.JoinHostPort("127.0.0.1", port)}
		if hasIPv6Loopback() {
			addrs = append(addrs, net.JoinHostPort("::1", port))
		}
	}

	var lns []net.Listener
	for _, addr := range addrs {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			for _, l := range lns {
				l.Close()
			}
			return fmt.Errorf("login: cannot listen on %s (is another instance running?): %w", addr, err)
		}
		lns = append(lns, ln)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ln := range lns {
		srv := &http.Server{Handler: s.handler, ReadHeaderTimeout: 10 * time.Second}
		s.servers = append(s.servers, srv)
		go func() {
			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("login server", "addr", ln.Addr(), "err", err)
			}
		}()
	}
	return nil
}

func hasIPv6Loopback() bool {
	ln, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

func (s *Server) Close() error {
	s.mu.Lock()
	servers := s.servers
	s.servers = nil
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var errs []error
	for _, srv := range servers {
		errs = append(errs, srv.Shutdown(ctx))
	}
	return errors.Join(errs...)
}

// guard rejects foreign Host headers (DNS rebinding) and sets headers common to every page.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !slices.Contains(s.hosts, r.Host) {
			http.Error(w, "unexpected host", http.StatusMisdirectedRequest)
			return
		}
		h := w.Header()
		h.Set("Cache-Control", "no-store")
		h.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data: https://images.evetech.net; frame-ancestors 'none'")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// render inlines data into index.html. json.Marshal escapes <, > and &, so the data
// cannot close the script element.
func (s *Server) render(w http.ResponseWriter, status int, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		slog.Error("login: render", "err", err)
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	filled := slices.Concat([]byte(pageDataOpen), b, []byte(pageDataClose))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	w.Write(bytes.Replace(s.index, pageDataSlot, filled, 1))
}

func (s *Server) fail(w http.ResponseWriter, status int, msg string) {
	s.render(w, status, errorPage{Page: "error", Message: msg})
}

func (s *Server) picker(w http.ResponseWriter, r *http.Request) {
	features := s.features
	if features == nil {
		features = []Feature{}
	}
	s.render(w, http.StatusOK, pickerPage{Page: "picker", CSRF: s.csrf, Features: features})
}

func (s *Server) start(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.fail(w, http.StatusBadRequest, "Could not read the form.")
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf")), []byte(s.csrf)) != 1 {
		s.fail(w, http.StatusForbidden, "This form has expired. Start again.")
		return
	}
	if s.client.Config().ClientID == "" {
		s.fail(w, http.StatusServiceUnavailable, "No SSO application is configured. See the README.")
		return
	}
	names := r.PostForm["feature"]
	scopes, err := s.scopesFor(names)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err.Error())
		return
	}

	p, err := sso.NewPKCE()
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "Could not start the login.")
		return
	}
	authURL, err := s.client.AuthorizeURL(r.Context(), p, scopes)
	if err != nil {
		slog.Warn("login: authorize url", "err", err)
		s.fail(w, http.StatusBadGateway, "Could not reach EVE SSO. Check your connection and try again.")
		return
	}
	s.add(p.State, &attempt{pkce: p, scopes: scopes, features: names})
	http.Redirect(w, r, authURL, http.StatusSeeOther)
}

func (s *Server) scopesFor(names []string) ([]string, error) {
	var scopes []string
	for _, name := range names {
		i := slices.IndexFunc(s.features, func(f Feature) bool { return f.Name == name })
		if i < 0 {
			return nil, fmt.Errorf("Unknown feature %q.", name)
		}
		scopes = append(scopes, s.features[i].Scopes...)
	}
	slices.Sort(scopes)
	return slices.Compact(scopes), nil
}

func (s *Server) add(state string, a *attempt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.order) >= maxAttempts {
		delete(s.attempts, s.order[0])
		s.order = s.order[1:]
	}
	s.attempts[state] = a
	s.order = append(s.order, state)
}

func (s *Server) remove(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.attempts, state)
	if i := slices.Index(s.order, state); i >= 0 {
		s.order = slices.Delete(s.order, i, i+1)
	}
}

func (s *Server) finish(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	state := q.Get("state")
	doneURL := "/done?" + url.Values{"state": {state}}.Encode()

	s.mu.Lock()
	a := s.attempts[state]
	var pkce *sso.PKCE
	switch {
	case a == nil:
	case a.name != "":
	case a.busy:
	default:
		a.busy = true
		pkce = a.pkce
	}
	s.mu.Unlock()

	switch {
	case a == nil:
		s.fail(w, http.StatusBadRequest, "This login attempt is unknown or has expired. Start again.")
		return
	case a.name != "":
		http.Redirect(w, r, doneURL, http.StatusSeeOther)
		return
	case pkce == nil:
		s.fail(w, http.StatusConflict, "This login is already being processed.")
		return
	}

	if e := q.Get("error"); e != "" {
		s.remove(state)
		if e == "access_denied" {
			s.fail(w, http.StatusOK, "The login was cancelled.")
			return
		}
		slog.Warn("login: sso error", "error", e, "description", q.Get("error_description"))
		s.fail(w, http.StatusBadGateway, "EVE SSO reported an error: "+e)
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), finishTimeout)
	defer cancel()
	res, err := s.complete(ctx, q.Get("code"), pkce)
	if err != nil {
		s.remove(state)
		slog.Warn("login: complete", "err", err)
		s.fail(w, http.StatusBadGateway, "The login could not be completed: "+err.Error())
		return
	}

	s.mu.Lock()
	a.name, a.id = res.Name, res.CharacterID
	a.pkce = nil
	s.mu.Unlock()
	http.Redirect(w, r, doneURL, http.StatusSeeOther)
}

func (s *Server) complete(ctx context.Context, code string, p *sso.PKCE) (*Result, error) {
	if code == "" {
		return nil, errors.New("no authorization code")
	}
	tokens, err := s.client.Exchange(ctx, code, p)
	if err != nil {
		return nil, err
	}
	id, err := s.verifier.Verify(ctx, tokens.AccessToken)
	if err != nil {
		return nil, err
	}
	res := &Result{Identity: *id, Tokens: *tokens}
	if err := s.onLogin(ctx, res); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Server) done(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	page := donePage{Page: "done", CSRF: s.csrf, Features: []string{}}
	if a := s.attempts[r.URL.Query().Get("state")]; a != nil {
		page.Name, page.ID = a.name, a.id
		if a.features != nil {
			page.Features = a.features
		}
	}
	s.mu.Unlock()
	if page.Name == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, http.StatusOK, page)
}

func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
