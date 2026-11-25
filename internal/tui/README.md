# TUI prototype

Status: first fixture-driven workbench on feat/tui, not a live debugger or release. No changes to shared observer types or engine.

From repository root with Go installed and module network access:

    go run -mod=mod ./cmd/c-observer-preview
    go run -mod=mod ./cmd/c-observer-preview -fixture testdata/snapshots/cancelled.json
    go run -mod=mod ./cmd/c-observer-preview -fixture testdata/snapshots/compile-failed.json

Preview is labeled SYNTHETIC PREVIEW. R resets the synthetic record; it does not execute C code. The real entrypoint can inject observer.Engine via NewLive(engine, config), which runs in a Bubble Tea command and waits for a completed RunResult; Esc requests cancellation, Q requests cancellation and exit after cleanup. No streaming. No live engine wiring yet.

Keys: Left/Right previous/next snapshot, Home/End first/terminal result, Tab/Shift+Tab focus source/variables/console, Up/Down scroll/select, Enter expand/collapse variables or switch console tab, R rerun (preview reset), Q quit. End shows final captured stdout, not a fabricated snapshot. Output tabs input/stdout/stderr/expected. Wide layout splits source and variables, narrow layout shows one focused area.

Implemented: C lexical highlighting, source mapping, safe control character escaping, nested variable tree including array indices and partial status, conservative previous-snapshot diff, result/diagnostic display, output prefixes, responsive panels. Test coverage includes synthetic records and navigation. UI strings initially English to simplify first review; Chinese localization, more readable array and record tables, range-selection dialog, standalone source/input picker, history search and integrated real engine remain pending. Confirm TUI dependency versions and entrypoint before merging shared changes.

Validation: local environment has no network for Go dependencies or clone; remote workflow runs go mod tidy, go test, go vet and build. Do not claim tests passed until actual workflow results are available.
