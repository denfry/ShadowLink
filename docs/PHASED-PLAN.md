# ShadowLink — Phased Plan

Each phase is a **working increment** with explicit acceptance criteria. A phase is
"done" only when its criteria pass (TDD; evidence before claims). All four
reliability features the user requested — auto-reconnect+health-check,
kill-switch+anti-DNS-leak, multiple servers+switching, split-tunneling — are
present; they are sequenced across increments, not dropped.

Legend: **Personal release** = good enough for the author to rely on.
**Friends release** = installable in one click by a non-technical friend.

---

## Phase 0 — Skeleton + compatibility gate  ⟶ *de-risk everything*

**Goal:** prove the core assumption (embedded sing-box client connects to the
existing Xray+REALITY server) before building anything on top of it.

- Go module, repo layout (`cmd/`, `internal/`), CI (build + vet + test on
  win/mac/linux), pinned deps (`sing-box v1.13.13`, Go 1.24+).
- `internal/core` minimal path: parse one `vless://` link → build `option.Options`
  with a TUN inbound + VLESS+REALITY+Vision outbound → `include.Context` →
  `box.New` → `Start`/`Close`.
- A bare `shadowlink connect <vless-uri>` that brings the tunnel up (elevated).

**Acceptance:**
- Builds on all three OSes; CI green.
- On a real machine behind (or simulating) the filter, `connect` to the user's
  **actual** server succeeds and external IP changes; `disconnect` restores normal.
- If interop fails here, we re-evaluate (xray-core embed / sing-tun) *now*, not later.

---

## Phase 1 — MVP core (CLI)  ⟶ *Personal release*

**Goal:** a dependable single-server CLI VPN with leak protection.

- `internal/config` domain model + persistence (config dir per OS).
- `internal/subscription` parsing of `vless://` and base64 subscriptions (lenient
  decode; constraint validation per ARCHITECTURE §4).
- `internal/manager` state machine + auto-reconnect (backoff+jitter) + health-check
  (`internal/health` via Clash API delay).
- DNS forced through tunnel (DoH-over-proxy; `dns.final` never system).
- **Kill-switch (Windows first)**: firewall fail-closed, independent of core
  lifecycle; `--fail-open` opt-out.
- **Distinct TUN identity** (gate finding): set a non-default `interface_name`
  (e.g. `shadowlink0`) and non-default subnet (e.g. `172.18.x`) in render.go so we
  don't collide with other sing-box clients (Hiddify/Nekoray default to `tun0` /
  `172.19.0.1`). Detect-and-warn if the address is already in use.
- CLI: `connect`, `disconnect`, `status`, `import`, `logs`. `import` accepts a
  subscription URL (gate showed real configs arrive only as sub links).

**Acceptance:**
- Unit tests: parser, domain→options translation, state machine (mock core),
  backoff, secret masking — all green.
- Kill the core process while connected → **no traffic leaks** (firewall holds);
  DNS-leak test passes; external IP is the server's.
- Auto-reconnect recovers from a forced network blip without user action.

---

## Phase 2 — Multiple servers + switching

- Multiple outbounds from a subscription; `urltest` (auto) + `selector` (manual).
- `servers list|select`, ping display, `sub update` (refresh through the tunnel).
- Persist selected server (`cache_file store_selected`).
- Basic auto-failover: on sustained health failure, switch to the next-best node.

**Acceptance:** with ≥2 servers, manual switch works at runtime (no full restart);
killing the active server's reachability triggers a switch within the health window;
selection survives a restart.

---

## Phase 3 — Tray GUI + daemon/IPC

- Split into privileged daemon/service + unprivileged UI; `internal/ipc`
  (socket/named pipe + peer-cred auth).
- `cmd/shadowlink-tray` (`fyne.io/systray`): status, connect/disconnect, server
  picker, logs, quit. `service install` + autostart.

**Acceptance:** tray connects/disconnects and switches servers via the daemon
without elevation of the tray itself; CLI still works against the same daemon;
clean install/uninstall of the service.

---

## Phase 4 — Split-tunneling

- Route rules + RU rule-sets (`geosite-category-ru`, `geoip-ru`); proxied download
  + bundled local `.srs` fallback; optional Antizapret-derived sets.
- User-managed include/exclude lists; toggle in CLI + tray.

**Acceptance:** with split-tunnel on, a known RU site resolves/routes **direct**
(real IP) while a blocked site routes **through** the tunnel; toggling off sends
everything through; rule-set refresh works even when the raw host is blocked.

---

## Phase 5 — Packaging + distribution  ⟶ *Friends release*

- Installers: Windows (bundles `wintun.dll`, service, UAC) ; Linux
  (deb/rpm/AppImage + systemd + CAP_NET_ADMIN) ; macOS (pkg + System Extension +
  notarization).
- Code signing where possible; checksum-verified assets; README quick-start aimed
  at a non-technical friend; optional auto-update.

**Acceptance:** a non-technical tester installs from the artifact and connects in
one click on a clean machine of each OS; uninstall is clean (incl. firewall rules
and macOS System Extension).

---

## Sequencing notes

- Kill-switch ships per-platform: **Windows (Phase 1) → Linux → macOS**, gated each
  time by the kill-the-core E2E test (ARCHITECTURE §14).
- Anything that bounds coverage (e.g. "Windows-only kill-switch in Phase 1") is
  stated, never silent.
- The compatibility gate (Phase 0) is the single most important checkpoint; we do
  not invest in Phases 1+ until it passes.
  **Status: PASSED 2026-06-19** against the live server (sing-box v1.13.7, build
  c7e7fcb) — IP changed, no DNS leak, clean disconnect. See `docs/GATE-PHASE0.md`.
  Phase 1 is unblocked.
