# 变量身份与输出缓冲研究

基于 main 的 `fbc651f`（0.1.1），对应 [交接 PR #12](https://github.com/Routilet/c-observer/pull/12)。本轮实机为 Linux x86-64、Clang/LLDB-DAP 19.1.7、GCC 14.2.0、GDB 16.3 和 glibc 2.41。维护者另用验证包在 macOS arm64 原生 Xcode LLDB-DAP 上验证了代表性生日夹具。本文记录不包含维护者的原始源码或个人路径。

## P0：变量身份

- 现有 LLDB 代码要求 `$__lldb_extensions.declaration` 的路径和行号与 Clang AST 一致。在真实 Linux 生日夹具的 1,680 条变量响应项中，80 项带有该扩展，标准 `declarationLocationReference`、`valueLocationReference` 和 `namedVariables` 均未出现。对无引用的适配器没有额外发 `locations`。这只是 Linux 适配器的观测。
- 在真实 DAP 响应解码前移除扩展，旧引擎就能重现「可读值有变化，身份不可靠」。有完整栈和可验证源码映射时，新引擎只对入口 main 最外层的单一普通局部声明使用 AST 回退。现有调试器声明优先。遇到名称冲突、早于或位于声明行、来源不匹配、短生命周期或不可靠的栈时保留原降级。回退不增加调试器往返。
- 完整的 C Exam Kit 展开头生成了约 30 MB AST，超过旧版 8 MiB 上限。原先截断后丢弃整个索引。当前逐个解码顶层声明，保留编译进程和总 AST 字节上限，解析不完整时继续放弃索引。
- Linux LLDB 原生响应、移除扩展后的完整 preset 和 Linux GDB 均在同一夹具测得 `birthdays[2].date.day` 从 1 变 3。代表性 LLDB 记录中，ID 12→13，较早暂停在第 19 行：旧引擎缺扩展时两端不可比较。新引擎两端 ID 均为 `entry-main:11:birthdays/[2]/.date/.day`。映射后的可靠根对象包括 `n`、`birthdays` 和 `q`。
- 三个循环的 `i` 以及循环块内的 `index`、`x` 仍保守降级。通过新夹具和现有实机测试复核了同名遮蔽、循环重新进入、递归、显式或自动回调停靠、静态/全局和数组局部页。普通调用及递归的可靠动态生命周期尚未实现。AST 确定的是声明身份，不能确定初始化是否发生；调试器能读到的值不构成赋值证据。
- 本轮未修改字符串与结构体完整性规则。没有 `namedVariables` 的结构体即使看起来返回了全部字段，仍缺少完整性证据。C 字符串摘要有独立的截断判断，字符数组的子项覆盖率也不能证明逻辑字符串完整；仍需独立核对适配器的摘要和截断边界。
- 待 Apple 原始记录到位后，先核对冻结的实际装配源码、可见源码和 SourceMap，再用本地 Xcode `lldb-dap` 核查变量响应具体提供哪些字段，以及是否能恢复维护者记录的 ID 16→17 实际变化。公开记录不包含其原始 JSON，也不对旧记录回填身份。

### macOS arm64 原生验收

维护者运行验证包（后端提交 `499f492`），使用 Xcode 自带的 `lldb-dap`。程序正常退出，共 25 条快照。代表性生日夹具的第 14、15 条快照跨过 `birthday.c:19`，`birthdays[2].date.day` 从 1 变到 3。两端均报告 `reliable=true`，ID 同为 `entry-main:11:birthdays/[2]/.date/.day`，验证脚本输出 PASS。

该结果确认修复后的引擎在这一 Xcode 环境可以可靠比较该变化。此验证包使用内置夹具，尚未核对最初那份 29 快照的课程记录，也未保存 Apple 适配器的原始 DAP 响应。

## P1：输出缓冲

用同一份 C 夹具分别运行默认、显式行缓冲、显式无缓冲三种构建。各方案仍把 stdout 重定向到普通文件。显式方案在程序中调用一次 `setvbuf` 作为实验对照。这个实验没有启用任何正式开关。

| 情况 | 源码停靠处看到的 stdout 长度变化 | 正常最终输出 | abort 前新增的无换行 `pending` |
| --- | --- | --- | --- |
| 默认文件缓冲 | 0 → 9（显式 fflush） | `AB\r\n中\nC` | 丢在缓冲区 |
| 行缓冲 | 0 → 8（换行）→ 9（fflush） | 相同 | 丢在缓冲区 |
| 无缓冲 | 0 → 1 → 2 → 8 → 9 | 相同 | 被捕获 |

两套引擎测得相同字节序列。正常案例打印的返回值为 `1,66,6,1,0`，errno 为 0、isatty 为 0。正常退出案例中，显式缓冲构建比基线在 LLDB 多两个停靠、在 GDB 多一个停靠，单轮样本不足以推断运行开销。128 字节截断案例设置了 stdout 截断标记，偏移没有越过保留的字节数。abort 的输出差异说明更改缓冲会改变程序可见行为。当前未做外部注入、用户重复设置缓冲、PTY、seek 验证、大样例性能评估或 Mac/Windows 实测。建议维持当前偏移语义，不提前显示仍在 libc 中的字节。

## 复现

```sh
python3 research/identity-output/run.py lldb /tmp/identity-native.jsonl
python3 research/identity-output/run.py lldb /tmp/identity-fallback.jsonl --strip-declarations
python3 research/identity-output/run.py gdb /tmp/identity-gdb.jsonl
```

脚本用 Go overlay 加上仅本次运行生效的字段计数。移除扩展模拟的是字段缺失情形，进程、停靠和值来自真实 Linux LLDB。传入 `--preset-header /path/to/expanded.h` 可复测较大的装配输入；本轮使用 C Exam Kit `38bc5c643799b0b5dc14ce805acc9a5937845aac` 的 manifest 组件生成头。脱敏后的实际捕获和输出偏移数据见 [records](../research/identity-output/records)。
