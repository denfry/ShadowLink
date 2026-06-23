// SPDX-License-Identifier: GPL-3.0-only
package subscription

import (
	"encoding/base64"
	"testing"
)

func TestParseSubscriptionBase64(t *testing.T) {
	links := "vless://u1@a.com:443?pbk=K1#n1\nvless://u2@b.com:443?pbk=K2#n2\n"
	body := base64.StdEncoding.EncodeToString([]byte(links))
	servers, err := ParseSubscription([]byte(body))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(servers) != 2 || servers[0].Host != "a.com" || servers[1].Host != "b.com" {
		t.Fatalf("wrong servers: %+v", servers)
	}
}

func TestParseSubscriptionRawURLBase64(t *testing.T) {
	// Raw-url-safe base64 (no padding) is the second decode branch; std base64
	// rejects it (wrong length/alphabet), so this specifically exercises it.
	links := "vless://u1@a.com:443?pbk=K1#n1\nvless://u2@b.com:443?pbk=K2#n2\n"
	body := base64.RawURLEncoding.EncodeToString([]byte(links))
	servers, err := ParseSubscription([]byte(body))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(servers) != 2 || servers[0].Host != "a.com" || servers[1].Host != "b.com" {
		t.Fatalf("raw-url base64 path wrong: %+v", servers)
	}
}

func TestParseSubscriptionRawFallback(t *testing.T) {
	raw := "vless://u@a.com:443?pbk=K#n\n# a comment line\n\n"
	servers, err := ParseSubscription([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("want 1 server, got %d", len(servers))
	}
}

func TestDecodeLenientRejectsNonURIBase64(t *testing.T) {
	// "aGVsbG8=" is valid std base64 ("hello") but has no share URIs; it must
	// fall through to plain text (which has no vless:// lines) -> no-servers
	// error, NOT a silent garbage decode.
	if _, err := ParseSubscription([]byte("aGVsbG8=")); err == nil {
		t.Fatal("expected no-servers error for non-URI base64 body")
	}
}
