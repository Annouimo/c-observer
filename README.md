# C Observer

面向 C 语言课程学生的轻量、离线程序运行观察器。核心定位：替代 printf 调试，而非替代 GDB。

所属组织：[Routilet](https://github.com/Routilet) · 原始设计者：Annouimo。

**本分支：下一代 Web GUI 开发中（拟 0.2.0，尚未发行）。** main 保持原版 0.1.2 TUI；本分支逐步实现数据优先的单界面时间回放 GUI、复用既有引擎，再精简数据接口。技术实验已经通过 Windows 11 x64、macOS Apple Silicon 基础窗口与离线验收。

- [产品定义](docs/product.md)
- [前端设计](docs/design.md)
- [实施范围](docs/frontend-next.md)
- [路线图](docs/roadmap.md)
- [技术预研报告](experiments/webview/TEST_REPORT.md)
- [平台与分发决策](experiments/webview/DECISIONS.md)
- [现有观察引擎接口](docs/interface.md)
- [Companion 调用协议](docs/integration.md)
- [现有观察记录格式](docs/record-format.md)
- [协作约定](AGENTS.md)

新 GUI 使用 Go + Wails v3 + Svelte/TypeScript。Windows 以课程 GCC 16.2.0 一键工具链为基线；macOS 沿用 LLDB。离线、只读、不生成解题代码，考试使用遵循课程规定。旧 TUI 在迁移完成前保持可用，不再新增功能。

MIT License.
