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
}
