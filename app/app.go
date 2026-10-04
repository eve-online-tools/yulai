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
	"github.com/eve-online-tools/yulai/feature/skills"
	"github.com/eve-online-tools/yulai/feature/sync"
	"github.com/eve-online-tools/yulai/identity/login"
	"github.com/eve-online-tools/yulai/identity/sso"
	"github.com/eve-online-tools/yulai/identity/token"
)

const (
	syncWorkers = 4
	taskWorkers = 8
)

// EventTasksChanged fires, debounced, when task run status or progress changes.
const EventTasksChanged = "task:changed"

func init() {
	application.RegisterEvent[struct{}](character.EventChanged)
	application.RegisterEvent[struct{}](sync.EventJobsChanged)
	application.RegisterEvent[struct{}](presence.EventChanged)
	application.RegisterEvent[struct{}](charactersheet.EventChanged)
	application.RegisterEvent[struct{}](skills.EventChanged)
	application.RegisterEvent[struct{}](sde.EventChanged)
	application.RegisterEvent[struct{}](EventTasksChanged)
	application.RegisterEvent[task.ProgressEvent](EventProgressStart)
	application.RegisterEvent[task.ProgressEvent](EventProgressUpdate)
	application.RegisterEvent[task.ProgressEvent](EventProgressDone)
}

// App wires the packages together.
type App struct {
	Config     *Config
	Characters *character.Service
	Scheduler  *sync.Scheduler
	Login      *login.Server
	SDE        *sde.Updater

	conn   *sql.DB
	sde    *sde.Store
	cancel context.CancelFunc
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
	app.SDE = sde.NewUpdater(store, &http.Client{}, emitter{})
	tokens := token.NewStore(conn, ssoClient, verifier, sealer, func(ctx context.Context, id int64, reason string) {
		if err := app.Characters.MarkNeedsLogin(ctx, id, reason); err != nil {
			slog.Warn("mark needs login", "character", id, "err", err)
		}
	})
	esiClient := esi.NewClient(cfg.SSO.Name, cfg.SSO.Description, cfg.CachePath())

	// Every opt-in feature is registered here. Order is what the UI shows.
	features := []feature.Feature{
		presence.NewFeature(conn, esiClient, tokens, characters{app}, emitter{}),
		skills.NewFeature(conn, esiClient, tokens, characters{app}, emitter{}),
	}

	app.Scheduler = sync.NewScheduler(conn, features, tokens, syncWorkers)
	loginFeatures := make([]login.Feature, 0, len(features))
	for _, f := range features {
		loginFeatures = append(loginFeatures, login.Feature{Name: f.Name(), Scopes: f.Scopes()})
	}
	app.Login, err = login.New(ssoClient, verifier, web, loginFeatures, func(ctx context.Context, r *login.Result) error {
		return app.Characters.Store(ctx, r)
	})
	if err != nil {
		conn.Close()
		store.Close()
		return nil, err
	}
	app.Characters = character.NewService(conn, app.Login.URL(), browser{}, emitter{}, tokens, esiClient, features, app.Scheduler)

	task.Default = task.NewScheduler(task.Options{
		Workers:  taskWorkers,
		OnChange: func() { emitter{}.Emit(EventTasksChanged) },
		OnProgress: func(ev task.ProgressEvent) {
			application.Get().Event.Emit(progressEvents[ev.State], ev)
		},
	})
	task.Default.Register(app.SDE.Tasks()...)
	task.Default.Register(app.Characters.Tasks()...)
	// Always on, so not in features: it needs no scopes and has nothing to opt into.
	task.Default.Register(charactersheet.NewSheet(conn, esiClient, app.Characters, emitter{}).Tasks()...)
	for _, f := range features {
		task.Default.Register(f.Tasks()...)
	}

	return app, nil
}

// Start binds the login server, enrolls existing characters and runs the schedulers
// until ctx ends or Close. A busy login port is fatal.
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
	ctx, a.cancel = context.WithCancel(ctx)
	a.done = make(chan struct{})
	go a.Scheduler.Start(ctx)
	go func() {
		defer close(a.done)
		if err := task.Default.Start(ctx); err != nil {
			slog.Error("task scheduler", "err", err)
		}
	}()
	return nil
}

func (a *App) Services() []application.Service {
	return []application.Service{
		application.NewService(a.Characters),
		application.NewService(sync.NewService(a.Scheduler)),
		application.NewService(newSetupService(a.Config, a.Login.URL())),
		application.NewService(sde.NewService(a.SDE, task.Default.List)),
		application.NewService(&ProgressService{s: task.Default}),
	}
}

// Close stops the schedulers and waits for in-flight runs before closing the
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

// browser implements character.Browser.
type browser struct{}

func (browser) OpenURL(url string) error { return application.Get().Browser.OpenURL(url) }

// characters implements presence.Characters and skills.Characters once app.Characters is set.
type characters struct{ app *App }

func (c characters) WithFeature(ctx context.Context, f feature.Feature) ([]int64, error) {
	return c.app.Characters.WithFeature(ctx, f)
}

// emitter implements character.Emitter.
type emitter struct{}

func (emitter) Emit(name string) { application.Get().Event.Emit(name, struct{}{}) }
