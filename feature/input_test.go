package feature

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type fakeChars map[string][]int64

func (f fakeChars) WithScopes(_ context.Context, scopes ...string) ([]int64, error) {
	if len(scopes) == 0 {
		return f[""], nil
	}
	return f[scopes[0]], nil
}

func TestScopeSeed(t *testing.T) {
	chars := fakeChars{"": {1, 2}, "esi-a.v1": {2}}
	tests := []struct {
		name   string
		scopes []string
		want   []CharacterInput
	}{
		{"any working token", nil, []CharacterInput{{1}, {2}}},
		{"scoped", []string{"esi-a.v1"}, []CharacterInput{{2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ScopeSeed(chars, tt.scopes...)(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCharacterSeedError(t *testing.T) {
	boom := errors.New("boom")
	if _, err := CharacterSeed(func(context.Context) ([]int64, error) { return nil, boom })(t.Context()); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

func TestSubject(t *testing.T) {
	if CharacterSubject(90000001) != (CharacterInput{CharacterID: 90000001}).Subject() {
		t.Error("CharacterSubject and Subject disagree")
	}
	if got := (CharacterInput{CharacterID: 90000001}).Subject(); got != "char:90000001" {
		t.Errorf("Subject = %q", got)
	}
}
