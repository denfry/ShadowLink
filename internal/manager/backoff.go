// SPDX-License-Identifier: GPL-3.0-only
package manager

import (
	"math"
	"math/rand"
	"time"
)

// Backoff computes exponential backoff with equal jitter, capped at Max.
type Backoff struct {
	Base      time.Duration
	Max       time.Duration
	randFloat func() float64 // injectable for tests; defaults to rand.Float64
}

// Next returns the delay for a zero-based attempt number.
func (b *Backoff) Next(attempt int) time.Duration {
	rf := b.randFloat
	if rf == nil {
		rf = rand.Float64
	}
	exp := float64(b.Base) * math.Pow(2, float64(attempt))
	if exp > float64(b.Max) {
		exp = float64(b.Max)
	}
	// equal jitter in [exp/2, exp]: the exp/2 floor keeps us away from 0 for a
	// positive Base, while still spreading retries to avoid thundering herds.
	jittered := exp*rf()*0.5 + exp*0.5
	d := time.Duration(jittered)
	if d <= 0 {
		d = b.Base
	}
	return d
}
