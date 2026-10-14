package task

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
)

func TestPriorityServedFirst(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := newPool(1)
		ctx := t.Context()
		if err := p.acquire(ctx, false); err != nil {
			t.Fatal(err)
		}

		var order []string
		done := make(chan struct{}, 2)
		wait := func(name string, prio bool) {
			if err := p.acquire(ctx, prio); err != nil {
				t.Error(err)
			}
			order = append(order, name)
			p.release()
			done <- struct{}{}
		}
		go wait("low", false)
		synctest.Wait()
		go wait("high", true)
		synctest.Wait()

		p.release()
		<-done
		<-done
		if len(order) != 2 || order[0] != "high" {
			t.Fatalf("order = %v, want high first", order)
		}
	})
}

func TestAcquireCancelled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := newPool(1)
		if err := p.acquire(t.Context(), false); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		errc := make(chan error)
		go func() { errc <- p.acquire(ctx, false) }()
		synctest.Wait()
		cancel()
		if err := <-errc; !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}

		// The cancelled waiter must not have kept a slot.
		p.release()
		if err := p.acquire(t.Context(), false); err != nil {
			t.Fatal(err)
		}
	})
}
