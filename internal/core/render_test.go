// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"encoding/json"
	"testing"

	"github.com/shadowlink/shadowlink/internal/config"
)

func render(t *testing.T) map[string]any {
	t.Helper()
	srv := config.Server{Tag: "n1", UUID: "u", Host: "ex.com", Port: 443, Flow: "xtls-rprx-vision", Network: "tcp", SNI: "www.microsoft.com", Fingerprint: "chrome", PublicKey: "PBK", ShortID: "ab"}
	set := config.DefaultSettings()
	set.ClashAPISecret = "secret"
	b, err := RenderConfig(BuildParams{Server: srv, Settings: set})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("emitted config is not valid JSON: %v", err)
	}
	return m
}

func TestRenderHasVlessRealityOutbound(t *testing.T) {
	m := render(t)
	outs := m["outbounds"].([]any)
	var proxy map[string]any
	for _, o := range outs {
		om := o.(map[string]any)
		if om["tag"] == "proxy" {
			proxy = om
		}
	}
	if proxy == nil {
		t.Fatal("no proxy outbound")
	}
	if proxy["type"] != "vless" || proxy["flow"] != "xtls-rprx-vision" || proxy["network"] != "tcp" {
		t.Fatalf("vless fields wrong: %+v", proxy)
	}
	tls := proxy["tls"].(map[string]any)
	reality := tls["reality"].(map[string]any)
	if reality["public_key"] != "PBK" || reality["short_id"] != "ab" {
		t.Fatalf("reality fields wrong: %+v", reality)
	}
	utls := tls["utls"].(map[string]any)
	if utls["fingerprint"] != "chrome" || utls["enabled"] != true {
		t.Fatalf("utls fields wrong: %+v", utls)
	}
	if tls["server_name"] != "www.microsoft.com" {
		t.Fatalf("server_name wrong: %v", tls["server_name"])
	}
}

func TestRenderHasTunInboundUnifiedAddress(t *testing.T) {
	m := render(t)
	ins := m["inbounds"].([]any)
	tun := ins[0].(map[string]any)
	if tun["type"] != "tun" || tun["auto_route"] != true || tun["strict_route"] != true {
		t.Fatalf("tun fields wrong: %+v", tun)
	}
	if _, ok := tun["address"]; !ok {
		t.Fatal("tun must use unified 'address'")
	}
	if _, bad := tun["inet4_address"]; bad {
		t.Fatal("must NOT use removed inet4_address")
	}
}

func TestRenderDNSIsProxiedDoHNoSystemFallback(t *testing.T) {
	m := render(t)
	dns := m["dns"].(map[string]any)
	if dns["final"] != "proxy-doh" {
		t.Fatalf("dns.final must be proxy-doh, got %v", dns["final"])
	}
	servers := dns["servers"].([]any)
	doh := servers[0].(map[string]any)
	if doh["detour"] != "proxy" {
		t.Fatalf("DoH must be detoured through proxy, got %v", doh["detour"])
	}
}
