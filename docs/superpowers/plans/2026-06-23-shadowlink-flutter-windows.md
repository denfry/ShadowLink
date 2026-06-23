# ShadowLink Flutter (Windows-first) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Windows Flutter app that connects/disconnects to a selected server and shows live status, driving the embedded sing-box core through a shared Go c-shared library over `dart:ffi` — reusing all Phase 1 `internal/*` packages.

**Architecture:** Three front-ends over one engine. Phase 1's `internal/*` is the engine. A new pure-Go `internal/ffiapi` exposes a JSON API over it (unit-tested with fakes). A thin cgo shim `cmd/libshadowlink` re-exports `ffiapi` as a C ABI built `-buildmode=c-shared` into `shadowlink_core.dll`. A Flutter app calls that DLL via `dart:ffi`.

**Tech Stack:** Go 1.24+, sing-box v1.13.7 (tags `with_utls with_gvisor with_clash_api`), cgo `-buildmode=c-shared` (mingw-w64 on the build machine), Flutter (stable, Windows desktop), Dart `package:ffi`.

## Global Constraints

These apply to **every** task; copied verbatim from the spec (`docs/superpowers/specs/2026-06-23-shadowlink-flutter-windows-design.md`) and the Phase 1 plan.

- Go **1.24+**; sing-box pinned **v1.13.7**; build/test the engine with tags **`with_utls with_gvisor with_clash_api`** (REALITY needs `with_utls`).
- Every Go file starts with `// SPDX-License-Identifier: GPL-3.0-only`. License **GPL-3.0**.
- **Secrets** (UUID, REALITY keys, subscription URLs, ClashAPI secret) are **masked** in every FFI response/error via `secret.Mask`/`secret.MaskServer`. Never leak them.
- `internal/ffiapi` is **pure Go** — it MUST NOT `import "C"` and MUST be unit-testable without cgo, a DLL, privileges, or a real tunnel (inject fakes via `manager.Deps`, exactly like Phase 1's manager tests).
- The cgo shim `cmd/libshadowlink` is **build-tag-gated** (`//go:build libshadowlink`) with a non-cgo stub (`//go:build !libshadowlink`) so default `go build ./...` / `go test ./...` / CI stay green in environments without a working 64-bit C toolchain.
- FFI string memory: incoming `char*` → `C.GoString`; outgoing → `C.CString` (caller frees via `SL_FreeString`). No Go pointers retained by C.
- Every FFI response is a JSON object: `{"ok":true,...}` or `{"ok":false,"error":"<masked>"[,"needsAdmin":bool]}`.
- **Environment reality:** this sandbox CANNOT build the c-shared DLL (no 64-bit mingw-w64) or Flutter. Phase A (Go `ffiapi`) is fully TDD'd here. Phase B (cgo shim, Flutter, build script) is **authored** here and **verified on the user's Windows machine** with the commands each task names. Do not claim a Phase B task is "passing" from this environment — its verification is the user's build output.
- TDD for all Phase A Go code: failing test → run-it-fails → minimal impl → run-it-passes → commit.

## File structure

```
internal/ffiapi/ffiapi.go        API type + Deps + Default(); response helpers      (Phase A, Go, tested)
internal/ffiapi/profile.go       ListServers/Import/Select over the profile          (Phase A, Go, tested)
internal/ffiapi/conn.go          Start/Stop/Status over the manager                  (Phase A, Go, tested)
internal/ffiapi/*_test.go        unit tests (fakes; no cgo/DLL/privileges)            (Phase A, Go, tested)
cmd/libshadowlink/shim.go        cgo //export wrappers (//go:build libshadowlink)    (Phase B, user-built)
cmd/libshadowlink/stub.go        empty main (//go:build !libshadowlink)              (Phase B, here-built)
build/build_windows.ps1          build the DLL + copy DLLs into the runner           (Phase B, user-run)
app/pubspec.yaml                 Flutter manifest (dependency: ffi)                   (Phase B, user-built)
app/lib/main.dart                entry, theme, navigation                            (Phase B, user-built)
app/lib/ffi/core.dart            CoreApi interface + models + FakeCore                (Phase B, user-built)
app/lib/ffi/core_ffi.dart        dart:ffi ShadowlinkCore (DynamicLibrary)            (Phase B, user-built)
app/lib/state/connection_controller.dart  ChangeNotifier + status polling            (Phase B, user-built)
app/lib/ui/home_page.dart        connect toggle + status                             (Phase B, user-built)
app/lib/ui/servers_page.dart     list + import + select                              (Phase B, user-built)
app/test/connection_controller_test.dart  widget/unit test against FakeCore          (Phase B, user-verified)
docs/ACCEPTANCE-FLUTTER-WINDOWS.md  manual GUI acceptance runbook                    (Phase B, doc)
```

---

# PHASE A — Go FFI engine (TDD here)

Outcome: `internal/ffiapi` fully implements the JSON API over the Phase 1 engine, unit-tested with fakes, plus the build-safe cgo shim scaffolding. This is the entire verifiable-here deliverable.

---

### Task A1: `ffiapi` scaffold — API type, Deps, response helpers, Version

**Files:**
- Create: `internal/ffiapi/ffiapi.go`, `internal/ffiapi/ffiapi_test.go`

**Interfaces:**
- Consumes: `version.String`, `secret.Mask`, `manager.Deps`, `manager.Tunnel`, `platform.KillSwitch`, `config.Profile`, `config.Server`, `core.BuildParams`.
- Produces:
  - `ffiapi.Prober interface { Delay(ctx context.Context, tag, url string, timeoutMS int) (int, error) }`
  - `ffiapi.API` struct with injectable fields (see code).
  - `ffiapi.Default() *API` — real deps.
  - `(*API).Version() string` → `{"ok":true,"version":"..."}`.
  - response helpers `okJSON(map[string]any) string`, `errJSON(err error, needsAdmin bool) string` (masking applied).

- [ ] **Step 1: Write the failing test** — `internal/ffiapi/ffiapi_test.go`

```go
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
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/ffiapi/`
Expected: FAIL — undefined `Default`/`errJSON`.

- [ ] **Step 3: Implement** — `internal/ffiapi/ffiapi.go`

```go
// SPDX-License-Identifier: GPL-3.0-only

// Package ffiapi is the pure-Go JSON API over the ShadowLink engine. It holds
// the single live tunnel and renders every result as a JSON string, so a thin
// cgo shim can re-export it as a C ABI. It imports no cgo and is fully
// unit-testable with fakes (no DLL, no privileges, no real tunnel).
package ffiapi

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/core"
	"github.com/shadowlink/shadowlink/internal/manager"
	"github.com/shadowlink/shadowlink/internal/platform"
	"github.com/shadowlink/shadowlink/internal/secret"
	"github.com/shadowlink/shadowlink/internal/subscription"
	"github.com/shadowlink/shadowlink/internal/version"
)

// Prober measures outbound latency (satisfied by *core.ClashClient).
type Prober interface {
	Delay(ctx context.Context, tag, url string, timeoutMS int) (int, error)
}

// API holds the engine's injectable collaborators and the single live tunnel.
type API struct {
	// injectable collaborators (real in Default(), fakes in tests)
	render     func(core.BuildParams) ([]byte, error)
	newTunnel  func([]byte) (manager.Tunnel, error)
	newKS      func() platform.KillSwitch
	newProber  func(port int, secret string) Prober
	load       func() (config.Profile, error)
	save       func(config.Profile) error
	importFn   func(string) ([]config.Server, error)
	randSecret func() string

	mu     sync.Mutex
	mgr    *manager.Manager
	prober Prober
	active config.Server // the server the live tunnel was started with
}

// Default wires the real engine collaborators.
func Default() *API {
	return &API{
		render:     core.RenderConfig,
		newTunnel:  func(b []byte) (manager.Tunnel, error) { return core.New(b) },
		newKS:      platform.NewKillSwitch,
		newProber:  func(port int, sec string) Prober { return core.NewClashClient(port, sec) },
		load:       config.Load,
		save:       config.Save,
		importFn:   subscription.Import,
		randSecret: randomSecret,
	}
}

// Version reports the build version (also a trivial FFI smoke check).
func (a *API) Version() string {
	return okJSON(map[string]any{"version": version.String()})
}

func okJSON(extra map[string]any) string {
	m := map[string]any{"ok": true}
	for k, v := range extra {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// errJSON renders a failure, masking any secret-looking content in the message.
func errJSON(err error, needsAdmin bool) string {
	m := map[string]any{"ok": false, "error": secret.Scrub(err.Error())}
	if needsAdmin {
		m["needsAdmin"] = true
	}
	b, _ := json.Marshal(m)
	return string(b)
}
```

> Note: `secret.Scrub` does not exist yet — Task A2 step 0 adds it. For A1, replace `secret.Scrub(err.Error())` with `secret.Mask(err.Error())` to compile and pass; A2 introduces `Scrub` and swaps it in. (Keeping A1 self-contained.)

Also add `randomSecret` (engine-local, mirrors the CLI) at the bottom of `ffiapi.go`:

```go
import (
	"crypto/rand"
	"encoding/hex"
)

func randomSecret() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "shadowlink-fallback-secret"
	}
	return hex.EncodeToString(b)
}
```

(Merge the `crypto/rand`/`encoding/hex` imports into the single import block.)

- [ ] **Step 4: Run it, verify it passes**

Run: `go test ./internal/ffiapi/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ffiapi/ffiapi.go internal/ffiapi/ffiapi_test.go
git commit -m "feat(ffiapi): API scaffold, Deps, JSON helpers, Version"
```

---

### Task A2: secret scrubber + profile ops (ListServers / Import / Select)

**Files:**
- Create: `internal/secret/scrub.go`, `internal/secret/scrub_test.go`
- Create: `internal/ffiapi/profile.go`, `internal/ffiapi/profile_test.go`
- Modify: `internal/ffiapi/ffiapi.go` (use `secret.Scrub` in `errJSON`)

**Interfaces:**
- Produces:
  - `secret.Scrub(s string) string` — masks UUID-like and long base64-like tokens inside a free-form string.
  - `(*API).ListServers() string` → `{"ok":true,"servers":[{"tag","host","masked"}],"selected":"<tag>"}`.
  - `(*API).Import(input string) string` → imports + persists, returns the refreshed list.
  - `(*API).Select(tag string) string` → sets `Selected`, persists.

- [ ] **Step 1: Write the failing test** — `internal/secret/scrub_test.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package secret

import (
	"strings"
	"testing"
)

func TestScrubHidesUUIDAndKeys(t *testing.T) {
	in := "start failed for 11111111-2222-3333-4444-555555555555 pbk=SUPERSECRETKEY1234567890"
	out := Scrub(in)
	if strings.Contains(out, "2222-3333") || strings.Contains(out, "SUPERSECRETKEY1234567890") {
		t.Fatalf("secret leaked: %q", out)
	}
	if !strings.Contains(out, "start failed for") {
		t.Fatalf("scrub destroyed the message: %q", out)
	}
}
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/secret/ -run Scrub`
Expected: FAIL — undefined `Scrub`.

- [ ] **Step 3: Implement** — `internal/secret/scrub.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package secret

import "regexp"

// uuidRe matches a canonical UUID; tokenRe matches long base64/hex-ish runs
// (REALITY keys, secrets) of 16+ url-safe-base64 characters.
var (
	uuidRe  = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	tokenRe = regexp.MustCompile(`[A-Za-z0-9_\-]{16,}`)
)

// Scrub masks UUID-like and long token-like substrings inside a free-form
// string (e.g. an error message), so secrets never reach logs/UI.
func Scrub(s string) string {
	s = uuidRe.ReplaceAllStringFunc(s, Mask)
	s = tokenRe.ReplaceAllStringFunc(s, Mask)
	return s
}
```

- [ ] **Step 4: Run it, verify it passes**

Run: `go test ./internal/secret/ -run Scrub`
Expected: PASS. Then swap `errJSON` in `ffiapi.go` from `secret.Mask(...)` to `secret.Scrub(...)` and run `go test ./internal/ffiapi/` — still PASS.

- [ ] **Step 5: Write the failing profile test** — `internal/ffiapi/profile_test.go`

```go
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
```

- [ ] **Step 6: Run it, verify it fails**

Run: `go test ./internal/ffiapi/ -run "Import|Select"`
Expected: FAIL — undefined `Import`/`ListServers`/`Select`.

- [ ] **Step 7: Implement** — `internal/ffiapi/profile.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package ffiapi

import (
	"fmt"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/secret"
)

// ListServers returns the stored servers (masked) and the current selection.
func (a *API) ListServers() string {
	prof, err := a.load()
	if err != nil {
		return errJSON(err, false)
	}
	return okJSON(map[string]any{
		"servers":  serverSummaries(prof.Servers),
		"selected": prof.Selected,
	})
}

// Import adds server(s) from a uri/file/inline body, persists, returns the list.
func (a *API) Import(input string) string {
	servers, err := a.importFn(input)
	if err != nil {
		return errJSON(err, false)
	}
	for _, s := range servers {
		if verr := s.Validate(); verr != nil {
			return errJSON(verr, false)
		}
	}
	prof, err := a.load()
	if err != nil {
		return errJSON(err, false)
	}
	prof.Servers = mergeServers(prof.Servers, servers)
	if prof.Selected == "" && len(prof.Servers) > 0 {
		prof.Selected = prof.Servers[0].Tag
	}
	if err := a.save(prof); err != nil {
		return errJSON(err, false)
	}
	return okJSON(map[string]any{
		"servers":  serverSummaries(prof.Servers),
		"selected": prof.Selected,
	})
}

// Select sets the active server by tag.
func (a *API) Select(tag string) string {
	prof, err := a.load()
	if err != nil {
		return errJSON(err, false)
	}
	for _, s := range prof.Servers {
		if s.Tag == tag {
			prof.Selected = tag
			if err := a.save(prof); err != nil {
				return errJSON(err, false)
			}
			return okJSON(nil)
		}
	}
	return errJSON(fmt.Errorf("no server with tag %q", tag), false)
}

func serverSummaries(servers []config.Server) []map[string]any {
	out := make([]map[string]any, 0, len(servers))
	for _, s := range servers {
		out = append(out, map[string]any{
			"tag":    s.Tag,
			"host":   s.Host,
			"masked": secret.MaskServer(s),
		})
	}
	return out
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
```

- [ ] **Step 8: Run it, verify it passes**

Run: `go test ./internal/ffiapi/ ./internal/secret/`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/secret/scrub.go internal/secret/scrub_test.go internal/ffiapi/profile.go internal/ffiapi/profile_test.go internal/ffiapi/ffiapi.go
git commit -m "feat(ffiapi): secret scrubber + profile ops (list/import/select)"
```

---

### Task A3: connection ops (Start / Stop / Status)

**Files:**
- Create: `internal/ffiapi/conn.go`, `internal/ffiapi/conn_test.go`

**Interfaces:**
- Consumes: `manager.New`, `manager.Tunnel`, `platform.KillSwitch`, `config.Server/Settings`, the `Prober`.
- Produces:
  - `(*API).Start(optsJSON string) string` — opts `{"failOpen":bool,"tag":string?}`; connects the selected (or named) server; `{"ok":true}` or `{"ok":false,"error":...,"needsAdmin":bool}`.
  - `(*API).Stop() string`.
  - `(*API).Status() string` → `{"ok":true,"state":"...","server":"<masked>","delayMs":N,"error":""}`.

- [ ] **Step 1: Write the failing test** — `internal/ffiapi/conn_test.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package ffiapi

import (
	"context"
	"testing"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/manager"
	"github.com/shadowlink/shadowlink/internal/platform"
)

type fakeTunnel struct{ started, closed bool }

func (f *fakeTunnel) Start() error { f.started = true; return nil }
func (f *fakeTunnel) Close() error { f.closed = true; return nil }

type fakeKS struct{ enabled bool }

func (k *fakeKS) Enable(string) error { k.enabled = true; return nil }
func (k *fakeKS) Disable() error       { k.enabled = false; return nil }

type fakeProber struct{ ms int }

func (p fakeProber) Delay(context.Context, string, string, int) (int, error) { return p.ms, nil }

// apiWithFakeEngine wires Start/Stop to fakes (no real tunnel/privileges).
func apiWithFakeEngine() *API {
	a, prof := newTestAPI()
	*prof = config.Profile{
		Servers:  []config.Server{{Tag: "n1", UUID: "u", Host: "1.2.3.4", Port: 443, Flow: "xtls-rprx-vision", Network: "tcp", PublicKey: "K"}},
		Selected: "n1",
		Settings: config.DefaultSettings(),
	}
	a.render = func(core.BuildParams) ([]byte, error) { return []byte("{}"), nil }
	a.newTunnel = func([]byte) (manager.Tunnel, error) { return &fakeTunnel{}, nil }
	a.newKS = func() platform.KillSwitch { return &fakeKS{} }
	a.newProber = func(int, string) Prober { return fakeProber{ms: 42} }
	return a
}

func TestStartStatusStop(t *testing.T) {
	a := apiWithFakeEngine()
	if decode(t, a.Start(`{}`))["ok"] != true {
		t.Fatal("start should succeed with fakes")
	}
	st := decode(t, a.Status())
	if st["state"] != "connected" {
		t.Fatalf("want connected, got %v", st["state"])
	}
	if st["delayMs"].(float64) != 42 {
		t.Fatalf("want delay 42, got %v", st["delayMs"])
	}
	if decode(t, a.Stop())["ok"] != true {
		t.Fatal("stop should succeed")
	}
	if decode(t, a.Status())["state"] != "disconnected" {
		t.Fatal("status after stop should be disconnected")
	}
}

func TestStartNoServersErrors(t *testing.T) {
	a, _ := newTestAPI() // empty profile
	if decode(t, a.Start(`{}`))["ok"] != false {
		t.Fatal("start with no servers must fail")
	}
}
```

> Note: this test references `core.BuildParams`; add `"github.com/shadowlink/shadowlink/internal/core"` to the test's imports.

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/ffiapi/ -run "Start|Status"`
Expected: FAIL — undefined `Start`/`Stop`/`Status`.

- [ ] **Step 3: Implement** — `internal/ffiapi/conn.go`

```go
// SPDX-License-Identifier: GPL-3.0-only
package ffiapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shadowlink/shadowlink/internal/config"
	"github.com/shadowlink/shadowlink/internal/core"
	"github.com/shadowlink/shadowlink/internal/manager"
	"github.com/shadowlink/shadowlink/internal/secret"
)

type startOpts struct {
	FailOpen bool   `json:"failOpen"`
	Tag      string `json:"tag"`
}

// Start brings up the tunnel for the selected (or named) server.
func (a *API) Start(optsJSON string) string {
	var opts startOpts
	if strings.TrimSpace(optsJSON) != "" {
		if err := json.Unmarshal([]byte(optsJSON), &opts); err != nil {
			return errJSON(fmt.Errorf("bad start options: %w", err), false)
		}
	}
	prof, err := a.load()
	if err != nil {
		return errJSON(err, false)
	}
	srv, err := pickServer(prof, opts.Tag)
	if err != nil {
		return errJSON(err, false)
	}
	set := prof.Settings
	if opts.FailOpen {
		set.KillSwitch = false
		set.FailOpen = true
	}
	set.ClashAPISecret = a.randSecret()

	a.mu.Lock()
	defer a.mu.Unlock()
	mgr := manager.New(manager.Deps{Render: a.render, NewTunnel: a.newTunnel, KillSwitch: a.newKS()})
	if err := mgr.Connect(srv, set); err != nil {
		return errJSON(err, isAccessDenied(err))
	}
	a.mgr = mgr
	a.active = srv
	a.prober = a.newProber(set.ClashAPIPort, set.ClashAPISecret)
	return okJSON(nil)
}

// Stop tears the tunnel down.
func (a *API) Stop() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.mgr == nil {
		return okJSON(nil)
	}
	err := a.mgr.Disconnect()
	a.mgr = nil
	a.prober = nil
	a.active = config.Server{}
	if err != nil {
		return errJSON(err, false)
	}
	return okJSON(nil)
}

// Status reports the live state, the active server (masked), and a delay probe.
func (a *API) Status() string {
	a.mu.Lock()
	mgr, prober, srv := a.mgr, a.prober, a.active
	a.mu.Unlock()

	state := string(manager.Disconnected)
	if mgr != nil {
		state = string(mgr.State())
	}
	out := map[string]any{"state": state, "server": "", "delayMs": 0, "error": ""}
	if mgr != nil {
		out["server"] = secret.MaskServer(srv)
	}
	if prober != nil && state == string(manager.Connected) {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		if ms, err := prober.Delay(ctx, "proxy", "https://www.gstatic.com/generate_204", 5000); err == nil {
			out["delayMs"] = ms
		} else {
			out["error"] = secret.Scrub(err.Error())
		}
	}
	return okJSON(out)
}

func pickServer(prof config.Profile, tag string) (config.Server, error) {
	if len(prof.Servers) == 0 {
		return config.Server{}, fmt.Errorf("no servers; import one first")
	}
	want := tag
	if want == "" {
		want = prof.Selected
	}
	if want != "" {
		for _, s := range prof.Servers {
			if s.Tag == want {
				return s, nil
			}
		}
	}
	return prof.Servers[0], nil
}

func isAccessDenied(err error) bool {
	e := strings.ToLower(err.Error())
	return strings.Contains(e, "access is denied") ||
		strings.Contains(e, "permission denied") ||
		strings.Contains(e, "operation not permitted") ||
		strings.Contains(e, "requires elevation") ||
		strings.Contains(e, "admin")
}
```

> Note: `core` is imported for `core.BuildParams` references in the test only; `conn.go` itself imports `core` is unused — REMOVE the `core` import from `conn.go` if `go vet` flags it. (The `manager.Deps` fields are `func(core.BuildParams)...` typed, but `conn.go` passes `a.render` which already has that type, so `conn.go` does not name `core` directly — drop the import.)

- [ ] **Step 4: Run it, verify it passes**

Run: `go test ./internal/ffiapi/`
Expected: PASS. Also `go vet ./internal/ffiapi/` — clean (drop unused imports if any).

- [ ] **Step 5: Commit**

```bash
git add internal/ffiapi/conn.go internal/ffiapi/conn_test.go
git commit -m "feat(ffiapi): Start/Stop/Status over the manager (fakes-tested)"
```

---

### Task A4: cgo shim (build-tag-gated) + stub

**Files:**
- Create: `cmd/libshadowlink/shim.go`, `cmd/libshadowlink/stub.go`

**Interfaces:**
- Consumes: `ffiapi.Default()` and its methods.
- Produces the C ABI: `SL_Version`, `SL_ListServers`, `SL_Import`, `SL_Select`, `SL_Start`, `SL_Stop`, `SL_Status`, `SL_FreeString`.

This task is **author-only here**: the real shim needs cgo + a 64-bit C toolchain. The stub keeps default builds/CI green. Verification HERE is only `go build ./...` / `go vet ./...` / `go test ./...` staying green (they build the stub, skip the shim). The real shim compiles in Task B1 on the user's machine.

- [ ] **Step 1: Implement the stub** — `cmd/libshadowlink/stub.go`

```go
// SPDX-License-Identifier: GPL-3.0-only

//go:build !libshadowlink

// Package main is the c-shared entrypoint. Without the `libshadowlink` build
// tag it is an empty stub so default `go build ./...` and CI stay green in
// environments without a 64-bit C toolchain. The real cgo shim (shim.go) builds
// only with `-tags libshadowlink`.
package main

func main() {}
```

- [ ] **Step 2: Implement the shim** — `cmd/libshadowlink/shim.go`

```go
// SPDX-License-Identifier: GPL-3.0-only

//go:build libshadowlink

package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"github.com/shadowlink/shadowlink/internal/ffiapi"
)

var api = ffiapi.Default()

//export SL_Version
func SL_Version() *C.char { return C.CString(api.Version()) }

//export SL_ListServers
func SL_ListServers() *C.char { return C.CString(api.ListServers()) }

//export SL_Import
func SL_Import(input *C.char) *C.char { return C.CString(api.Import(C.GoString(input))) }

//export SL_Select
func SL_Select(tag *C.char) *C.char { return C.CString(api.Select(C.GoString(tag))) }

//export SL_Start
func SL_Start(opts *C.char) *C.char { return C.CString(api.Start(C.GoString(opts))) }

//export SL_Stop
func SL_Stop() *C.char { return C.CString(api.Stop()) }

//export SL_Status
func SL_Status() *C.char { return C.CString(api.Status()) }

//export SL_FreeString
func SL_FreeString(p *C.char) { C.free(unsafe.Pointer(p)) }

func main() {}
```

> The shim needs `import "unsafe"` for `SL_FreeString`. Add `"unsafe"` to the import block (kept separate here for clarity; merge into one import group).

- [ ] **Step 3: Verify default builds stay green (HERE)**

Run: `go build ./... && go vet ./... && go test -tags "with_utls with_gvisor with_clash_api" ./...`
Expected: all green; `cmd/libshadowlink` shows `[no test files]` and builds the **stub** (the shim is excluded without `-tags libshadowlink`). If `go build ./...` errors with "build constraints exclude all Go files in cmd/libshadowlink", the stub's `//go:build !libshadowlink` is wrong — fix it.

- [ ] **Step 4: Commit**

```bash
git add cmd/libshadowlink/
git commit -m "feat(libshadowlink): cgo C-ABI shim (tag-gated) + non-cgo stub"
```

---

# PHASE B — Flutter Windows app + build (authored here, verified on the user's machine)

Outcome: a buildable Flutter Windows app and the DLL build pipeline. **Every Phase B task's verification is a command the USER runs on Windows** (Flutter/mingw not available here). The agentic worker authors the exact code/scripts; it does not claim local passes.

---

### Task B1: build the DLL (user's machine) — prove the FFI boundary

**Files:**
- Create: `build/build_windows.ps1`

**Interfaces:**
- Produces: `app/windows/native/shadowlink_core.dll` + `shadowlink_core.h`.

- [ ] **Step 1: Implement** — `build/build_windows.ps1`

```powershell
# Build the Go core as a c-shared DLL and stage the native DLLs for Flutter.
# Prereqs: Go 1.24+, mingw-w64 (x86_64) on PATH (CC=x86_64-w64-mingw32-gcc).
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$native = Join-Path $root "app\windows\native"
New-Item -ItemType Directory -Force $native | Out-Null

$env:CGO_ENABLED = "1"
$env:CC = "x86_64-w64-mingw32-gcc"
go build -tags "libshadowlink with_utls with_gvisor with_clash_api" `
  -buildmode=c-shared `
  -o (Join-Path $native "shadowlink_core.dll") `
  ./cmd/libshadowlink
Copy-Item (Join-Path $root "bin\wintun.dll") $native -Force
Write-Host "Built shadowlink_core.dll + staged wintun.dll into $native"
```

- [ ] **Step 2: Verify (USER, Windows)**

Run (from repo root, in a shell with mingw-w64 on PATH):
```powershell
pwsh build/build_windows.ps1
```
Expected: `app/windows/native/shadowlink_core.dll`, `shadowlink_core.h`, and `wintun.dll` exist; no build errors. If `cc1.exe: 64-bit mode not compiled in`, the C compiler is 32-bit — install mingw-w64 (x86_64) and set `CC`.

- [ ] **Step 3: Commit**

```bash
git add build/build_windows.ps1
git commit -m "build: c-shared DLL build script (Windows, mingw-w64)"
```

> Do not commit the built `.dll`/`.h` — add `app/windows/native/` to `.gitignore` (Task B5).

---

### Task B2: Flutter scaffold + CoreApi interface + FakeCore + models

**Files:**
- Create: `app/pubspec.yaml`, `app/lib/ffi/core.dart`, `app/test/connection_controller_test.dart` (the fake is exercised in B4's test; here we land the contract + fake)

**Interfaces:**
- Produces: `CoreApi` (abstract), `ConnStatus`, `ServerInfo`, `FakeCore`.

- [ ] **Step 1: Implement** — `app/pubspec.yaml`

```yaml
name: shadowlink
description: ShadowLink cross-platform VPN client (Windows-first).
publish_to: "none"
version: 0.0.1
environment:
  sdk: ">=3.4.0 <4.0.0"
dependencies:
  flutter:
    sdk: flutter
  ffi: ^2.1.0
dev_dependencies:
  flutter_test:
    sdk: flutter
  flutter_lints: ^4.0.0
flutter:
  uses-material-design: true
```

- [ ] **Step 2: Implement** — `app/lib/ffi/core.dart`

```dart
// SPDX-License-Identifier: GPL-3.0-only

/// ConnStatus is the parsed result of SL_Status.
class ConnStatus {
  final String state; // disconnected | connecting | connected | reconnecting | disconnecting | error
  final String server; // masked
  final int delayMs;
  final String error;
  const ConnStatus(
      {required this.state, this.server = '', this.delayMs = 0, this.error = ''});

  factory ConnStatus.fromJson(Map<String, dynamic> m) => ConnStatus(
        state: (m['state'] ?? 'disconnected') as String,
        server: (m['server'] ?? '') as String,
        delayMs: (m['delayMs'] ?? 0) as int,
        error: (m['error'] ?? '') as String,
      );

  bool get connected => state == 'connected';
  static const disconnected = ConnStatus(state: 'disconnected');
}

/// ServerInfo is one entry from SL_ListServers.
class ServerInfo {
  final String tag;
  final String host;
  final String masked;
  const ServerInfo({required this.tag, required this.host, required this.masked});
  factory ServerInfo.fromJson(Map<String, dynamic> m) => ServerInfo(
      tag: m['tag'] as String, host: m['host'] as String, masked: m['masked'] as String);
}

/// Thrown when a core call returns {"ok":false}.
class CoreException implements Exception {
  final String message;
  final bool needsAdmin;
  CoreException(this.message, {this.needsAdmin = false});
  @override
  String toString() => 'CoreException($message, needsAdmin=$needsAdmin)';
}

/// CoreApi is the engine boundary. ShadowlinkCore (FFI) is the real impl;
/// FakeCore drives widget/controller tests without a DLL.
abstract class CoreApi {
  String version();
  List<ServerInfo> listServers();
  String selected();
  List<ServerInfo> import(String input);
  void select(String tag);
  void start({bool failOpen = false, String? tag});
  void stop();
  ConnStatus status();
}

/// FakeCore is an in-memory CoreApi for tests.
class FakeCore implements CoreApi {
  final List<ServerInfo> _servers = [];
  String _selected = '';
  bool _connected = false;
  bool failOnStart = false;

  @override
  String version() => '0.0.0-fake';
  @override
  List<ServerInfo> listServers() => List.unmodifiable(_servers);
  @override
  String selected() => _selected;
  @override
  List<ServerInfo> import(String input) {
    _servers.add(const ServerInfo(tag: 'n1', host: 'ex.com', masked: 'n1 (ex.com:443 uuid=uu** pbk=K***)'));
    if (_selected.isEmpty) _selected = 'n1';
    return listServers();
  }

  @override
  void select(String tag) {
    if (!_servers.any((s) => s.tag == tag)) {
      throw CoreException('no server with tag $tag');
    }
    _selected = tag;
  }

  @override
  void start({bool failOpen = false, String? tag}) {
    if (failOnStart) throw CoreException('boom', needsAdmin: true);
    if (_servers.isEmpty) throw CoreException('no servers; import one first');
    _connected = true;
  }

  @override
  void stop() => _connected = false;
  @override
  ConnStatus status() => _connected
      ? const ConnStatus(state: 'connected', server: 'n1 (ex.com:443 ...)', delayMs: 42)
      : ConnStatus.disconnected;
}
```

- [ ] **Step 3: Verify (USER, Windows)**

Run (in `app/`):
```powershell
flutter pub get
flutter analyze lib/ffi/core.dart
```
Expected: `pub get` resolves; `flutter analyze` reports no issues for `core.dart`.

- [ ] **Step 4: Commit**

```bash
git add app/pubspec.yaml app/lib/ffi/core.dart
git commit -m "feat(app): CoreApi contract, models, FakeCore"
```

---

### Task B3: dart:ffi binding — ShadowlinkCore

**Files:**
- Create: `app/lib/ffi/core_ffi.dart`

**Interfaces:**
- Consumes: `CoreApi`, the C ABI from Task A4/B1.
- Produces: `ShadowlinkCore implements CoreApi`.

- [ ] **Step 1: Implement** — `app/lib/ffi/core_ffi.dart`

```dart
// SPDX-License-Identifier: GPL-3.0-only
import 'dart:convert';
import 'dart:ffi';
import 'dart:io';
import 'package:ffi/ffi.dart';
import 'core.dart';

typedef _NoArgC = Pointer<Utf8> Function();
typedef _NoArgD = Pointer<Utf8> Function();
typedef _StrArgC = Pointer<Utf8> Function(Pointer<Utf8>);
typedef _StrArgD = Pointer<Utf8> Function(Pointer<Utf8>);
typedef _FreeC = Void Function(Pointer<Utf8>);
typedef _FreeD = void Function(Pointer<Utf8>);

/// ShadowlinkCore is the real CoreApi, calling shadowlink_core.dll via dart:ffi.
class ShadowlinkCore implements CoreApi {
  final DynamicLibrary _lib;
  late final _NoArgD _version = _lib.lookupFunction<_NoArgC, _NoArgD>('SL_Version');
  late final _NoArgD _list = _lib.lookupFunction<_NoArgC, _NoArgD>('SL_ListServers');
  late final _StrArgD _import = _lib.lookupFunction<_StrArgC, _StrArgD>('SL_Import');
  late final _StrArgD _select = _lib.lookupFunction<_StrArgC, _StrArgD>('SL_Select');
  late final _StrArgD _start = _lib.lookupFunction<_StrArgC, _StrArgD>('SL_Start');
  late final _NoArgD _stop = _lib.lookupFunction<_NoArgC, _NoArgD>('SL_Stop');
  late final _NoArgD _status = _lib.lookupFunction<_NoArgC, _NoArgD>('SL_Status');
  late final _FreeD _free = _lib.lookupFunction<_FreeC, _FreeD>('SL_FreeString');

  ShadowlinkCore._(this._lib);

  /// Loads shadowlink_core.dll from beside the executable.
  factory ShadowlinkCore.open() {
    final name = Platform.isWindows
        ? 'shadowlink_core.dll'
        : Platform.isMacOS
            ? 'libshadowlink_core.dylib'
            : 'libshadowlink_core.so';
    return ShadowlinkCore._(DynamicLibrary.open(name));
  }

  Map<String, dynamic> _call(Pointer<Utf8> ptr) {
    try {
      final s = ptr.toDartString();
      final m = jsonDecode(s) as Map<String, dynamic>;
      if (m['ok'] != true) {
        throw CoreException((m['error'] ?? 'unknown error') as String,
            needsAdmin: m['needsAdmin'] == true);
      }
      return m;
    } finally {
      _free(ptr);
    }
  }

  Pointer<Utf8> _withStr(_StrArgD fn, String arg) {
    final p = arg.toNativeUtf8();
    try {
      return fn(p);
    } finally {
      calloc.free(p);
    }
  }

  @override
  String version() => _call(_version())['version'] as String;
  @override
  List<ServerInfo> listServers() => ((_call(_list())['servers']) as List)
      .map((e) => ServerInfo.fromJson(e as Map<String, dynamic>))
      .toList();
  @override
  String selected() => (_call(_list())['selected'] ?? '') as String;
  @override
  List<ServerInfo> import(String input) => ((_call(_withStr(_import, input))['servers']) as List)
      .map((e) => ServerInfo.fromJson(e as Map<String, dynamic>))
      .toList();
  @override
  void select(String tag) => _call(_withStr(_select, tag));
  @override
  void start({bool failOpen = false, String? tag}) =>
      _call(_withStr(_start, jsonEncode({'failOpen': failOpen, if (tag != null) 'tag': tag})));
  @override
  void stop() => _call(_stop());
  @override
  ConnStatus status() => ConnStatus.fromJson(_call(_status()));
}
```

- [ ] **Step 2: Verify (USER, Windows)**

Run (in `app/`):
```powershell
flutter analyze lib/ffi/core_ffi.dart
```
Expected: no analyzer issues. (Runtime FFI is exercised in B6's smoke run once the DLL is built.)

- [ ] **Step 3: Commit**

```bash
git add app/lib/ffi/core_ffi.dart
git commit -m "feat(app): dart:ffi ShadowlinkCore over shadowlink_core.dll"
```

---

### Task B4: connection controller (state + polling) + its test

**Files:**
- Create: `app/lib/state/connection_controller.dart`, `app/test/connection_controller_test.dart`

**Interfaces:**
- Consumes: `CoreApi`, `ConnStatus`.
- Produces: `ConnectionController extends ChangeNotifier` with `status`, `connect()`, `disconnect()`, `refresh()`.

- [ ] **Step 1: Implement** — `app/lib/state/connection_controller.dart`

```dart
// SPDX-License-Identifier: GPL-3.0-only
import 'dart:async';
import 'package:flutter/foundation.dart';
import '../ffi/core.dart';

/// ConnectionController owns the live connection state and polls SL_Status.
class ConnectionController extends ChangeNotifier {
  final CoreApi core;
  ConnStatus status = ConnStatus.disconnected;
  String? lastError;
  bool needsAdmin = false;
  Timer? _poll;

  ConnectionController(this.core);

  Future<void> connect({bool failOpen = false}) async {
    lastError = null;
    needsAdmin = false;
    try {
      await Future(() => core.start(failOpen: failOpen));
      _startPolling();
    } on CoreException catch (e) {
      lastError = e.message;
      needsAdmin = e.needsAdmin;
    }
    refresh();
  }

  Future<void> disconnect() async {
    _poll?.cancel();
    _poll = null;
    try {
      await Future(() => core.stop());
    } on CoreException catch (e) {
      lastError = e.message;
    }
    refresh();
  }

  void refresh() {
    try {
      status = core.status();
    } on CoreException catch (e) {
      lastError = e.message;
    }
    notifyListeners();
  }

  void _startPolling() {
    _poll?.cancel();
    _poll = Timer.periodic(const Duration(seconds: 2), (_) => refresh());
  }

  @override
  void dispose() {
    _poll?.cancel();
    super.dispose();
  }
}
```

- [ ] **Step 2: Write the test** — `app/test/connection_controller_test.dart`

```dart
// SPDX-License-Identifier: GPL-3.0-only
import 'package:flutter_test/flutter_test.dart';
import 'package:shadowlink/ffi/core.dart';
import 'package:shadowlink/state/connection_controller.dart';

void main() {
  test('connect reaches connected, disconnect returns to disconnected', () async {
    final core = FakeCore();
    core.import('vless://x');
    final c = ConnectionController(core);

    await c.connect();
    expect(c.status.connected, isTrue);

    await c.disconnect();
    expect(c.status.state, 'disconnected');
  });

  test('start failure surfaces needsAdmin without crashing', () async {
    final core = FakeCore()..failOnStart = true;
    core.import('vless://x');
    final c = ConnectionController(core);

    await c.connect();
    expect(c.status.connected, isFalse);
    expect(c.needsAdmin, isTrue);
    expect(c.lastError, isNotNull);
  });
}
```

- [ ] **Step 3: Verify (USER, Windows)**

Run (in `app/`):
```powershell
flutter test test/connection_controller_test.dart
```
Expected: 2 tests pass (these use `FakeCore`, no DLL needed).

- [ ] **Step 4: Commit**

```bash
git add app/lib/state/connection_controller.dart app/test/connection_controller_test.dart
git commit -m "feat(app): connection controller + tests (FakeCore)"
```

---

### Task B5: UI (home + servers), main, manifest, .gitignore

**Files:**
- Create: `app/lib/main.dart`, `app/lib/ui/home_page.dart`, `app/lib/ui/servers_page.dart`
- Modify: `app/windows/runner/runner.exe.manifest` (requireAdministrator) — generated by `flutter create`
- Modify: `.gitignore` (ignore `app/build/`, `app/windows/native/`, `app/.dart_tool/`)

**Interfaces:**
- Consumes: `ConnectionController`, `CoreApi`, `ShadowlinkCore`, `FakeCore`.

- [ ] **Step 1: Implement** — `app/lib/main.dart`

```dart
// SPDX-License-Identifier: GPL-3.0-only
import 'package:flutter/material.dart';
import 'ffi/core.dart';
import 'ffi/core_ffi.dart';
import 'state/connection_controller.dart';
import 'ui/home_page.dart';

void main() {
  // Swap ShadowlinkCore.open() for FakeCore() to run the UI without the DLL.
  final CoreApi core = ShadowlinkCore.open();
  runApp(ShadowLinkApp(controller: ConnectionController(core)));
}

class ShadowLinkApp extends StatelessWidget {
  final ConnectionController controller;
  const ShadowLinkApp({super.key, required this.controller});

  @override
  Widget build(BuildContext context) => MaterialApp(
        title: 'ShadowLink',
        theme: ThemeData(colorSchemeSeed: Colors.indigo, useMaterial3: true),
        home: HomePage(controller: controller),
      );
}
```

- [ ] **Step 2: Implement** — `app/lib/ui/home_page.dart`

```dart
// SPDX-License-Identifier: GPL-3.0-only
import 'package:flutter/material.dart';
import '../state/connection_controller.dart';
import 'servers_page.dart';

class HomePage extends StatefulWidget {
  final ConnectionController controller;
  const HomePage({super.key, required this.controller});
  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  bool _killSwitch = true;

  @override
  void initState() {
    super.initState();
    widget.controller.refresh();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('ShadowLink'), actions: [
        IconButton(
          icon: const Icon(Icons.dns),
          tooltip: 'Servers',
          onPressed: () => Navigator.push(context,
              MaterialPageRoute(builder: (_) => ServersPage(core: widget.controller.core))),
        ),
      ]),
      body: ListenableBuilder(
        listenable: widget.controller,
        builder: (context, _) {
          final c = widget.controller;
          final st = c.status;
          return Center(
            child: Column(mainAxisAlignment: MainAxisAlignment.center, children: [
              Text(st.state.toUpperCase(),
                  style: Theme.of(context).textTheme.headlineSmall),
              const SizedBox(height: 24),
              FilledButton(
                onPressed: () => st.connected
                    ? c.disconnect()
                    : c.connect(failOpen: !_killSwitch),
                child: Text(st.connected ? 'DISCONNECT' : 'CONNECT'),
              ),
              const SizedBox(height: 16),
              if (st.connected) Text('${st.server} · ${st.delayMs} ms'),
              SwitchListTile(
                title: const Text('Kill-switch'),
                value: _killSwitch,
                onChanged: st.connected ? null : (v) => setState(() => _killSwitch = v),
              ),
              if (c.lastError != null)
                Padding(
                  padding: const EdgeInsets.all(12),
                  child: Text(
                    c.needsAdmin
                        ? 'Run as administrator to bring up the tunnel.'
                        : 'Error: ${c.lastError}',
                    style: const TextStyle(color: Colors.red),
                  ),
                ),
            ]),
          );
        },
      ),
    );
  }
}
```

- [ ] **Step 3: Implement** — `app/lib/ui/servers_page.dart`

```dart
// SPDX-License-Identifier: GPL-3.0-only
import 'package:flutter/material.dart';
import '../ffi/core.dart';

