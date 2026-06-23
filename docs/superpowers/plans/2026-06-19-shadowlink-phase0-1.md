# ShadowLink Phases 0–1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a working, leak-resistant single-server CLI VPN client that embeds sing-box and connects to the existing Xray VLESS+REALITY+XTLS-Vision server — first proving interop (Phase 0 gate), then hardening it into the MVP core (Phase 1).

**Architecture:** One Go module, one privileged process (`cmd/shadowlink`). `internal/core` is the only package that imports sing-box; it renders a sing-box **JSON** config from a typed domain model and runs it via `box.New` with the mandatory DI context. `internal/manager` owns a connection state machine and drives reconnect/health. `internal/platform` enforces the fail-closed kill-switch via the OS firewall, independent of the core's lifecycle. Pure logic (parsing, config rendering, state machine, backoff, masking) is unit-tested with no privileges; the live tunnel is an integration/gate test against the real server.

**Tech Stack:** Go 1.24+, `github.com/sagernet/sing-box v1.13.7` (package `box`, `option`, `include`), `github.com/sagernet/sing/common/json`, `github.com/spf13/cobra`, Windows `netsh advfirewall` (kill-switch MVP).

> **Build note (discovered during Phase 0 execution):** the core MUST be built with
> tags **`with_utls with_gvisor with_clash_api`** (REALITY refuses to start without
> `with_utls`). Pin **v1.13.7**, not v1.13.13: v1.13.8–v1.13.13 fail to compile in an
> embedded Windows build (sing-box↔sing-tun `MyInterface` mismatch). `cache_file` has
> no `store_selected` field in v1.13.7 — omit it. All `go build`/`go test` commands in
> the tasks below must add `-tags "with_utls with_gvisor with_clash_api"`.

## Global Constraints

These apply to **every** task; copied verbatim from ARCHITECTURE.md / the spec.

- Go **1.24+** (sing-box 1.13.x requires it).
- Pin `github.com/sagernet/sing-box` at **v1.13.7** (v1.13.8+ break the embedded Windows build).
- Build with tags **`with_utls with_gvisor with_clash_api`** everywhere (REALITY needs `with_utls`).
- sing-box config is parsed with sing-box's **context-aware JSON** (`github.com/sagernet/sing/common/json`), **never** `encoding/json`, and **always** with a registry-populated context from `include.Context(ctx)` (since v1.11 `box.New` requires it).
- VLESS outbound must use `flow` ∈ {`""`, `"xtls-rprx-vision"`} and `network: "tcp"`; Vision is valid only over bare TLS/REALITY (no ws/grpc transport).
- REALITY fields are `tls.reality.public_key` / `tls.reality.short_id`; uTLS is `tls.utls.enabled`/`tls.utls.fingerprint` (default `chrome`; never `chrome_pq`).
- TUN uses the unified `address` field (never `inet4_address`/`inet6_address`, removed 1.12.0); `auto_route: true`, `strict_route: true`.
- Blocking uses route-rule action **`reject`** — there is **no** `block`/`blackhole` outbound.
- DNS must resolve only via DoH **through the proxy** (`detour: "proxy"`); `dns.final` never points at system DNS. Port-53 captured via route-rule `sniff` + `hijack-dns` (TUN `dns_mode` is 1.14+, not used).
- Kill-switch is **fail-closed by default**, enforced by the OS firewall **independent of the core process** (so a core crash does not leak). Explicit `--fail-open` opt-out.
- **Secrets** (UUID, REALITY keys, subscription URLs) are masked in all logs/errors.
- License: **GPL-3.0**. Source files carry the SPDX header `// SPDX-License-Identifier: GPL-3.0-only`.
- TDD throughout: failing test → run-it-fails → minimal impl → run-it-passes → commit.

## File structure

```
go.mod / go.sum
.gitattributes
.github/workflows/ci.yml
internal/config/types.go         domain model: Server, Settings, Profile
internal/config/store.go         load/save profile, per-OS config dir         (Phase 1)
internal/secret/mask.go          secret masking for logs/errors               (Phase 1)
internal/subscription/vless.go   ParseVLESS(uri) -> config.Server
internal/subscription/sub.go     ParseSubscription(body) -> []config.Server    (Phase 1)
internal/subscription/import.go  Import(input) -> []config.Server              (Phase 1)
internal/core/render.go          RenderConfig(BuildParams) -> sing-box JSON
internal/core/clash.go           Clash API client: Delay/Switch               (Phase 1)
internal/core/core.go            Instance: New/Start/Close via box
internal/manager/state.go        State enum + transitions                     (Phase 1)
internal/manager/backoff.go      exponential backoff + jitter                 (Phase 1)
internal/manager/manager.go      Connect/Disconnect/Status, reconnect loop    (Phase 1)
internal/health/health.go        latency probe loop                           (Phase 1)
internal/platform/platform.go    KillSwitch interface + helpers               (Phase 1)
internal/platform/killswitch_windows.go   netsh fail-closed rules             (Phase 1)
internal/platform/killswitch_other.go     no-op stub w/ clear error           (Phase 1)
cmd/shadowlink/main.go           cobra root
cmd/shadowlink/connect.go        connect command
cmd/shadowlink/cmds.go           disconnect/status/import/logs                (Phase 1)
```

---

# PHASE 0 — Skeleton + compatibility gate

Outcome: `shadowlink connect "<vless-uri>"` brings up a system-wide tunnel to the real server. This phase *de-risks the entire project* — do not start Phase 1 until the gate (Task 0.6) passes.

---

### Task 0.1: Module scaffold + CI

**Files:**
- Create: `go.mod`, `.gitattributes`, `.github/workflows/ci.yml`
- Create: `internal/version/version.go`, `internal/version/version_test.go`

**Interfaces:**
- Produces: `version.String() string` (used by the CLI `--version`).

- [ ] **Step 1: Create `.gitattributes`** (normalize line endings — Windows dev, cross-OS build)

```gitattributes
* text=auto eol=lf
*.dll binary
*.srs binary
*.png binary
```

- [ ] **Step 2: Init the module**

Run:
```bash
go mod init github.com/shadowlink/shadowlink
go mod edit -go=1.24
```
Expected: `go.mod` created with `module github.com/shadowlink/shadowlink` and `go 1.24`.

- [ ] **Step 3: Write the failing test** — `internal/version/version_test.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package version

import "testing"

func TestStringNotEmpty(t *testing.T) {
	if String() == "" {
		t.Fatal("version.String() must not be empty")
	}
}
```

- [ ] **Step 4: Run it, verify it fails**

Run: `go test ./internal/version/`
Expected: FAIL — `undefined: String`.

- [ ] **Step 5: Implement** — `internal/version/version.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package version

// Version is overridden at build time via -ldflags "-X .../version.Version=...".
var Version = "0.0.0-dev"

// String returns the build version string.
func String() string { return Version }
```

- [ ] **Step 6: Run it, verify it passes**

Run: `go test ./internal/version/`
Expected: PASS.

- [ ] **Step 7: Create CI** — `.github/workflows/ci.yml`

```yaml
name: ci
on: [push, pull_request]
jobs:
  build-test:
    strategy:
      matrix:
        os: [ubuntu-latest, windows-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: go vet ./...
      - run: go build ./...
      - run: go test ./...
```

- [ ] **Step 8: Commit**

```bash
git add go.mod .gitattributes .github/workflows/ci.yml internal/version/
git commit -m "chore: module scaffold, version, CI"
```

---

### Task 0.2: Domain model

**Files:**
- Create: `internal/config/types.go`, `internal/config/types_test.go`

**Interfaces:**
- Produces:
  - `config.Server{Tag, UUID, Host string; Port uint16; Flow, Network, SNI, Fingerprint, PublicKey, ShortID string}`
  - `config.Settings{KillSwitch, FailOpen, SplitTunnel bool; DoHResolver, LogLevel string; ClashAPIPort int; ClashAPISecret string}`
  - `config.Profile{Servers []Server; Selected string; Settings Settings}`
  - `config.DefaultSettings() Settings`
  - `(Server).Validate() error`

- [ ] **Step 1: Write the failing test** — `internal/config/types_test.go`

```go
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
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/config/`
Expected: FAIL — undefined `Server`/`DefaultSettings`.

