// SPDX-License-Identifier: GPL-3.0-only
package manager

import (
	"fmt"
	"sync"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/core"
	"github.com/shadowlink/shadowlink/internal/platform"
)

// Tunnel is the minimal lifecycle the manager drives (satisfied by *core.Instance).
type Tunnel interface {
	Start() error
	Close() error
}

// Deps are the manager's injectable collaborators (real or fake).
type Deps struct {
	Render     func(core.BuildParams) ([]byte, error)
	NewTunnel  func(configJSON []byte) (Tunnel, error)
	KillSwitch platform.KillSwitch
}

// Manager owns the connection state and coordinates core + kill switch.
type Manager struct {
	deps  Deps
	mu    sync.Mutex
	state State
	tun   Tunnel
	set   config.Settings
}

// New builds a Manager from its dependencies.
func New(deps Deps) *Manager {
	return &Manager{deps: deps, state: Disconnected}
}

// State returns the current connection state.
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *Manager) set_(s State) { m.state = s }

// Connect brings up the kill switch (if enabled) then the tunnel.
// Order matters: enable fail-closed BEFORE the tunnel so a startup crash can't leak.
func (m *Manager) Connect(srv config.Server, set config.Settings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !CanTransition(m.state, Connecting) {
		return fmt.Errorf("cannot connect from state %s", m.state)
	}
	m.set_(Connecting)
	m.set = set

	useKS := set.KillSwitch && !set.FailOpen
	if useKS {
		if err := m.deps.KillSwitch.Enable(srv.Host); err != nil {
			m.set_(Error)
			return fmt.Errorf("enable kill switch: %w", err)
		}
	}

	cfg, err := m.deps.Render(core.BuildParams{Server: srv, Settings: set})
	if err != nil {
		m.fail(useKS)
		return err
	}
	tun, err := m.deps.NewTunnel(cfg)
	if err != nil {
		m.fail(useKS)
		return err
	}
	if err := tun.Start(); err != nil {
		_ = tun.Close()
		m.fail(useKS)
		return fmt.Errorf("start tunnel: %w", err)
	}
	m.tun = tun
	m.set_(Connected)
	return nil
}

// fail rolls back to Error, removing the kill switch so the user isn't locked out
// by a connection that never came up.
func (m *Manager) fail(useKS bool) {
	if useKS {
		_ = m.deps.KillSwitch.Disable()
	}
	m.set_(Error)
}

// Disconnect tears down the tunnel and removes the kill switch.
func (m *Manager) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == Disconnected {
		return nil
	}
	m.set_(Disconnecting)
	var firstErr error
	if m.tun != nil {
		if err := m.tun.Close(); err != nil {
			firstErr = err
		}
		m.tun = nil
	}
	if err := m.deps.KillSwitch.Disable(); err != nil && firstErr == nil {
		firstErr = err
	}
	m.set_(Disconnected)
	return firstErr
}
