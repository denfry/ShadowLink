# Flutter Windows GUI Acceptance

The Windows-first slice of the cross-platform app: a Flutter UI driving the
sing-box core through `shadowlink_core.dll` over `dart:ffi`. This is a manual
gate — it needs the build toolchain, your live server, and Administrator rights.
Record the result at the bottom.

**Prereqs:** Flutter (stable, Windows desktop enabled: `flutter config --enable-windows-desktop`),
Visual Studio with the "Desktop development with C++" workload, **mingw-w64
(x86_64)** on PATH (`CC=x86_64-w64-mingw32-gcc`), Go 1.24+, and `bin/wintun.dll`.

## Build & run

1. **Build the Go core DLL:** from the repo root, `pwsh build/build_windows.ps1`
   → produces `app/windows/native/shadowlink_core.dll` + `shadowlink_core.h`,
   and stages `wintun.dll` beside it. If you see
   `cc1.exe: 64-bit mode not compiled in`, your `gcc` is 32-bit — install
   mingw-w64 (x86_64) and set `CC`.
2. **Scaffold the Flutter Windows runner** (first time only), in `app/`:
   `flutter create --platforms=windows --org com.shadowlink .`
   This generates `windows/`. Then **edit `app/windows/runner/runner.exe.manifest`**:
   inside the `<trustInfo>` → `<security>` → `<requestedPrivileges>` block, set
   ```xml
   <requestedExecutionLevel level="requireAdministrator" uiAccess="false" />
   ```
   (so the app launches elevated — the TUN + firewall need it). Then `flutter pub get`.
3. **Stage the native DLLs beside the runner exe.** The build script writes them
   to `app/windows/native/`; for `flutter run` copy `shadowlink_core.dll` and
   `wintun.dll` into `app/build/windows/x64/runner/Debug/` (next to the built
   `.exe`, where `dart:ffi` resolves the DLL and sing-box loads wintun). For a
   release build, have `windows/runner` CMake bundle them into the output.
4. **Run elevated:** from an **Administrator** terminal, in `app/`: `flutter run -d windows`.

> UI preview without the DLL: temporarily swap `ShadowlinkCore.open()` for
> `FakeCore()` in `app/lib/main.dart` to click through the screens without a
> built core (useful for layout work). `flutter test` already runs against
> `FakeCore` and needs no DLL.

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
