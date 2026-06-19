// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"testing"

	"github.com/shadowlink/shadowlink/internal/config"
)

// New must accept our rendered config (schema/registry valid) WITHOUT starting it.
func TestNewAcceptsRenderedConfig(t *testing.T) {
	// A format-valid REALITY key (base64url of 32 bytes) and hex short_id, so box.New's
	// strict REALITY validation passes during construction (no live handshake here).
	srv := config.Server{Tag: "n1", UUID: "11111111-2222-3333-4444-555555555555", Host: "ex.com", Port: 443, Flow: "xtls-rprx-vision", Network: "tcp", SNI: "www.microsoft.com", Fingerprint: "chrome", PublicKey: "v69ocnoewAUJbgZARTrcy_Sf5n4eT6sfsJiR9T7iKio", ShortID: "ab"}
	set := config.DefaultSettings()
	set.ClashAPISecret = "x"
	b, err := RenderConfig(BuildParams{Server: srv, Settings: set})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	inst, err := New(b)
	if err != nil {
		t.Fatalf("core.New rejected rendered config: %v", err)
	}
	if inst == nil {
		t.Fatal("nil instance")
	}
	// Do not Start() (needs privileges); just ensure construction/parse path works.
	_ = inst.Close()
}
