// Package feature defines what a feature is and what it is built from. Concrete
// features live in subpackages; app lists them and wires what they return.
package feature

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/eve-online-tools/lib-esi-go/middleware/authentication"
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/eve-online-tools/yulai/core/sde"
	"github.com/eve-online-tools/yulai/core/task"
)

// Feature is what a feature package hands to app: its consent, background work
// and Wails services.
type Feature struct {
	// ID is stable. The frontend gates pages on it.
	ID string
	// Name is shown on the login picker.
	Name string
	// Scopes must all be on a character's token for the feature to be on. None
	// means always on and not on the picker.
	Scopes   []string
	Tasks    []task.Binding
	Services []application.Service
}

// Deps is what every feature can build itself from. A constructor takes extra
// arguments for anything feature-specific.
type Deps struct {
	DB         *sql.DB
	ESI        *http.Client
	SDE        *sde.Store
	Tokens     Tokens
	Characters Characters
	Events     Emitter
}

type Tokens interface {
	For(characterID int64) authentication.RefreshableToken
}

// Characters is implemented by feature/character.
type Characters interface {
	// WithScopes lists characters with a working token that covers every scope.
	WithScopes(ctx context.Context, scopes ...string) ([]int64, error)
}

// Emitter tells the frontend that stored data changed. It refetches the queries
// under that topic.
type Emitter interface {
	Changed(topic string)
}

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
