// Package task runs background and on-demand work. A task is a package-level value
// built from a method expression and options; a scheduler runs it once it is bound
// to a receiver that carries the dependencies. Nothing here knows about ESI.
package task

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"slices"
	"strings"
)

// Subjecter is optional on inputs. The subject keys dedupe, backoff and subject
// pauses, conventionally "kind:id" (e.g. "char:123"). Inputs without it share one key.
type Subjecter interface {
	Subject() string
}

// Task is immutable after New. All run state lives in the scheduler.
type Task[R, I, O any] struct {
	run  func(R, context.Context, I) (O, error)
	cfg  config
	name string
}

// New defines a task. run is a method expression such as (*Wallet).Balance, from
// which R, I and O are inferred. Invalid options panic, so mistakes surface on
// process or test start.
func New[R, I, O any](run func(R, context.Context, I) (O, error), opts ...Option) *Task[R, I, O] {
	if run == nil {
		panic("task: New with nil run")
	}
	t := &Task[R, I, O]{run: run}
	for _, o := range opts {
		o(&t.cfg)
	}
	t.name = t.cfg.name
	if t.name == "" {
		t.name = funcName(run)
	}
	validate[R, I](t.name, t.cfg)
	return t
}

// Name is the Pausable name, or one derived from the method.
func (t *Task[R, I, O]) Name() string { return t.name }

// Bind attaches the receiver. Options that need runtime dependencies (e.g. a
// condition over a live store) can be added here.
func (t *Task[R, I, O]) Bind(r R, opts ...Option) Binding {
	cfg := t.cfg
	cfg.conds = slices.Clone(cfg.conds)
	for _, o := range opts {
		o(&cfg)
	}
	name := t.name
	if cfg.name != "" {
		name = cfg.name
	}
	validate[R, I](name, cfg)

	e := &entry{
		key:      t,
		name:     name,
		pausable: cfg.name != "",
		cfg:      cfg,
		zero:     func() any { var z I; return z },
		run: func(ctx context.Context, in any) (any, error) {
			return t.run(r, ctx, in.(I))
		},
	}
	if seed, _ := cfg.seed.(Seed[I]); seed != nil {
		e.seed = func(ctx context.Context) ([]any, error) {
			ins, err := seed(ctx)
			if err != nil {
				return nil, err
			}
			out := make([]any, len(ins))
			for i, in := range ins {
				out[i] = in
			}
			return out, nil
		}
	}
	if check, _ := cfg.check.(func(R, context.Context, I) Verdict); check != nil {
		e.check = func(ctx context.Context, in any) Verdict { return check(r, ctx, in.(I)) }
	}
	return e
}

// Queue runs the task in the background on the scheduler from ctx, or Default.
func (t *Task[R, I, O]) Queue(ctx context.Context, in I) error {
	s, err := resolve(ctx)
	if err != nil {
		return err
	}
	return t.On(s).Queue(ctx, in)
}

// Run queues the task and waits for its result. Cancelling ctx stops the wait,
// not the run.
func (t *Task[R, I, O]) Run(ctx context.Context, in I) (O, error) {
	s, err := resolve(ctx)
	if err != nil {
		var zero O
		return zero, err
	}
	return t.On(s).Run(ctx, in)
}

// On targets a specific scheduler instead of resolving one from ctx.
func (t *Task[R, I, O]) On(s *Scheduler) Handle[I, O] {
	return Handle[I, O]{s: s, key: t}
}

// Handle is a task pinned to a scheduler.
type Handle[I, O any] struct {
	s   *Scheduler
	key any
}

func (h Handle[I, O]) Queue(ctx context.Context, in I) error {
	return h.s.request(ctx, h.key, in, nil)
}

func (h Handle[I, O]) Run(ctx context.Context, in I) (O, error) {
	var zero O
	ch := make(chan result, 1)
	if err := h.s.request(ctx, h.key, in, ch); err != nil {
		return zero, err
	}
	select {
	case r := <-ch:
		out, _ := r.out.(O)
		return out, r.err
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

// Binding is a task attached to its receiver, ready to Register.
type Binding interface {
	binding() *entry
}

// entry is the type-erased form of a bound task.
type entry struct {
	key      any // the *Task, identifies the task within a scheduler
	name     string
	pausable bool
	cfg      config
	zero     func() any
	seed     func(ctx context.Context) ([]any, error) // nil: one tick runs the zero input
	check    func(ctx context.Context, in any) Verdict
	run      func(ctx context.Context, in any) (any, error)
}

func (e *entry) binding() *entry { return e }

func (e *entry) allowed(ctx context.Context, in any) Verdict {
	for _, c := range e.cfg.conds {
		if v := c.Check(ctx); !v.OK {
			return v
		}
	}
	if e.check != nil {
		return e.check(ctx, in)
	}
	return Allow()
}

func subject(in any) string {
	if s, ok := in.(Subjecter); ok {
		return s.Subject()
	}
	return ""
}

func validate[R, I any](name string, c config) {
	if c.seed != nil {
		if _, ok := c.seed.(Seed[I]); !ok {
			panic(fmt.Sprintf("task %s: WithParameters seed is %T, want task.Seed[%s]", name, c.seed, typeName[I]()))
		}
		if c.interval <= 0 && !c.startup {
			panic(fmt.Sprintf("task %s: WithParameters needs WithInterval or WithStartup", name))
		}
	}
	if c.check != nil {
		if _, ok := c.check.(func(R, context.Context, I) Verdict); !ok {
			panic(fmt.Sprintf("task %s: WithCheck is %T, want func(%s, context.Context, %s) task.Verdict",
				name, c.check, typeName[R](), typeName[I]()))
		}
	}
	if c.interval < 0 || c.timeout < 0 {
		panic(fmt.Sprintf("task %s: negative interval or timeout", name))
	}
}

func typeName[T any]() string { return reflect.TypeFor[T]().String() }

// funcName turns "github.com/x/y/feature/wallet/tasks.(*Wallet).Balance" into
// "wallet/tasks.(*Wallet).Balance".
func funcName(fn any) string {
	f := runtime.FuncForPC(reflect.ValueOf(fn).Pointer())
	if f == nil {
		return fmt.Sprintf("%T", fn)
	}
	name := f.Name()
	parts := strings.Split(name, "/")
	if len(parts) > 2 {
		name = strings.Join(parts[len(parts)-2:], "/")
	}
	return name
}
