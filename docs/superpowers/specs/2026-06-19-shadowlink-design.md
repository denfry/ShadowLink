# ShadowLink — Design Spec (decision record)

Date: 2026-06-19
Status: approved (design baseline)
Authoritative architecture: [`/ARCHITECTURE.md`](../../../ARCHITECTURE.md)
Build order: [`/docs/PHASED-PLAN.md`](../../PHASED-PLAN.md)

This spec captures **why** ShadowLink is the way it is — the requirements, the
alternatives considered, and the decisions taken. `ARCHITECTURE.md` is the source
of truth for **what/how**; this document does not duplicate it, it justifies it.

---

## 1. Problem & purpose

A small group of friends needs reliable, system-wide access to the open internet
from a heavily DPI-filtered network (Russian TSPU). A server already exists. We are
building the **client**: a management/UI layer over an embedded proxy core that
stays connected and unfingerprintable on a hostile network, and is usable by both
technical and non-technical people.

This is legitimate anti-censorship / privacy work: restoring open access for the
user and their friends, against a national-scale DPI filter. It is not a tool for
attacking third parties.

## 2. Requirements (from brainstorming)

Confirmed with the user, in order asked:

1. **Users & platform:** mixed technical levels; **desktop app with system tray +
   CLI core**; Windows/macOS/Linux.
2. **Server side:** **client only** — server already running; configs distributed
   by the user via subscription link / file.
3. **Server protocol:** **VLESS + REALITY + XTLS-Vision (Xray-core)**.
4. **Traffic mode:** **TUN (system-wide VPN)**, kill-switch capable.
5. **Reliability (all required for MVP):** auto-reconnect + health-check;
   kill-switch + anti-DNS-leak; multiple configs + switching; split-tunneling.
6. **Core approach:** **embed sing-box in-process (Go)** — Approach A.

Non-functional: single self-contained binary; least-privilege; secrets never
leaked to logs; honest failure handling (fail-closed by default).

## 3. Success criteria

- A non-technical friend installs the Friends release and connects in one click on
  a clean machine (each OS).
- The author can rely on it daily: it reconnects through blips, switches servers,
  and **never leaks the real IP/DNS** — verified by killing the core and observing
  the firewall hold.
- The client interoperates with the existing Xray REALITY server with no server
  changes.

## 4. Alternatives considered

**Core (decision: A).**
- **A — embed sing-box (Go), chosen.** Library-first design; natively provides TUN,
  DNS, routing/split-tunnel, multi-outbound selection; one static binary; client
  interoperates with the Xray server. Thin, testable management layer.
- **B — embed xray-core (Go).** Maximal "native" protocol parity with the server,
  but no built-in TUN (needs a separate tun2socks stack) and manual kill-switch /
  split-tunnel plumbing — we'd reimplement what sing-box gives for free.
- **C — official core binary as a subprocess + manager.** Independent core updates,
  but not a single file, harder packaging/signing, version drift; worst for
  distribution to non-technical friends. Retained only as the fallback if a future
  closed-source requirement conflicts with sing-box's GPL-3.0 (see §6).

**Traffic mode (decision: TUN).** TUN gives the one-click, whole-system UX and
supports a real kill-switch; the cost (admin rights, TUN driver) is accepted and
handled in `internal/platform`. A local SOCKS/HTTP proxy was rejected for MVP
(doesn't capture all apps, weaker guarantees) though the architecture could expose
it later as a no-admin fallback.

**Privilege model (decision: evolve).** Start single elevated process (CLI);
introduce privileged-daemon + unprivileged-UI split only when the tray (a second
client) appears. Avoids building IPC before it's needed, and avoids ever running
the tray as root.

## 5. Key risks & mitigations

- **sing-box↔Xray interop on this server** → Phase 0 compatibility gate against the
  real server before any further investment.
- **No built-in kill-switch; fail-open on core crash** → fail-closed enforced by OS
  firewall in `internal/platform`, independent of the core; verified by the
  kill-the-core E2E test, per platform.
- **macOS distribution weight** (System Extension + notarization) → sequenced last;
  a packaging risk, not an architecture risk.
- **Rule-set/subscription blocked in RU** → proxied downloads + bundled local
  fallback.
- **sing-box API churn** → pinned `v1.13.13`; embedding gate re-validates on bumps.

## 6. Decisions with lasting consequences

- **License: GPL-3.0.** Embedding sing-box (GPL-3.0) in-process makes ShadowLink a
  derivative work; the project is GPL-3.0. Accepted (open tool among friends).
- **Pinned stack:** sing-box `v1.13.13`, Go 1.24+, `fyne.io/systray v1.12.2`.
- **Server constraints are validated at import**, not assumed (Vision-over-TCP,
  matching REALITY `pbk`/`sid`, camouflage `sni`).

## 7. Out of scope

Server provisioning; mobile clients; multi-tenant/service features; new protocols
or crypto. (Architecture must not preclude mobile later, but it isn't built.)

---

## Appendix A — Verification of technical premises (2026-06)

Premises were checked against live upstream sources (sing-box docs, `SagerNet/
sing-box` source at `v1.13.13`, pkg.go.dev) with an adversarial second pass on the
load-bearing claims. Corrections folded into the design:

- **Embedding (refuted→corrected):** API confirmed (`box.New`/`box.Options`/
  `include.Context`/`PreStart|Start|Close`; sing-box custom JSON, not
  `encoding/json`). Corrections: license is **GPL-3.0** (not GPL-2.0); latest stable
  **v1.13.13** (2026-06-04), Go **1.24+**.
- **VLESS+REALITY+Vision interop (supported):** sing-box client ⇄ Xray server is the
  canonical path; `flow` ∈ {`""`,`xtls-rprx-vision`}; Vision only over bare
  TLS/REALITY (no ws/grpc); uTLS fingerprint `chrome`/`firefox`/`edge`
  (`chrome_pq` is broken with REALITY).
- **TUN (supported):** unified `address` (legacy `inet4_address`/`inet6_address`
  removed 1.12.0); `auto_route`+`strict_route`; Windows needs `wintun.dll`+Admin;
  Linux root/`CAP_NET_ADMIN`; macOS root(CLI)/NetworkExtension(app).
- **Kill-switch/DNS (uncertain→corrected):** **no built-in kill-switch** (#2222
  not-planned); fail-closed must be OS-firewall-based in the management layer; use
  route action **`reject`** (there is **no `blackhole` outbound** in sing-box).
- **Split-tunnel (high):** `.srs` rule-sets; `geosite-category-ru` + `geoip-ru`
  (note: `geosite-ru` does not exist); action `route`+`outbound`, `final`.
- **Health/selection (high):** `urltest` (auto, 3m/50ms hysteresis) + `selector`
  (manual) via Clash API on loopback; persist with `cache_file store_selected`.
- **Tray (high):** `fyne.io/systray v1.12.2` (getlantern fork is stale);
  `Run()` on main thread; Linux needs an SNI/AppIndicator host.
- **Subscription (high):** base64 newline-separated `vless://`; lenient decode;
  ignore unknown query keys (forward-compat with `pqv`/`mlkem`); `spx` dropped on
  import to sing-box.
