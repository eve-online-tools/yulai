package sso

import "testing"

func TestNormalizeIssuer(t *testing.T) {
	for in, want := range map[string]string{
		"":                                 DefaultIssuer,
		"login.eveonline.com":              "https://login.eveonline.com",
		"https://login.testeveonline.com/": "https://login.testeveonline.com",
		"http://127.0.0.1:8080":            "http://127.0.0.1:8080",
	} {
		if got := NormalizeIssuer(in); got != want {
			t.Errorf("NormalizeIssuer(%q) = %q, want %q", in, got, want)
		}
	}
}
