package pool

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
)

func TestPriorityServedFirst(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := Create(1)
		ctx := t.Context()
		if err := p.Acquire(ctx, false); err != nil {
			t.Fatal(err)
		}

		var order []string
		done := make(chan struct{}, 2)
		wait := func(name string, prio bool) {
			if err := p.Acquire(ctx, prio); err != nil {
				t.Error(err)
			}
			order = append(order, name)
			p.Release()
			done <- struct{}{}
		}
		go wait("low", false)
		synctest.Wait()
		go wait("high", true)
		synctest.Wait()

		p.Release()
		<-done
		<-done
		if len(order) != 2 || order[0] != "high" {
			t.Fatalf("order = %v, want high first", order)
		}
	})
}

func TestAcquireCancelled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := Create(1)
		if err := p.Acquire(t.Context(), false); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		errc := make(chan error)
		go func() { errc <- p.Acquire(ctx, false) }()
		synctest.Wait()
		cancel()
		if err := <-errc; !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}

		// The cancelled waiter must not have kept a slot.
		p.Release()
		if err := p.Acquire(t.Context(), false); err != nil {
			t.Fatal(err)
		}
	})
}
