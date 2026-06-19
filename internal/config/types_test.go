// SPDX-License-Identifier: GPL-3.0-only
package config

import "testing"

func TestServerValidate(t *testing.T) {
	good := Server{Tag: "n1", UUID: "11111111-2222-3333-4444-555555555555", Host: "ex.com", Port: 443, Flow: "xtls-rprx-vision", Network: "tcp", SNI: "www.microsoft.com", Fingerprint: "chrome", PublicKey: "pbk", ShortID: "ab"}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid server rejected: %v", err)
	}
	bad := good
	bad.Flow = "xtls-rprx-direct" // unsupported by sing-box
	if err := bad.Validate(); err == nil {
		t.Fatal("expected error for unsupported flow")
	}
	noKey := good
	noKey.PublicKey = ""
	if err := noKey.Validate(); err == nil {
		t.Fatal("expected error for missing reality public_key")
	}
}

func TestDefaultSettings(t *testing.T) {
	s := DefaultSettings()
	if !s.KillSwitch || s.FailOpen {
		t.Fatal("kill switch must default fail-closed")
	}
	if s.DoHResolver == "" {
		t.Fatal("DoH resolver must have a default")
	}
}
