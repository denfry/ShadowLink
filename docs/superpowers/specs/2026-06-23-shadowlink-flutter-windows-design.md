# ShadowLink — Cross-Platform App (Flutter + sing-box core), Windows-first

**Status:** Design approved 2026-06-23. Feeds the implementation plan (writing-plans).
**Supersedes nothing:** builds on the Phase 1 Go engine (`feat/phase-1`); reuses its `internal/*` packages.

## 1. Goal & Context

The user wants a **single cross-platform VPN app** for Windows, macOS, Linux, Android and iOS — the Hiddify model: a **Flutter UI** over the **sing-box core**, with the core exposed as a native library (FFI). Strategy chosen: **shared Go core via FFI** (approach A, the hiddify-core/libbox pattern), built once and consumed on every platform.

**Sequencing:** architecture targets all five platforms, but we build and harden **one platform at a time, Windows first** (the user is on Windows; fastest iteration; the Phase 1 Go engine already runs there).

**This spec covers the FIRST sub-project only:** a Windows Flutter app that connects/disconnects to a selected server and shows live status, driving sing-box through the shared Go core over `dart:ffi`. Android/iOS/macOS/Linux, rich UI, and packaging are explicitly deferred to later sub-projects (§10).

**Core principle — three front-ends over one engine:** Phase 1's `internal/*` packages are the engine. The CLI (`cmd/shadowlink`), the FFI library (`cmd/libshadowlink`), and the Flutter app are all thin front-ends over it. Nothing from Phase 1 is thrown away.

## 2. Verified Environment Constraint

This development sandbox **cannot build the c-shared DLL or the Flutter app.** Verified empirically: the only 64-bit-capable state fails — the installed `gcc` is MinGW.org 6.3.0 (32-bit only: `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`), and the available `clang` targets `x86_64-pc-windows-msvc` and rejects Go cgo's `-mthreads` flag. There is no `mingw-w64`, no Flutter SDK, no Android NDK, no Xcode.

**Consequence:** in this environment we **author** all code (Go `ffiapi`, the cgo shim, Dart/Flutter, build scripts) and **unit-test the pure-Go layer** (`internal/ffiapi`). **Compiling the DLL and building/running Flutter happen on the user's machine** with the prerequisites in §11. The approach itself is proven: `hiddify-core` builds sing-box as a c-shared library for Windows/Linux/macOS with mingw-w64.

## 3. Architecture

```
Flutter (Dart UI)  ── dart:ffi (LoadLibrary at runtime) ──►  shadowlink_core.dll
  home / servers / settings                                   (cmd/libshadowlink, cgo shim)
  ffi bindings + ChangeNotifier state                              │ calls
                                                                   ▼
                                                          internal/ffiapi  (pure Go, JSON in/out — UNIT TESTED)
                                                                   │ orchestrates
                                                                   ▼
                                          internal/manager (kill-switch BEFORE tunnel, safe rollback)
                                          internal/core (render.go, box lifecycle, Clash API)
                                          internal/subscription · config · secret · platform
                                                                   │ embeds
                                                                   ▼
                                                  sing-box (VLESS+REALITY+Vision, TUN, DoH-over-proxy)
```

The DLL is loaded at runtime by `dart:ffi` (`DynamicLibrary.open`), so the MSVC toolchain that builds Flutter and the mingw-w64 toolchain that builds the Go DLL never link together — the boundary is a plain C ABI. One DLL image per process (Windows ref-counts `LoadLibrary`), so the Go singleton state is shared across Dart isolates.

## 4. Components

### 4.1 `internal/ffiapi` — pure-Go API layer (unit-tested here)

No `C.*` types. Functions take/return JSON strings (and Go types), orchestrate via `manager` using injected `Deps` (same DI pattern as Phase 1, so tests use fakes — no privileges, no real tunnel). Holds the process-wide singleton: current `*manager.Manager` + `*core.ClashClient`, guarded by a `sync.Mutex`.

| Go function | Request JSON | Response JSON | Reuses |
|---|---|---|---|
| `Version() string` | — | `{"ok":true,"version":"..."}` | `version` |
| `ListServers() string` | — | `{"ok":true,"servers":[{"tag","host","masked"}],"selected":"<tag>"}` | `config.Load`, `secret.MaskServer` |
| `Import(input string) string` | raw input (uri/file/body) | `{"ok":true,"servers":[...]}` (also persists profile) | `subscription.Import`, `config.Save` |
| `Select(tag string) string` | — | `{"ok":true}` / error | `config.Load/Save` |
| `Start(optsJSON string) string` | `{"failOpen":bool,"tag":string?}` | `{"ok":true}` / `{"ok":false,"error":"<masked>","needsAdmin":bool}` | `manager.Connect`, `render`, `core.New`, `platform` |
| `Stop() string` | — | `{"ok":true}` / error | `manager.Disconnect` |
| `Status() string` | — | `{"ok":true,"state":"connected","server":"<masked>","delayMs":42,"error":""}` | `manager.State`, `core.ClashClient.Delay` |

