package app

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/eve-online-tools/yulai/core/crypt"
	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/keyring"
	"github.com/eve-online-tools/yulai/feature"
	"github.com/eve-online-tools/yulai/feature/character"
	"github.com/eve-online-tools/yulai/feature/sync"
	"github.com/eve-online-tools/yulai/identity/login"
	"github.com/eve-online-tools/yulai/identity/sso"
	"github.com/eve-online-tools/yulai/identity/token"
)

const syncWorkers = 4

func init() {
	application.RegisterEvent[struct{}](character.EventChanged)
	application.RegisterEvent[struct{}](sync.EventJobsChanged)
}

// App wires the packages together.
type App struct {
	Config     *Config
	Characters *character.Service
	Scheduler  *sync.Scheduler
	Login      *login.Server

	conn *sql.DB
}

func New(ctx context.Context, cfg *Config) (*App, error) {
	// The database is opened here once core/db is implemented:
	//   conn, err := db.Open(ctx, cfg.DBPath())
	var conn *sql.DB

	sealer, err := crypt.Open(keyring.OS{Service: "eve-online-tools/" + Name, User: "master-key"})
	if err != nil {
		return nil, err
	}

	ssoClient := sso.NewClient(cfg.SSO)
	verifier := sso.NewVerifier(ssoClient)
	tokens := token.NewStore(conn, ssoClient, verifier, sealer)
	esiClient := esi.NewClient(cfg.SSO.Name, cfg.SSO.Description, cfg.CachePath())

	app := &App{Config: cfg, conn: conn}

	// Every opt-in feature is registered here. Order is what the UI shows.
	features := []feature.Feature{}

	app.Scheduler = sync.NewScheduler(conn, features, tokens, syncWorkers, func(ctx context.Context, id int64, reason string) {
		_ = app.Characters.MarkNeedsLogin(ctx, id, reason)
	})
	loginFeatures := make([]login.Feature, 0, len(features))
	for _, f := range features {
		loginFeatures = append(loginFeatures, login.Feature{Name: f.Name(), Scopes: f.Scopes()})
	}
	app.Login, err = login.New(ssoClient, verifier, loginFeatures, func(ctx context.Context, r *login.Result) error {
		return app.Characters.Store(ctx, r)
	})
	if err != nil {
		return nil, err
	}
	app.Characters = character.NewService(conn, app.Login.URL(), browser{}, tokens, esiClient, features, app.Scheduler)

	return app, nil
}

// Start binds the login server, enrolls existing characters and runs the scheduler
// until ctx ends. A busy login port is fatal.
func (a *App) Start(ctx context.Context) error {
	if err := a.Login.Listen(); err != nil {
		return err
	}
	chars, err := a.Characters.List(ctx)
	if err != nil {
		return err
	}
	for _, c := range chars {
		if err := a.Scheduler.Enroll(ctx, c.ID); err != nil {
			slog.Warn("enroll", "character", c.ID, "err", err)
		}
	}
	go a.Scheduler.Start(ctx)
	return nil
}

func (a *App) Services() []application.Service {
	return []application.Service{
		application.NewService(a.Characters),
		application.NewService(sync.NewService(a.Scheduler)),
	}
}

func (a *App) Close() error {
	err := a.Login.Close()
	if a.conn != nil {
		err = errors.Join(err, a.conn.Close())
	}
	return err
}

// browser implements character.Browser.
type browser struct{}

func (browser) OpenURL(url string) error { return application.Get().Browser.OpenURL(url) }