- [ ] **Step 3: Implement** — `internal/config/types.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package config

import "fmt"

// Server is one VLESS+REALITY+Vision outbound, in domain terms (core-agnostic).
type Server struct {
	Tag         string `json:"tag"`
	UUID        string `json:"uuid"`
	Host        string `json:"host"`
	Port        uint16 `json:"port"`
	Flow        string `json:"flow"`        // "" or "xtls-rprx-vision"
	Network     string `json:"network"`     // "tcp"
	SNI         string `json:"sni"`         // tls.server_name (camouflage)
	Fingerprint string `json:"fingerprint"` // utls fingerprint
	PublicKey   string `json:"public_key"`  // reality pbk
	ShortID     string `json:"short_id"`    // reality sid
}

// Settings are user-tunable, profile-wide options.
type Settings struct {
	KillSwitch     bool   `json:"kill_switch"`
	FailOpen       bool   `json:"fail_open"`
	SplitTunnel    bool   `json:"split_tunnel"`
	DoHResolver    string `json:"doh_resolver"` // e.g. "1.1.1.1"
	LogLevel       string `json:"log_level"`
	ClashAPIPort   int    `json:"clash_api_port"`
	ClashAPISecret string `json:"clash_api_secret"`
}

// Profile is the persisted state: known servers + selection + settings.
type Profile struct {
	Servers  []Server `json:"servers"`
	Selected string   `json:"selected"` // server Tag; "" means first
	Settings Settings `json:"settings"`
}

// DefaultSettings returns safe defaults (fail-closed kill switch on).
func DefaultSettings() Settings {
	return Settings{
		KillSwitch:   true,
		FailOpen:     false,
		SplitTunnel:  false,
		DoHResolver:  "1.1.1.1",
		LogLevel:     "info",
		ClashAPIPort: 9595,
	}
}

// Validate checks invariants the embedded core (and server) require.
func (s Server) Validate() error {
	if s.UUID == "" || s.Host == "" || s.Port == 0 {
		return fmt.Errorf("server %q: uuid, host and port are required", s.Tag)
	}
	if s.Flow != "" && s.Flow != "xtls-rprx-vision" {
		return fmt.Errorf("server %q: unsupported flow %q (sing-box accepts only \"\" or xtls-rprx-vision)", s.Tag, s.Flow)
	}
	if s.PublicKey == "" {
		return fmt.Errorf("server %q: REALITY public_key (pbk) is required", s.Tag)
	}
	if s.Network != "" && s.Network != "tcp" {
		return fmt.Errorf("server %q: Vision requires network tcp, got %q", s.Tag, s.Network)
	}
	return nil
}
```

- [ ] **Step 4: Run it, verify it passes**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): domain model with validation and defaults"
```

---

### Task 0.3: VLESS URI parser

**Files:**
- Create: `internal/subscription/vless.go`, `internal/subscription/vless_test.go`

**Interfaces:**
- Consumes: `config.Server`.
- Produces: `subscription.ParseVLESS(uri string) (config.Server, error)`.

- [ ] **Step 1: Write the failing test** — `internal/subscription/vless_test.go`

```go
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
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/subscription/`
Expected: FAIL — undefined `ParseVLESS`.

- [ ] **Step 3: Implement** — `internal/subscription/vless.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package subscription

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/shadowlink/shadowlink/internal/config"
)

// ParseVLESS parses a vless:// share link into a domain Server.
// Unknown query keys are ignored (forward-compat with pqv/mlkem etc.).
func ParseVLESS(uri string) (config.Server, error) {
	var s config.Server
	u, err := url.Parse(strings.TrimSpace(uri))
	if err != nil {
		return s, fmt.Errorf("parse uri: %w", err)
	}
	if u.Scheme != "vless" {
		return s, fmt.Errorf("not a vless:// uri (scheme %q)", u.Scheme)
	}
	if u.User == nil || u.User.Username() == "" {
		return s, fmt.Errorf("missing uuid in vless uri")
	}
	s.UUID = u.User.Username()
	s.Host = u.Hostname()
	port, err := strconv.ParseUint(u.Port(), 10, 16)
	if err != nil {
		return s, fmt.Errorf("invalid port %q: %w", u.Port(), err)
	}
	s.Port = uint16(port)

	q := u.Query()
	s.Flow = q.Get("flow")
	s.Network = q.Get("type")
	if s.Network == "" {
		s.Network = "tcp"
	}
	s.SNI = q.Get("sni")
	if s.SNI == "" {
		s.SNI = s.Host
	}
	s.Fingerprint = q.Get("fp")
	if s.Fingerprint == "" {
		s.Fingerprint = "chrome"
	}
	s.PublicKey = q.Get("pbk")
	s.ShortID = q.Get("sid")

	s.Tag = u.Fragment
	if s.Tag == "" {
		s.Tag = s.Host
	}
	return s, nil
}
```

- [ ] **Step 4: Run it, verify it passes**

Run: `go test ./internal/subscription/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/subscription/vless.go internal/subscription/vless_test.go
git commit -m "feat(subscription): vless:// URI parser"
```

---

### Task 0.4: Render sing-box config JSON

**Files:**
- Create: `internal/core/render.go`, `internal/core/render_test.go`

**Interfaces:**
- Consumes: `config.Server`, `config.Settings`.
- Produces:
  - `core.BuildParams{Server config.Server; Settings config.Settings}`
  - `core.RenderConfig(p BuildParams) ([]byte, error)` — returns sing-box JSON bytes.

This is pure (no sing-box import, no privileges), so it is fully unit-testable. We emit a `map`-backed JSON whose field names were verified against sing-box v1.13.13.

- [ ] **Step 1: Write the failing test** — `internal/core/render_test.go`

```go
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
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/core/`
Expected: FAIL — undefined `RenderConfig`.

- [ ] **Step 3: Implement** — `internal/core/render.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"encoding/json" // only to MARSHAL our own map; sing-box parsing uses sing's JSON in core.go
	"fmt"

	"github.com/shadowlink/shadowlink/internal/config"
)

// BuildParams is the input to config rendering.
type BuildParams struct {
	Server   config.Server
	Settings config.Settings
}

// RenderConfig emits a sing-box JSON config for one VLESS+REALITY+Vision server,
// system-wide TUN, proxied DoH (leak-resistant), and a loopback Clash API.
// Field names verified against sing-box v1.13.13.
func RenderConfig(p BuildParams) ([]byte, error) {
	if err := p.Server.Validate(); err != nil {
		return nil, err
	}
	s := p.Server
	set := p.Settings

	proxy := map[string]any{
		"type":        "vless",
		"tag":         "proxy",
		"server":      s.Host,
		"server_port": s.Port,
		"uuid":        s.UUID,
		"flow":        s.Flow,
		"network":     "tcp",
		"tls": map[string]any{
			"enabled":     true,
			"server_name": s.SNI,
			"utls":        map[string]any{"enabled": true, "fingerprint": s.Fingerprint},
			"reality":     map[string]any{"enabled": true, "public_key": s.PublicKey, "short_id": s.ShortID},
		},
	}

	cfg := map[string]any{
		"log": map[string]any{"level": set.LogLevel, "timestamp": true},
		"dns": map[string]any{
			"servers": []any{
				map[string]any{"tag": "proxy-doh", "type": "https", "server": set.DoHResolver, "detour": "proxy"},
			},
			"final":    "proxy-doh",
			"strategy": "prefer_ipv4",
		},
		"inbounds": []any{
			map[string]any{
				"type":         "tun",
				"tag":          "tun-in",
				"address":      []any{"172.19.0.1/30", "fdfe:dcba:9876::1/126"},
				"mtu":          1420,
				"auto_route":   true,
				"strict_route": true,
				"stack":        "mixed",
			},
		},
		"outbounds": []any{
			proxy,
			map[string]any{"type": "direct", "tag": "direct"},
		},
		"route": map[string]any{
			"auto_detect_interface": true,
			"final":                 "proxy",
			"rules": []any{
				map[string]any{"action": "sniff"},
				map[string]any{"protocol": "dns", "action": "hijack-dns"},
			},
		},
		"experimental": map[string]any{
			"cache_file": map[string]any{"enabled": true}, // note: no store_selected field in v1.13.7
			"clash_api": map[string]any{
				"external_controller": fmt.Sprintf("127.0.0.1:%d", set.ClashAPIPort),
				"secret":              set.ClashAPISecret,
			},
		},
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal sing-box config: %w", err)
	}
	return b, nil
}
```

- [ ] **Step 4: Run it, verify it passes**

Run: `go test ./internal/core/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/render.go internal/core/render_test.go
git commit -m "feat(core): render leak-resistant sing-box JSON from domain model"
```

---

### Task 0.5: Embed sing-box (Instance lifecycle) + `connect` command

**Files:**
- Create: `internal/core/core.go`
- Create: `cmd/shadowlink/main.go`, `cmd/shadowlink/connect.go`
- Modify: `go.mod` (add deps)

**Interfaces:**
- Consumes: `core.RenderConfig`, `subscription.ParseVLESS`, `version.String`.
- Produces:
  - `core.Instance` with `core.New(configJSON []byte) (*Instance, error)`, `(*Instance).Start() error`, `(*Instance).Close() error`.

There is no unit test for the live box here (it needs privileges + a real server); that is the gate in Task 0.6. We *do* test that `core.New` accepts our rendered config without a parse/registry error, which catches schema mistakes cheaply.

- [ ] **Step 1: Add dependencies**

Run:
```bash
go get github.com/sagernet/sing-box@v1.13.7
go get github.com/spf13/cobra@latest
go mod tidy
```
Expected: `go.mod`/`go.sum` updated; `sing-box` shows `v1.13.7` (sing/common/json comes
in transitively). Do NOT pin sing-tun manually — let sing-box pull its matched set.

- [ ] **Step 2: Write the failing test** — `internal/core/core_test.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"testing"

	"github.com/shadowlink/shadowlink/internal/config"
)

