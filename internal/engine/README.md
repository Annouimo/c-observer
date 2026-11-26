# Observation engine

`engine.Engine` implements the shared `observer.Engine` contract using Clang and LLDB's DAP adapter. One call compiles local C, runs with preset stdin and returns a bounded replay history and separate stdout/stderr. The engine searches the selected Xcode toolchain via `xcrun` on macOS, then falls back to PATH. The launch configuration can override either executable.

The engine sets entry breakpoints for function definitions in the primary C source, then uses source-level steps. This records direct calls and library-invoked user callbacks without manual stop locations. Clang source inspection is bounded and optional. When it cannot establish function locations, the engine still supports explicit source stops. It captures frame variables, C arrays and structure fields. An array range can select a page outside the default prefix. The engine matches simple outer-block variables in the original main invocation across stops using Clang declaration locations and a complete LLDB stack. It also retains identities for captured fields and array elements. Nested scopes, repeated loop locals, recursive/helper frames, ambiguous declarations, and unverified source mappings use snapshot-local identities. The engine classifies matched LLDB objects from the primary source as globals, file statics, or function statics. Their declaration identity is shared across frames, and each object appears at most once per snapshot. Global enumeration is partial because LLDB can expose different function statics at different stops. The GDB backend is described below.

## Verification

```sh
go test ./...
go vet ./...
C_OBSERVER_REAL_DEBUGGER=1 go test -v ./internal/engine
```

The opt-in tests launch genuine C programs under a locally installed Clang and `lldb-dap`. They cover non-ASCII paths, separate I/O, a selected element of a large struct array, automatic entry into a qsort callback and resumed capture in main, a non-prefix page in a global matrix, static storage across three recursive frames, limits, crash handling, and cancelling a running inferior. The test checks that the inferior PID is gone after cancellation. Linux needs permission to trace the test process.

Linux runtime was exercised with Clang and LLVM 19.1.7. The package cross-builds for macOS arm64. The maintainer confirmed the arm64 v4 trial works on macOS 27.0.1 with the Xcode LLDB adapter. The v4 changes select the macOS SDK explicitly, include DAP pathFormat in initialize, and complete breakpoint configuration before waiting for a deferred launch response. The automated tests cover both deferred and immediate launch responses. Cleanup has a bounded adapter process-group fallback. Descendants outside that group and an unresponsive adapter are not covered by the real-process tests. Output limits bound the retained prefixes, and the temporary redirection files live until the run ends. The current history budget counts the encoded snapshots, not the Go heap.

To export a real replay for frontend work, set `C_OBSERVER_RECORD` to an output path when running the opt-in `TestRealObservation`. The result includes platform, stdin and source snapshots and is marked non-synthetic.

### Assembled source

For a Companion source map, the engine resolves Clang AST byte offsets through
the assembled source's line directives. A declaration can retain its identity
only when its physical line matches the frozen user-source line and any
explicit Clang presumed location agrees. The debugger declaration location
must also match. Unmatched lines remain observable with conservative
identities. Preset function definitions are excluded from automatic stops;
user callbacks are still observable. Library arguments such as `-lm` are
passed after the source for linking and omitted from syntax-only inspection.

## GCC/GDB on Linux and Windows

Use `engine.GDBEngine{}` through the same `observer.Engine` interface.
Defaults are `gcc` and `gdb`; compiler and debugger overrides remain available.
MI drives the inferior and an embedded, read-only GDB Python script reads DWARF
scopes and bounded values. This requires a Python-enabled GDB and does not
invoke Python as a separate executable. The default `engine.Engine{}` still
uses Clang/LLDB.

```sh
C_OBSERVER_REAL_GDB=1 go test -count=1 -v ./internal/engine
C_OBSERVER_REAL_DEBUGGER=1 go test -count=1 -v ./internal/engine
```

The same integration tests run against both backends. They cover normal and
nonzero exits, separate I/O, logical source mapping through #line, non-prefix
array pages, user callbacks, recursion, scope boundaries, globals and statics,
limits, abort, and cancellation with inferior PID verification. GDB identity
is conservative for recursive frames and nested locals. Files not exposed by
the selected debug symbols can be absent from snapshots. The GDB backend has
been run on Linux with GCC 14.2 and GDB 16.3. See below for native Windows verification.

### Windows GDB verification

Verified on Windows 11 x64 build 26200 with Go 1.26.5 and Windows ANSI code
page 936. The primary toolchain was native MSYS2 UCRT64 in the portable
installation's `ucrt64/bin` (GCC 16.2.0 Rev7, GDB 18.1, embedded Python
3.14.10). The real suite also passed with the Brecht Sanders MinGW-w64 UCRT
toolchain (GCC 16.2.0, GDB 17.2, embedded Python 3.9.7).

```powershell
go test ./...
$env:C_OBSERVER_REAL_GDB = '1'
go test -count=1 -v ./internal/engine
go vet ./...
```

The real suite covers Chinese characters and spaces in the source filename
and working directory. It also passed with `TEMP` and `TMP` set to a directory
named `观察 临时`. GCC uses an isolated ASCII build directory under the user's
normal LocalAppData Temp in that case. The GDB sample redirection files still
use the configured temporary directory. Paths passed to native GDB must be
representable in the active Windows ANSI code page on this toolchain.

The stdout and stderr prefixes retain the actual Windows CRLF bytes. The
mapped-source fixture exits with status 7 and the crash fixture is classified
as signaled. Cancellation preserved snapshots and the inferior PID had exited.
A separate Windows Job Object test verified nested job startup and forced cleanup
with real parent and descendant process handles. The compiler and GDB start suspended and join the
job before resuming. If a host job prevents assignment, startup reports the
Windows error and terminates the suspended process.

The v1 session fixture exercises the actual session loader, source mapping,
`-lm` linking, and nonzero exit. The matching Linux GDB suite and full
`go test` and `go vet` passed on Ubuntu WSL with GCC 15.2.0 and GDB 17.1.

Incompatible IDE/CI job policies and older Dev-C++ toolchains have yet to be
exercised. The release CLI exposes LLDB on macOS and Linux.

Windows GitHub hosted runner uses a different ANSI locale from the tested Chinese Windows machine. The real tests retain Chinese names when representable; on a locale that cannot encode them, use ASCII paths with spaces and independently assert the actionable rejection of an unrepresentable path. This is not full Unicode support. Chinese paths on ACP 936 remain a real acceptance requirement. No compiler downloads or IDE scanning are part of the application.
