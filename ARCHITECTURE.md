# ShadowLink — Architecture

> Cross-platform personal VPN **client** for a small group of friends, built as a
> thin, reliable management/UI layer over an embedded [sing-box](https://sing-box.sagernet.org)
> proxy core. Optimized for staying connected and unfingerprintable on a heavily
> DPI-filtered network (Russian TSPU).

Status: **design baseline** (pre-MVP). This document is the authoritative description
of *what* the system is and *how* it fits together. The build order lives in
[`docs/PHASED-PLAN.md`](docs/PHASED-PLAN.md); the decision record and rejected
alternatives live in [`docs/superpowers/specs/2026-06-19-shadowlink-design.md`](docs/superpowers/specs/2026-06-19-shadowlink-design.md).

---

## 1. Goals and non-goals

**Goals**

- Reliable, system-wide VPN on a hostile, DPI-filtered network (TSPU): stays
  connected, recovers automatically, and never silently leaks the real IP/DNS.
- Usable by non-technical friends: one-click connect from a system-tray app.
- Usable by technical users: a scriptable CLI exposing the same capability.
- Single, self-contained, cross-platform binary that is easy to hand to a friend.
- Client speaks **VLESS + REALITY + XTLS-Vision**, interoperating with the
  already-running Xray-core server.

**Non-goals (for this project)**

- We do **not** build or provision the server. The server already exists and is
  out of scope. (Server requirements it must satisfy are documented in §4.)
- We do **not** invent new circumvention protocols or crypto. The embedded core
  already solves the proxy/transport problem; our value is the management layer.
- No mobile (Android/iOS) in scope. The architecture must not *preclude* mobile
  later, but desktop (Windows/macOS/Linux) is the only target.
- Not a multi-tenant service. It is a personal tool for a handful of people.

---

## 2. Threat model: TSPU and what we counter

TSPU (Технические средства противодействия угрозам) is Russia's centralized,
ISP-level DPI/blocking system. The properties that drive our design:

| TSPU behavior | Our countermeasure |
|---|---|
| SNI-based blocking of TLS connections | REALITY borrows a real, allowed site's TLS handshake; the SNI on the wire is the camouflage host, not our server. |
| TLS fingerprint analysis (JA3/JA4), active probing | uTLS produces a genuine browser ClientHello (`chrome`/`firefox`/`edge`); REALITY makes active probes see the real camouflage site. |
| Protocol fingerprinting of known VPNs (WireGuard/OpenVPN/vanilla Shadowsocks) | VLESS+Vision over REALITY looks like ordinary TLS to a real CDN/site. |
| DNS spoofing / poisoning / port-53 interception | All DNS is forced through the tunnel via DoH to a resolver reached *through* the proxy; system/ISP DNS is never consulted. |
| Connection throttling / RST injection / flaky links | Health-checks + automatic reconnect with backoff; multiple servers with switching. |
| Real-IP exposure if the tunnel drops | **Fail-closed kill switch** enforced by the OS firewall, independent of the core's lifecycle. |

What we explicitly do **not** defend against: endpoint compromise, a malicious
server operator (the server is trusted — it's the user's own), traffic-correlation
by a global passive adversary, or WebRTC/app-level IP leaks beyond what the OS
firewall + TUN can contain (documented as a residual risk in §9).

---

## 3. Technology choices

All versions and API facts below were verified against live upstream sources
(sing-box docs, the `SagerNet/sing-box` source at tag `v1.13.13`, and pkg.go.dev)
in June 2026. See the spec's verification appendix for citations.

| Concern | Choice | Why |
|---|---|---|
| Language | **Go 1.24+** | The core is Go; embedding in-process yields one static cross-platform binary. sing-box 1.13.x requires Go 1.24+. |
| Proxy core | **sing-box `v1.13.13`**, embedded in-process via `github.com/sagernet/sing-box` (package `box`) | Designed as an embeddable library; natively covers TUN, DNS, routing/split-tunnel, multi-outbound selection — everything our feature set needs, in one config schema. Its VLESS+REALITY+Vision **client interoperates with the Xray-core server** (verified). |
| CLI framework | **spf13/cobra** | De-facto standard, subcommand ergonomics. |
| System tray | **`fyne.io/systray` `v1.12.2`** | The actively-maintained fork of `getlantern/systray` (which is stale since 2023). DBus/StatusNotifierItem on Linux, no GTK/CGO build dependency. |
| Config / state | JSON files in the per-user config dir | Simple, inspectable, no DB needed for a handful of servers. |
| Runtime core control | sing-box **Clash API** (`experimental.clash_api`) bound to `127.0.0.1` with a random secret | Lets the manager query per-node latency and switch the active outbound at runtime without restarting the core. |

### Licensing consequence (load-bearing)

sing-box is **GPL-3.0** (the `v1.13.13` `LICENSE` file states "version 3 of the
License"). Embedding it **in-process** makes ShadowLink a derivative work, so
**ShadowLink is licensed GPL-3.0**. This is acceptable for an open tool shared
among friends, but it is a deliberate, recorded decision: the project cannot be
closed-source while embedding the core this way. (If a future requirement forced
closed-source, the fallback is Approach C — core as a separate process — which we
rejected for MVP; see the spec.)

### Pinned versions

- `github.com/sagernet/sing-box` — `v1.13.13` (config schema: `.../option`,
  registry bootstrap: `.../include`).
- JSON helpers: `github.com/sagernet/sing/common/json` (sing-box uses its own
  context-aware JSON, **not** `encoding/json`).
- `fyne.io/systray` — `v1.12.2`.
- Rule-sets (split-tunnel): `geosite-category-ru.srs`, `geoip-ru.srs` from the
  SagerNet `sing-geosite`/`sing-geoip` `rule-set` branches (see §10).

Renovate/Dependabot will track these; the gate test in Phase 0 re-validates the
embedding API against whatever patch we pin.

---

## 4. Server requirements (the client must match these)

ShadowLink is client-only, but the client config is only valid against a server
configured a specific way. The server (already running, Xray-core) must provide:

- **VLESS + REALITY + XTLS-Vision over raw TCP.** sing-box's client accepts
  `flow` of only `""` or `"xtls-rprx-vision"`, and Vision is valid only over bare
  TLS/REALITY (no `ws`/`grpc`/`xhttp` transport). So the server's VLESS user must
  use `flow: xtls-rprx-vision`, `network: tcp`.
- A REALITY key pair whose **public key** the client carries (`pbk`), and at least
  one **short ID** (`sid`) the client also carries.
- A **camouflage `serverName`/dest** (an allowed, real TLS site) that the client
  uses as `tls.server_name` / SNI.
- Field-name note: Xray uses `publicKey`/`shortIds`/`serverName`; sing-box uses
  `reality.public_key`/`reality.short_id`/`tls.server_name`. The *values* must
  match across cores; the *key names* differ. `spiderX` (`spx`) has no sing-box
  equivalent and is dropped on import.

These constraints are surfaced to the user in docs and validated at import time
(see `internal/subscription`).

---

## 5. High-level architecture

A single Go module producing (up to) two executables that share all the internal
packages:

- `shadowlink` — the **CLI + privileged core host**.
- `shadowlink-tray` — the **unprivileged tray UI** (added in Phase 3).

### Privilege model (and why it evolves)

TUN device creation, route programming, and firewall kill-switch rules are all
**privileged** (Administrator on Windows; root or `CAP_NET_ADMIN` on Linux; root
or a NetworkExtension/System Extension on macOS). A tray icon must **not** require
elevation. We introduce the split only when a second client appears (YAGNI):

- **Phase 1 (CLI only):** one elevated process. The CLI drives `manager` + `core`
  directly, in-process. No IPC.
- **Phase 3 (tray added):** split into a **privileged daemon/service** (hosts the
  core, owns TUN/routes/firewall) and an **unprivileged UI** (CLI + tray) that
  talks to the daemon over a local IPC channel (Unix domain socket / Windows named
  pipe) with peer-credential authorization. The IPC contract is the same one the
  CLI uses, so the CLI keeps working unchanged.

```
                         ┌──────────────────────────────────────────────┐
   user ── tray ──┐      │  shadowlink daemon (privileged)               │
                  ├─IPC─▶│                                               │
   user ── CLI  ──┘      │   manager (state machine)                     │
                         │     ├── core      → embedded sing-box (box)   │
                         │     ├── health    → latency probes            │
                         │     ├── config    → domain model ⇄ sing-box   │
                         │     ├── subscription → import/refresh         │
                         │     └── platform  → TUN deps, firewall        │
                         │                       kill-switch, service     │
                         └──────────────────────────────────────────────┘
                                         │  TUN (all OS traffic)
                                         ▼
                              VLESS+REALITY+Vision ── Xray server
```

---

## 6. Components

Each package has one responsibility, a narrow interface, and is testable in
isolation. The dependency rule: `core` is the only package that imports sing-box;
everything else speaks the domain model.

### `internal/config` — domain model + persistence
- Owns `Server`, `Profile`, `Settings` types and reads/writes them to the per-user
  config dir (`%AppData%\ShadowLink` / `~/.config/shadowlink` / `~/Library/...`).
- Knows nothing about sing-box. This keeps the user-facing model stable even if
  the core changes.

### `internal/subscription` — import & refresh
- Parses `vless://` URIs and **base64 subscription blobs** (newline-separated
  share links). Lenient decoding: try standard base64 → URL-safe base64 → raw
  text; ignore unknown query keys (forward-compat with post-quantum REALITY params
  like `pqv`/`mlkem`).
- Maps URI params → domain `Server` (uuid, host, port, `flow`, `sni`, `fp`, `pbk`,
  `sid`, transport). Validates server constraints from §4 and rejects configs that
  can't possibly work (e.g. a non-Vision flow, or a v2ray transport with Vision).
- Refresh can be routed through the active tunnel (the subscription host may itself
  be blocked).

### `internal/core` — the sing-box boundary
- The **only** package importing `github.com/sagernet/sing-box`.
- Translates a resolved domain `Server` + `Settings` into `option.Options`, then:
  `ctx := include.Context(context.Background())` → build/parse options with that
  ctx → `box.New(box.Options{Context: ctx, Options: opts})` → `Start()` / `Close()`.
  (Since v1.11 the DI context is mandatory; a plain `context.Background()` fails.)
- Generates the config blocks described in §7. Exposes a small interface
  (`Start`, `Stop`, `Stats`, `SwitchOutbound`, `ProbeDelay`) so `manager` never
  sees sing-box types.

### `internal/manager` — orchestrator / single source of truth
- Owns the connection **state machine**:
  `Disconnected → Connecting → Connected → Reconnecting → Error` (+ `Disconnecting`).
- Applies settings (kill-switch, split-tunnel, DNS) by coordinating `core` and
  `platform`. Drives health-checks, reconnect/backoff, and server switching.
- Publishes state changes to subscribers (CLI status, tray, logs).

### `internal/health` — liveness & latency
- Periodic latency probes via the core's Clash API (`GET /proxies/{tag}/delay`)
  and/or sing-box `urltest`. Signals `manager` on degradation/recovery so it can
  reconnect or switch.

### `internal/platform` — OS-specific privileged operations
- **TUN prerequisites**: ship/locate `wintun.dll` on Windows; ensure
  `CAP_NET_ADMIN`/root on Linux; the privileged path on macOS.
- **Firewall kill-switch** (the critical fail-closed piece — see §9): WFP on
  Windows, nftables/iptables on Linux, pf on macOS. Applied/removed independently
  of the core's lifecycle.
- **Service install** + **privilege elevation** + config-dir resolution.
- Behind one interface with a per-OS implementation (`_windows.go`, `_linux.go`,
  `_darwin.go`).

### `internal/ipc` — local control plane (Phase 3)
- Daemon-side server + client library over Unix socket / named pipe, with
  peer-credential checks. Mirrors the manager's command/event surface.

### `cmd/shadowlink` — CLI
- Cobra commands: `connect`, `disconnect`, `status`, `servers list|select`,
  `sub import|update`, `import <uri|file>`, `settings`, `logs`, `service install`.

### `cmd/shadowlink-tray` — tray UI (Phase 3)
- `fyne.io/systray`; `Run()` on the main goroutine (macOS Cocoa requirement).
  Menu: status, connect/disconnect, server picker, open logs, quit. A thin client
  over `internal/ipc`.

---

## 7. sing-box config generation

`internal/core` assembles these blocks. Concrete, verified field names:

**TUN inbound** (system-wide capture):
- `type: "tun"`, `address: ["172.19.0.1/30", "fdfe:dcba:9876::1/126"]` (the unified
  `address` field — `inet4_address`/`inet6_address` were removed in 1.12.0),
  `auto_route: true`, `strict_route: true`, `stack` (`system` preferred on Windows;
  `mixed`/`gvisor` cross-platform), `route_exclude_address` for LAN/RFC1918,
  `auto_redirect: true` on Linux only.

**VLESS outbound** (the tunnel):
```json
{
  "type": "vless", "tag": "proxy",
  "server": "<host>", "server_port": 443,
  "uuid": "<uuid>", "flow": "xtls-rprx-vision", "network": "tcp",
  "tls": {
    "enabled": true, "server_name": "<camouflage SNI>",
    "utls": { "enabled": true, "fingerprint": "chrome" },
    "reality": { "enabled": true, "public_key": "<pbk>", "short_id": "<sid>" }
  }
}
```

**DNS (anti-leak):** `dns.servers` includes a DoH server with `detour: "proxy"`;
`dns.final` points at that DoH server so the system/ISP resolver is never used. On
1.13.x, port-53 is captured with route-rule `sniff` + `hijack-dns` (the TUN
`dns_mode` field is 1.14+, so not used on our pinned stable).

**Route (split-tunnel + loop avoidance):** `route.auto_detect_interface: true`
(binds the proxy's own outbound to the physical NIC to prevent a routing loop);
rules send RU traffic to `direct` and `final: "proxy"` sends the rest through the
tunnel; rule-sets via `rule_set` (see §10).

**Group outbounds (multi-server):** a `urltest` group (auto, lowest-latency) and/or
a `selector` (manual), controlled at runtime via the Clash API (§11).

**Experimental:** `clash_api` on `127.0.0.1:<port>` with a random `secret`;
`cache_file.enabled: true` (+ `store_selected`) to cache rule-sets and persist the
chosen server across restarts.

Blocking uses the route-rule action **`reject`** (`method: default` = RST+ICMP, or
`drop`). There is no `block`/`blackhole` *outbound* in current sing-box.

---

## 8. Reliability and failure handling

- **State machine** in `manager` is the single source of truth; all UIs render it.
- **Auto-reconnect** with exponential backoff + jitter and a max-attempts cap;
  after the cap, the active server is marked unhealthy.
- **Health-check** drives proactive action: on sustained degradation the manager
  reconnects or switches server (auto-switch is Phase 2+; manual switch earlier).
- **Server switching** at runtime via the Clash API (no full core restart needed
  for a `selector`).
- **Structured logging** with rotation; **secrets (UUID, REALITY keys) are masked**
  in all logs and error messages.
- **Typed error codes** cross the IPC/CLI boundary so UIs can show actionable text.

---

## 9. Kill-switch and DNS-leak prevention (fail-closed)

This is the subtlest part of the design, corrected after verification:

> sing-box has **no built-in kill switch** (feature request #2222 closed as
> not-planned). `strict_route` and `reject` only act **while the core is running**.
> If the core crashes or is killed, the OS tears down the TUN and its routes, the
> physical default route returns, and **traffic fails open (leaks)**.

Therefore the kill-switch is a **`internal/platform` responsibility built on the
OS firewall**, applied independently of the core:

1. On connect, **before** starting the core, install a default-deny-egress rule set
   that permits traffic **only** via the TUN interface, plus a narrow allow for the
   proxy server IP on the physical NIC (needed for the REALITY handshake), plus
   loopback and DHCP.
2. Keep those rules in place for the whole connected lifetime — so a core crash
   leaves the machine **fail-closed**, not leaking.
3. Remove the rules only on an explicit, clean `disconnect`.

In-core measures complement this: `strict_route` blocks side-channels and (on
Windows) installs WFP filters that block port-53 on non-TUN interfaces;
`auto_detect_interface` prevents loops; DNS is DoH-over-proxy with `dns.final`
never falling back to system DNS.

**Settings:** kill-switch defaults to **fail-closed**, with an explicit user
opt-out (`fail-open`) for people who would rather keep flaky connectivity than lose
all traffic. Residual risks we document, not silently swallow: WebRTC/local-IP
discovery and interface-pinned apps can bypass a TUN unless caught by the firewall
rules; `strict_route` on Windows can break VirtualBox-style adapters.

Per-platform kill-switch implementations are staged: Windows first (primary
audience), then Linux, then macOS (see PHASED-PLAN).

---

## 10. Split-tunneling

Implemented purely with sing-box route rules + rule-sets (no custom routing code):

- Rule-sets use the binary **`.srs`** format (the old bundled `geoip.db`/
  `geosite.db` were removed in 1.8.0). Russian sets: **`geosite-category-ru`**
  (domains) and **`geoip-ru`** (IPs). Note the exact name is `geosite-category-ru`
  — `geosite-ru` does not exist.
- Default policy: **RU bypasses the tunnel** (`action: route, outbound: direct`),
  everything else goes through (`final: "proxy"`). Inverse policy is a one-line
  swap.
- Rule-sets are fetched as `remote` with `cache_file` enabled, or bundled as
  `local` `.srs` for offline reliability. Because `raw.githubusercontent.com` may
  itself be blocked in RU, rule-set downloads are routed through the proxy
  (`http_client`, formerly `download_detour`).
- For higher accuracy than the coarse country GeoIP, the design allows swapping in
  **Antizapret/Roskomnadzor-derived** custom `.srs`
  (e.g. `savely-krasovsky/antizapret-sing-box`). This is a config choice, not a
  code change.

---

## 11. Multi-server and health-based selection

- Multiple servers from the subscription become multiple VLESS outbounds.
- A **`urltest`** group auto-selects the lowest-latency node (probe `interval` 3m,
  `tolerance` 50ms hysteresis to prevent flapping); a **`selector`** offers a manual
  override. The user picks a server (or "auto") in the CLI/tray.
- Runtime control is via the Clash API on loopback: `PUT /proxies/{name}` switches
  the selector; `GET /proxies/{name}/delay` measures a node. The chosen server is
  persisted via `cache_file` (`store_selected`) across restarts.
- Auto-failover *between* servers on degradation is layered on top of `urltest` in
  Phase 2+; manual switching exists from the moment multiple servers do.

---

## 12. Security model

- **Secrets** (server UUIDs, REALITY keys, subscription URLs) live in the per-user
  config dir with owner-only permissions; they are masked in logs and never sent
  over IPC in cleartext beyond what the core needs.
- **IPC** (Phase 3) authorizes by peer credentials; the Clash API binds only to
  `127.0.0.1` with a random secret.
- **Least privilege**: only the daemon/core host is elevated; the tray runs
  unprivileged.
- **Supply chain**: pinned module versions, checksum-verified `wintun.dll` and
  rule-sets, reproducible builds where feasible.
- **License**: GPL-3.0 (see §3) — sources shipped/available to recipients.

---

## 13. Platform-specific concerns

| | Windows | Linux | macOS |
|---|---|---|---|
| TUN | `wintun.dll` shipped beside the exe; Admin required; `stack: system` recommended | root or `CAP_NET_ADMIN`; `auto_redirect` (nftables) for throughput | utun; root for the CLI, or NetworkExtension + System Extension for a packaged app |
| Kill-switch | WFP (`netsh advfirewall`/WFP API) | nftables/iptables | pf |
| Tray | `-ldflags -H=windowsgui` to hide console | needs an SNI/AppIndicator host (libayatana-appindicator); GNOME needs an extension/snixembed | `Run()` on main thread; `.app` bundle, `LSUIElement=true` |
| Elevation | UAC | systemd service / pkexec | privileged helper / SMJobBless |

---

## 14. Testing strategy

TDD throughout (red→green→refactor). Layers:

- **Unit**: subscription/`vless://` parsing (incl. malformed/forward-compat cases),
  domain→`option.Options` translation, state-machine transitions (with a mock
  `core`), backoff math, secret masking.
- **Integration**: bring up the embedded core against a known config; the **Phase 0
  compatibility gate** connects to the user's real Xray+REALITY server and verifies
  end-to-end reachability — this is the make-or-break test that de-risks the whole
  sing-box↔Xray assumption early.
- **E2E (semi-manual, scripted checks)**: after connect, assert external IP changed,
  run a DNS-leak check, and verify the kill-switch blocks traffic when the core is
  forcibly killed.

---

## 15. Open risks to validate during build

1. **sing-box↔Xray live interop** — proven in general; must be confirmed against
   *this* server in Phase 0 (the gate).
2. **Per-platform kill-switch correctness** — firewall rules are fiddly and
   OS-version-sensitive; each platform's fail-closed behavior is verified by the
   kill-the-core E2E test before that platform is declared done.
3. **macOS distribution** — System Extension approval + notarization is heavy; may
   slip relative to Windows/Linux. Tracked as a packaging risk, not an architecture
   one.
4. **Rule-set availability in RU** — mitigated by proxied download + bundled local
   `.srs` fallback.
5. **sing-box API churn** — we pin `v1.13.13`; the embedding gate re-validates on
   any bump (the v1.11 DI-context change is the kind of break to watch for).