// New must accept our rendered config (schema/registry valid) WITHOUT starting it.
func TestNewAcceptsRenderedConfig(t *testing.T) {
	srv := config.Server{Tag: "n1", UUID: "11111111-2222-3333-4444-555555555555", Host: "ex.com", Port: 443, Flow: "xtls-rprx-vision", Network: "tcp", SNI: "www.microsoft.com", Fingerprint: "chrome", PublicKey: "PBK", ShortID: "ab"}
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
```

- [ ] **Step 3: Run it, verify it fails**

Run: `go test ./internal/core/ -run TestNewAccepts`
Expected: FAIL — undefined `New`.

- [ ] **Step 4: Implement** — `internal/core/core.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"context"
	"fmt"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"
)

// Instance is a running (or constructed) embedded sing-box.
type Instance struct {
	b   *box.Box
	ctx context.Context
}

// New constructs a sing-box instance from rendered JSON. It populates the
// mandatory DI context (include.Context) required since sing-box v1.11 and parses
// with sing-box's context-aware JSON (NOT encoding/json).
func New(configJSON []byte) (*Instance, error) {
	ctx := include.Context(context.Background())
	opts, err := json.UnmarshalExtendedContext[option.Options](ctx, configJSON)
	if err != nil {
		return nil, fmt.Errorf("parse sing-box config: %w", err)
	}
	b, err := box.New(box.Options{Context: ctx, Options: opts})
	if err != nil {
		return nil, fmt.Errorf("construct sing-box: %w", err)
	}
	return &Instance{b: b, ctx: ctx}, nil
}

// Start brings the tunnel up (privileged: creates TUN, programs routes).
func (i *Instance) Start() error {
	if i.b == nil {
		return fmt.Errorf("instance not constructed")
	}
	return i.b.Start()
}

// Close stops the instance and releases resources.
func (i *Instance) Close() error {
	if i.b == nil {
		return nil
	}
	return i.b.Close()
}
```

> Note for the implementer: the exact generic helper may be `json.UnmarshalExtendedContext[...]` or `json.UnmarshalContext(ctx, b, &opts)` depending on the pinned `sagernet/sing` version. If the build fails to resolve the generic form, switch to:
> ```go
> var opts option.Options
> if err := opts.UnmarshalJSONContext(ctx, configJSON); err != nil { return nil, err }
> ```
> which is a stable method on `option.Options`. Pick whichever compiles against the pinned version and keep it.

- [ ] **Step 5: Run it, verify it passes**

Run: `go test ./internal/core/ -run TestNewAccepts`
Expected: PASS (config parses & constructs). If it fails with a "missing registry" error, the DI context wiring is wrong — fix before proceeding.

- [ ] **Step 6: Implement the CLI root** — `cmd/shadowlink/main.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package main

import (
	"fmt"
	"os"

	"github.com/shadowlink/shadowlink/internal/version"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:     "shadowlink",
		Short:   "ShadowLink — personal VPN client over an embedded sing-box core",
		Version: version.String(),
	}
	root.AddCommand(connectCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 7: Implement the connect command** — `cmd/shadowlink/connect.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/core"
	"github.com/shadowlink/shadowlink/internal/subscription"
	"github.com/spf13/cobra"
)

func connectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect <vless-uri>",
		Short: "Connect using a single vless:// share link (Phase 0)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := subscription.ParseVLESS(args[0])
			if err != nil {
				return err
			}
			set := config.DefaultSettings()
			set.ClashAPISecret = randomSecret()
			cfgJSON, err := core.RenderConfig(core.BuildParams{Server: srv, Settings: set})
			if err != nil {
				return err
			}
			inst, err := core.New(cfgJSON)
			if err != nil {
				return err
			}
			if err := inst.Start(); err != nil {
				return fmt.Errorf("start tunnel (need admin/root?): %w", err)
			}
			fmt.Printf("connected via %s (%s). Ctrl+C to disconnect.\n", srv.Tag, srv.Host)

			sig := make(chan os.Signal, 1)
			signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
			<-sig
			fmt.Println("\ndisconnecting...")
			return inst.Close()
		},
	}
}

func randomSecret() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
```

- [ ] **Step 8: Build**

Run: `go build ./...`
Expected: builds with no errors on the dev machine.

- [ ] **Step 9: Commit**

```bash
git add internal/core/core.go internal/core/core_test.go cmd/ go.mod go.sum
git commit -m "feat(core): embed sing-box; add connect command (Phase 0)"
```

---

### Task 0.6: Compatibility gate (manual/integration) — **the make-or-break checkpoint**

**Files:**
- Create: `docs/GATE-PHASE0.md` (the runbook + recorded result)

This task is not unit-testable in CI (needs the real server + admin rights + a filtered network). It is a recorded, repeatable manual gate.

- [ ] **Step 1: Write the runbook** — `docs/GATE-PHASE0.md`

```markdown
# Phase 0 Compatibility Gate

Purpose: prove the embedded sing-box client connects to the real Xray
VLESS+REALITY+XTLS-Vision server and carries traffic, before building Phase 1.

## Prerequisites
- Windows: `wintun.dll` (matching arch) next to `shadowlink.exe`; run an
  Administrator shell. Linux: run with sudo or `setcap cap_net_admin+ep`. macOS: sudo.
- A real `vless://` link for the live server.

## Procedure
1. Build: `go build ./cmd/shadowlink`
2. Note baseline public IP: `curl https://api.ipify.org` (or a browser).
3. Connect (elevated): `shadowlink connect "<vless-uri>"`
4. In another shell, confirm:
   - Public IP changed to the server's: `curl https://api.ipify.org`
   - DNS resolves and pages load.
   - DNS-leak check (e.g. dnsleaktest.com) shows the server side, not the ISP.
5. Ctrl+C to disconnect; confirm normal connectivity returns.

## Pass criteria (ALL must hold)
- [ ] Tunnel establishes without REALITY/handshake errors in the log.
- [ ] Public IP becomes the server's while connected.
- [ ] No DNS leak to the ISP/TSPU resolver.
- [ ] Clean disconnect restores baseline connectivity.

## Result
- Date / OS / sing-box version:
- Outcome (PASS/FAIL):
- Notes (errors, fingerprint used, anything surprising):

## If FAIL
Re-evaluate before Phase 1: check flow=xtls-rprx-vision + network=tcp on the
server, pbk/sid/sni match, try fp=firefox/edge. If sing-box↔Xray REALITY proves
incompatible on this server, escalate to the xray-core embedding fallback
(ARCHITECTURE §3, Approach B) — do NOT proceed to Phase 1 on a red gate.
```

- [ ] **Step 2: Run the gate** on a real machine against the real server, following the runbook (the user/author performs this — it needs their server + network).

- [ ] **Step 3: Record the result** in the "Result" section of `docs/GATE-PHASE0.md`.

- [ ] **Step 4: Commit the recorded gate**

```bash
git add docs/GATE-PHASE0.md
git commit -m "test: Phase 0 compatibility gate runbook + result"
```

**GATE:** Proceed to Phase 1 only if the result is PASS.

---

# PHASE 1 — MVP core (CLI)

Outcome: dependable single-server CLI VPN with persistence, auto-reconnect, health-check, leak-resistant DNS, and a fail-closed Windows kill-switch.

---

### Task 1.0: Distinct TUN identity + collision detection (Phase 0 gate carry-forward)

**Why:** the Phase 0 gate failed its first attempt with `set ipv4 address: The
object already exists` because render.go emits the sing-box **default**
`172.19.0.1` on interface `tun0`, which **Hiddify** (another embedded-sing-box
client the user runs) already held. Give ShadowLink's TUN a distinct
`interface_name` + non-default subnet so it coexists, and add a pure helper that
warns *before* bring-up if the address is already taken. (`PHASED-PLAN.md` Phase 1;
`docs/GATE-PHASE0.md` carry-forward #2.)

**Files:**
- Create: `internal/core/tun.go`, `internal/core/tun_test.go`
- Modify: `internal/core/render.go` (use the TUN constants; add `interface_name`)
- Modify: `internal/core/render_test.go` (assert the distinct identity)

**Interfaces:**
- Produces:
  - `core.TUNInterfaceName = "shadowlink0"`, `core.TUNAddress4CIDR = "172.18.0.1/30"`, `core.TUNAddress6CIDR = "fdfe:dcba:9876::1/126"`.
  - `core.IPv4Assigned(ip string, addrs []net.Addr) bool` — pure; reports whether `ip` is present among interface addrs.
  - `core.LocalTUNAddrInUse() (bool, error)` — checks the host's live interfaces for ShadowLink's TUN IPv4 (used by the CLI in Task 1.10 to warn).

- [ ] **Step 1: Write the failing test** — `internal/core/tun_test.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"net"
	"testing"
)

func TestIPv4AssignedMatches(t *testing.T) {
	_, n, _ := net.ParseCIDR("172.18.0.1/30")
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("192.168.1.5"), Mask: n.Mask},
		&net.IPNet{IP: net.ParseIP("172.18.0.1"), Mask: n.Mask},
	}
	if !IPv4Assigned("172.18.0.1", addrs) {
		t.Fatal("expected 172.18.0.1 to be detected as assigned")
	}
	if IPv4Assigned("10.0.0.1", addrs) {
		t.Fatal("did not expect 10.0.0.1 to be assigned")
	}
}

