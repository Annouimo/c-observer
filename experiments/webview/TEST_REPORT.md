# WebView 技术验证报告

- 验收日期：2026-11-27
- 状态：**通过，技术预研完成**
- 实验提交：[6604e8f](https://github.com/Routilet/c-observer/commit/6604e8f0f884ad81fe02d4d8722177b9ee742cfa)
- 实验分支：`experiment/webview`
- 构建记录：[GitHub Actions #45913493888](https://github.com/Routilet/c-observer/actions/runs/45913493888)

## 1. 验证目标与范围

验证 Web 技术开发的界面能否通过系统 WebView 在独立原生窗口中正常呈现和交互，并可离线运行。此次实验不是 Observer 新版开发，不接入观察引擎，不验证业务数据模型、调试器通信或 Companion 集成。

仅验证 Windows 11 amd64 和 macOS Apple Silicon（arm64），其他平台不在验收范围内。

技术组合：Go 1.26、Wails v3.1.0、Svelte 5、TypeScript、Vite。前端资源通过 go:embed 嵌入应用，使用系统 WebView（Windows WebView2 / macOS WKWebView），无需另行分发完整浏览器引擎。

## 2. 自动构建

以下结果来自 GitHub Actions 已完成的运行记录：

| 构建目标 | Runner | 产物 | 结果 |
| --- | --- | --- | --- |
| Windows amd64 | windows-2025 | EXE | 通过 |
| macOS arm64 | macos-15 | APP（ad-hoc 签名，未公证） | 通过 |

整体工作流结果：success。

| 下载产物（Actions artifact） | GitHub API 报告的字节数 |
| --- | ---: |
| webview-windows-amd64 | 4,064,763 |
| webview-darwin-arm64 | 3,390,607 |

以上为 Actions artifact 压缩归档体积，约 4.06 MB 和 3.39 MB（十进制），并非解压后的应用大小，也不包含系统 WebView 运行时。构建生成的依赖锁文件另存于对应 artifact。

## 3. 真机验收

以下结果由项目维护者在两台目标平台设备上实际运行后反馈；全部 12 项平台检查均通过。

| 验收项目 | Windows 11 amd64 | macOS arm64 |
| --- | --- | --- |
| 双击后正常打开独立窗口 | 通过 | 通过 |
| 基础 Web 组件正常渲染 | 通过 | 通过 |
| 按钮、输入框等交互正常 | 通过 | 通过 |
| 中文输入及显示正常 | 通过 | 通过 |
| 窗口缩放、最大化、关闭正常 | 通过 | 通过 |
| 断网后仍可正常启动 | 通过 | 通过 |

维护者反馈：完全没有问题，上述所有测试项目均通过。

具体设备型号、系统小版本、启动时间、内存占用、解压后体积未提供，故不作量化结论。未单独验证移除 Node.js 或 Go 后的启动情况；构建方式不要求用户运行前端开发工具链。本报告不将 CI 构建成功等同于真机运行成功，两类证据独立记录。

## 4. 结论

本次预研目标已达到。Wails + 系统 WebView + Web 前端在两个目标平台上均完成构建及基本窗口、渲染、交互、离线启动验收，未发现阻碍继续采用该技术路线的问题。

建议保留 Go 后端，以 Wails + Web 前端作为下一阶段的候选实现基础。无需继续扩大本次技术实验，也不需要为了验证窗口可行性重写后端。

本结论仅覆盖本次实验范围，不代表正式 Observer GUI、后端接入或分发流程已完成验证。实验分支不合入 main、不发布正式版本；正式新版开发另行安排。
