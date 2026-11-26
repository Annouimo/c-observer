# C Observer

面向 C 语言课程学生的轻量、离线程序运行观察器，以执行历史和变量变化为核心。

所属组织：[Routilet](https://github.com/Routilet) · 原始设计者：Annouimo。

**状态：正式版发行准备中。** `cmd/c-observer` 提供独立入口，主界面采用数据优先设计。

```sh
go run -mod=mod ./cmd/c-observer-preview -fixture testdata/snapshots/adaptive-array.json
go run -mod=mod ./cmd/c-observer-preview -live
```

第一条使用合成数据；第二条在已有 Clang/LLDB DAP 环境中显式运行真实 C 程序。正式入口支持 macOS LLDB、Linux GDB/LLDB、Windows x64 GDB。

- [产品定位与 MVP](docs/product.md)
- [前后端接口](docs/interface.md)
- [TUI 设计规范](docs/design.md)
- [本轮前端优化计划](docs/frontend-next.md)
- [Codex 引擎开发交接](docs/engine-next.md)
- [前端预览与按键说明](internal/tui/README.md)
- [Companion 调用协议](docs/integration.md)
- [发行准备](docs/release.md)
- [协作约定](AGENTS.md)
- [MIT License](LICENSE)

Go 单模块，Bubble Tea TUI，LLDB/DAP 与 GDB/MI 观察引擎。保持独立、离线、只读，不生成解题代码；Companion 装配回放已有真实 CI 验证。考试使用以课程规定为准。
