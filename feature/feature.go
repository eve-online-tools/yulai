// Package feature defines what an opt-in feature is. Concrete features live in subpackages.
package feature

import "github.com/eve-online-tools/yulai/core/task"

// Feature is a unit of functionality a user consents to through SSO scopes.
type Feature interface {
	Name() string
	// Scopes must all be present on the character's token, or the feature is off.
	Scopes() []string
	Tasks() []task.Binding
}

// Enabled reports whether a token with the given scopes unlocks the feature.
func Enabled(f Feature, scopes []string) bool { return Covers(scopes, f.Scopes()) }

// Covers reports whether have includes every scope in need.
func Covers(have, need []string) bool {
	set := map[string]bool{}
	for _, s := range have {
		set[s] = true
	}
	for _, s := range need {
		if !set[s] {
			return false
		}
	}
	return true
}