class ServersPage extends StatefulWidget {
  final CoreApi core;
  const ServersPage({super.key, required this.core});
  @override
  State<ServersPage> createState() => _ServersPageState();
}

class _ServersPageState extends State<ServersPage> {
  final _input = TextEditingController();
  String? _error;

  @override
  Widget build(BuildContext context) {
    final servers = widget.core.listServers();
    final selected = widget.core.selected();
    return Scaffold(
      appBar: AppBar(title: const Text('Servers')),
      body: Column(children: [
        Padding(
          padding: const EdgeInsets.all(12),
          child: Row(children: [
            Expanded(
              child: TextField(
                controller: _input,
                decoration: const InputDecoration(
                    labelText: 'vless:// link, file path, or subscription body'),
              ),
            ),
            const SizedBox(width: 8),
            FilledButton(
              onPressed: () {
                try {
                  widget.core.import(_input.text.trim());
                  _input.clear();
                  setState(() => _error = null);
                } on CoreException catch (e) {
                  setState(() => _error = e.message);
                }
              },
              child: const Text('Import'),
            ),
          ]),
        ),
        if (_error != null) Text(_error!, style: const TextStyle(color: Colors.red)),
        Expanded(
          child: ListView(
            children: servers
                .map((s) => ListTile(
                      leading: Icon(s.tag == selected
                          ? Icons.radio_button_checked
                          : Icons.radio_button_unchecked),
                      title: Text(s.masked),
                      onTap: () {
                        try {
                          widget.core.select(s.tag);
                          setState(() => _error = null);
                        } on CoreException catch (e) {
                          setState(() => _error = e.message);
                        }
                      },
                    ))
                .toList(),
          ),
        ),
      ]),
    );
  }
}
```

- [ ] **Step 4: Patch the Windows manifest + .gitignore**

After `flutter create --platforms=windows --org com.shadowlink .` (run by the user in `app/`), edit `app/windows/runner/runner.exe.manifest`: inside `<trustInfo>`, set the requested execution level:

```xml
<requestedExecutionLevel level="requireAdministrator" uiAccess="false" />
```

Append to the repo-root `.gitignore`:

```gitignore
# Flutter app build artifacts + staged native libs
/app/build/
/app/.dart_tool/
/app/windows/native/
/app/.flutter-plugins
/app/.flutter-plugins-dependencies
```

- [ ] **Step 5: Verify (USER, Windows)**

Run (in `app/`):
```powershell
flutter analyze
flutter test
```
Expected: analyzer clean; the controller test passes. (UI is exercised live in B6.)

- [ ] **Step 6: Commit**

```bash
git add app/lib/main.dart app/lib/ui/ .gitignore
git commit -m "feat(app): home + servers UI, elevated manifest, gitignore"
```

---

### Task B6: end-to-end GUI acceptance (manual, recorded)

**Files:**
- Create: `docs/ACCEPTANCE-FLUTTER-WINDOWS.md`

- [ ] **Step 1: Write the runbook** — `docs/ACCEPTANCE-FLUTTER-WINDOWS.md`

```markdown
# Flutter Windows GUI Acceptance