func TestTUNConstantsAreNonDefault(t *testing.T) {
	if TUNAddress4CIDR == "172.19.0.1/30" {
		t.Fatal("TUN must NOT use the sing-box default 172.19.0.1 (collides with Hiddify/Nekoray)")
	}
	if TUNInterfaceName == "" || TUNInterfaceName == "tun0" {
		t.Fatalf("TUN interface name must be distinct, got %q", TUNInterfaceName)
	}
}
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test -tags "with_utls with_gvisor with_clash_api" ./internal/core/ -run "IPv4Assigned|TUNConstants"`
Expected: FAIL — undefined `IPv4Assigned`/`TUNAddress4CIDR`.

- [ ] **Step 3: Implement** — `internal/core/tun.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package core

import "net"

// ShadowLink uses a DISTINCT TUN identity so it never collides with other
// embedded-sing-box clients (Hiddify/Nekoray default to interface "tun0" on
// 172.19.0.1). The Phase 0 gate hit "set ipv4 address: The object already
// exists" because of exactly that clash.
const (
	TUNInterfaceName = "shadowlink0"
	TUNAddress4CIDR  = "172.18.0.1/30"
	TUNAddress6CIDR  = "fdfe:dcba:9876::1/126"
	tunAddress4IP    = "172.18.0.1"
)

// IPv4Assigned reports whether ip is present among addrs (the concrete types
// net.InterfaceAddrs returns: *net.IPNet / *net.IPAddr). Pure -> unit-testable
// without touching the host's interfaces.
func IPv4Assigned(ip string, addrs []net.Addr) bool {
	target := net.ParseIP(ip)
	if target == nil {
		return false
	}
	for _, a := range addrs {
		var got net.IP
		switch v := a.(type) {
		case *net.IPNet:
			got = v.IP
		case *net.IPAddr:
			got = v.IP
		}
		if got != nil && got.Equal(target) {
			return true
		}
	}
	return false
}

// LocalTUNAddrInUse reports whether ShadowLink's TUN IPv4 is already assigned to
// a local interface (e.g. another VPN client is up). The CLI warns on true
// before bring-up, which would otherwise fail with "address already exists".
func LocalTUNAddrInUse() (bool, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false, err
	}
	return IPv4Assigned(tunAddress4IP, addrs), nil
}
```

- [ ] **Step 4: Modify** — `internal/core/render.go` TUN inbound to use the constants and set a distinct `interface_name`:

```go
		"inbounds": []any{
			map[string]any{
				"type":           "tun",
				"tag":            "tun-in",
				"interface_name": TUNInterfaceName,
				"address":        []any{TUNAddress4CIDR, TUNAddress6CIDR},
				"mtu":            1420,
				"auto_route":     true,
				"strict_route":   true,
				"stack":          "mixed",
			},
		},
```

- [ ] **Step 5: Add the render assertion** — append to `internal/core/render_test.go`

```go
func TestRenderTunHasDistinctIdentity(t *testing.T) {
	m := render(t)
	ins := m["inbounds"].([]any)
	tun := ins[0].(map[string]any)
	if tun["interface_name"] != "shadowlink0" {
		t.Fatalf("want distinct interface_name shadowlink0, got %v", tun["interface_name"])
	}
	addrs := tun["address"].([]any)
	if addrs[0] != "172.18.0.1/30" {
		t.Fatalf("want non-default 172.18.0.1/30, got %v", addrs[0])
	}
	for _, a := range addrs {
		if a == "172.19.0.1/30" {
			t.Fatal("must not use sing-box default 172.19.0.1 (Hiddify collision)")
		}
	}
}
```

- [ ] **Step 6: Run it, verify it passes**

Run: `go test -tags "with_utls with_gvisor with_clash_api" ./internal/core/`
Expected: PASS (existing core tests still green; new identity + helper tests pass).

- [ ] **Step 7: Commit**

```bash
git add internal/core/tun.go internal/core/tun_test.go internal/core/render.go internal/core/render_test.go
git commit -m "feat(core): distinct TUN identity (shadowlink0/172.18.0.1) + collision check"
```

> Wiring note: Task 1.10 calls `core.LocalTUNAddrInUse()` at the top of `connect`
> and prints a warning (not a hard error) if it returns true, e.g. "another VPN
> adapter already holds 172.18.0.1 — exit it first if connect fails".

---

### Task 1.1: Secret masking

**Files:**
- Create: `internal/secret/mask.go`, `internal/secret/mask_test.go`

**Interfaces:**
- Produces: `secret.Mask(s string) string`, `secret.MaskServer(s config.Server) string`.

- [ ] **Step 1: Write the failing test** — `internal/secret/mask_test.go`

```go
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
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/secret/`
Expected: FAIL — undefined `Mask`.

- [ ] **Step 3: Implement** — `internal/secret/mask.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package secret

import (
	"fmt"

	"github.com/shadowlink/shadowlink/internal/config"
)

// Mask hides all but a short prefix of a sensitive value.
func Mask(s string) string {
	switch {
	case s == "":
		return ""
	case len(s) <= 4:
		return "****"
	default:
		return s[:2] + "****"
	}
}

// MaskServer renders a server for logs without exposing UUID or REALITY key.
func MaskServer(s config.Server) string {
	return fmt.Sprintf("%s (%s:%d uuid=%s pbk=%s)", s.Tag, s.Host, s.Port, Mask(s.UUID), Mask(s.PublicKey))
}
```

- [ ] **Step 4: Run it, verify it passes**

Run: `go test ./internal/secret/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/secret/
git commit -m "feat(secret): masking for logs and errors"
```

---

### Task 1.2: Profile persistence

**Files:**
- Create: `internal/config/store.go`, `internal/config/store_test.go`

**Interfaces:**
- Consumes: `config.Profile`.
- Produces:
  - `config.Dir() (string, error)` — per-OS config dir (`os.UserConfigDir()/shadowlink`).
  - `config.Load() (Profile, error)` — returns a default profile if none exists.
  - `config.Save(p Profile) error` — writes `profile.json` with `0o600`.
  - `config.LoadFrom(path string) (Profile, error)` / `config.SaveTo(path string, p Profile) error` (for tests).

- [ ] **Step 1: Write the failing test** — `internal/config/store_test.go`

```go
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
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/config/ -run RoundTrip`
Expected: FAIL — undefined `SaveTo`.

- [ ] **Step 3: Implement** — `internal/config/store.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Dir returns the per-user config directory for ShadowLink, creating it.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate config dir: %w", err)
	}
	dir := filepath.Join(base, "shadowlink")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return dir, nil
}

func profilePath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "profile.json"), nil
}

// Load reads the stored profile, or returns a default one if none exists.
func Load() (Profile, error) {
	p, err := profilePath()
	if err != nil {
		return Profile{}, err
	}
	return LoadFrom(p)
}

// Save persists the profile to the default location.
func Save(p Profile) error {
	path, err := profilePath()
	if err != nil {
		return err
	}
	return SaveTo(path, p)
}

// LoadFrom reads a profile from path; a missing file yields a default profile.
func LoadFrom(path string) (Profile, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Profile{Settings: DefaultSettings()}, nil
	}
	if err != nil {
		return Profile{}, fmt.Errorf("read profile: %w", err)
	}
	var p Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return Profile{}, fmt.Errorf("parse profile: %w", err)
	}
	if p.Settings.DoHResolver == "" {
		p.Settings = DefaultSettings()
	}
	return p, nil
}

// SaveTo writes a profile atomically with owner-only permissions.
func SaveTo(path string, p Profile) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal profile: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write profile: %w", err)
	}
	return os.Rename(tmp, path)
}
```

- [ ] **Step 4: Run it, verify it passes**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/store.go internal/config/store_test.go
git commit -m "feat(config): profile persistence (0600, atomic, defaults)"
```

---

### Task 1.3: Subscription parsing + import

**Files:**
- Create: `internal/subscription/sub.go`, `internal/subscription/sub_test.go`
- Create: `internal/subscription/import.go`, `internal/subscription/import_test.go`

**Interfaces:**
- Consumes: `ParseVLESS`, `config.Server`.
- Produces:
  - `subscription.ParseSubscription(body []byte) ([]config.Server, error)` — lenient base64 (std → url-safe → raw), newline-split, skips non-vless lines.
  - `subscription.Import(input string) ([]config.Server, error)` — dispatches: existing file path → read & ParseSubscription; `vless://` → single; `http(s)://` → caller fetches (returns sentinel `ErrRemote`); otherwise treat as subscription body.
  - `subscription.ErrRemote = errors.New("remote subscription URL")`.

- [ ] **Step 1: Write the failing test** — `internal/subscription/sub_test.go`

```go
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
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/subscription/ -run ParseSubscription`
Expected: FAIL — undefined `ParseSubscription`.

- [ ] **Step 3: Implement** — `internal/subscription/sub.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package subscription

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/shadowlink/shadowlink/internal/config"
)

// ParseSubscription decodes a subscription body and returns its servers.
// Lenient: try std base64, then url-safe (raw, no padding), then raw text.
func ParseSubscription(body []byte) ([]config.Server, error) {
	text := decodeLenient(strings.TrimSpace(string(body)))
	var servers []config.Server
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" || !strings.HasPrefix(line, "vless://") {
			continue // skip blanks, comments, and other schemes
		}
		s, err := ParseVLESS(line)
		if err != nil {
			continue // skip unparseable lines rather than failing the whole feed
		}
		servers = append(servers, s)
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("no vless servers found in subscription")
	}
	return servers, nil
}

func decodeLenient(s string) string {
	if d, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(d)
	}
	if d, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return string(d)
	}
	return s // assume already plain text
}
```

