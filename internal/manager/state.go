// SPDX-License-Identifier: GPL-3.0-only
package manager

// State is a connection lifecycle state.
type State string

const (
	Disconnected  State = "disconnected"
	Connecting    State = "connecting"
	Connected     State = "connected"
	// Reconnecting is reserved for the Phase 2 client-driven reconnect/failover
	// loop (with Backoff). Phase 1 never enters it: recovery is sing-box-internal.
	Reconnecting  State = "reconnecting"
	Disconnecting State = "disconnecting"
	Error         State = "error"
)

var allowed = map[State][]State{
	Disconnected:  {Connecting},
	Connecting:    {Connected, Error, Disconnecting},
	Connected:     {Reconnecting, Disconnecting, Error},
	Reconnecting:  {Connected, Error, Disconnecting},
	Disconnecting: {Disconnected},
	Error:         {Connecting, Disconnecting, Disconnected},
}

// CanTransition reports whether from->to is a legal state change.
func CanTransition(from, to State) bool {
	for _, s := range allowed[from] {
		if s == to {
			return true
		}
	}
	return false
}
