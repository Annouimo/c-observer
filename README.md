# C Observer

面向 C 语言课程学生的轻量、离线程序运行观察器，以执行历史和变量变化为核心。

**状态：项目初始化与接口评审，尚不可用于真实调试。**

- [产品定位与 MVP](docs/product.md)
- [前后端接口草案](docs/interface.md)
- [TUI 设计规范](docs/design.md)
- [开发路线及验收](docs/roadmap.md)
- [协作约定](AGENTS.md)

Go 单模块项目；计划使用 Bubble Tea 构建 TUI，GDB/MI 与 LLDB/DAP 由观察引擎封装。独立运行，也可由 Accoding Companion 传入源码及样例。完全离线，不生成解题代码。

接口草案尚待 Codex 审核。初始化完成后，使用 `go test ./...` 验证共享模型与演示数据；`go run ./cmd/c-observer` 仅显示项目状态，不启动真实调试。
