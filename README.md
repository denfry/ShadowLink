# ShadowLink

A cross-platform **personal VPN client** for a small group of friends — a thin,
reliable management/UI layer over an embedded [sing-box](https://sing-box.sagernet.org)
proxy core, tuned to stay connected and unfingerprintable on a heavily DPI-filtered
network (Russian TSPU).

ShadowLink is the **client only**. It connects to a server you already run
(Xray-core, VLESS + REALITY + XTLS-Vision). It does not provision servers.

> **Status: design baseline (pre-MVP).** Architecture and plan are set; code lands
> per the [phased plan](docs/PHASED-PLAN.md). Build/run commands below activate as
> phases land — each is marked with the phase that enables it.

## Why

On TSPU-class networks, the hard part isn't the proxy math (the core solves that) —
it's the management layer: keeping the tunnel alive through throttling and resets,
never leaking your real IP/DNS when it drops, and being usable by people who aren't
you. That's what ShadowLink is. See [`ARCHITECTURE.md`](ARCHITECTURE.md) for the
full design and threat model.

## Features

- System-wide VPN via TUN (all OS traffic), one-click connect.
- VLESS + REALITY + XTLS-Vision client (interoperates with your Xray server).
- Auto-reconnect with backoff + health-checks.
- **Fail-closed kill-switch** (OS-firewall based) + DNS-leak prevention.
- Multiple servers with manual/auto switching (lowest-latency).
- Split-tunneling (e.g. Russian sites bypass the tunnel).
- CLI for power users + system-tray app for everyone else.

## Server requirements

Your existing Xray-core server must offer **VLESS + REALITY + XTLS-Vision over raw
TCP**:

- VLESS user with `flow: xtls-rprx-vision`, `network: tcp` (Vision does not work
  over ws/grpc/xhttp).
- A REALITY key pair + at least one short ID; a real, allowed camouflage site as
  `serverName`/dest.
- You hand friends a `vless://` link or a base64 subscription that encodes
  `uuid`, `host`, `port`, `sni`, `fp`, `pbk` (public key), `sid` (short id),
  `flow`, `type=tcp`. ShadowLink validates these on import.

## Install (Phase 5)

Signed installers per OS (Windows bundles `wintun.dll`; Linux deb/rpm/AppImage +
systemd; macOS pkg + System Extension). Not available yet.

## Build from source

Requirements: **Go 1.24+**, a C toolchain only where a platform needs it, and Git.

Build tags are **required** (not optional): `with_utls` (REALITY needs uTLS, or the
core refuses to start), `with_gvisor` (TUN network stack), `with_clash_api` (runtime
control). The `Makefile` encodes them.

```sh
git clone https://github.com/denfry/ShadowLink shadowlink && cd shadowlink
make build            # -> bin/shadowlink   (uses the required tags)
# or directly:
go build -tags "with_utls with_gvisor with_clash_api" -o bin/shadowlink ./cmd/shadowlink
go test  -tags "with_utls with_gvisor with_clash_api" ./...
```

> Pinned to **sing-box v1.13.7**: v1.13.8–v1.13.13 currently fail to compile in an
> embedded Windows build (an upstream sing-box↔sing-tun API mismatch). v1.13.7 is the
> latest 1.13.x that builds; revisit when upstream realigns.

Platform build notes:

- **Windows:** place `wintun.dll` (matching your arch) next to the binary. The tray
  build (Phase 3) uses `-ldflags -H=windowsgui` to hide the console.
- **Linux:** grant the binary `CAP_NET_ADMIN` (or run via the systemd unit) instead
  of running as root; the tray (Phase 3) needs a desktop with an AppIndicator/SNI
  host (e.g. `libayatana-appindicator`; on GNOME, the AppIndicator extension).
- **macOS:** the CLI needs `sudo` to create the utun device; the packaged app
  (Phase 5) uses a NetworkExtension/System Extension instead.

## Run

> These require **administrator/root** because creating the TUN device, programming
> routes, and installing the kill-switch firewall rules are privileged operations.

```sh
# (Phase 0) connect with a single share link:
sudo ./shadowlink connect "vless://<uuid>@<host>:443?security=reality&encryption=none&type=tcp&flow=xtls-rprx-vision&sni=<camouflage>&fp=chrome&pbk=<publickey>&sid=<shortid>#MyNode"

# (Phase 1) import then use a stored profile:
./shadowlink import "<vless-uri-or-subscription-url>"
sudo ./shadowlink connect
./shadowlink status
sudo ./shadowlink disconnect
```

The system-tray app (`shadowlink-tray`) arrives in Phase 3.

## Project layout

```
cmd/shadowlink        CLI + privileged core host
cmd/shadowlink-tray   system-tray UI (Phase 3)
internal/core         the only package importing sing-box
internal/config       domain model + persistence
internal/subscription vless:// & subscription parsing
internal/manager      connection state machine (source of truth)
internal/health       latency probes / liveness
internal/platform     TUN deps, firewall kill-switch, service (per-OS)
internal/ipc          local control plane (Phase 3)
docs/                 plan + design spec
```

## Security & license

- Secrets (UUIDs, REALITY keys, subscription URLs) are stored owner-only and masked
  in logs. Kill-switch defaults to fail-closed.
- **License: GPL-3.0.** ShadowLink embeds sing-box (GPL-3.0) in-process, which makes
  it a derivative work; the project is licensed GPL-3.0 accordingly. See
  [`LICENSE`](LICENSE).

## Documentation

- [`ARCHITECTURE.md`](ARCHITECTURE.md) — full design, threat model, components.
- [`docs/PHASED-PLAN.md`](docs/PHASED-PLAN.md) — build order + acceptance criteria.
- [`docs/superpowers/specs/2026-06-19-shadowlink-design.md`](docs/superpowers/specs/2026-06-19-shadowlink-design.md)
  — decision record + verification appendix.
