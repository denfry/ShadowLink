// SPDX-License-Identifier: GPL-3.0-only
package manager

import (
	"errors"
	"sync"
	"testing"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/core"
)

type fakeTunnel struct {
	startErr error
	started  bool
	closed   bool
}

func (f *fakeTunnel) Start() error { f.started = true; return f.startErr }
func (f *fakeTunnel) Close() error { f.closed = true; return nil }

type fakeKS struct {
	mu      sync.Mutex
	enabled bool
}

func (k *fakeKS) Enable(string) error { k.mu.Lock(); defer k.mu.Unlock(); k.enabled = true; return nil }
func (k *fakeKS) Disable() error      { k.mu.Lock(); defer k.mu.Unlock(); k.enabled = false; return nil }

func deps(tun *fakeTunnel, ks *fakeKS) Deps {
	return Deps{
		Render:     func(core.BuildParams) ([]byte, error) { return []byte("{}"), nil },
		NewTunnel:  func([]byte) (Tunnel, error) { return tun, nil },
		KillSwitch: ks,
	}
}

func srv() config.Server {
	return config.Server{Tag: "n1", UUID: "u", Host: "1.2.3.4", Port: 443, Flow: "xtls-rprx-vision", Network: "tcp", PublicKey: "K"}
}

func TestConnectReachesConnectedAndEnablesKillSwitch(t *testing.T) {
	tun := &fakeTunnel{}
	ks := &fakeKS{}
	m := New(deps(tun, ks))
	set := config.DefaultSettings()
	if err := m.Connect(srv(), set); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if m.State() != Connected {
		t.Fatalf("want Connected, got %s", m.State())
	}
	if !tun.started || !ks.enabled {
		t.Fatalf("tunnel/kill-switch not engaged: %+v %+v", tun, ks)
	}
}

func TestConnectFailureGoesToErrorAndCleansUp(t *testing.T) {
	tun := &fakeTunnel{startErr: errors.New("boom")}
	ks := &fakeKS{}
	m := New(deps(tun, ks))
	set := config.DefaultSettings()
	err := m.Connect(srv(), set)
	if err == nil {
		t.Fatal("expected connect error")
	}
	if m.State() != Error {
		t.Fatalf("want Error, got %s", m.State())
	}
	// On failure the kill switch must not be left enabled (no silent lockout).
	if ks.enabled {
		t.Fatal("kill switch left enabled after failed connect")
	}
}

func TestDisconnectTearsDown(t *testing.T) {
	tun := &fakeTunnel{}
	ks := &fakeKS{}
	m := New(deps(tun, ks))
	_ = m.Connect(srv(), config.DefaultSettings())
	if err := m.Disconnect(); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if m.State() != Disconnected || !tun.closed || ks.enabled {
		t.Fatalf("teardown incomplete: state=%s closed=%v ks=%v", m.State(), tun.closed, ks.enabled)
	}
}

func TestFailOpenSkipsKillSwitch(t *testing.T) {
	tun := &fakeTunnel{}
	ks := &fakeKS{}
	m := New(deps(tun, ks))
	set := config.DefaultSettings()
	set.KillSwitch = false
	set.FailOpen = true
	_ = m.Connect(srv(), set)
	if ks.enabled {
		t.Fatal("kill switch enabled despite fail-open")
	}
}
