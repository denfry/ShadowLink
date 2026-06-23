# Flutter Windows GUI Acceptance

The Windows-first slice of the cross-platform app: a Flutter UI driving the
sing-box core through `shadowlink_core.dll` over `dart:ffi`. This is a manual
gate — it needs the build toolchain, your live server, and Administrator rights.
Record the result at the bottom.

**Prereqs:** Flutter (stable, Windows desktop enabled: `flutter config --enable-windows-desktop`),
Visual Studio with the "Desktop development with C++" workload, **mingw-w64
(x86_64)** on PATH (`CC=x86_64-w64-mingw32-gcc`), Go 1.24+, and `bin/wintun.dll`.

## Build status — validated 2026-06-23 (on the author's Windows machine)

These steps were RUN and confirmed; only the **live tunnel** (needs admin + your
real server) is left for you:
- `build/build_windows.ps1` builds `app/windows/native/shadowlink_core.dll`
  (46 MB, sing-box + cgo shim) — validates that sing-box compiles as a c-shared
  DLL with mingw-w64 16.1.0, and that the cgo shim is correct. The `.h` exports
  all 8 `SL_*` functions.
- `app/` Flutter: `flutter pub get` ok, `flutter analyze` = **No issues found**,
  `flutter test` = **2/2 passed** (controller on `FakeCore`).
- The Windows runner is scaffolded (committed under `app/windows/`).
- `flutter build windows --release` → `shadowlink.exe` builds, and **launching it
  starts cleanly and stays running** — i.e. `ShadowlinkCore.open()` loads the DLL
  via `dart:ffi`, the `SL_Status` round-trip works, and the UI renders
  DISCONNECTED. The whole `Go ffiapi → cgo shim → DLL → dart:ffi → UI` chain works.

## Build & run

1. **Build the Go core DLL:** `pwsh build/build_windows.ps1` (mingw-w64 x86_64 on
   PATH). If you see `cc1.exe: 64-bit mode not compiled in`, your `gcc` is 32-bit
   — install mingw-w64 (e.g. `choco install mingw`).
2. **App deps:** in `app/`, `flutter pub get`. (The `windows/` runner is already
   committed, so `flutter create` is not needed again.)
3. **Native DLLs are bundled automatically.** `app/windows/CMakeLists.txt` installs
   `shadowlink_core.dll` + `wintun.dll` (from `app/windows/native/`) next to the
   built `.exe` on every `flutter build`/`flutter run` — no manual copy. (So run
   step 1 BEFORE the Flutter build; if `native/` is empty the build errors clearly.)
4. **Build + run ELEVATED:** `flutter build windows --release`, then run
   `app/build/windows/x64/runner/Release/shadowlink.exe` **as administrator**
   (right-click → Run as administrator), OR launch normally and use the
   **"Relaunch as administrator"** button that appears on the needsAdmin banner.

> **Auto-UAC note:** the natural `requireAdministrator` manifest is left
> **commented out** in `app/windows/runner/runner.exe.manifest` because embedding
> an elevation manifest trips `mt.exe` (`LINK : fatal error LNK1327`) when Defender
> locks the binary during the manifest merge. So the build ships without auto-UAC;
> elevate at runtime (above). To re-enable auto-UAC, add a Defender exclusion for
> the build dir and uncomment the block.

> UI preview without the DLL: temporarily swap `ShadowlinkCore.open()` for
> `FakeCore()` in `app/lib/main.dart` to click through the screens without a
> built core. `flutter test` already runs against `FakeCore` and needs no DLL.

## Verify

- App opens; status shows **DISCONNECTED**.
- **Servers** page (top-right icon): import your `vless://` link (or a saved
  subscription file path / inline body); it appears in the list **masked**
  (`uuid=..**` `pbk=..**`) and is auto-selected.
- **Home**: **CONNECT** → status **CONNECTED**, the delay (ms) shows; in another
  shell `curl https://api.ipify.org` is the **server's** IP.
- **Browsing works with kill-switch ON** (the carried-over Phase 1 step-4 risk):
  open a normal site while connected — it must load. If it doesn't, the
  kill-switch is blocking tunneled traffic — see `docs/ACCEPTANCE-PHASE1.md`
  step 4 (allow the TUN subnet/interface).
- **DISCONNECT** → DISCONNECTED; normal networking restored.
- Launch the built `.exe` **non-elevated** → on CONNECT the
  "Run as administrator" banner appears (the `needsAdmin` path).

## Pass criteria (ALL)

- [ ] DLL builds with mingw-w64; `flutter test` (controller, FakeCore) passes; `flutter analyze` clean.
- [ ] App launches elevated; status reflects the real connection state.
- [ ] Import / select / list work and are **masked** (no UUID/key on screen).
- [ ] Connect changes the public IP; delay shows; disconnect restores networking.
- [ ] **Browsing works with the kill-switch ON.**
- [ ] No secret (UUID, REALITY key, ClashAPI secret) appears anywhere in the UI.

## Result

- Date / Flutter version / Go version / outcome (PASS/FAIL):
- DLL build (mingw-w64): PASS / FAIL — notes:
- Browsing with kill-switch on (step from §Verify): PASS / FAIL — notes:
- Other notes (analyzer warnings, FFI issues, anything that needed a workaround):
