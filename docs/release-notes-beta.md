# C Observer 0.2.0-beta.1

首个图形界面预发行版本，面向 C 语言课程的离线 printf 调试替代工具。

- 时间线拖动、直接关注数据变化、按需详情、源码方向反馈。
- 结构体、数组、矩阵、字符串与有限指向对象的类型化展示。
- 独立选择源码和样例，运行、取消、重跑、保存及离线回放观察记录。
- 保留 session v1 与 manifest v1；Companion 新 GUI 完整联调仍待验收。

支持 Windows 11 x64（课程 GCC 16.2.0 与 Python-enabled GDB，已配置 PATH）及 Apple Silicon macOS（Clang/LLDB）。Windows 需要系统 WebView2，Mac APP 为 ad-hoc 签名、未公证；不提供 Intel Mac/Linux GUI，不安装编译器、调试器或运行时，不要求 Node.js。请使用可信来源下载并核对 SHA256SUMS。旧 0.1.2 仍为 npm latest；本版仅 next。已知限制：采集预算、复杂数据覆盖、不同调试器的值格式及 Windows 路径编码；记录不是完整内存，样例对照不是 OJ 判定。遵守课程考试规则。