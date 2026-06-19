# Phase 0 Compatibility Gate

Purpose: prove the embedded sing-box client connects to the **real** Xray
VLESS+REALITY+XTLS-Vision server and carries traffic, before building Phase 1.

This is a manual gate (it needs your live server, admin rights, and ideally the
filtered network). Record the result at the bottom.

## Prerequisites

- **Build with the required tags** (REALITY won't start without `with_utls`):
  ```sh
  make build
  # or:
  go build -tags "with_utls with_gvisor with_clash_api" -o bin/shadowlink ./cmd/shadowlink
  ```
- **Windows:** put `wintun.dll` (matching your CPU arch) next to `bin/shadowlink.exe`,
  and run from an **Administrator** shell. `wintun.dll` ships inside the official
  sing-box Windows release archive (or from wintun.net).
- **Linux:** run with `sudo`, or `sudo setcap cap_net_admin+ep ./bin/shadowlink`.
- **macOS:** run with `sudo`.
- A real `vless://` link for the live server (REALITY+Vision over TCP).

## Procedure

1. Note baseline public IP: `curl https://api.ipify.org` (or a browser).
2. Connect (elevated):
   ```sh
   sudo ./bin/shadowlink connect "vless://<uuid>@<host>:443?security=reality&encryption=none&type=tcp&flow=xtls-rprx-vision&sni=<camouflage>&fp=chrome&pbk=<publickey>&sid=<shortid>#MyNode"
   ```
   (Windows: run `bin\shadowlink.exe connect "..."` from the Admin shell.)
3. In another shell, confirm:
   - Public IP changed to the server's: `curl https://api.ipify.org`
   - DNS resolves and pages load.
   - DNS-leak check (e.g. dnsleaktest.com) shows the server side, not the ISP/TSPU.
4. Ctrl+C to disconnect; confirm normal connectivity returns.

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

Re-evaluate before Phase 1: confirm `flow=xtls-rprx-vision` + `type=tcp` on the
server, that `pbk`/`sid`/`sni` match the server's REALITY `publicKey`/`shortIds`/
`serverName`, and try `fp=firefox` or `fp=edge`. If sing-box↔Xray REALITY proves
incompatible against this specific server, escalate to the xray-core embedding
fallback (ARCHITECTURE §3, Approach B) — do NOT proceed to Phase 1 on a red gate.
