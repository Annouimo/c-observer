# C Observer 0.2.0

首个正式 GUI 版本。面向 C 语言课程的离线运行观察工具，以数据及时间线替代 printf 调试，而不是专业调试器。

- 直接拖动时间线、点击数据关注变化并按需展开详情。
- 类型化展示标量、数组、矩阵、字符串、结构体及有限指向内容。
- 支持独立运行、样例对照、取消、重跑、保存及离线回放。
- 保留 manifest v1 和 session v1。旧 Companion 0.2.8 的 ^0.1.0 依赖继续使用 0.1.x；需要后续 Companion 更新及真实 GUI 联调才能默认使用 0.2.0，不要求更换现有 Launcher 0.3.0。

本版仅支持 Windows 11 x64（课程 GCC 16.2.0、Python-enabled GDB、WebView2）与 Apple Silicon macOS（Clang/LLDB，macOS 12+，ad-hoc 签名、未公证）。Intel Mac 和 Linux GUI 不在支持范围，旧 0.1.x 仍可安装。不附带工具链，不依赖 Node.js 运行。采集范围有上限；复杂数据采集性能及 Windows 完整 GUI 工作流仍需实机反馈。自动化测试不是实机验收。使用须遵循课程考试规则。
