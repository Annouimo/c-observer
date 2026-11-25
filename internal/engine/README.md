# Observation engine

`engine.Engine` implements the shared `observer.Engine` contract using Clang and LLDB's DAP adapter. One call compiles local C, runs with preset stdin and returns a bounded replay history and separate stdout/stderr. The engine searches the selected Xcode toolchain via `xcrun` on macOS, then falls back to PATH. The launch configuration can override either executable.

The first implementation takes source-level steps over calls and supports explicit source stops inside user callbacks. It captures frame variables, C arrays and structure fields. An array range can select a page outside the default prefix. Frames and objects currently have snapshot-local identities. Cross-stop identity, globals and static storage classification, GDB/MI, and Windows runtime support are separate work.

## Verification

```sh
go test ./...
go vet ./...
C_OBSERVER_REAL_DEBUGGER=1 go test -v ./internal/engine
```

The opt-in tests launch genuine C programs under a locally installed Clang and `lldb-dap`. They cover non-ASCII paths, separate I/O, a selected element of a large struct array, an explicit stop inside a qsort callback, limits, crash handling, and cancelling a running inferior. The test checks that the inferior PID is gone after cancellation. Linux needs permission to trace the test process.

Linux runtime was exercised with Clang and LLVM 19.1.7. The package cross-builds for macOS arm64. Native macOS adapter discovery, launch and cleanup still need verification. Cleanup has a bounded adapter process-group fallback. Descendants outside that group and an unresponsive adapter are not covered by the real-process tests. Output limits bound the retained prefixes, and the temporary redirection files live until the run ends. The current history budget counts the encoded snapshots, not the Go heap.
