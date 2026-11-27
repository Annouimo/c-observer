# Agent collaboration

Product owner: Annouimo. Product decisions require owner review. Read docs/product.md, docs/design.md, docs/frontend-next.md and docs/roadmap.md before changing code. Next generation: printf-first Web GUI, not a simplified professional debugger.

## Scope

- Frontend: Svelte/TypeScript, single-view data-first replay, stable data layout, draggable timeline, change indexing, fixture tests.
- Engine: internal/engine/**; preserve existing engine until GUI acceptance, then simplify from measured needs.
- Shared: internal/observer/**, session protocol, entrypoint, record format; coordinate contract changes. Existing docs/interface.md is the legacy/current engine contract until explicitly revised.
- Windows course GCC 16.2.0 installer and PATH configuration are the accepted environment baseline. Do not research legacy Dev-C++ or alternative toolchains, add IDE discovery, expand target platforms or build compatibility infrastructure without a concrete baseline failure and owner approval.
- No remote execution service, AI features, code generation or changes to Companion without separate approval.

## Workflow

Work on the independent refactor branch, no force-push or deletion of unrelated work. Keep main and VERSION unchanged until owner-approved release. Phase 1 accepted; Phase 2 integrates existing engine through an application controller, shares record/backend code and unifies Go module. Keep engine behavior, stable protocol, old TUI and publishing unchanged until real workflow acceptance. Prefer small, reviewable commits. Experiment results are retained; no additional framework pre-research.

Prioritize typical student programs. Use simple name matching and real snapshots; do not reconstruct control flow. Remove superseded UI and dedicated logic instead of adding auxiliary modes. Preserve source, values, observation order, missing/truncated state; avoid claims of exact assignment/iteration when not observed.

Run gofmt, go test ./..., go vet ./... when Go code is changed; run frontend checks and target builds when frontend changes. Report actual test environment, CI versus native visual acceptance and any unverified work. Synthetic fixtures must be clearly labeled. Do not represent prior experiment as business GUI validation. No release without approval.

GUI 主体已获维护者认可，优先交付 0.2.0-beta.1；PREVIEW_VERSION 管理 beta，VERSION 与 latest 保持 0.1.2。双平台真实测试、固定锁文件、原生安装检查仍为自动发行门槛。维护者明确授权 beta 先公开、人工业务验收后置；main 的 PREVIEW_VERSION 变更只触发 next。引擎精简和旧 TUI 删除后置。不得改 Companion 或直接推动 stable。

维护者已批准 Observer 0.2.0 正式发行（只改本仓库）；稳定 VERSION 变更触发 latest，自动测试仍为门槛。Companion 兼容更新另行安排，不擅自改其他仓库。npm 和 GitHub Release 分离，遇权限拒绝保留构建产物，不扩大令牌权限。
