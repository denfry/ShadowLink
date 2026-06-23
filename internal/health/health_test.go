// SPDX-License-Identifier: GPL-3.0-only
package health

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeProber struct{ fails atomic.Int32 }

func (f *fakeProber) Delay(_ context.Context, _, _ string, _ int) (int, error) {
	if f.fails.Load() > 0 {
		f.fails.Add(-1)
		return 0, errors.New("down")
	}
	return 10, nil
}

func TestCheckerFiresDownThenUp(t *testing.T) {
	fp := &fakeProber{}
	fp.fails.Store(2)
	c := &Checker{Prober: fp, Tag: "proxy", URL: "https://x/generate_204", Interval: 5 * time.Millisecond, Failures: 2}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	down := make(chan struct{}, 1)
	up := make(chan struct{}, 1)
	go c.Run(ctx, func() { down <- struct{}{} }, func() { up <- struct{}{} })

	select {
	case <-down:
	case <-time.After(time.Second):
		t.Fatal("onDown never fired")
	}
	select {
	case <-up:
	case <-time.After(time.Second):
		t.Fatal("onUp never fired after recovery")
	}
}
