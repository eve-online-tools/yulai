package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eve-online-tools/yulai/identity/sso"
)

func setBuild(t *testing.T, id, callback, host string) {
	t.Helper()
	oldID, oldCallback, oldHost := buildClientID, buildCallbackURL, buildSSOHost
	buildClientID, buildCallbackURL, buildSSOHost = id, callback, host
	t.Cleanup(func() { buildClientID, buildCallbackURL, buildSSOHost = oldID, oldCallback, oldHost })
}

func writeSSO(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sso.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveSSO(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "sso.json")
	tests := []struct {
		name  string
		build [3]string
		file  string
		env   map[string]string
		want  sso.Config
	}{
		{
			name: "defaults",
			want: sso.Config{CallbackURL: DefaultCallbackURL, Issuer: sso.DefaultIssuer},
		},
		{
			name:  "build values",
			build: [3]string{"build-id", "http://localhost:1/cb", "sisilogin.testeveonline.com"},
			want:  sso.Config{ClientID: "build-id", CallbackURL: "http://localhost:1/cb", Issuer: "https://sisilogin.testeveonline.com"},
		},
		{
			name:  "file overrides only the fields it sets",
			build: [3]string{"build-id", "http://localhost:1/cb", ""},
			file:  `{"ssoHost": "https://sisilogin.testeveonline.com/"}`,
			want:  sso.Config{ClientID: "build-id", CallbackURL: "http://localhost:1/cb", Issuer: "https://sisilogin.testeveonline.com"},
		},
		{
			name:  "env wins",
			build: [3]string{"build-id", "", ""},
			file:  `{"clientId": "file-id", "ssoHost": "sisilogin.testeveonline.com"}`,
			env:   map[string]string{ClientIDEnv: "env-id", HostEnv: "login.eveonline.com"},
			want:  sso.Config{ClientID: "env-id", CallbackURL: DefaultCallbackURL, Issuer: sso.DefaultIssuer},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setBuild(t, tt.build[0], tt.build[1], tt.build[2])
			for _, k := range []string{ClientIDEnv, CallbackURLEnv, HostEnv} {
				t.Setenv(k, tt.env[k])
			}
			path := missing
			if tt.file != "" {
				path = writeSSO(t, tt.file)
			}
			got, err := resolveSSO(path)
			if err != nil {
				t.Fatal(err)
			}
			tt.want.Name, tt.want.Description = Name, "Yulai"
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestResolveSSOBadFile(t *testing.T) {
	if _, err := resolveSSO(writeSSO(t, "{")); err == nil {
		t.Fatal("want parse error")
	}
}
