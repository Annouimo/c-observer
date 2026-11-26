# 标准输入读取位置预研

基线 `ef102c1`（main），任务来自 [PR #10](https://github.com/Routilet/c-observer/pull/10)。改动限于测试夹具、实验脚本和报告。正式引擎、共享接口和默认运行行为保持原样。

## 结论

建议继续使用静态输入预览，暂不把输入进度加入正式快照。Linux 上 `ftell(stdin)` 能给出普通文件的逻辑读取位置，包括缓冲预读时和同字符退回后的位置，但这一数字不能独自证明仍在读取原样例。调试器执行目标函数还会影响目标状态，LLDB 逐步调用的开销会逼近默认运行预算。读取 glibc 缓冲的方案避开了目标函数调用，但依赖 libc 私有结构和 Linux /proc，不能作为跨平台的默认能力。

| 环境与路线 | 结果 |
| --- | --- |
| Linux x86-64，GCC 14.2.0 / GDB 16.3，glibc 2.41，MI + GDB Python 调用 `ftell` | 普通重定向文件有条件准确。换流无法仅靠返回值识别，非可寻址流上的调用改变了 `errno` |
| Linux x86-64，Clang 19.1.7 / LLDB-DAP 19.1.7，glibc 2.41，DAP `evaluate` | 同样可获得普通文件逻辑位置。逐快照求值成本较高，存在目标状态及预算风险 |
| Linux x86-64，GDB Python 只读 glibc 字段及 /proc | 测试夹具内能识别文件预读、EOF 和替换文件。依赖 glibc 私有布局及标志，实验结果不构成正式支持 |
| macOS arm64，Apple Clang / Xcode LLDB-DAP | 本轮无原生运行环境。Linux 的 DAP 结果不能证明 Apple Adapter、darwin libc 或表达式 `__stdinp` 兼容 |
| Windows x64，原生 MinGW GCC / GDB | 本轮无原生运行环境。UCRT 表达式及 MSVCRT/旧 Dev-C++ 尚未验证，尤其不能从文本模式的 `ftell` 推断 LF、CRLF 或混合换行的原始字节位置 |

## 复现

在带有本地编译器和调试器的源码树运行。GDB 走实际 MI/Python，LLDB 走实际 DAP。Linux 调试器需要能跟踪本次启动的子进程。Windows 需要原生、启用了 Python 的 MinGW GDB。Python 3 脚本以 Go overlay 将探针挂在现有引擎已保存的停靠点，仅此实验进程使用探针。没有替换编译、重定向、步进、快照、限额或清理逻辑。

```sh
python3 research/input-progress/run.py gdb /tmp/input-gdb.jsonl
python3 research/input-progress/run.py lldb /tmp/input-lldb.jsonl
```

Windows 用 `py -3` 和 Windows 输出路径。Linux 和 Mac 的默认编译器、调试器发现仍由引擎负责。输出是逐次运行的 JSONL，记录原始样例的十六进制、两路输出、退出状态、清理错误、停靠行序列，以及逐停靠的快照 ID、行号、探测原始结果/错误和微秒数。`off` 是同一夹具无探针的对照，`same_result` 比较退出原因、退出码及原始两路输出，`same_lines` 比较停靠行序列；取消案例的快照数允许随时间变化。夹具通过自己的 stdout 报告操作返回值、EOF、流错误和 errno，按对应 `LOG` 行关联。探测没执行的快照也仍在行序列中。记录中的位置数字是实验诊断数据。

普通 LF、CRLF、混合换行和 UTF-8 用 Python/Go 字节串产生。代表性课程案例使用同一套真实 C Exam Kit 的 `read_word`、`read_int`、`Date`，采用改编后的生日夹具和新样例。本轮没有维护者的原始课程文件、原始样例及 0.6.9 展开头文件。测试用 C Exam Kit 提交 `38bc5c643799b0b5dc14ce805acc9a5937845aac`，其 `npm/accoding-preset/preset/manifest.json` 的 SHA-256 为 `d54cb962ff0c41f403559fd6727ff01c52a9745b333e6e6c92e8c334d1350956`。以下命令使用现有 manifest 中按依赖顺序存放的组件，生成临时头文件：

```sh
tmp=$(mktemp -d)
cp testdata/programs/input_progress_birthday.c "$tmp/main.c"
python3 - "$tmp" /path/to/c-exam-kit/npm/accoding-preset/preset/manifest.json <<'PY'
import json
from pathlib import Path
import sys

directory, path = map(Path, sys.argv[1:])
manifest = json.loads(path.read_text(encoding="utf-8"))
header = [f"#include <{name}>" for name in manifest["systemHeaders"]]
header += [part["code"] for part in manifest["components"]]
(directory / "exam.h").write_text("\n".join(header) + "\n", encoding="utf-8")
(directory / "sample.in").write_bytes(b"2\nAda 2000 2 28\nLin 2004 2 29\n1\n1 1\n")
PY
python3 research/input-progress/run.py lldb /tmp/input-lldb.jsonl \
  --course-source "$tmp/main.c" --course-input "$tmp/sample.in"
```

## 实机结果

Linux x86-64，内核 6.18.57，Debian glibc 2.41-12+deb13u5，GDB 内置 Python 3.13.5。两套调试器分别用本机 GCC 和 Clang 以 `-std=c99 -g -O0` 编译。除指定的取消与故意卡住的探针，预算保持默认。原始逐停靠记录在 [research/input-progress/records](../research/input-progress/records)。其中 LF 与管道样例可直接阅读 JSONL，两套完整记录以 gzip 保存。

### 固定 LF 样例

原始十六进制为 `31322033340a6162630a580a`。两个调试器在各自所有 28 或 29 条停靠均取得数值，`ftell` 与 `ftello` 一致，探测前后的停靠、stdout、stderr 和退出状态相同。表中的 EOF 来自夹具读取的 `feof`，不是从偏移推导。

| 停靠源码行 | 刚完成的动作 | C 返回值或结果 | 预期／GDB／LLDB 字节位置 | EOF |
| ---: | --- | --- | ---: | --- |
| 20 | 初始 | 无 | 0 / 0 / 0 | 否 |
| 28 | 第一次 `scanf` | 1，读到 12 | 2 / 2 / 2 | 否 |
| 30 | 第二次 `scanf` | 1，读到 34 | 5 / 5 / 5 | 否 |
| 32 | `getchar` | 10 | 6 / 6 / 6 | 否 |
| 34 | `fgets` | 非空，读到 `abc\n` | 10 / 10 / 10 | 否 |
| 36 | `getchar` | 88 | 11 / 11 / 11 | 否 |
| 38 | `ungetc` | 88 | 10 / 10 / 10 | 否 |
| 40 | `getchar` | 88 | 11 / 11 / 11 | 否 |
| 42 | `getchar` | 10 | 12 / 12 / 12 | 否 |
| 44 | `getchar` | −1 | 12 / 12 / 12 | 是 |

读取第一个整数后，系统 fd 已经位于 **12**，C stdio 才消费到 **2**。因此系统 fd 位置不能作为进度的后备值。glibc 只读探针在表中各点得到了同样的逻辑偏移，并分辨了最后两处相同偏移、不同 EOF 的状态。

### 边界和课程用法

- **空与换行。** 空输入初始位置 0，首次读到 EOF 后仍为 0。Linux 的 CRLF 行中，`getchar` 真正返回 CR（13），消耗一个字节。混合换行与 UTF-8 样例中，读到 `中\r\n` 后偏移从 7 到 12，按五个原始字节计算。最后一个字符无换行也正常。以上不涉及 Windows 文本转换。Mac arm64 和 Windows x64 的 Go 实验测试包均交叉编译成功，只验证了 Go 构建。
- **部分读取及失败。** 匹配失败的 `scanf("%d")` 返回 0，偏移仍从 0 到 4，因为已跳过前导空白。随后验证了 `%c`、带前导空格的 `" %c"`、`%s` 和 `fread`。长行由 `fgets` 部分读取，下一次仍从剩余行读取。D 型样例最终读完 25 字节并到达 EOF。
- **回退、不同字符和换流。** 放回不同字符 Z 时偏移 11 → 10，下一次读回 Z，原始输入第 10 字节却不是 Z。这个阶段不能说光标后的字节就是程序接下来会读到的内容。glibc 探针这里可见备用缓冲。定位到字节 1、`rewind` 回到 0 都能观察到。从另一条流读字符时 stdin 位置保持不变。`freopen` 打开替代文件后，`ftell` 又返回了有效的 0、1，仅看数字会误指向原样例。Linux 只读方案用文件身份发现替换。这依赖本机 /proc，未覆盖重新打开同一个文件但独立打开状态、并发修改等情形。
- **真实 preset。** 生日夹具的两套引擎各记录 24 处暂停。偏移在读到人数、首个名字、首组日期、次个名字、次组日期后分别是 1、5、15、19、29，最后一组查询读取后是 35，原始 36 字节中的末尾换行尚未消费。输出都是 `Ada 2000-02-29\nLin 2004-02-29\n`。该版 preset 通过 libc 的 `fgetc` 和同字符 `ungetc` 保留分隔符，偏移不需要识别助手函数名称。
- **可复现的副作用。** 夹具把 stdin 切换为有一个字节的管道，主动清零 errno。普通观察报告 0，开启 `ftell` 后两套调试器都返回 −1 并把程序随后读到的 errno 改成 29（ESPIPE），stdout 因此改变，尽管退出码和源码停靠序列保持一致。将故意未初始化的测试流指针传给 `ftell` 时，GDB 产生 SIGSEGV 并终止原本会正常结束的运行，LLDB 报表达式信号并在这组夹具里恢复了观察。就算目标是只读，调用本身也不是无副作用的读取。

### 千快照和清理

Y 型输入为同一行 650 个 `1`（`31 20` 重复 650 次）。每组串行运行三轮，使用 1,000 快照、10 秒运行、2 秒单请求预算。总耗时含编译和清理，探针延迟从发送请求到返回，不把独立 MI 请求的成本当成 Python 内执行时间。

| 路线 | 无探针总耗时 | 开启后总耗时 | 快照数 | 探针成功及位置精度 | 单次中位数 / P95 |
| --- | --- | --- | --- | --- | --- |
| GDB `ftell` | 2.18 / 2.20 / 2.39 秒 | 3.22 / 3.21 / 2.98 秒 | 每轮 1,000 | 3,000 / 3,000，对比每次实际读完后的原始字节边界均相符 | 0.84 / 1.17 ms |
| GDB 只读 | 同上 | 3.30 / 3.29 / 2.89 秒 | 每轮 1,000 | 3,000 / 3,000，边界均相符 | 0.85 / 1.13 ms |
| LLDB `ftell` | 4.10 / 4.51 / 3.81 秒 | 10.09 / 10.29 / 9.57 秒 | 724 / 851 / 1,000 | 2,574 / 2,575，所有取得的数值都相符 | 5.64 / 13.69 ms |

GDB 的 `ftell` 端到端增量约 0.59–1.04 ms/快照。LLDB 唯一完成 1,000 快照的一轮增加约 5.76 ms/快照，另两轮触发运行超时，已保存的快照没有丢失。GDB 只读探针在实验里也有单独往返。并入现有 Python 采集可能省掉往返，但没有测过，也解决不了 CRT 布局维护及跨平台问题。

两套引擎都记录了正常退出、主动 abort、5 快照采集上限、取消和重新运行。取消样例按 500 ms 触发，该机器上的 LLDB 可能尚未完成启动，记录为 0 或 1 条已有快照。已有实机回归另在确认程序启动后取消，并验证 inferior PID 已退出。故意请求执行 `sleep(20)` 时，GDB 是真实的 2 秒请求超时，总耗时约 3.14 秒，保留首条快照，但报告正常清理未完成并进入现有兜底路径。复测跟踪了 inferior PID，确认程序已停止执行，但在此 Linux 容器中仍残留由 PID 1 接管的僵尸进程。LLDB 在约 730 ms 返回了表达式被断点打断、已恢复状态的错误，实验结束并保留首条快照，清理无报错。此结果不等于验证了 LLDB 的 2 秒请求超时。记录没有合成结束快照。

## 正式功能取舍

当前证据不足以建议加字段或启用任何平台的默认探测，保留原始样例静态预览。没有理由引入 stdio 包装、强制二进制模式或根据调用次数猜测。如果将来已有低成本的只读来源，考虑以下验收条件：确认仍是原样例和原始字节边界，明确区分位置到末尾与真正 EOF，允许回退，对不明流返回未知，失败时不打断普通观察。先在 Xcode LLDB-DAP 和原生 MinGW UCRT/MSVCRT 用同一脚本取得 LF、CRLF、混合换行和预算结果，再讨论共享接口。

## 交接状态

本轮预研到此结束。产品继续静态展示原始 stdin 样例，正式接口和默认运行行为维持现状。

讨论过另一条路线：用空管道接 stdin，调试器捕获程序对 fd 0 的读取请求时再送一行。跨行 scanf 可在同一条 C 语句里多次请求数据，样例送完后关闭写端提供 EOF。这一方案只经过讨论，尚无实验代码或实机结果。即使可行，回放也只能标注「程序请求到了第几行」，而且管道与文件重定向的寻址行为不同。现阶段不安排后续验证。
