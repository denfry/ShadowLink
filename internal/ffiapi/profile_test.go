// SPDX-License-Identifier: GPL-3.0-only
package ffiapi

import (
	"strings"
	"testing"

	"github.com/shadowlink/shadowlink/internal/config"
)

// newTestAPI returns an API whose profile lives in memory (no disk, no engine).
func newTestAPI() (*API, *config.Profile) {
	prof := &config.Profile{Settings: config.DefaultSettings()}
	a := Default()
	a.load = func() (config.Profile, error) { return *prof, nil }
	a.save = func(p config.Profile) error { *prof = p; return nil }
	a.importFn = func(in string) ([]config.Server, error) {
		return []config.Server{{Tag: "n1", UUID: "uuuuuuuu", Host: "ex.com", Port: 443, PublicKey: "K", Network: "tcp"}}, nil
	}
	return a, prof
}

func TestImportThenListMasksAndSelects(t *testing.T) {
	a, _ := newTestAPI()
	if m := decode(t, a.Import("vless://whatever")); m["ok"] != true {
		t.Fatalf("import not ok: %v", m)
	}
	m := decode(t, a.ListServers())
	if m["selected"] != "n1" {
		t.Fatalf("first import should auto-select n1, got %v", m["selected"])
	}
	servers := m["servers"].([]any)
	masked := servers[0].(map[string]any)["masked"].(string)
	if strings.Contains(masked, "uuuuuuuu") {
		t.Fatalf("uuid leaked in masked summary: %q", masked)
	}
}

func TestSelectUnknownErrors(t *testing.T) {
	a, _ := newTestAPI()
	_ = a.Import("vless://whatever")
	if decode(t, a.Select("nope"))["ok"] != false {
		t.Fatal("selecting an unknown tag should fail")
	}
}