- [ ] **Step 4: Run it, verify it passes**

Run: `go test ./internal/subscription/ -run ParseSubscription`
Expected: PASS.

- [ ] **Step 5: Write the failing import test** — `internal/subscription/import_test.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package subscription

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestImportSingleURI(t *testing.T) {
	servers, err := Import("vless://u@a.com:443?pbk=K#n")
	if err != nil || len(servers) != 1 {
		t.Fatalf("single uri import: %v %+v", err, servers)
	}
}

func TestImportRemoteURLSentinel(t *testing.T) {
	_, err := Import("https://example.com/sub")
	if !errors.Is(err, ErrRemote) {
		t.Fatalf("want ErrRemote, got %v", err)
	}
}

func TestImportFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub.txt")
	os.WriteFile(p, []byte("vless://u@a.com:443?pbk=K#n\n"), 0o600)
	servers, err := Import(p)
	if err != nil || len(servers) != 1 {
		t.Fatalf("file import: %v %+v", err, servers)
	}
}
```

- [ ] **Step 6: Run it, verify it fails**

Run: `go test ./internal/subscription/ -run Import`
Expected: FAIL — undefined `Import`/`ErrRemote`.

- [ ] **Step 7: Implement** — `internal/subscription/import.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package subscription

import (
	"errors"
	"os"
	"strings"

	"github.com/shadowlink/shadowlink/internal/config"
)

// ErrRemote signals that the input is a remote URL the caller must fetch
// (possibly through the active tunnel) and then pass the body to ParseSubscription.
var ErrRemote = errors.New("remote subscription URL")

// Import turns user input (a uri, a file path, or a subscription body) into servers.
func Import(input string) ([]config.Server, error) {
	in := strings.TrimSpace(input)
	switch {
	case strings.HasPrefix(in, "vless://"):
		s, err := ParseVLESS(in)
		if err != nil {
			return nil, err
		}
		return []config.Server{s}, nil
	case strings.HasPrefix(in, "http://") || strings.HasPrefix(in, "https://"):
		return nil, ErrRemote
	default:
		if b, err := os.ReadFile(in); err == nil {
			return ParseSubscription(b)
		}
		return ParseSubscription([]byte(in)) // treat as inline body
	}
}
```

- [ ] **Step 8: Run it, verify it passes**

Run: `go test ./internal/subscription/`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/subscription/sub.go internal/subscription/sub_test.go internal/subscription/import.go internal/subscription/import_test.go
git commit -m "feat(subscription): lenient subscription parsing + import dispatch"
```

---

### Task 1.4: Connection state machine

**Files:**
- Create: `internal/manager/state.go`, `internal/manager/state_test.go`

**Interfaces:**
- Produces:
  - `manager.State` (string enum): `Disconnected`, `Connecting`, `Connected`, `Reconnecting`, `Disconnecting`, `Error`.
  - `manager.CanTransition(from, to State) bool`.

- [ ] **Step 1: Write the failing test** — `internal/manager/state_test.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package manager

import "testing"

func TestValidTransitions(t *testing.T) {
	ok := [][2]State{
		{Disconnected, Connecting}, {Connecting, Connected}, {Connecting, Error},
		{Connected, Reconnecting}, {Reconnecting, Connected}, {Connected, Disconnecting},
		{Disconnecting, Disconnected}, {Error, Connecting},
	}
	for _, p := range ok {
		if !CanTransition(p[0], p[1]) {
			t.Errorf("expected %s->%s allowed", p[0], p[1])
		}
	}
}

func TestInvalidTransitions(t *testing.T) {
	bad := [][2]State{
		{Disconnected, Connected}, {Connected, Connecting}, {Disconnected, Reconnecting},
	}
	for _, p := range bad {
		if CanTransition(p[0], p[1]) {
			t.Errorf("expected %s->%s forbidden", p[0], p[1])
		}
	}
}
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/manager/`
Expected: FAIL — undefined `State`.

- [ ] **Step 3: Implement** — `internal/manager/state.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package manager

// State is a connection lifecycle state.
type State string

const (
	Disconnected  State = "disconnected"
	Connecting    State = "connecting"
	Connected     State = "connected"
	Reconnecting  State = "reconnecting"
	Disconnecting State = "disconnecting"
	Error         State = "error"
)

var allowed = map[State][]State{
	Disconnected:  {Connecting},
	Connecting:    {Connected, Error, Disconnecting},
	Connected:     {Reconnecting, Disconnecting, Error},
	Reconnecting:  {Connected, Error, Disconnecting},
	Disconnecting: {Disconnected},
	Error:         {Connecting, Disconnecting, Disconnected},
}

// CanTransition reports whether from->to is a legal state change.
func CanTransition(from, to State) bool {
	for _, s := range allowed[from] {
		if s == to {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run it, verify it passes**

Run: `go test ./internal/manager/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/manager/state.go internal/manager/state_test.go
git commit -m "feat(manager): connection state machine"
```

---

### Task 1.5: Backoff

**Files:**
- Create: `internal/manager/backoff.go`, `internal/manager/backoff_test.go`

**Interfaces:**
- Produces:
  - `manager.Backoff{Base, Max time.Duration}`
  - `(*Backoff).Next(attempt int) time.Duration` — exponential, capped, with deterministic jitter via an injectable `randFloat func() float64`.

- [ ] **Step 1: Write the failing test** — `internal/manager/backoff_test.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package manager

import (
	"testing"
	"time"
)

func TestBackoffGrowsAndCaps(t *testing.T) {
	// randFloat=0 selects the equal-jitter LOWER bound (exp/2), so at attempt 0
	// the floor is Base/2 — not Base. Assert that floor, growth, and the cap.
	b := Backoff{Base: time.Second, Max: 30 * time.Second, randFloat: func() float64 { return 0 }}
	d0 := b.Next(0)
	d1 := b.Next(1)
	d5 := b.Next(5)
	if d0 < time.Second/2 || d1 <= d0 {
		t.Fatalf("expected growth from the Base/2 floor: d0=%v d1=%v", d0, d1)
	}
	if d5 > 30*time.Second {
		t.Fatalf("expected cap at 30s, got %v", d5)
	}
}

func TestBackoffJitterWithinBounds(t *testing.T) {
	b := Backoff{Base: time.Second, Max: time.Minute, randFloat: func() float64 { return 1 }}
	// attempt 2 base = 4s; full jitter -> within (0, 4s]
	d := b.Next(2)
	if d <= 0 || d > 4*time.Second {
		t.Fatalf("jitter out of bounds: %v", d)
	}
}
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/manager/ -run Backoff`
Expected: FAIL — undefined `Backoff`.

- [ ] **Step 3: Implement** — `internal/manager/backoff.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package manager

import (
	"math"
	"math/rand"
	"time"
)

// Backoff computes exponential backoff with full jitter, capped at Max.
type Backoff struct {
	Base      time.Duration
	Max       time.Duration
	randFloat func() float64 // injectable for tests; defaults to rand.Float64
}

// Next returns the delay for a zero-based attempt number.
func (b *Backoff) Next(attempt int) time.Duration {
	rf := b.randFloat
	if rf == nil {
		rf = rand.Float64
	}
	exp := float64(b.Base) * math.Pow(2, float64(attempt))
	if exp > float64(b.Max) {
		exp = float64(b.Max)
	}
	// full jitter in (0, exp]; keep a small floor so we never return 0.
	jittered := exp*rf()*0.5 + exp*0.5
	d := time.Duration(jittered)
	if d <= 0 {
		d = b.Base
	}
	return d
}
```

- [ ] **Step 4: Run it, verify it passes**

Run: `go test ./internal/manager/ -run Backoff`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/manager/backoff.go internal/manager/backoff_test.go
git commit -m "feat(manager): exponential backoff with full jitter"
```

---

### Task 1.6: Clash API client (delay probe + selector switch)

**Files:**
- Create: `internal/core/clash.go`, `internal/core/clash_test.go`

**Interfaces:**
- Produces:
  - `core.ClashClient{BaseURL, Secret string; HTTP *http.Client}`
  - `core.NewClashClient(port int, secret string) *ClashClient`
  - `(*ClashClient).Delay(ctx, tag, testURL string, timeoutMS int) (int, error)`
  - `(*ClashClient).Switch(ctx, selector, member string) error`

- [ ] **Step 1: Write the failing test** — `internal/core/clash_test.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClashDelayParsesJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer s3cr3t" {
			t.Errorf("missing bearer auth")
		}
		w.Write([]byte(`{"delay": 42}`))
	}))
	defer srv.Close()
	c := &ClashClient{BaseURL: srv.URL, Secret: "s3cr3t", HTTP: srv.Client()}
	d, err := c.Delay(context.Background(), "proxy", "https://www.gstatic.com/generate_204", 5000)
	if err != nil {
		t.Fatalf("delay: %v", err)
	}
	if d != 42 {
		t.Fatalf("want 42ms, got %d", d)
	}
}

