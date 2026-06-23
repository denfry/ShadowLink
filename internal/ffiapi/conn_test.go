// SPDX-License-Identifier: GPL-3.0-only
package ffiapi

import (
	"context"
	"errors"
	"testing"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/core"
	"github.com/shadowlink/shadowlink/internal/manager"
	"github.com/shadowlink/shadowlink/internal/platform"
)

type fakeTunnel struct {
	started, closed bool
	startErr        error
}

func (f *fakeTunnel) Start() error { f.started = true; return f.startErr }
func (f *fakeTunnel) Close() error { f.closed = true; return nil }

type fakeKS struct{ enabled bool }

func (k *fakeKS) Enable(string) error { k.enabled = true; return nil }
func (k *fakeKS) Disable() error       { k.enabled = false; return nil }

type fakeProber struct{ ms int }

func (p fakeProber) Delay(context.Context, string, string, int) (int, error) { return p.ms, nil }

// apiWithFakeEngine wires Start/Stop to fakes (no real tunnel/privileges).
func apiWithFakeEngine() *API {
	a, prof := newTestAPI()
	*prof = config.Profile{
		Servers:  []config.Server{{Tag: "n1", UUID: "u", Host: "1.2.3.4", Port: 443, Flow: "xtls-rprx-vision", Network: "tcp", PublicKey: "K"}},
		Selected: "n1",
		Settings: config.DefaultSettings(),
	}
	a.render = func(core.BuildParams) ([]byte, error) { return []byte("{}"), nil }
	a.newTunnel = func([]byte) (manager.Tunnel, error) { return &fakeTunnel{}, nil }
	a.newKS = func() platform.KillSwitch { return &fakeKS{} }
	a.newProber = func(int, string) Prober { return fakeProber{ms: 42} }
	return a
}

func TestStartStatusStop(t *testing.T) {
	a := apiWithFakeEngine()
	if decode(t, a.Start(`{}`))["ok"] != true {
		t.Fatal("start should succeed with fakes")
	}
	st := decode(t, a.Status())
	if st["state"] != "connected" {
		t.Fatalf("want connected, got %v", st["state"])
	}
	if st["delayMs"].(float64) != 42 {
		t.Fatalf("want delay 42, got %v", st["delayMs"])
	}
	if decode(t, a.Stop())["ok"] != true {
		t.Fatal("stop should succeed")
	}
	if decode(t, a.Status())["state"] != "disconnected" {
		t.Fatal("status after stop should be disconnected")
	}
}

func TestStartNoServersErrors(t *testing.T) {
	a, _ := newTestAPI() // empty profile
	if decode(t, a.Start(`{}`))["ok"] != false {
		t.Fatal("start with no servers must fail")
	}
}

func TestStartStaleSelectionErrors(t *testing.T) {
	a, prof := newTestAPI()
	*prof = config.Profile{
		Servers:  []config.Server{{Tag: "n1", UUID: "u", Host: "1.2.3.4", Port: 443, Flow: "xtls-rprx-vision", Network: "tcp", PublicKey: "K"}},
		Selected: "gone", // points at a server that isn't in the list
		Settings: config.DefaultSettings(),
	}
	a.render = func(core.BuildParams) ([]byte, error) { return []byte("{}"), nil }
	a.newTunnel = func([]byte) (manager.Tunnel, error) { return &fakeTunnel{}, nil }
	a.newKS = func() platform.KillSwitch { return &fakeKS{} }
	// Must NOT silently connect to n1; a stale selection is an error.
	if decode(t, a.Start(`{}`))["ok"] != false {
		t.Fatal("start with a stale selected tag must fail, not fall back to another server")
	}
}

func TestStartRejectsDoubleStart(t *testing.T) {
	a := apiWithFakeEngine()
	if decode(t, a.Start(`{}`))["ok"] != true {
		t.Fatal("first start should succeed")
	}
	if decode(t, a.Start(`{}`))["ok"] != false {
		t.Fatal("second start while connected must be rejected (would orphan the tunnel)")
	}
}

func TestFailOpenKeepsKillSwitchOff(t *testing.T) {
	a := apiWithFakeEngine()
	ks := &fakeKS{}
	a.newKS = func() platform.KillSwitch { return ks }
	if decode(t, a.Start(`{"failOpen":true}`))["ok"] != true {
		t.Fatal("failOpen start should succeed")
	}
	if ks.enabled {
		t.Fatal("kill-switch must stay off when failOpen is set")
	}
}

func TestStartAccessDeniedSetsNeedsAdmin(t *testing.T) {
	a := apiWithFakeEngine()
	a.newTunnel = func([]byte) (manager.Tunnel, error) {
		return &fakeTunnel{startErr: errors.New("Access is denied.")}, nil
	}
	m := decode(t, a.Start(`{}`))
	if m["ok"] != false {
		t.Fatal("start should fail when the tunnel can't come up")
	}
	if m["needsAdmin"] != true {
		t.Fatalf("an access-denied failure should set needsAdmin, got %v", m)
	}
}
