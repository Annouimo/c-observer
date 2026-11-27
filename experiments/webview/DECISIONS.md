# Web GUI 技术决策：软件签名与平台兼容性

- 日期：2026-11-27
- 状态：已决定
- 适用范围：C Observer 下一代 Web GUI 的开发与分发
- 验证依据：[WebView 技术验证报告](TEST_REPORT.md)

## 1. 决策原则

Observer 是面向 C 语言课程学生的小型开源工具，而不是需要覆盖各种历史系统与企业环境的商业产品。以实际用户群体和已出现的问题为依据确定支持范围，不为尚未出现的少数兼容性问题提前建设基础设施。

主要用户使用近几年的个人笔记本电脑，Windows 11 为主。课程考试采用录屏，不存在专门的考试管理软件或浏览器进程拦截机制；浏览器仅允许访问 ACCoding，独立 WebView 应用可以使用。应用自身仍应保持离线，不引入互联网依赖。

## 2. 软件签名

| 项目 | 决策 |
| --- | --- |
| Windows Authenticode 可信签名 | 不做 |
| Windows 自签名证书 | 不做 |
| macOS ad-hoc 签名 | 保留现有构建步骤 |
| macOS Developer ID 签名 | 不做 |
| Apple 公证（Notarization） | 不做 |
| 付费证书、第三方签名服务及专门签名工作流 | 不引入 |

Windows 普通桌面应用不以可信签名为运行前提。维护者实测现有 Windows 启动器无需额外授权即可使用；macOS 下载的未公证程序可能出现 Gatekeeper 安全提示。实验中的 macOS APP 已采用 ad-hoc 签名，并通过真机验收。

这里的“不解决签名问题”指不建设面向公开可信发行的签名、公证体系，而非删除 Apple Silicon 所需的基础代码签名。ad-hoc 签名不能证明受信任的开发者身份，也不能保证消除 Gatekeeper 提示。

对于确认为可信来源的程序，如遇 macOS 安全提示，可提供系统自带的单个应用授权说明；不要求用户全局关闭 Gatekeeper、SmartScreen 或杀毒软件，不自动移除系统安全属性。只有实际分发遇到明显障碍时才重新评估正式签名，不把它列为新版开发或发行的前置任务。

参考：[Pake macOS 默认签名配置](https://github.com/tw93/Pake/blob/main/src-tauri/tauri.macos.conf.json)、[Pake 可选签名与公证工作流](https://github.com/tw93/Pake/blob/main/.github/workflows/single-app.yaml)。其公开配置可证明支持 ad-hoc 回退及可选正式签名，不能证明所有发行产物实际均已签名或公证。

## 3. 平台与 WebView 兼容性

| 平台 | 支持策略 |
| --- | --- |
| Windows 11 amd64（x64） | 正式支持、构建与验收 |
| macOS Apple Silicon（arm64） | 正式支持、构建与验收 |
| Windows 10 | 不主动适配或验收 |
| Intel Mac、Windows ARM64、Linux 及更旧系统 | 暂不纳入支持范围 |

以上是新版 Web GUI 的目标支持范围，不改变其他项目或旧版 Observer 已有的支持策略。

采用系统 WebView：Windows 使用预装的 WebView2 Evergreen Runtime，macOS 使用 WKWebView。复用系统运行时，不自带 Chromium，不固定 WebView2 版本，不开发 WebView2 自动安装、下载或更新机制，不维护 TUI 兼容回退。

Windows 11 正常系统预装 WebView2，缺失运行时、长期未更新、精简系统、企业策略、显卡或权限异常等仍可能造成个别问题。系统 WebView 版本和跨平台渲染差异属于正常开发约束，并不要求扩大平台支持范围。若实际遇到运行时缺失，优先给出微软官方安装链接及必要的考前说明，不在考试期间自动下载组件。

参考：[Microsoft WebView2 Evergreen 与 Fixed Version 发行模式](https://learn.microsoft.com/en-us/microsoft-edge/webview2/concepts/evergreen-vs-fixed-version)、[WebView2 官方下载](https://developer.microsoft.com/en-us/microsoft-edge/webview2/#download-section)。

## 4. 实施边界与后续

双目标平台的最小 Wails WebView 实验已经通过 CI 构建与维护者真机验收，包括独立窗口、基础 Web 组件、交互、中文显示、窗口操作及离线启动。这足以支持当前技术选型，但不代表所有设备和未来版本均无兼容性问题。

本阶段结束，不为签名或兼容性追加预研，不因此改变现有构建流程，不提前建设额外安装器。正式开发阶段沿用 Go + Wails + Web 前端方向，针对真实反馈处理具体问题。业务引擎集成与正式分发仍属于后续开发范围。
