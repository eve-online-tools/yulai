package login

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"

	"github.com/eve-online-tools/yulai/identity/sso"
)

const clientID = "test-client"

// fakeSSO serves discovery, JWKS and a token endpoint that accepts the code "good".
type fakeSSO struct {
	*httptest.Server
	exchanges atomic.Int32
}

func newFakeSSO(t *testing.T) *fakeSSO {
	t.Helper()
	raw, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := jwk.Import(raw)
	if err != nil {
		t.Fatal(err)
	}
	_ = priv.Set(jwk.KeyIDKey, "k1")
	_ = priv.Set(jwk.AlgorithmKey, jwa.RS256())
	pub, err := jwk.PublicKeyOf(priv)
	if err != nil {
		t.Fatal(err)
	}
	set := jwk.NewSet()
	_ = set.AddKey(pub)

	f := &fakeSSO{}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 f.URL,
			"authorization_endpoint": f.URL + "/v2/oauth/authorize",
			"token_endpoint":         f.URL + "/v2/oauth/token",
			"jwks_uri":               f.URL + "/oauth/jwks",
		})
	})
	mux.HandleFunc("/oauth/jwks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(set)
	})
	mux.HandleFunc("/v2/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		f.exchanges.Add(1)
		_ = r.ParseForm()
		if r.PostForm.Get("code") != "good" || r.PostForm.Get("code_verifier") == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		tok, _ := jwt.NewBuilder().
			Issuer(f.URL).
			Subject("CHARACTER:EVE:9000").
			IssuedAt(time.Now()).
			Expiration(time.Now().Add(20*time.Minute)).
			Claim("azp", clientID).
			Claim("name", "Test <Pilot>").
			Claim("owner", "owner-hash").
			Claim("scp", []string{"esi-assets.read_assets.v1"}).
			Build()
		signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256(), priv))
		if err != nil {
			t.Error(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  string(signed),
			"refresh_token": "refresh",
			"expires_in":    1199,
		})
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

type harness struct {
	sso    *fakeSSO
	srv    *Server
	ts     *httptest.Server
	http   *http.Client
	got    chan *Result
	failOn atomic.Bool
}

// web stands in for the apps/webserver build.
var web = fstest.MapFS{
	"index.html":    {Data: []byte(`<!doctype html><div id="root"></div>` + pageDataOpen + pageDataClose)},
	"assets/app.js": {Data: []byte(`console.log("app")`)},
}

var features = []Feature{
	{Name: "Assets", Scopes: []string{"esi-assets.read_assets.v1"}},
	{Name: "Wallet", Scopes: []string{"esi-wallet.read_character_wallet.v1", "esi-assets.read_assets.v1"}},
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{sso: newFakeSSO(t), got: make(chan *Result, 4)}

	h.ts = httptest.NewUnstartedServer(nil)
	client := sso.NewClient(sso.Config{
		ClientID:    clientID,
		CallbackURL: "http://" + h.ts.Listener.Addr().String() + "/callback",
		Issuer:      h.sso.URL,
	})
	srv, err := New(client, sso.NewVerifier(client), web, features, func(ctx context.Context, r *Result) error {
		if h.failOn.Load() {
			return errors.New("store failed")
		}
		h.got <- r
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	h.srv = srv
	h.ts.Config.Handler = srv.Handler()
	h.ts.Start()
	t.Cleanup(h.ts.Close)

	h.http = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return h
}

func (h *harness) do(t *testing.T, req *http.Request) (*http.Response, string) {
	t.Helper()
	resp, err := h.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func (h *harness) get(t *testing.T, path string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, h.ts.URL+path, nil)
	return h.do(t, req)
}

func (h *harness) post(t *testing.T, path string, form url.Values) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, h.ts.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return h.do(t, req)
}

var csrfRe = regexp.MustCompile(`"csrf":"([^"]+)"`)

// begin loads the picker, submits it and returns the state sent to the SSO.
func (h *harness) begin(t *testing.T, feats ...string) (state string, authorize *url.URL) {
	t.Helper()
	resp, body := h.get(t, "/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("picker: %d", resp.StatusCode)
	}
	m := csrfRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatal("picker has no csrf token")
	}
	resp, body = h.post(t, "/start", url.Values{"csrf": {m[1]}, "feature": feats})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("start: %d %s", resp.StatusCode, body)
	}
	u, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get("state"), u
}