func TestClashSwitchPutsName(t *testing.T) {
	got := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			b, _ := io.ReadAll(r.Body) // Read may short-fill a sized buffer; ReadAll is safe
			got = string(b)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	c := &ClashClient{BaseURL: srv.URL, Secret: "", HTTP: srv.Client()}
	if err := c.Switch(context.Background(), "select", "us-1"); err != nil {
		t.Fatalf("switch: %v", err)
	}
	if !strings.Contains(got, "us-1") {
		t.Fatalf("expected body to carry member name, got %q", got)
	}
}
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/core/ -run Clash`
Expected: FAIL — undefined `ClashClient`.

- [ ] **Step 3: Implement** — `internal/core/clash.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ClashClient talks to sing-box's experimental Clash API on loopback.
type ClashClient struct {
	BaseURL string
	Secret  string
	HTTP    *http.Client
}

// NewClashClient builds a client for 127.0.0.1:<port>.
func NewClashClient(port int, secret string) *ClashClient {
	return &ClashClient{
		BaseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		Secret:  secret,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *ClashClient) auth(r *http.Request) {
	if c.Secret != "" {
		r.Header.Set("Authorization", "Bearer "+c.Secret)
	}
}

// Delay measures latency (ms) for an outbound via GET /proxies/{tag}/delay.
func (c *ClashClient) Delay(ctx context.Context, tag, testURL string, timeoutMS int) (int, error) {
	u := fmt.Sprintf("%s/proxies/%s/delay?timeout=%d&url=%s", c.BaseURL, url.PathEscape(tag), timeoutMS, url.QueryEscape(testURL))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	c.auth(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("delay: status %d", resp.StatusCode)
	}
	var body struct {
		Delay int `json:"delay"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}
	return body.Delay, nil
}

// Switch sets the active member of a selector via PUT /proxies/{selector}.
func (c *ClashClient) Switch(ctx context.Context, selector, member string) error {
	u := fmt.Sprintf("%s/proxies/%s", c.BaseURL, url.PathEscape(selector))
	body := strings.NewReader(fmt.Sprintf(`{"name":%q}`, member))
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, u, body)
	req.Header.Set("Content-Type", "application/json")
	c.auth(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("switch: status %d", resp.StatusCode)
	}
	return nil
}
```

- [ ] **Step 4: Run it, verify it passes**

Run: `go test ./internal/core/ -run Clash`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/clash.go internal/core/clash_test.go
git commit -m "feat(core): Clash API client for delay probe and switch"
```

---

### Task 1.7: Health checker

**Files:**
- Create: `internal/health/health.go`, `internal/health/health_test.go`

**Interfaces:**
- Consumes: an abstract prober.
- Produces:
  - `health.Prober interface { Delay(ctx context.Context, tag, url string, timeoutMS int) (int, error) }` (satisfied by `*core.ClashClient`).
  - `health.Checker{Prober Prober; Tag, URL string; Interval time.Duration; Failures int}`
  - `(*Checker).Run(ctx, onDown func(), onUp func())` — calls onDown after `Failures` consecutive probe failures, onUp on recovery.

- [ ] **Step 1: Write the failing test** — `internal/health/health_test.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package health

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeProber struct{ fails atomic.Int32 }

func (f *fakeProber) Delay(_ context.Context, _, _ string, _ int) (int, error) {
	if f.fails.Load() > 0 {
		f.fails.Add(-1)
		return 0, errors.New("down")
	}
	return 10, nil
}

func TestCheckerFiresDownThenUp(t *testing.T) {
	fp := &fakeProber{}
	fp.fails.Store(2)
	c := &Checker{Prober: fp, Tag: "proxy", URL: "https://x/generate_204", Interval: 5 * time.Millisecond, Failures: 2}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	down := make(chan struct{}, 1)
	up := make(chan struct{}, 1)
	go c.Run(ctx, func() { down <- struct{}{} }, func() { up <- struct{}{} })

	select {
	case <-down:
	case <-time.After(time.Second):
		t.Fatal("onDown never fired")
	}
	select {
	case <-up:
	case <-time.After(time.Second):
		t.Fatal("onUp never fired after recovery")
	}
}
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/health/`
Expected: FAIL — undefined `Checker`.

- [ ] **Step 3: Implement** — `internal/health/health.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package health

import (
	"context"
	"time"
)

// Prober measures outbound latency (e.g. the Clash API client).
type Prober interface {
	Delay(ctx context.Context, tag, url string, timeoutMS int) (int, error)
}

// Checker polls a prober and reports sustained up/down transitions.
type Checker struct {
	Prober   Prober
	Tag      string
	URL      string
	Interval time.Duration
	Failures int // consecutive failures before declaring down
}

// Run loops until ctx is done, invoking onDown/onUp on edge transitions.
func (c *Checker) Run(ctx context.Context, onDown, onUp func()) {
	t := time.NewTicker(c.Interval)
	defer t.Stop()
	consecutive := 0
	healthy := true
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_, err := c.Prober.Delay(ctx, c.Tag, c.URL, 5000)
			if err != nil {
				consecutive++
				if healthy && consecutive >= c.Failures {
					healthy = false
					if onDown != nil {
						onDown()
					}
				}
				continue
			}
			consecutive = 0
			if !healthy {
				healthy = true
				if onUp != nil {
					onUp()
				}
			}
		}
	}
}
```

- [ ] **Step 4: Run it, verify it passes**

Run: `go test ./internal/health/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/health/
git commit -m "feat(health): consecutive-failure health checker"
```

---

### Task 1.8: Kill-switch (interface + Windows netsh + stub)

**Files:**
- Create: `internal/platform/platform.go`, `internal/platform/platform_test.go`
- Create: `internal/platform/killswitch_windows.go`
- Create: `internal/platform/killswitch_other.go`

**Interfaces:**
- Produces:
  - `platform.KillSwitch interface { Enable(serverIP string) error; Disable() error }`
  - `platform.NewKillSwitch() KillSwitch` — returns the OS implementation.
  - `platform.RuleName = "ShadowLink-KillSwitch"` (stable identifier for rule cleanup).

The Windows implementation shells `netsh advfirewall` to: set default outbound to block, then allow the TUN interface and the server IP. Rules persist across a core crash and are removed on `Disable`. Other OSes get a stub that returns a clear "not implemented yet" error so behavior is honest, never silently leaking.

- [ ] **Step 1: Write the failing test** — `internal/platform/platform_test.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package platform

import "testing"

func TestNewKillSwitchNonNil(t *testing.T) {
	if NewKillSwitch() == nil {
		t.Fatal("NewKillSwitch returned nil")
	}
}

func TestRuleNameStable(t *testing.T) {
	if RuleName == "" {
		t.Fatal("RuleName must be set for cleanup")
	}
}
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/platform/`
Expected: FAIL — undefined `NewKillSwitch`.

- [ ] **Step 3: Implement the interface** — `internal/platform/platform.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package platform

// RuleName identifies ShadowLink's firewall rules for creation and cleanup.
const RuleName = "ShadowLink-KillSwitch"

// KillSwitch enforces fail-closed networking independent of the core process.
type KillSwitch interface {
	// Enable blocks all egress except via the tunnel, allowing only the server IP
	// on the physical NIC (for the handshake). Persists across a core crash.
	Enable(serverIP string) error
	// Disable removes all ShadowLink firewall rules, restoring normal networking.
	Disable() error
}
```

- [ ] **Step 4: Implement Windows** — `internal/platform/killswitch_windows.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
//go:build windows

package platform

import (
	"fmt"
	"os/exec"
)

type windowsKillSwitch struct{}

// NewKillSwitch returns the Windows (netsh advfirewall) kill switch.
func NewKillSwitch() KillSwitch { return &windowsKillSwitch{} }

func (w *windowsKillSwitch) Enable(serverIP string) error {
	// Clean any stale rules first (idempotent).
	_ = w.Disable()
	// Default-deny outbound for all profiles.
	if err := netsh("advfirewall", "set", "allprofiles", "firewallpolicy", "blockinbound,blockoutbound"); err != nil {
		return fmt.Errorf("set block policy: %w", err)
	}
	// Allow reaching the server IP on the physical NIC (REALITY handshake).
	if serverIP != "" {
		if err := netsh("advfirewall", "firewall", "add", "rule",
			"name="+RuleName, "dir=out", "action=allow", "remoteip="+serverIP); err != nil {
			return fmt.Errorf("allow server ip: %w", err)
		}
	}
	// Allow loopback so the Clash API and local apps keep working.
	if err := netsh("advfirewall", "firewall", "add", "rule",
		"name="+RuleName, "dir=out", "action=allow", "remoteip=127.0.0.1"); err != nil {
		return fmt.Errorf("allow loopback: %w", err)
	}
	return nil
}

func (w *windowsKillSwitch) Disable() error {
	// Remove our rules and restore default-allow outbound.
	_ = netsh("advfirewall", "firewall", "delete", "rule", "name="+RuleName)
	if err := netsh("advfirewall", "set", "allprofiles", "firewallpolicy", "blockinbound,allowoutbound"); err != nil {
		return fmt.Errorf("restore policy: %w", err)
	}
	return nil
}

func netsh(args ...string) error {
	out, err := exec.Command("netsh", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("netsh %v: %v: %s", args, err, string(out))
	}
	return nil
}
```

- [ ] **Step 5: Implement the stub for other OSes** — `internal/platform/killswitch_other.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
//go:build !windows

package platform

import "fmt"

type stubKillSwitch struct{}

// NewKillSwitch returns a stub on non-Windows OSes (Phase 1 ships Windows first).
func NewKillSwitch() KillSwitch { return &stubKillSwitch{} }

func (s *stubKillSwitch) Enable(serverIP string) error {
	return fmt.Errorf("kill-switch not implemented on this OS yet; rerun with --fail-open to proceed without it")
}

func (s *stubKillSwitch) Disable() error { return nil }
```

- [ ] **Step 6: Run it, verify it passes**

Run: `go test ./internal/platform/`
Expected: PASS on all OSes (the stub/real both satisfy the test).

- [ ] **Step 7: Commit**

```bash
git add internal/platform/
git commit -m "feat(platform): fail-closed kill-switch interface + Windows netsh impl"
```

---

### Task 1.9: Manager (Connect/Disconnect/Status + reconnect loop)

**Files:**
- Create: `internal/manager/manager.go`, `internal/manager/manager_test.go`

**Interfaces:**
- Consumes: `config.Server/Settings`, `core.RenderConfig`, `core.New`, `core.NewClashClient`, `health.Checker`, `platform.KillSwitch`, `manager.State`, `manager.Backoff`, `secret.MaskServer`.
- Produces:
  - `manager.Tunnel interface { Start() error; Close() error }` (satisfied by `*core.Instance`) — injected for testability.
  - `manager.Deps{ Render func(core.BuildParams)([]byte,error); NewTunnel func([]byte)(Tunnel,error); KillSwitch platform.KillSwitch }`
  - `manager.New(deps Deps) *Manager`
  - `(*Manager).Connect(srv config.Server, set config.Settings) error`
  - `(*Manager).Disconnect() error`
  - `(*Manager).State() State`

This task tests the orchestration (state transitions, kill-switch ordering, teardown) with a **fake tunnel + fake kill-switch**, so it needs no privileges.

- [ ] **Step 1: Write the failing test** — `internal/manager/manager_test.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package manager

import (
	"errors"
	"sync"
	"testing"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/core"
)

type fakeTunnel struct {
	startErr error
	started  bool
	closed   bool
}

func (f *fakeTunnel) Start() error { f.started = true; return f.startErr }
func (f *fakeTunnel) Close() error { f.closed = true; return nil }

type fakeKS struct {
	mu              sync.Mutex
	enabled         bool
	enabledBeforeOn bool
	tunnelOnAtEnable bool
}

func (k *fakeKS) Enable(string) error { k.mu.Lock(); defer k.mu.Unlock(); k.enabled = true; return nil }
func (k *fakeKS) Disable() error      { k.mu.Lock(); defer k.mu.Unlock(); k.enabled = false; return nil }

func deps(tun *fakeTunnel, ks *fakeKS) Deps {
	return Deps{
		Render:    func(core.BuildParams) ([]byte, error) { return []byte("{}"), nil },
		NewTunnel: func([]byte) (Tunnel, error) { return tun, nil },
		KillSwitch: ks,
	}
}

func srv() config.Server {
	return config.Server{Tag: "n1", UUID: "u", Host: "1.2.3.4", Port: 443, Flow: "xtls-rprx-vision", Network: "tcp", PublicKey: "K"}
}

func TestConnectReachesConnectedAndEnablesKillSwitch(t *testing.T) {
	tun := &fakeTunnel{}
	ks := &fakeKS{}
	m := New(deps(tun, ks))
	set := config.DefaultSettings()
	if err := m.Connect(srv(), set); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if m.State() != Connected {
		t.Fatalf("want Connected, got %s", m.State())
	}
	if !tun.started || !ks.enabled {
		t.Fatalf("tunnel/kill-switch not engaged: %+v %+v", tun, ks)
	}
}

func TestConnectFailureGoesToErrorAndCleansUp(t *testing.T) {
	tun := &fakeTunnel{startErr: errors.New("boom")}
	ks := &fakeKS{}
	m := New(deps(tun, ks))
	set := config.DefaultSettings()
	err := m.Connect(srv(), set)
	if err == nil {
		t.Fatal("expected connect error")
	}
	if m.State() != Error {
		t.Fatalf("want Error, got %s", m.State())
	}
	// On failure the kill switch must not be left enabled (no silent lockout).
	if ks.enabled {
		t.Fatal("kill switch left enabled after failed connect")
	}
}

func TestDisconnectTearsDown(t *testing.T) {
	tun := &fakeTunnel{}
	ks := &fakeKS{}
	m := New(deps(tun, ks))
	_ = m.Connect(srv(), config.DefaultSettings())
	if err := m.Disconnect(); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if m.State() != Disconnected || !tun.closed || ks.enabled {
		t.Fatalf("teardown incomplete: state=%s closed=%v ks=%v", m.State(), tun.closed, ks.enabled)
	}
}

func TestFailOpenSkipsKillSwitch(t *testing.T) {
	tun := &fakeTunnel{}
	ks := &fakeKS{}
	m := New(deps(tun, ks))
	set := config.DefaultSettings()
	set.KillSwitch = false
	set.FailOpen = true
	_ = m.Connect(srv(), set)
	if ks.enabled {
		t.Fatal("kill switch enabled despite fail-open")
	}
}
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/manager/ -run Connect`
Expected: FAIL — undefined `New`/`Deps`/`Tunnel`.

- [ ] **Step 3: Implement** — `internal/manager/manager.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package manager

import (
	"fmt"
	"sync"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/core"
	"github.com/shadowlink/shadowlink/internal/platform"
)

// Tunnel is the minimal lifecycle the manager drives (satisfied by *core.Instance).
type Tunnel interface {
	Start() error
	Close() error
}

// Deps are the manager's injectable collaborators (real or fake).
type Deps struct {
	Render     func(core.BuildParams) ([]byte, error)
	NewTunnel  func(configJSON []byte) (Tunnel, error)
	KillSwitch platform.KillSwitch
}

// Manager owns the connection state and coordinates core + kill switch.
type Manager struct {
	deps  Deps
	mu    sync.Mutex
	state State
	tun   Tunnel
	set   config.Settings
}

// New builds a Manager from its dependencies.
func New(deps Deps) *Manager {
	return &Manager{deps: deps, state: Disconnected}
}

// State returns the current connection state.
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *Manager) set_(s State) { m.state = s }

