// SPDX-License-Identifier: GPL-3.0-only
package secret

import (
	"strings"
	"testing"

	"github.com/shadowlink/shadowlink/internal/config"
)

func TestMaskKeepsTailHidesMiddle(t *testing.T) {
	got := Mask("11111111-2222-3333-4444-555555555555")
	if strings.Contains(got, "2222") || strings.Contains(got, "3333") {
		t.Fatalf("secret leaked: %q", got)
	}
	if !strings.HasPrefix(got, "11") {
		t.Fatalf("want short prefix hint, got %q", got)
	}
}

func TestMaskServerHidesUUIDAndKey(t *testing.T) {
	s := config.Server{Tag: "n1", UUID: "abcdefab-0000-0000-0000-000000000000", Host: "ex.com", Port: 443, PublicKey: "SUPERSECRETKEY", ShortID: "ab"}
	out := MaskServer(s)
	if strings.Contains(out, "SUPERSECRETKEY") || strings.Contains(out, "abcdefab-0000") {
		t.Fatalf("server secret leaked: %q", out)
	}
	if !strings.Contains(out, "ex.com") || !strings.Contains(out, "n1") {
		t.Fatalf("want host/tag visible, got %q", out)
	}
}
