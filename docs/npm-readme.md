# C Observer

面向 C 语言课程的离线程序运行观察器。记录 C 程序的执行快照，在终端查看源码、变量、数组、调用栈与输入输出。不是 AI 解题工具，也不是在线评测系统。

本包是 Accoding Companion 使用的原生二进制分发包。npm 只负责分发，不在观察期间运行 Node.js，也不通过安装脚本联网下载文件。

## 预览版

当前包含 macOS arm64、macOS x64 和 Linux x64 的预览二进制；需要本机已安装兼容 Clang / LLDB。Windows / GDB 尚未纳入此打包清单。二进制能够启动不等于本机调试工具链已经配置好。

Companion 根据 manifest.json 选择本机程序并使用版本化会话协议调用。独立使用者应从项目的 GitHub Releases（正式发行后）获取对应平台的原生程序，无需 npm 或 Node.js。

```sh
c-observer
c-observer main.c
c-observer --version
c-observer --capabilities
```

以上命令以已位于对应原生程序所在目录或 PATH 中为前提；npm 包不安装全局 CLI shim。

源码与协议：https://github.com/Routilet/c-observer

MIT License；第三方许可证及声明见 THIRD_PARTY_NOTICES.txt。考试中使用须符合课程规定。
