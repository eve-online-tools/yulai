package app

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/eve-online-tools/yulai/core/crypt"
	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/keyring"
	"github.com/eve-online-tools/yulai/core/sde"
	"github.com/eve-online-tools/yulai/core/task"
	"github.com/eve-online-tools/yulai/feature"
	"github.com/eve-online-tools/yulai/feature/character"
	"github.com/eve-online-tools/yulai/feature/charactersheet"
	"github.com/eve-online-tools/yulai/feature/presence"
	"github.com/eve-online-tools/yulai/feature/shiptree"
	"github.com/eve-online-tools/yulai/feature/skills"
	"github.com/eve-online-tools/yulai/feature/starmap"
	"github.com/eve-online-tools/yulai/identity/login"
	"github.com/eve-online-tools/yulai/identity/sso"
	"github.com/eve-online-tools/yulai/identity/token"
)

const taskWorkers = 8

// EventChanged carries a topic. The frontend refetches the queries under it.
const EventChanged = "data:changed"

// TopicTasks is sent, debounced, when task run status changes. It is keys.tasks in the frontend.
const TopicTasks = "tasks"

func init() {
	application.RegisterEvent[string](EventChanged)
	application.RegisterEvent[task.ProgressEvent](EventProgressStart)
	application.RegisterEvent[task.ProgressEvent](EventProgressUpdate)
	application.RegisterEvent[task.ProgressEvent](EventProgressDone)
}

// App wires the packages together.
type App struct {
	Config     *Config
	Characters *character.Service
	Login      *login.Server

	conn     *sql.DB
	sde      *sde.Store
	tokens   *token.Store
	tasks    *task.Scheduler
	features []feature.Feature
	cancel   context.CancelFunc
	// done closes when the task scheduler has drained its in-flight runs.
	done chan struct{}
}

// web is the apps/webserver build the login server serves.
func New(ctx context.Context, cfg *Config, web fs.FS) (*App, error) {
	conn, err := db.Open(ctx, cfg.DBPath())
	if err != nil {
		return nil, err
	}

	sealer, err := crypt.Open(keyring.OS{Service: "eve-online-tools/" + Name, User: "master-key"})
	if err != nil {
		conn.Close()
		return nil, err
	}

	ssoClient := sso.NewClient(cfg.SSO)
	verifier := sso.NewVerifier(ssoClient)
	store, err := sde.Open(ctx, cfg.DataDir, slog.Default())
	if err != nil {
		conn.Close()
		return nil, err
	}
	app := &App{Config: cfg, conn: conn, sde: store}
	app.tokens = token.NewStore(conn, ssoClient, verifier, sealer, func(ctx context.Context, id int64, reason string) {
		if err := app.Characters.MarkNeedsLogin(ctx, id, reason); err != nil {
			slog.Warn("mark needs login", "character", id, "err", err)
		}
	})
	esiClient := esi.NewClient(cfg.SSO.Name, cfg.SSO.Description, cfg.CachePath())
	app.Characters = character.NewService(conn, emitter{}, app.tokens, esiClient)

	app.tasks = task.NewScheduler(task.Options{
		Workers:  taskWorkers,
		OnChange: func() { emitter{}.Changed(TopicTasks) },
		OnProgress: func(ev task.ProgressEvent) {
			application.Get().Event.Emit(progressEvents[ev.State], ev)
		},
	})
	updater := sde.NewUpdater(store, &http.Client{}, emitter{})

	d := feature.Deps{
		DB:         conn,
		ESI:        esiClient,
		SDE:        store,
		Tokens:     app.tokens,
		Characters: app.Characters,
		Events:     emitter{},
	}
	// Every feature is listed here. Order is the picker order.
	app.features = []feature.Feature{
		// core/sde cannot import feature, so its entry is built here.
		{
			ID:       "sde",
			Tasks:    updater.Tasks(),
			Services: []application.Service{application.NewService(sde.NewService(updater, app.tasks))},
		},
		character.Feature(app.Characters),
		charactersheet.Feature(d, app.Characters),
		presence.Feature(d),
		skills.Feature(d),
		shiptree.Feature(d),
		starmap.Feature(d),
	}

	var picker []login.Feature
	for _, f := range app.features {
		app.tasks.Register(f.Tasks...)
		if len(f.Scopes) > 0 {
			picker = append(picker, login.Feature{ID: f.ID, Name: f.Name, Scopes: f.Scopes})
		}
	}
	app.Login, err = login.New(ssoClient, verifier, web, picker, func(ctx context.Context, r *login.Result) error {
		if err := app.Characters.Store(ctx, r); err != nil {
			return err
		}
		// Sync the character now instead of on the next tick, which can be an hour away.
		app.tasks.Refresh(feature.CharacterSubject(r.CharacterID))
		return nil
	})
	if err != nil {
		conn.Close()
		store.Close()
		return nil, err
	}
	return app, nil
}

// Start binds the login server and runs the scheduler until ctx ends or Close. A
// busy login port is fatal.
func (a *App) Start(ctx context.Context) error {
	if err := a.Login.Listen(); err != nil {
		return err
	}
	ctx, a.cancel = context.WithCancel(ctx)
	a.done = make(chan struct{})
	go func() {
		defer close(a.done)
		if err := a.tasks.Start(ctx); err != nil {
			slog.Error("task scheduler", "err", err)
		}
	}()
	return nil
}

func (a *App) Services() []application.Service {
	out := []application.Service{
		application.NewService(login.NewConsent(a.Login, browser{}, a.tokens)),
		application.NewService(newSetupService(a.Config, a.Login.URL())),
		application.NewService(&TaskService{s: a.tasks}),
	}
	for _, f := range a.features {
		out = append(out, f.Services...)
	}
	return out
}

// Close stops the scheduler and waits for in-flight runs before closing the
// login server and the database they write to.
func (a *App) Close() error {
	if a.cancel != nil {
		a.cancel()
		<-a.done
	}
	err := a.Login.Close()
	if a.conn != nil {
		err = errors.Join(err, a.conn.Close())
	}
	return errors.Join(err, a.sde.Close())
}

// browser implements login.Browser.
type browser struct{}

func (browser) OpenURL(url string) error { return application.Get().Browser.OpenURL(url) }

// emitter implements feature.Emitter.
type emitter struct{}

func (emitter) Changed(topic string) { application.Get().Event.Emit(EventChanged, topic) }
