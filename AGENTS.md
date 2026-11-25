# Agent collaboration

## Scope and ownership
- Product owner: Annouimo. Product decisions are reviewed by the owner.
- Codex / `feat/engine`: `internal/engine/**`, real debugger integration, toolchain discovery, capture, process lifecycle, platform tests and real C fixtures in `testdata/programs/**`.
- ChatGPT / `feat/tui`: `internal/tui/**`, replay, visualization, navigation, fixture-driven UI tests, `testdata/snapshots/**`, UI design.
- Shared: `internal/observer/**`, `cmd/c-observer/**`, `go.mod`, docs/interface.md, fixture schema. Changes to shared contracts require both sides to review; coordinate dependencies and entrypoint changes before merging.
- Do not change the other side's owned files without coordination. No remote execution service or AI features. Never modify Companion as part of this repo without separate approval.

## Current phase and workflow
Initial LLDB integration has been reported successful. Codex: read [docs/engine-next.md](docs/engine-next.md) for the current engine tasks and acceptance criteria. The reviewed blocking Engine.Run contract remains the baseline. Reconcile the tested local integration with remote branches before starting short-lived feature branches; preserve unmerged work. Record agreed interface changes in docs/interface.md and update models, fixtures and tests together. Use separate small PRs to main. No force-push or deletion of unrelated work.
Before a PR: gofmt, go test ./..., go vet ./...; state what platforms were actually run, and what is only simulated or cross-built. UI fixtures are synthetic and must never be represented as real captures.
Read docs/product.md, docs/interface.md, docs/design.md and docs/roadmap.md before implementation. Prefer simple Go interfaces, avoid speculative abstractions and unnecessary dependencies.
