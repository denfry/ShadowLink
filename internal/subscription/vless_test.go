// SPDX-License-Identifier: GPL-3.0-only
package subscription

import "testing"

const sample = "vless://11111111-2222-3333-4444-555555555555@ex.com:443?security=reality&encryption=none&type=tcp&flow=xtls-rprx-vision&sni=www.microsoft.com&fp=chrome&pbk=PUBKEY&sid=0123abcd&spx=%2F#My%20Node"

func TestParseVLESS(t *testing.T) {
	s, err := ParseVLESS(sample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.UUID != "11111111-2222-3333-4444-555555555555" || s.Host != "ex.com" || s.Port != 443 {
		t.Fatalf("identity wrong: %+v", s)
	}
	if s.Flow != "xtls-rprx-vision" || s.SNI != "www.microsoft.com" || s.Fingerprint != "chrome" {
		t.Fatalf("tls params wrong: %+v", s)
	}
	if s.PublicKey != "PUBKEY" || s.ShortID != "0123abcd" {
		t.Fatalf("reality params wrong: %+v", s)
	}
	if s.Tag != "My Node" {
		t.Fatalf("tag wrong: %q", s.Tag)
	}
}

func TestParseVLESSDefaultsAndErrors(t *testing.T) {
	if _, err := ParseVLESS("https://ex.com"); err == nil {
		t.Fatal("expected error for non-vless scheme")
	}
	// fp defaults to chrome when absent
	s, err := ParseVLESS("vless://u@h:443?security=reality&pbk=K#n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Fingerprint != "chrome" {
		t.Fatalf("fp default should be chrome, got %q", s.Fingerprint)
	}
}
