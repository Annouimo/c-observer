# Agent collaboration

Product owner: Annouimo. Product decisions require owner review. Read docs/product.md, docs/design.md, docs/frontend-next.md and docs/roadmap.md before changing code. Next generation: printf-first Web GUI, not a simplified professional debugger.

## Scope

- Frontend: Svelte/TypeScript, replay, source tokens, execution document, display-only grouping, fixture tests.
- Engine: internal/engine/**; preserve existing engine until GUI acceptance, then simplify from measured needs.
- Shared: internal/observer/**, session protocol, entrypoint, record format; coordinate contract changes. Existing docs/interface.md is the legacy/current engine contract until explicitly revised.
- Windows course GCC 16.2.0 installer and PATH configuration are the accepted environment baseline. Do not research legacy Dev-C++ or alternative toolchains, add IDE discovery, expand target platforms or build compatibility infrastructure without a concrete baseline failure and owner approval.
- No remote execution service, AI features, code generation or changes to Companion without separate approval.

## Workflow

Work on the independent refactor branch, no force-push or deletion of unrelated work. Keep main and VERSION unchanged until owner-approved release. Phase 1 consumes existing records; no engine, stable protocol or publishing changes. Prefer small, reviewable commits. Experiment results are retained; no additional framework pre-research.

Prioritize typical student programs. Use simple name matching and lexical structure hints; lossless fallback to original observations. Preserve source, values, observation order, missing/truncated state; avoid claims of exact assignment/iteration when not observed.

Run gofmt, go test ./..., go vet ./... when Go code is changed; run frontend checks and target builds when frontend changes. Report actual test environment, CI versus native visual acceptance and any unverified work. Synthetic fixtures must be clearly labeled. Do not represent prior experiment as business GUI validation. No release without approval.
