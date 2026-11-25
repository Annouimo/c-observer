# Agent collaboration

## Scope and ownership
- Product owner: Annouimo. Product decisions are reviewed by the owner.
- Codex / `feat/engine`: `internal/engine/**`, real debugger integration, toolchain discovery, capture, process lifecycle, platform tests and real C fixtures in `testdata/programs/**`.
- ChatGPT / `feat/tui`: `internal/tui/**`, replay, visualization, navigation, fixture-driven UI tests, `testdata/snapshots/**`, UI design.
- Shared: `internal/observer/**`, `cmd/c-observer/**`, `go.mod`, docs/interface.md, fixture schema. Changes to shared contracts require both sides to review; coordinate dependencies and entrypoint changes before merging.
- Do not change the other side's owned files without coordination. No remote execution service or AI features. Never modify Companion as part of this repo without separate approval.

## Workflow
The initial contract on main is a draft, not a frozen API. Codex must review it against real GDB/LLDB findings first. Record agreed revisions in docs/interface.md and update models, fixtures and tests together. Then branch feat/engine and feat/tui from that same reviewed revision. Use separate PRs to main; keep PRs small. No force-push or deletion of unrelated work.
Before a PR: gofmt, go test ./..., go vet ./...; state what platforms were actually run, and what is only simulated or cross-built. UI fixtures are synthetic and must never be represented as real captures.
Read docs/product.md, docs/interface.md, docs/design.md and docs/roadmap.md before implementation. Prefer simple Go interfaces, avoid speculative abstractions and unnecessary dependencies.