func TestFullFlow(t *testing.T) {
	h := newHarness(t)

	state, authorize := h.begin(t, "Assets", "Wallet")
	if !strings.HasPrefix(authorize.String(), h.sso.URL+"/v2/oauth/authorize?") {
		t.Fatalf("redirected to %s", authorize)
	}
	q := authorize.Query()
	if got, want := q.Get("scope"), "esi-assets.read_assets.v1 esi-wallet.read_character_wallet.v1"; got != want {
		t.Errorf("scope = %q, want %q", got, want)
	}
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" || q.Get("redirect_uri") != h.ts.URL+"/callback" {
		t.Errorf("authorize params: %v", q)
	}

	resp, body := h.get(t, "/callback?"+url.Values{"code": {"good"}, "state": {state}}.Encode())
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("callback: %d %s", resp.StatusCode, body)
	}
	doneURL := resp.Header.Get("Location")

	r := <-h.got
	if r.CharacterID != 9000 || r.OwnerHash != "owner-hash" || r.RefreshToken != "refresh" {
		t.Errorf("result: %+v", r)
	}

	resp, body = h.get(t, doneURL)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"name":"Test \u003cPilot\u003e"`) {
		t.Errorf("done: %d %s", resp.StatusCode, body)
	}
	// "Add another" reposts the same choice to /start.
	if !strings.Contains(body, `"features":["Assets","Wallet"]`) || !strings.Contains(body, `"csrf":"`+h.srv.csrf+`"`) {
		t.Errorf("done page lacks previous choice: %s", body)
	}

	// A reload of the callback must not replay the used code.
	resp, _ = h.get(t, "/callback?"+url.Values{"code": {"good"}, "state": {state}}.Encode())
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != doneURL {
		t.Errorf("callback reload: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	if n := h.sso.exchanges.Load(); n != 1 {
		t.Errorf("exchanges = %d, want 1", n)
	}
}

func TestNoFeatures(t *testing.T) {
	h := newHarness(t)
	_, authorize := h.begin(t)
	if s := authorize.Query().Get("scope"); s != "" {
		t.Errorf("scope = %q, want empty", s)
	}
}

func TestRejectsForeignHost(t *testing.T) {
	h := newHarness(t)
	req, _ := http.NewRequest(http.MethodGet, h.ts.URL+"/", nil)
	req.Host = "evil.example:" + h.srv.callback.Port()
	if resp, _ := h.do(t, req); resp.StatusCode != http.StatusMisdirectedRequest {
		t.Errorf("status = %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, h.ts.URL+"/", nil)
	req.Host = "localhost:" + h.srv.callback.Port()
	if resp, _ := h.do(t, req); resp.StatusCode != http.StatusOK {
		t.Errorf("localhost status = %d", resp.StatusCode)
	}
}

func TestStartRejects(t *testing.T) {
	h := newHarness(t)
	if resp, _ := h.post(t, "/start", url.Values{"csrf": {"wrong"}}); resp.StatusCode != http.StatusForbidden {
		t.Errorf("bad csrf: %d", resp.StatusCode)
	}
	if resp, _ := h.post(t, "/start", url.Values{"csrf": {h.srv.csrf}, "feature": {"Nope"}}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown feature: %d", resp.StatusCode)
	}

	h.srv.client = sso.NewClient(sso.Config{CallbackURL: h.srv.callback.String(), Issuer: h.sso.URL})
	if resp, _ := h.post(t, "/start", url.Values{"csrf": {h.srv.csrf}}); resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("no client id: %d", resp.StatusCode)
	}
}

func TestCallbackErrors(t *testing.T) {
	h := newHarness(t)

	if resp, _ := h.get(t, "/callback?code=good&state=unknown"); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown state: %d", resp.StatusCode)
	}

	state, _ := h.begin(t, "Assets")
	resp, body := h.get(t, "/callback?"+url.Values{"error": {"access_denied"}, "state": {state}}.Encode())
	if !strings.Contains(body, "cancelled") {
		t.Errorf("access_denied: %d %s", resp.StatusCode, body)
	}
	if resp, _ := h.get(t, "/callback?"+url.Values{"code": {"good"}, "state": {state}}.Encode()); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("state reused after cancel: %d", resp.StatusCode)
	}

	state, _ = h.begin(t, "Assets")
	if resp, _ := h.get(t, "/callback?"+url.Values{"code": {"bad"}, "state": {state}}.Encode()); resp.StatusCode != http.StatusBadGateway {
		t.Errorf("bad code: %d", resp.StatusCode)
	}

	h.failOn.Store(true)
	state, _ = h.begin(t, "Assets")
	resp, body = h.get(t, "/callback?"+url.Values{"code": {"good"}, "state": {state}}.Encode())
	if resp.StatusCode != http.StatusBadGateway || !strings.Contains(body, "store failed") {
		t.Errorf("handler error: %d %s", resp.StatusCode, body)
	}
}

func TestDoneWithoutLoginRedirects(t *testing.T) {
	h := newHarness(t)
	state, _ := h.begin(t, "Assets")
	resp, _ := h.get(t, "/done?state="+state)
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
		t.Errorf("done: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestAttemptsAreCapped(t *testing.T) {
	h := newHarness(t)
	first, _ := h.begin(t)
	for range maxAttempts {
		h.begin(t)
	}
	if n := len(h.srv.attempts); n != maxAttempts {
		t.Errorf("attempts = %d", n)
	}
	if _, ok := h.srv.attempts[first]; ok {
		t.Error("oldest attempt not evicted")
	}
}

func TestNewRejectsCallback(t *testing.T) {
	for _, cb := range []string{
		"https://localhost:45538/callback",
		"http://example.com:45538/callback",
		"http://localhost/callback",
		"http://localhost:45538/",
	} {
		if _, err := New(sso.NewClient(sso.Config{CallbackURL: cb}), nil, web, nil, nil); err == nil {
			t.Errorf("%s: accepted", cb)
		}
	}
}

func TestListenBusyPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	client := sso.NewClient(sso.Config{CallbackURL: "http://" + ln.Addr().String() + "/callback"})
	srv, err := New(client, nil, web, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Listen(); err == nil {
		srv.Close()
		t.Fatal("listen on busy port succeeded")
	}
}

func TestListenLocalhost(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	client := sso.NewClient(sso.Config{CallbackURL: "http://localhost:" + strconv.Itoa(port) + "/callback"})
	srv, err := New(client, nil, web, features, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	resp, err := http.Get(srv.URL())
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestServesAssets(t *testing.T) {
	h := newHarness(t)
	resp, body := h.get(t, "/assets/app.js")
	if resp.StatusCode != http.StatusOK || body != `console.log("app")` {
		t.Errorf("asset: %d %s", resp.StatusCode, body)
	}
	if resp, _ := h.get(t, "/index.html"); resp.StatusCode == http.StatusOK {
		t.Error("index.html is served without page data")
	}
}

func TestNewRejectsWebWithoutSlot(t *testing.T) {
	client := sso.NewClient(sso.Config{CallbackURL: "http://localhost:1/callback"})
	bad := fstest.MapFS{"index.html": {Data: []byte(`<div id="root"></div>`)}}
	if _, err := New(client, nil, bad, nil, nil); err == nil {
		t.Error("accepted index.html without page-data element")
	}
}
