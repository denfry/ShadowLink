// SPDX-License-Identifier: GPL-3.0-only
package manager

import "testing"

func TestValidTransitions(t *testing.T) {
	ok := [][2]State{
		{Disconnected, Connecting}, {Connecting, Connected}, {Connecting, Error},
		{Connected, Reconnecting}, {Reconnecting, Connected}, {Connected, Disconnecting},
		{Disconnecting, Disconnected}, {Error, Connecting},
	}
	for _, p := range ok {
		if !CanTransition(p[0], p[1]) {
			t.Errorf("expected %s->%s allowed", p[0], p[1])
		}
	}
}

func TestInvalidTransitions(t *testing.T) {
	bad := [][2]State{
		{Disconnected, Connected}, {Connected, Connecting}, {Disconnected, Reconnecting},
	}
	for _, p := range bad {
		if CanTransition(p[0], p[1]) {
			t.Errorf("expected %s->%s forbidden", p[0], p[1])
		}
	}
}
