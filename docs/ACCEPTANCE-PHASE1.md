# Phase 1 Acceptance (Windows first)

Validates the Phase 1 MVP: dependable single-server CLI VPN with persistence,
leak-resistant DNS, health detection, and a **fail-closed Windows kill-switch**.
This is a manual gate — it needs your live server, Administrator rights, and
(ideally) the filtered network. Record the result at the bottom.

> Built on the Phase 0 gate (PASSED 2026-06-19). Same prerequisites: build WITH
> the required tags, `wintun.dll` beside the exe, run from an **Administrator**
> shell, and **fully exit other sing-box clients (Hiddify/Nekoray) first** — though
> Phase 1 now uses a distinct TUN identity (`shadowlink0` / `172.18.0.1`) to avoid
> the default-`172.19.0.1` collision, so coexistence should work; step 0 verifies it.

## Build

```sh
go build -tags "with_utls with_gvisor with_clash_api" -o bin/shadowlink.exe ./cmd/shadowlink
```
(REALITY will not start without `with_utls`. Put `wintun.dll` for your CPU arch
next to `bin\shadowlink.exe`. Run everything below from an **Admin** shell.)

## Procedure

0. **TUN coexistence (distinct identity).** With Hiddify (or another sing-box
   client) RUNNING, run `bin\shadowlink.exe connect "<vless-uri>"`. ShadowLink
   should NOT fail with `set ipv4 address: The object already exists` (it uses
   `172.18.0.1`, not the shared `172.19.0.1`). If the TUN address is already in
   use you'll see a `warning: 172.18.0.1/30 is already assigned ...` line — heed it.
   Ctrl+C to stop, then continue with the profile flow below.

1. **Import + status.**
   - `bin\shadowlink.exe import "<vless-uri>"` → "imported 1 server(s); 1 total".
     (Real configs arrive as subscription links: if you have a base64/sub *file*,
     `import <path-to-file>` works; a remote `http(s)://` sub URL is Phase 2 —
     for now save its body to a file and import the file.)
   - `bin\shadowlink.exe status` → lists the server with **UUID and key masked**
     (`uuid=11**` `pbk=PB**`), a `*` on the selected one, `kill-switch: true`.

2. **Connect + IP change.** `bin\shadowlink.exe connect` → prints
   "connected via <tag> (host masked)". In another shell:
   `curl https://api.ipify.org` → public IP is the **server's**, not yours.

3. **DNS-leak.** Run a DNS-leak test (e.g. dnsleaktest.com). The resolver must be
   **server-side**, never the ISP/TSPU resolver. (DNS is DoH-over-proxy; `dns.final`
   never points at the system resolver.)

4. **Browsing WORKS with the kill-switch ON (critical — verify before step 5).**
   While connected (kill-switch default = on), open a normal site
   (`curl https://example.com` or a browser). It MUST load.
   > WHY THIS STEP EXISTS: the Windows kill-switch sets a default-deny outbound
   > policy and allows only the server IP + loopback. If WFP evaluates the app's
   > real destination (arbitrary IPs reached *through* the TUN) before routing,
   > tunneled traffic could be blocked — making the default-on kill-switch break
   > all browsing. If this step FAILS, the fix is to also allow the TUN
   > interface/subnet in `internal/platform/killswitch_windows.go` (see the
   > carry-forward note); record it and do not ship the default-on kill-switch
   > until browsing works with it on.

5. **Kill-switch fail-closed (the core safety test).**
   - While connected, **force-kill** the shadowlink process
     (`taskkill /F /IM shadowlink.exe`, or Task Manager).
   - Immediately attempt traffic (`curl https://example.com` / browser). It MUST
     **fail** — no leak to your real IP. (Firewall rules live in WFP, independent
     of the core process, so a crash stays fail-closed.)
   - Recover: `bin\shadowlink.exe connect` again works; then `Ctrl+C`
     (or `disconnect`) → normal networking restored.
   - Confirm restore: `netsh advfirewall show allprofiles firewallpolicy` shows
     **allowoutbound** again and no `ShadowLink-KillSwitch` rule remains
     (`netsh advfirewall firewall show rule name=ShadowLink-KillSwitch` → "No rules match").

6. **Health detection / reconnect after a blip.** While connected, disable then
   re-enable your NIC briefly (or pull/replug the network). Expect the health
   line to print `health: connection degraded` then `health: connection recovered`,
   and the session to keep working afterward.
   > Phase 1 does DETECTION + logging; the embedded sing-box core recovers its own
   > outbound after a blip. (Client-driven reconnect with backoff is staged for a
   > later phase.) If the session does NOT recover on its own, record it — that is
   > the signal to wire the manager's Backoff-driven reconnect.

7. **Fail-open escape hatch.** `bin\shadowlink.exe connect --fail-open` connects
   WITHOUT the kill-switch (use if step 4/5 firewall behavior locks you out while
   iterating). `status` still shows `kill-switch: true` (the profile default);
   `--fail-open` only affects that run.

## Pass criteria (ALL must hold)

- [ ] Coexists with another sing-box client (no TUN address collision).
- [ ] Connect changes the public IP to the server's; no DNS leak.
- [ ] **Normal browsing works while the kill-switch is ON** (step 4).
- [ ] Force-killing the core leaves the machine **fail-closed** (no traffic, no leak).
- [ ] `disconnect` fully restores networking (policy back to allowoutbound, rule removed).
- [ ] Health line reports degraded→recovered across a NIC blip; session keeps working.
- [ ] Secrets (UUID, REALITY key, ClashAPI secret) never appear in console output.

## Result

- Date / OS / sing-box version / build commit:
- Outcome (PASS / FAIL):
- Step 4 (browsing with kill-switch on): PASS / FAIL — notes:
- Step 5 (fail-closed on force-kill): PASS / FAIL — notes:
- Step 6 (blip recovery): PASS / FAIL — notes:
- Other notes (surprises, fingerprint used, anything that needed a workaround):

## If FAIL

- **Step 4 fails (kill-switch blocks tunneled traffic):** add a TUN
  interface/subnet allow in `killswitch_windows.go` (likely
  `localip=172.18.0.0/30` or an interface-scoped allow) and re-test; this may
  require passing the TUN subnet into `KillSwitch.Enable`.
- **Step 5 leaks:** the WFP policy or rule ordering is wrong — re-check that
  `Enable` does clear→block→allow (no `allowoutbound` window) and that the rules
  persist after the process dies.
- **Step 2/3 fail (no IP change / DNS leak):** this is a Phase 0-level interop
  regression — re-run the Phase 0 gate (`docs/GATE-PHASE0.md`).
