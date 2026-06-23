// SPDX-License-Identifier: GPL-3.0-only
package manager

import (
	"testing"
	"time"
)

func TestBackoffGrowsAndCaps(t *testing.T) {
	// randFloat=0 selects the equal-jitter LOWER bound (exp/2), so at attempt 0
	// the floor is Base/2 — not Base. Assert that floor, growth, and the cap.
	b := Backoff{Base: time.Second, Max: 30 * time.Second, randFloat: func() float64 { return 0 }}
	d0 := b.Next(0)
	d1 := b.Next(1)
	d5 := b.Next(5)
	if d0 < time.Second/2 || d1 <= d0 {
		t.Fatalf("expected growth from the Base/2 floor: d0=%v d1=%v", d0, d1)
	}
	if d5 > 30*time.Second {
		t.Fatalf("expected cap at 30s, got %v", d5)
	}
}

func TestBackoffJitterWithinBounds(t *testing.T) {
	b := Backoff{Base: time.Second, Max: time.Minute, randFloat: func() float64 { return 1 }}
	// attempt 2 base = 4s; full jitter -> within (0, 4s]
	d := b.Next(2)
	if d <= 0 || d > 4*time.Second {
		t.Fatalf("jitter out of bounds: %v", d)
	}
}