Prereqs: Flutter (Windows desktop enabled), Visual Studio C++ workload, mingw-w64
(x86_64) on PATH, Go 1.24+. wintun.dll in bin/.

1. Build the DLL: `pwsh build/build_windows.ps1` → app/windows/native/shadowlink_core.dll.
2. In app/: `flutter create --platforms=windows --org com.shadowlink .` (first time only),
   then re-apply the requireAdministrator manifest edit, then `flutter pub get`.
3. Ensure shadowlink_core.dll + wintun.dll are beside the runner exe (the build
   script stages them; for `flutter run` copy them into build/windows/x64/runner/Debug/).
4. From an ELEVATED terminal: `flutter run -d windows`.
5. Verify:
   - App opens; status shows DISCONNECTED.
   - Servers page: import your vless:// (or sub file); it appears masked and selected.
   - Home: CONNECT → status CONNECTED, delay shows ms; public IP is the server's.
   - Kill-switch on: normal browsing still works (the Phase 1 step-4 risk).
   - DISCONNECT → DISCONNECTED; networking restored.
   - Launching non-elevated → the "Run as administrator" banner appears on CONNECT.

## Pass criteria (ALL)
- [ ] DLL builds; app launches elevated.
- [ ] Import/select/list work and are masked (no UUID/key on screen).
- [ ] Connect changes public IP; delay shows; disconnect restores.
- [ ] Browsing works with kill-switch ON.
- [ ] No secret appears anywhere in the UI.

