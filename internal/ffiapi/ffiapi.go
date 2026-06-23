// SPDX-License-Identifier: GPL-3.0-only

// Package ffiapi is the pure-Go JSON API over the ShadowLink engine. It holds
// the single live tunnel and renders every result as a JSON string, so a thin
// cgo shim can re-export it as a C ABI. It imports no cgo and is fully
// unit-testable with fakes (no DLL, no privileges, no real tunnel).
package ffiapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/core"
	"github.com/shadowlink/shadowlink/internal/manager"
	"github.com/shadowlink/shadowlink/internal/platform"
	"github.com/shadowlink/shadowlink/internal/secret"
	"github.com/shadowlink/shadowlink/internal/subscription"
	"github.com/shadowlink/shadowlink/internal/version"
)

// Prober measures outbound latency (satisfied by *core.ClashClient).
type Prober interface {
	Delay(ctx context.Context, tag, url string, timeoutMS int) (int, error)
}

// API holds the engine's injectable collaborators and the single live tunnel.
type API struct {
	// injectable collaborators (real in Default(), fakes in tests)
	render     func(core.BuildParams) ([]byte, error)
	newTunnel  func([]byte) (manager.Tunnel, error)
	newKS      func() platform.KillSwitch
	newProber  func(port int, secret string) Prober
	load       func() (config.Profile, error)
	save       func(config.Profile) error
	importFn   func(string) ([]config.Server, error)
	randSecret func() string

	mu     sync.Mutex
	mgr    *manager.Manager
	prober Prober
	active config.Server // the server the live tunnel was started with
}

// Default wires the real engine collaborators.
func Default() *API {
	return &API{
		render:     core.RenderConfig,
		newTunnel:  func(b []byte) (manager.Tunnel, error) { return core.New(b) },
		newKS:      platform.NewKillSwitch,
		newProber:  func(port int, sec string) Prober { return core.NewClashClient(port, sec) },
		load:       config.Load,
		save:       config.Save,
		importFn:   subscription.Import,
		randSecret: randomSecret,
	}
}

// Version reports the build version (also a trivial FFI smoke check).
func (a *API) Version() string {
	return okJSON(map[string]any{"version": version.String()})
}

func okJSON(extra map[string]any) string {
	m := map[string]any{"ok": true}
	for k, v := range extra {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// errJSON renders a failure, masking any secret-looking content in the message.
func errJSON(err error, needsAdmin bool) string {
	m := map[string]any{"ok": false, "error": secret.Mask(err.Error())}
	if needsAdmin {
		m["needsAdmin"] = true
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func randomSecret() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "shadowlink-fallback-secret"
	}
	return hex.EncodeToString(b)
}
