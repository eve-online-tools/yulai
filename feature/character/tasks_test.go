package character

import (
	"context"
	"testing"

	"github.com/eve-online-tools/yulai/core/task"
)

func TestWithTokenSkipsNeedsLogin(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newTestService(t, pilot)
	if err := s.Store(ctx, result("esi-a.v1")); err != nil {
		t.Fatal(err)
	}
	got, err := s.withToken(ctx)
	if err != nil || len(got) != 1 || got[0].CharacterID != 90000001 {
		t.Fatalf("withToken = %v, %v", got, err)
	}
	if err := s.MarkNeedsLogin(ctx, 90000001, "invalid_grant"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.withToken(ctx); len(got) != 0 {
		t.Fatalf("withToken = %v after MarkNeedsLogin, want none", got)
	}
}

// Task options are validated at Bind and Register, which panic on mistakes.
func TestTasksRegister(t *testing.T) {
	s, _, _ := newTestService(t, pilot)
	task.NewScheduler(task.Options{}).Register(s.Tasks()...)
}

func TestWithScopes(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newTestService(t, pilot)
	if err := s.Store(ctx, result("esi-a.v1", "esi-b.v1")); err != nil {
		t.Fatal(err)
	}
	if got, err := s.WithScopes(ctx, "esi-a.v1", "esi-b.v1"); err != nil || len(got) != 1 {
		t.Fatalf("WithScopes = %v, %v", got, err)
	}
	if got, _ := s.WithScopes(ctx, "esi-a.v1", "esi-c.v1"); len(got) != 0 {
		t.Fatalf("WithScopes = %v with a missing scope, want none", got)
	}
	if got, _ := s.WithFeature(ctx, scoped{"esi-b.v1"}); len(got) != 1 {
		t.Fatalf("WithFeature = %v, want the pilot", got)
	}
	if got, _ := s.WithFeature(ctx, scoped{"esi-c.v1"}); len(got) != 0 {
		t.Fatalf("WithFeature = %v with a missing scope, want none", got)
	}
}

type scoped []string

func (scoped) Name() string          { return "scoped" }
func (f scoped) Scopes() []string    { return f }
func (scoped) Tasks() []task.Binding { return nil }

func TestSetAffiliation(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newTestService(t, pilot)
	if err := s.Store(ctx, result()); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAffiliation(ctx, 90000001, 98000002, nil); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.List(ctx)
	if rows[0].CorporationID != 98000002 || rows[0].AllianceID != nil {
		t.Fatalf("row = %+v", rows[0])
	}
	if ids, err := s.IDs(ctx); err != nil || len(ids) != 1 {
		t.Fatalf("IDs = %v, %v", ids, err)
	}
}
