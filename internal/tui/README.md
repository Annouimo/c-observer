# TUI prototype

Status: fixture-driven frontend under review on feat/tui; no live backend wired into the executable yet. Shared observer types and engine implementation are untouched.

## Preview

From repository root with Go installed:

    go run -mod=mod ./cmd/c-observer-preview
    go run -mod=mod ./cmd/c-observer-preview -setup
    go run -mod=mod ./cmd/c-observer-preview -fixture testdata/snapshots/cancelled.json
    go run -mod=mod ./cmd/c-observer-preview -fixture testdata/snapshots/compile-failed.json

All previews are explicitly labeled 合成数据预览. R resets the fixture; it does not execute C. The setup preview edits a launch configuration but does not start a process. Array-range confirmation in preview cannot collect new data.

## 工作台

宽屏显示源码 / 变量与输入 / 输出；窄屏通过 Tab 切换区域。变量默认折叠聚合对象，标出已记录子节点中的变化数量。D 打开类型化数据表：标量/成员列表、一维数组、二维数组、结构体数组。表格用真实下标，缺失项明确为未记录，横向列可分页。H 查看选定变量的可靠变化或当前源码行的已记录停靠点，Enter 跳回对应快照。A 修改数组采集范围，真实引擎下重新运行，不回填旧历史。源文件与运行记录保持一致。输出来自有界字节前缀，不保证逐行输出归因；结尾页显示最终输出及退出状态，输出比较为逐字节比较，不等同 OJ 判定。

- 左/右、Home/End：历史前后、首尾；End 是结束页，不是调试器快照。
- Tab/Shift+Tab：区域焦点；上/下、PgUp/PgDn：选择或滚动。
- Enter：展开/折叠变量，或切换输入输出视图。
- D：变量数据表；表内 [ 和 ] 翻列，左右键仍可切换快照。
- H：源代码焦点时当前行历史，变量焦点时对象历史；历史页 Enter 跳转。
- A：数组采集范围，Tab 切换字段，Ctrl+U 清空字段。
- R：重跑（合成预览重置）；E：独立模式修改启动配置；Esc：关闭详情或退出；Q：退出。
- 配置页：Tab 切换、Enter 换行或确认、Ctrl+R 开始、Esc 退出。

## Real engine integration

NewLive(engine, config) skips setup for Companion/CLI callers. NewSetup(engine, config) accepts source path and multiline stdin plus optional expected stdout. Engine.Run executes in a Bubble Tea command and returns a completed RunResult. Esc requests cancellation, Q requests cancellation and exit after cleanup. No streaming or true reverse execution.

Pending: real engine integration and native terminal visual review, stable handling of non-top stack frames (only active frame is rendered now), keyboard/UI refinement, production CLI wiring and packaging. Do not merge shared dependency or entrypoint changes without coordination.

Tests cover the fixture data, history, table cells, non-prefix pages, range rerun, startup form, cancellation/empty-history states, terminal sizing and navigation. See CI for the actual validation status of each revision.
