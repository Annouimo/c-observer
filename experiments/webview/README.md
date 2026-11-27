# WebView 技术实验

仅验证 Windows 11 amd64 和 macOS Apple Silicon (arm64) 上的 Wails v3.1.0 + Svelte + TypeScript 窗口与基础 Web 渲染。不调用 Observer 引擎、无外部资源、无业务接口，不改正式版本或分发流程。

## 构建

使用 Go 1.26、Node 22 和 npm。macOS 使用原生 arm64 runner 与 Xcode 工具链。Windows 使用原生 x64 runner。

在本目录执行：

```sh
cd frontend
npm install --no-audit --no-fund
npm run build
cd ..
go mod tidy
go build -tags production -trimpath -ldflags "-s -w" .
```

CI 额外使用 Windows GUI 子系统参数，以及 macOS .app 包装与 ad-hoc 签名。首次实验的传递依赖锁文件由构建生成并附在 artifact 中，暂未提交；后续若采用该架构再固化锁文件。

## 实机验收

2026-11-27：双平台真机验收通过。详见 [技术验证报告](TEST_REPORT.md)。以下清单保留供复测使用。

下载 Actions 的对应平台 artifact，解压后运行 EXE 或 APP（macOS 未做 Apple 公证，首次可能需按系统提示授权运行可信实验产物）。

- [ ] 独立窗口正常打开；不启动外部浏览器。
- [ ] 按钮、输入框（含中文 IME）、复选框、滑块可用。
- [ ] 表格滚动、源码等宽文本、深浅色切换正常。
- [ ] 窗口缩放、最大化和关闭正常。
- [ ] 断网后重新启动仍可用；不需要 Node 或 Go 运行环境。

记录平台、系统版本、压缩与解压体积、粗略启动时间与内存。CI 构建成功不代表 GUI 真机验收成功。

## 技术决策

[软件签名与平台兼容性决策](DECISIONS.md)：仅支持 Windows 11 amd64 与 macOS Apple Silicon，复用系统 WebView，不建设正式可信签名及额外兼容机制。
