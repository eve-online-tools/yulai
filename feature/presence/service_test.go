package presence

import (
	"context"
	"testing"
)

func TestServiceList(t *testing.T) {
	ctx := context.Background()
	f, _, _ := newTestFeature(t, fakeESI{"/online": onlineBody, "/location": locationBody})
	s := NewService(f)

	if rows, err := s.List(ctx); err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("before fetch = %#v, %v, want empty", rows, err)
	}
	if _, err := f.online(ctx, Input{pilot}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.location(ctx, Input{pilot}); err != nil {
		t.Fatal(err)
	}

	rows, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].CharacterID != pilot || rows[0].SolarSystemID != 30000142 || rows[0].Online == nil || !*rows[0].Online || rows[0].ShipName != nil {
		t.Fatalf("rows = %+v", rows)
	}
}
