// SPDX-License-Identifier: GPL-3.0-only
package ffiapi

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func decode(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("not valid JSON: %v (%q)", err, s)
	}
	return m
}

func TestVersionOK(t *testing.T) {
	m := decode(t, Default().Version())
	if m["ok"] != true {
		t.Fatalf("want ok=true, got %v", m["ok"])
	}
	if v, _ := m["version"].(string); v == "" {
		t.Fatalf("want non-empty version, got %q", v)
	}
}

func TestErrJSONMasksSecret(t *testing.T) {
	out := errJSON(errors.New("bad uuid 11111111-2222-3333"), true)
	m := decode(t, out)
	if m["ok"] != false || m["needsAdmin"] != true {
		t.Fatalf("want ok=false needsAdmin=true, got %v", m)
	}
	if msg, _ := m["error"].(string); strings.Contains(msg, "2222-3333") {
		t.Fatalf("error leaked secret: %q", msg)
	}
}