## Result
- Date / Flutter+Go versions / outcome / notes:
```

- [ ] **Step 2: Run it** on the user's Windows machine; **record** the result.

- [ ] **Step 3: Commit**

```bash
git add docs/ACCEPTANCE-FLUTTER-WINDOWS.md
git commit -m "test: Flutter Windows GUI acceptance runbook + result"
```

---

## Self-review (completed by plan author)

- **Spec coverage:** §3 architecture → the three-front-ends layering (A1–A4 + B*). §4.1 ffiapi API → A1 (Version/helpers), A2 (list/import/select + scrub), A3 (start/stop/status). §4.2 C ABI → A4 shim. §4.3 Flutter structure/UI → B2 (CoreApi+models+fake), B3 (ffi binding), B4 (controller+test), B5 (UI+main+manifest+gitignore). §5 data flow → A3 + B4. §6 error/admin → `needsAdmin` (A1/A3) + banner (B5). §7 build → B1. §8 testing → A* TDD + B4 widget test + B6 acceptance. §9 reuse → ffiapi consumes the Phase 1 packages unchanged. §10 scope → only the Windows slice; mobile/rich-UI deferred (no tasks, by design). §11 prereqs → B1/B6 runbooks.
- **Placeholder scan:** no TBD/TODO; every Go step has complete code + a real command. Two explicit author-notes (A1 `secret.Scrub` lands in A2; A3 drop-unused-`core`-import) are corrections, not placeholders. Phase B steps give the user's exact verify command and expected output.
- **Type consistency:** `CoreApi` methods (`version/listServers/selected/import/select/start/stop/status`) match across `core.dart` (B2), `core_ffi.dart` (B3), `FakeCore` (B2), and the controller (B4). The C ABI names (`SL_Version`/`SL_ListServers`/`SL_Import`/`SL_Select`/`SL_Start`/`SL_Stop`/`SL_Status`/`SL_FreeString`) match across A4 shim, B1 build, and B3 lookups. `ffiapi` method names (`Version/ListServers/Import/Select/Start/Stop/Status`) match across A1–A4. JSON keys (`ok/error/needsAdmin/version/servers/selected/tag/host/masked/state/server/delayMs/failOpen`) match between Go (`okJSON`/`errJSON`/summaries/status) and Dart (`ConnStatus`/`ServerInfo`/start opts).
- **Verification honesty:** Phase A is TDD-verified in any Go environment; Phase B verification commands run on the user's Windows machine — the plan never claims a Phase B local pass here.
```
