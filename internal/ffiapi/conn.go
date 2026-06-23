// SPDX-License-Identifier: GPL-3.0-only
package ffiapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/manager"
	"github.com/shadowlink/shadowlink/internal/secret"
)

type startOpts struct {
	FailOpen bool   `json:"failOpen"`
	Tag      string `json:"tag"`
}

// Start brings up the tunnel for the selected (or named) server.
func (a *API) Start(optsJSON string) string {
	var opts startOpts
	if strings.TrimSpace(optsJSON) != "" {
		if err := json.Unmarshal([]byte(optsJSON), &opts); err != nil {
			return errJSON(fmt.Errorf("bad start options: %w", err), false)
		}
	}
	prof, err := a.load()
	if err != nil {
		return errJSON(err, false)
	}
	srv, err := pickServer(prof, opts.Tag)
	if err != nil {
		return errJSON(err, false)
	}
	set := prof.Settings
	if opts.FailOpen {
		set.KillSwitch = false
		set.FailOpen = true
	}
	set.ClashAPISecret = a.randSecret()

	a.mu.Lock()
	defer a.mu.Unlock()
	mgr := manager.New(manager.Deps{Render: a.render, NewTunnel: a.newTunnel, KillSwitch: a.newKS()})
	if err := mgr.Connect(srv, set); err != nil {
		return errJSON(err, isAccessDenied(err))
	}
	a.mgr = mgr
	a.active = srv
	a.prober = a.newProber(set.ClashAPIPort, set.ClashAPISecret)
	return okJSON(nil)
}

// Stop tears the tunnel down.
func (a *API) Stop() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.mgr == nil {
		return okJSON(nil)
	}
	err := a.mgr.Disconnect()
	a.mgr = nil
	a.prober = nil
	a.active = config.Server{}
	if err != nil {
		return errJSON(err, false)
	}
	return okJSON(nil)
}

// Status reports the live state, the active server (masked), and a delay probe.
func (a *API) Status() string {
	a.mu.Lock()
	mgr, prober, srv := a.mgr, a.prober, a.active
	a.mu.Unlock()

	state := string(manager.Disconnected)
	if mgr != nil {
		state = string(mgr.State())
	}
	out := map[string]any{"state": state, "server": "", "delayMs": 0, "error": ""}
	if mgr != nil {
		out["server"] = secret.MaskServer(srv)
	}
	if prober != nil && state == string(manager.Connected) {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		if ms, err := prober.Delay(ctx, "proxy", "https://www.gstatic.com/generate_204", 5000); err == nil {
			out["delayMs"] = ms
		} else {
			out["error"] = secret.Scrub(err.Error())
		}
	}
	return okJSON(out)
}

func pickServer(prof config.Profile, tag string) (config.Server, error) {
	if len(prof.Servers) == 0 {
		return config.Server{}, fmt.Errorf("no servers; import one first")
	}
	want := tag
	if want == "" {
		want = prof.Selected
	}
	if want != "" {
		for _, s := range prof.Servers {
			if s.Tag == want {
				return s, nil
			}
		}
	}
	return prof.Servers[0], nil
}

func isAccessDenied(err error) bool {
	e := strings.ToLower(err.Error())
	return strings.Contains(e, "access is denied") ||
		strings.Contains(e, "permission denied") ||
		strings.Contains(e, "operation not permitted") ||
		strings.Contains(e, "requires elevation") ||
		strings.Contains(e, "admin")
}