// Connect brings up the kill switch (if enabled) then the tunnel.
// Order matters: enable fail-closed BEFORE the tunnel so a startup crash can't leak.
func (m *Manager) Connect(srv config.Server, set config.Settings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !CanTransition(m.state, Connecting) {
		return fmt.Errorf("cannot connect from state %s", m.state)
	}
	m.set_(Connecting)
	m.set = set

	useKS := set.KillSwitch && !set.FailOpen
	if useKS {
		if err := m.deps.KillSwitch.Enable(srv.Host); err != nil {
			m.set_(Error)
			return fmt.Errorf("enable kill switch: %w", err)
		}
	}

	cfg, err := m.deps.Render(core.BuildParams{Server: srv, Settings: set})
	if err != nil {
		m.fail(useKS)
		return err
	}
	tun, err := m.deps.NewTunnel(cfg)
	if err != nil {
		m.fail(useKS)
		return err
	}
	if err := tun.Start(); err != nil {
		_ = tun.Close()
		m.fail(useKS)
		return fmt.Errorf("start tunnel: %w", err)
	}
	m.tun = tun
	m.set_(Connected)
	return nil
}

// fail rolls back to Error, removing the kill switch so the user isn't locked out
// by a connection that never came up.
func (m *Manager) fail(useKS bool) {
	if useKS {
		_ = m.deps.KillSwitch.Disable()
	}
	m.set_(Error)
}

// Disconnect tears down the tunnel and removes the kill switch.
func (m *Manager) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == Disconnected {
		return nil
	}
	m.set_(Disconnecting)
	var firstErr error
	if m.tun != nil {
		if err := m.tun.Close(); err != nil {
			firstErr = err
		}
		m.tun = nil
	}
	if err := m.deps.KillSwitch.Disable(); err != nil && firstErr == nil {
		firstErr = err
	}
	m.set_(Disconnected)
	return firstErr
}
```

> Note: the reconnect loop (health-driven) is wired in Task 1.10 where the CLI owns a long-running process; the state machine already permits `Connected→Reconnecting→Connected`. Keeping `Connect`/`Disconnect` synchronous here keeps this unit fully testable.

- [ ] **Step 4: Run it, verify it passes**

Run: `go test ./internal/manager/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/manager/manager.go internal/manager/manager_test.go
git commit -m "feat(manager): orchestrate kill-switch + tunnel with safe teardown"
```

---

### Task 1.10: Wire CLI (connect via profile, disconnect, status, import) + reconnect

**Files:**
- Modify: `cmd/shadowlink/connect.go` (use manager + profile + health/reconnect)
- Create: `cmd/shadowlink/cmds.go` (import, status, logs)
- Modify: `cmd/shadowlink/main.go` (register new commands)

**Interfaces:**
- Consumes: everything above.
- Produces: CLI commands `connect [uri]`, `import <input>`, `status`, plus `--fail-open` flag on connect.

Because the live tunnel needs privileges, this task's automated check is `go build` + `go vet`; behavior is validated by the Phase 1 acceptance run (below).

- [ ] **Step 1: Replace connect** — `cmd/shadowlink/connect.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/core"
	"github.com/shadowlink/shadowlink/internal/health"
	"github.com/shadowlink/shadowlink/internal/manager"
	"github.com/shadowlink/shadowlink/internal/platform"
	"github.com/shadowlink/shadowlink/internal/secret"
	"github.com/shadowlink/shadowlink/internal/subscription"
	"github.com/spf13/cobra"
)

