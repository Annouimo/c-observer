# C Observer GUI

C 语言课程离线运行观察器，以数据和时间线替代 printf 调试。不是 AI 解题工具或 OJ。

支持 Apple Silicon macOS（Clang/LLDB，macOS 12+）和 Windows 11 x64（课程 GCC 16.2.0 / Python-enabled GDB，PATH 已配置，系统 WebView2）。不附带编译工具链；不支持 Intel Mac 或 Linux GUI。Mac 应用 ad-hoc 签名、未公证。

独立使用：下载 GitHub Releases 对应 ZIP，解压后运行 C Observer.app 或 c-observer.exe，无需 npm/Node.js。选择 .c 源文件、输入及可选预期输出，运行观察。点击数据关注变化和详情，拖动时间线回放。可重跑、取消、保存记录；记录包括源文件、本地路径、样例和数据，分享前检查。

npm 包 @routilet/c-observer 用于原生分发，不含安装脚本和全局 shim。beta 使用 next 标签，不替换 0.1.2 latest。manifest.json 提供可执行文件相对路径、SHA256 及 session 版本；Mac 路径位于 APP 内部。Companion 完整 GUI 联调尚未完成。支持 --version、--capabilities、--session FILE、--replay FILE。

MIT；第三方许可证见 THIRD_PARTY_NOTICES.txt。考试使用须符合课程规定。
