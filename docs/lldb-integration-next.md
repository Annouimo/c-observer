# LLDB 首版联调：映射源码的可靠身份

首版不等待 Windows/GDB。前端分支已合并 feat/engine-identity 的 90598c1 基线。当前 engine.go 仅在 SourceMap 为空时调用 inspectSource，因而映射后的 C Exam Kit 源码不会获得主函数局部变量的跨快照身份。普通 C 的 Companion 调用现在直接使用原始文本的同名冻结副本，免去冗余 #line。

## Codex 协作任务

请在独立 LLDB 后端分支评估真实装配源码的身份支持：
- Clang AST 的 physical / presumed location 与 #line 的对应关系，声明必须来自冻结的可见用户源码。不要用名字或行号单独推断身份。
- 保留主函数外层变量、数组下标和字段可靠身份；复杂重复作用域仍保守降级。
- 默认跳过 preset/* 的辅助函数但保留用户函数和回调。检查源路径映射与链接参数（尤其 -lm）。
- 不修改 TUI、Companion 或共享协议，若接口有变化先协调。
- 使用前端 PR 中 TestCompanionSessionReplay 及 Companion tests/standalone/observer-session-fixture.ts，后者由真实 prepareObservation 产生会话；先保证正常输出、映射和不进入辅助源码。可靠身份的断言在后端完成后加强。

对于身份尚不可靠的映射数据，首版 UI 明确保留全部采集范围，不推断变量变化。是否将此能力作为稳定发行的必要门槛，由维护者在真实使用验收后决定。GDB 分支保持独立。

## 联调进展

Codex `6172e88` 已合入 feat/tui（合并提交 `371e890`）：以物理行、冻结源码及 presumed location 核验身份；保留用户回调，跳过 preset 辅助函数；分离链接参数。TestCompanionSessionReplay 现要求真实可靠身份、变化与 0..4 自动关注范围，而非仅检查正常输出。仍需 CI 和 Mac 实际体验验收，复杂作用域的保守降级保持不变。