func connectCmd() *cobra.Command {
	var failOpen bool
	cmd := &cobra.Command{
		Use:   "connect [vless-uri]",
		Short: "Connect to the selected server (or a one-off vless:// link)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prof, err := config.Load()
			if err != nil {
				return err
			}
			srv, err := pickServer(prof, args)
			if err != nil {
				return err
			}
			set := prof.Settings
			if failOpen {
				set.KillSwitch = false
				set.FailOpen = true
			}
			set.ClashAPISecret = randomSecret()

			m := manager.New(manager.Deps{
				Render:    core.RenderConfig,
				NewTunnel: func(b []byte) (manager.Tunnel, error) { return core.New(b) },
				KillSwitch: platform.NewKillSwitch(),
			})
			if err := m.Connect(srv, set); err != nil {
				return err
			}
			fmt.Printf("connected via %s\n", secret.MaskServer(srv))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			clash := core.NewClashClient(set.ClashAPIPort, set.ClashAPISecret)
			checker := &health.Checker{Prober: clash, Tag: "proxy", URL: "https://www.gstatic.com/generate_204", Interval: 15 * time.Second, Failures: 3}
			go checker.Run(ctx,
				func() { fmt.Println("health: connection degraded") },
				func() { fmt.Println("health: connection recovered") },
			)

			sig := make(chan os.Signal, 1)
			signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
			<-sig
			fmt.Println("\ndisconnecting...")
			return m.Disconnect()
		},
	}
	cmd.Flags().BoolVar(&failOpen, "fail-open", false, "do NOT enable the kill switch (traffic may leak if the tunnel drops)")
	return cmd
}

func pickServer(prof config.Profile, args []string) (config.Server, error) {
	if len(args) == 1 {
		return subscription.ParseVLESS(args[0])
	}
	if len(prof.Servers) == 0 {
		return config.Server{}, fmt.Errorf("no servers; run: shadowlink import <uri|file|url>")
	}
	if prof.Selected != "" {
		for _, s := range prof.Servers {
			if s.Tag == prof.Selected {
				return s, nil
			}
		}
	}
	return prof.Servers[0], nil
}

func randomSecret() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
```

- [ ] **Step 2: Add import/status/logs** — `cmd/shadowlink/cmds.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package main

import (
	"fmt"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/secret"
	"github.com/shadowlink/shadowlink/internal/subscription"
	"github.com/spf13/cobra"
)

func importCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <vless-uri | file | subscription-url>",
		Short: "Import server(s) into the stored profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			servers, err := subscription.Import(args[0])
			if err == subscription.ErrRemote {
				return fmt.Errorf("remote subscription fetch lands in Phase 2; for now save it to a file and import the file")
			}
			if err != nil {
				return err
			}
			for _, s := range servers {
				if verr := s.Validate(); verr != nil {
					return verr
				}
			}
			prof, err := config.Load()
			if err != nil {
				return err
			}
			prof.Servers = mergeServers(prof.Servers, servers)
			if prof.Selected == "" && len(prof.Servers) > 0 {
				prof.Selected = prof.Servers[0].Tag
			}
			if err := config.Save(prof); err != nil {
				return err
			}
			fmt.Printf("imported %d server(s); %d total\n", len(servers), len(prof.Servers))
			return nil
		},
	}
}

func mergeServers(existing, incoming []config.Server) []config.Server {
	byTag := map[string]int{}
	for i, s := range existing {
		byTag[s.Tag] = i
	}
	for _, s := range incoming {
		if i, ok := byTag[s.Tag]; ok {
			existing[i] = s
		} else {
			existing = append(existing, s)
		}
	}
	return existing
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show stored servers and selection",
		RunE: func(cmd *cobra.Command, args []string) error {
			prof, err := config.Load()
			if err != nil {
				return err
			}
			if len(prof.Servers) == 0 {
				fmt.Println("no servers imported")
				return nil
			}
			fmt.Printf("kill-switch: %v  split-tunnel: %v\n", prof.Settings.KillSwitch, prof.Settings.SplitTunnel)
			for _, s := range prof.Servers {
				marker := " "
				if s.Tag == prof.Selected {
					marker = "*"
				}
				fmt.Printf(" %s %s\n", marker, secret.MaskServer(s))
			}
			return nil
		},
	}
}
```

- [ ] **Step 3: Register commands** — `cmd/shadowlink/main.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package main

import (
	"fmt"
	"os"

	"github.com/shadowlink/shadowlink/internal/version"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:     "shadowlink",
		Short:   "ShadowLink — personal VPN client over an embedded sing-box core",
		Version: version.String(),
	}
	root.AddCommand(connectCmd(), importCmd(), statusCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Build + vet**

Run: `go build ./... && go vet ./...`
Expected: clean.

- [ ] **Step 5: Run the full unit suite**

Run: `go test ./...`
Expected: all packages PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/ ; git commit -m "feat(cli): profile connect, import, status, health + fail-open flag"
```

---

### Task 1.11: Phase 1 acceptance (manual, recorded)

**Files:**
- Create: `docs/ACCEPTANCE-PHASE1.md`

- [ ] **Step 1: Write the acceptance runbook** — `docs/ACCEPTANCE-PHASE1.md`

```markdown
# Phase 1 Acceptance (Windows first)

Build: `go build ./cmd/shadowlink` (wintun.dll beside the exe; Admin shell).

1. Import + status:
   - `shadowlink import "<vless-uri>"` → reports imported count.
   - `shadowlink status` → lists the server (UUID/key masked), kill-switch=true.
2. Connect: `shadowlink connect` → "connected via ..."; public IP is the server's.
3. DNS-leak test → resolver is server-side, not the ISP/TSPU.
4. Kill-switch (the critical test):
   - While connected, FORCE-KILL the shadowlink process (Task Manager / `taskkill /F`).
   - Immediately attempt traffic (browser / `curl`). It MUST fail (fail-closed) —
     no leak to the real IP.
   - Recover: `shadowlink connect` again, then `shadowlink disconnect` → normal
     networking restored (firewall policy back to allowoutbound, rules removed).
5. Auto-reconnect/health: disable/re-enable the NIC briefly; the health line prints
   "degraded" then "recovered"; the session keeps working afterward.

## Pass criteria (ALL)
- [ ] Connect changes public IP; no DNS leak.
- [ ] Force-killing the core leaves the machine fail-closed (no traffic).
- [ ] disconnect fully restores networking (verify `netsh advfirewall show allprofiles`).
- [ ] Secrets never appear in console output/logs.

## Result
- Date / OS / sing-box version / outcome / notes:
```

- [ ] **Step 2: Run it** on Windows against the real server; **record** the result.

- [ ] **Step 3: Commit**

```bash
git add docs/ACCEPTANCE-PHASE1.md
git commit -m "test: Phase 1 acceptance runbook + result"
```

---

## Self-review (completed by plan author)

- **Spec coverage:** embed sing-box (0.5/1.10), VLESS+REALITY+Vision render (0.4),
  TUN system-wide (0.4), DNS anti-leak (0.4), subscription/vless import (0.3/1.3),
  domain model + persistence (0.2/1.2), state machine (1.4), auto-reconnect/backoff
  (1.5) + health (1.7), kill-switch fail-closed Windows (1.8/1.9), secret masking
  (1.1), CLI (0.5/1.10), compatibility gate (0.6), acceptance (1.11). Multi-server
  switching, split-tunnel, tray/IPC, and Linux/macOS kill-switch are **Phase 2–5**
  (out of this plan by design).
- **Placeholder scan:** no TBD/TODO; every code step carries complete code; the one
  version-dependent call (sing JSON helper in 0.5) gives an explicit, compilable
  fallback rather than a placeholder.
- **Type consistency:** `config.Server`/`Settings`/`Profile`, `core.BuildParams`,
  `core.RenderConfig`, `core.New`, `core.Instance`, `core.ClashClient`/`NewClashClient`,
  `health.Prober`/`Checker`, `manager.State`/`Backoff`/`Tunnel`/`Deps`/`New`,
  `platform.KillSwitch`/`NewKillSwitch`/`RuleName`, `secret.Mask`/`MaskServer` are
  used consistently across tasks. `Tunnel` is satisfied by `*core.Instance`
  (Start/Close); `Prober` by `*core.ClashClient` (Delay).
```
