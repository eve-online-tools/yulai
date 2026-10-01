package feature

import "testing"

type fake struct{ scopes []string }

func (f fake) Name() string     { return "fake" }
func (f fake) Scopes() []string { return f.scopes }
func (f fake) Jobs() []Job      { return nil }

func TestEnabled(t *testing.T) {
	f := fake{scopes: []string{"a", "b"}}

	if !Enabled(f, []string{"b", "a", "c"}) {
		t.Fatal("superset should enable")
	}
	if Enabled(f, []string{"a"}) {
		t.Fatal("missing scope should disable")
	}
	if !Enabled(fake{}, nil) {
		t.Fatal("feature without scopes is always enabled")
	}
}
