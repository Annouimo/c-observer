# C Observer

面向 C 语言课程学生的轻量离线程序运行观察器：替代 printf 调试，而不是替代 GDB。

下一代 GUI 以数据为主体、时间线为全局坐标、源码为上下文。选择 C 文件和样例，运行后拖动时间线；点击数据查看变化及详情。支持取消、重跑、保存和离线回放。

## 安装

GUI 正式发行目标 0.2.0。以 [Releases](https://github.com/Routilet/c-observer/releases) 实际公开资产为准；未发布时请勿将开发预览当作正式发行。支持 Windows 11 x64 和 Apple Silicon Mac。下载对应 ZIP，解压打开应用。无需 Node.js；Windows 使用课程 GCC 16.2.0 / Python-enabled GDB 及系统 WebView2，macOS 使用 Clang/LLDB（APP ad-hoc 签名、未公证）。不扫描 IDE、不安装工具链。0.2.0 发布成功后为 npm latest；旧 Companion 0.2.8 的 ^0.1.0 依赖仍解析到 0.1.x，不自动迁移 GUI。

## 开发

Go / Wails v3 / Svelte / TypeScript。正式 GUI 入口 gui/，旧 cmd/c-observer TUI 暂保留、不进入新版分发。参见 [开发预览](gui/README.md)、[设计规范](docs/design.md)、[接口](docs/interface.md)、[Companion 协议](docs/integration.md)、[发行流程](docs/release.md)、[路线图](docs/roadmap.md)。

Routilet · Annouimo · MIT License。考试使用遵循课程规定。
