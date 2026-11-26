# Companion 调用协议 v1（开发阶段）

唯一权威规范。正式入口 c-observer；无参数或单个 .c 路径进入配置，--session FILE 直接采集，--version 和 --capabilities 不启动终端 UI。能力查询返回 version、sessionSchemaVersions（整数列表）、platform（Go GOOS）、arch（GOARCH）、backends（编译进去的后端，不表示已检测到外部工具）。当前 macOS 支持 LLDB，Linux 支持 GDB/LLDB，Windows x64 支持 GDB。正常退出观察界面返回 0；命令行、协议或 TUI 启动失败非零。被观察程序失败属于界面中的 RunResult，不当作进程启动错误。

JSON 对象：
- schemaVersion: 1（必填）
- sourcePath: 编译入口 .c（必填）
- workDir: 程序工作目录（缺省为编译入口父目录）
- visibleSourcePaths: 冻结可见源文件
- sourceMap: 调试器逻辑路径到可见源码文件；键保持原样，值必须列于 visibleSourcePaths 或为 sourcePath
- skipSourcePaths: 辅助源码路径
- compiler/debugger: PATH 中的命令名或绝对路径
- compilerArgs: 观察构建附加参数（不包含 -O2 等正式评测选项；目前引擎参数位置需与 Codex 协调后验证链接库）
- stdinFile: 输入普通文件（必填，可为空文件）
- expectedStdoutFile: 可选预期输出文件；省略表示没有预期，空文件表示期望空输出。

文件路径相对于 session.json 所在目录解析，而非 workDir；工具路径需绝对或 PATH 命令名。JSON 最大 1 MiB，样例各 4 MiB，拒绝未知字段与不支持的版本。不传账号、令牌、Cookie。不调用 shell，不联网。仅接受本地信任的会话文件，不是安全沙箱。调用方负责临时文件的生命周期，退出后清理。

Companion 从同一次装配生成观察源码与冻结的用户源码，通过 #line 和 sourceMap 对应；不能使用后来修改的磁盘源码冒充测试时的版本。观察构建 -g -O0 与正式评测独立，不能替代 OJ 判定。首版必须验证 LLDB 的辅助函数跳过与链接参数；GDB 适配另行验收。测试详情入口须明确观察的是当前源码还是先前测试版本。

发行：npm 包内直接包含原生二进制及 manifest.json，无安装脚本；GitHub Release 使用相同构建产物。旧 launcher 已使用 npm install --ignore-scripts，不新增专属安装管理。Companion 通过运行时模块解析找到 manifest，并按 Node platform/arch 选择，检查 --capabilities 协议；不存在或不支持平台时正常降级。发布前进行跨平台实际调试、Companion 联调及原生包安装检查。

### 后端选择
Session v1 新增可选 `backend`：`auto`（或省略）、`gdb`、`lldb`。仅新版本支持该字段，旧程序严格拒绝未知字段。CLI `--backend` 仅独立运行使用；Session 自带配置。Windows x64 使用 GDB，macOS LLDB，Linux 自动按指定工具或 PATH 选择，显式选择优先。不扫描 IDE、不自动安装工具链。`--capabilities` 仅报告构建支持，非本机调试器自检。Windows 优先 GCC 同目录 GDB；实际可用性由运行检查决定。实际运行仍取决于本机工具链。

映射路径：调试器可能将相对 #line 路径报告为工作目录下绝对路径；引擎按工作目录匹配精确相对键，不按 basename 猜测。真实 LLDB 非零退出 Session 回归必须通过。

`--capabilities` 新增可选 sessionFeatures 列表。支持 comparison 的版本可接收 `comparison`（exact/trim-line-end/tokens）；保留原始字节对照，另显示有效 UTF-8 的样例规则一致性；不等同于 OJ 判定。旧版本不得收到未知字段。独立观察默认逐字节比较。
