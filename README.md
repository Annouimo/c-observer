# C Observer

面向 C 语言课程学生的轻量、离线程序运行观察器，以执行历史和变量变化为核心。

**状态：观察引擎原型开发中。已有调试引擎实现与测试，TUI 和实际课程工具链的完整验收尚未完成，暂不提供正式发行。**

所属组织：[Routilet](https://github.com/Routilet) · 原始设计者：Annouimo。

- [产品定位与 MVP](docs/product.md)
- [前后端接口草案](docs/interface.md)
- [TUI 设计规范](docs/design.md)
- [开发路线及验收](docs/roadmap.md)
- [协作约定](AGENTS.md)

Go 单模块项目；计划使用 Bubble Tea 构建 TUI，GDB/MI 与 LLDB/DAP 由观察引擎封装。目标是独立运行，并支持由 [Accoding Companion](https://github.com/Routilet/accoding-companion) 传入源码及样例；集成尚未正式完成。完全离线，不生成解题代码。

开发时使用 `go test ./...` 和 `go vet ./...` 验证共享模型、引擎及测试；`go run ./cmd/c-observer` 当前仅显示项目状态，不启动真实调试。实际平台验收以测试记录为准。
