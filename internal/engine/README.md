# Observation engine

`engine.Engine` implements the shared `observer.Engine` contract using Clang and LLDB's DAP adapter. One call compiles local C, runs with preset stdin and returns a bounded replay history and separate stdout/stderr. The engine searches the selected Xcode toolchain via `xcrun` on macOS, then falls back to PATH. The launch configuration can override either executable.

The engine sets entry breakpoints for function definitions in the primary C source, then uses source-level steps. This records direct calls and library-invoked user callbacks without manual stop locations. Clang source inspection is bounded and optional. When it cannot establish function locations, the engine still supports explicit source stops. It captures frame variables, C arrays and structure fields. An array range can select a page outside the default prefix. The engine matches simple outer-block variables in the original main invocation across stops using Clang declaration locations and a complete LLDB stack. It also retains identities for captured fields and array elements. Nested scopes, repeated loop locals, recursive/helper frames, ambiguous declarations, and unverified source mappings use snapshot-local identities. The engine classifies matched LLDB objects from the primary source as globals, file statics, or function statics. Their declaration identity is shared across frames, and each object appears at most once per snapshot. Global enumeration is partial because LLDB can expose different function statics at different stops. Additional backends are separate work.

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