Rules: every error string passes through `secret.Mask` (no UUID/key/secret leaks). `Start` maps an OS access-denied (no admin) to `needsAdmin:true`. `Start` builds a fresh `manager.New(Deps{Render: core.RenderConfig, NewTunnel: func(b)(manager.Tunnel,error){return core.New(b)}, KillSwitch: platform.NewKillSwitch()})`; `Stop` tears it down and drops the singleton.

### 4.2 `cmd/libshadowlink` — cgo shim (built into the DLL on the user's machine)

`package main` with `import "C"`. One exported wrapper per `ffiapi` function; each does `C.GoString` on inputs, calls `ffiapi`, returns `C.CString` of the JSON (caller frees). Plus `SL_FreeString`. The C ABI (generated `libshadowlink.h`):

```c
char* SL_Version(void);
char* SL_ListServers(void);
char* SL_Import(char* input);
char* SL_Select(char* tag);
char* SL_Start(char* optsJSON);
char* SL_Stop(void);
char* SL_Status(void);
void  SL_FreeString(char* p);
int   main(void); // required, empty, for c-shared
```

Memory: incoming `char*` → `C.GoString`; outgoing → `C.CString` (malloc), Dart must `SL_FreeString`. No Go pointers retained by C (cgo rule). Threading: the singleton mutex in `ffiapi` serializes concurrent cgo calls.

### 4.3 `app/` — Flutter (Windows-first, structured for all platforms)

```
app/
  pubspec.yaml            # dependency: package:ffi only (state via built-in ChangeNotifier)
  lib/
    main.dart             # entry, theme, Home/Servers navigation
    ffi/core_ffi.dart     # DynamicLibrary.open, typedefs, lookups, Utf8<->String, free
    ffi/core.dart         # CoreApi (interface) + ShadowlinkCore (FFI impl)
    models/{server,conn_status}.dart
    state/connection_controller.dart  # ChangeNotifier: status, SL_Status polling, connect()/disconnect()
    ui/{home_page,servers_page}.dart
  windows/                # generated by `flutter create --platforms=windows`;
                          #   app.manifest patched to requireAdministrator (TUN needs elevation)
  test/                   # widget tests against a fake CoreApi (no DLL)
```

**First-slice UI:** Home — connection status pill (Disconnected/Connecting/Connected/Error), a large Connect/Disconnect toggle, the selected server (masked) + live delay, a kill-switch checkbox (drives `failOpen`), and a "needs administrator" banner with a relaunch button. Servers — list of imported servers (masked, selected marked), an import text field (`vless://` / sub body / file path) + Import button, tap-to-select.

**State & threading:** no external state package — `ChangeNotifier` + `ListenableBuilder`. `Status` is a fast localhost delay probe polled by a ~2s timer. `ShadowlinkCore` sits behind the `CoreApi` interface so the controller and widgets are tested with a fake. **v1 runs the blocking FFI calls (`Start`/`Stop`/`Import`) on the main isolate** — a brief UI freeze during tunnel bring-up is acceptable for the first slice. Moving them to a background isolate is a deliberate follow-up (the isolate needs its own `DynamicLibrary` handle but still shares the one process-global Go singleton).

**Continuity:** the Go core reads/writes the same `profile.json` (`os.UserConfigDir()/shadowlink`) as the CLI — a server imported via either front-end is visible in the other.

## 5. Data Flow

- **Connect:** user taps toggle → controller calls `ShadowlinkCore.start({failOpen})` (main isolate in v1) → `SL_Start` → `ffiapi.Start` parses opts, loads the selected server from profile, `manager.Connect` (kill-switch enabled before tunnel, then `core.New`+`Start`) → JSON `{ok}` back → controller flips to Connecting/Connected; the 2s poller calls `SL_Status` for state + delay.
- **Status:** timer → `SL_Status` → `ffiapi.Status` returns `manager.State()` + a `ClashClient.Delay("proxy", …)` probe → UI updates.
- **Import/Select:** Servers page → `SL_Import`/`SL_Select` → persists profile → `SL_ListServers` refresh.
- **Disconnect:** toggle → `SL_Stop` → `manager.Disconnect` (tunnel Close + kill-switch Disable) → state Disconnected.

