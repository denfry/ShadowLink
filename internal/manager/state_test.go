// SPDX-License-Identifier: GPL-3.0-only
package manager

import "testing"

func TestValidTransitions(t *testing.T) {
	// Every entry in the allowed map is asserted here, so removing any legal
	// transition (several are load-bearing for the manager: cancel mid-connect,
	// sudden failure while connected, hard reset from error) fails a test.
	ok := [][2]State{
		{Disconnected, Connecting}, {Connecting, Connected}, {Connecting, Error},
		{Connecting, Disconnecting}, {Connected, Reconnecting}, {Connected, Disconnecting},
		{Connected, Error}, {Reconnecting, Connected}, {Reconnecting, Error},
		{Reconnecting, Disconnecting}, {Disconnecting, Disconnected},
		{Error, Connecting}, {Error, Disconnecting}, {Error, Disconnected},
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
		{Disconnecting, Connecting}, {Error, Connected}, {Error, Reconnecting},
	}
	for _, p := range bad {
		if CanTransition(p[0], p[1]) {
			t.Errorf("expected %s->%s forbidden", p[0], p[1])
		}
	}
}
