// SPDX-License-Identifier: GPL-3.0-only
package config

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	in := Profile{
		Servers:  []Server{{Tag: "n1", UUID: "u", Host: "ex.com", Port: 443, Flow: "xtls-rprx-vision", Network: "tcp", PublicKey: "K"}},
		Selected: "n1",
		Settings: DefaultSettings(),
	}
	if err := SaveTo(path, in); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(out.Servers) != 1 || out.Servers[0].Tag != "n1" || out.Selected != "n1" {
		t.Fatalf("round trip mismatch: %+v", out)
	}
}

func TestLoadFromMissingReturnsDefault(t *testing.T) {
	out, err := LoadFrom(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if !out.Settings.KillSwitch {
		t.Fatal("missing profile should carry default settings")
	}
}