## 6. Error & Admin Handling

Every FFI result is `{ok,...}`. `ShadowlinkCore` throws `CoreException(message, needsAdmin)` on `ok:false`. `needsAdmin` (TUN/firewall access denied) surfaces the Home banner + a "relaunch as administrator" button (`ShellExecute "runas"`, then exit). The `windows/runner` manifest already requests `requireAdministrator`, so a normally-launched build is elevated; `needsAdmin` is the belt-and-suspenders path for `flutter run` from a non-elevated terminal. Connect failures (REALITY handshake, unreachable server) show in the status area as state Error with retry. All error text is masked in Go.

## 7. Build Pipeline (Windows) — `build_windows.ps1`

1. `CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -buildmode=c-shared -tags "with_utls with_gvisor with_clash_api" -o app/windows/native/shadowlink_core.dll ./cmd/libshadowlink` → `shadowlink_core.dll` + `.h`.
2. Copy `shadowlink_core.dll` **and** `wintun.dll` into the Flutter runner output dir (beside the `.exe`, where `dart:ffi` resolves the DLL and sing-box loads wintun). For dev that is `app/build/windows/x64/runner/Debug/`; the runner `CMakeLists.txt` is extended to bundle both into release builds.
3. `flutter build windows` (release) or `flutter run -d windows` from an **elevated** terminal (dev).

## 8. Testing

- **Here (CI-able, no privileges):** TDD `internal/ffiapi` — request→response JSON, error masking, `ListServers`/`Import`/`Select`/`Status` marshalling, `Start`/`Stop` orchestration via fake `manager.Deps` (fake tunnel + fake kill-switch, exactly like Phase 1's manager tests). The cgo shim and DLL build are validated by the user's build.
- **User's machine:** Flutter widget tests against a fake `CoreApi`; a manual GUI acceptance step layered on `docs/ACCEPTANCE-PHASE1.md` (connect via the button, see Connected + delay; disconnect restores; kill-switch behavior).

## 9. Reuse Map

As-is from Phase 1: `internal/core`, `internal/manager`, `internal/subscription`, `internal/config`, `internal/secret`, `internal/platform`. New: `internal/ffiapi` (+tests), `cmd/libshadowlink` (cgo shim), `app/` (Flutter), `build_windows.ps1`. `cmd/shadowlink` (CLI) stays as a third front-end / debug tool.

## 10. Scope & Cross-Platform Roadmap

**In this sub-project:** Windows Flutter app — connect/disconnect to the selected server; live status (state + delay); import/select/list via the shared profile; kill-switch toggle; admin handling. Go `ffiapi` + `libshadowlink` + the c-shared build script.

**Deferred to later sub-projects (each its own spec → plan):**
- **Android:** the same Go core built via gomobile (`.aar`) + a Flutter plugin bridging to `VpnService` (the tunnel runs in-process; no privileged child process).
- **iOS/macOS:** Go core as `.xcframework` + `NetworkExtension` (Packet Tunnel Provider). Requires a Mac + Xcode.
- **Linux desktop:** Go core as `.so` + FFI (similar to Windows; CAP_NET_ADMIN instead of UAC).
- **Rich UI:** logs view, traffic graph, subscription auto-update + remote `http(s)://` fetch, split-tunnel toggle, multi-server `urltest`/selector switching.
- **Packaging:** per-platform installers, code signing, auto-update.

## 11. Toolchain Prerequisites (user's machine, Windows)

- **Flutter SDK** (stable) with Windows desktop enabled (`flutter config --enable-windows-desktop`).
- **Visual Studio** with the "Desktop development with C++" workload (Flutter's Windows toolchain).
- **mingw-w64** (x86_64) for the Go cgo c-shared build (`CC=x86_64-w64-mingw32-gcc`), e.g. via MSYS2 `mingw-w64-x86_64-gcc`.
- **Go 1.24+** (already present).
- `wintun.dll` (already in `bin/`) bundled beside the app.
- Run the built app (or `flutter run`) **elevated** (UAC) for TUN + firewall.

## 12. Open Validation Points (for the user's machine)

- sing-box compiled into a c-shared DLL brings the TUN up correctly when loaded by the Flutter process (proven by hiddify-core; confirm on the real machine).
- The kill-switch-vs-TUN-traffic question carried over from Phase 1 (`docs/ACCEPTANCE-PHASE1.md` step 4) still applies — the engine is identical.
