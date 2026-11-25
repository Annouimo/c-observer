# C Observer

面向 C 语言课程学生的轻量、离线程序运行观察器，以执行历史和变量变化为核心。

**状态：前端体验版与已发布 LLDB 引擎联调；尚未正式发行。** 正式 `cmd/c-observer` 入口等待双方协调合并。当前分支提供预览命令。

```sh
go run -mod=mod ./cmd/c-observer-preview -fixture testdata/snapshots/adaptive-array.json
go run -mod=mod ./cmd/c-observer-preview -live
```

第一条使用合成数据；第二条在已有 Clang/LLDB DAP 环境中显式运行真实 C 程序。Windows/GDB 尚未正式支持。

- [产品定位与 MVP](docs/product.md)
- [前后端接口](docs/interface.md)
- [TUI 设计规范](docs/design.md)
- [本轮前端优化计划](docs/frontend-next.md)
- [Codex 引擎开发交接](docs/engine-next.md)
- [前端预览与按键说明](internal/tui/README.md)
- [协作约定](AGENTS.md)

Go 单模块，Bubble Tea TUI，LLDB/DAP 观察引擎。保持独立、离线、只读，不生成解题代码；Companion 集成另行验收。考试使用以课程规定为准。
