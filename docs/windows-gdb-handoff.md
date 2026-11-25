# Windows GDB 后端交接

从 GitHub 的 `feat/engine-gdb` 开一个 Windows 功能分支。该分支基于 `90598c1`，包含尚未合入 main 的 LLDB 变量身份和静态采集，因此应以该分支作为测试基线。先读 `AGENTS.md`、`docs/interface.md` 和 `docs/engine-next.md`。保留 `observer.Engine.Run` 和 `RunResult`，改动主要放在 `internal/engine` 与真实 C 夹具。正式 CLI、TUI、共享接口和依赖遵循项目协作约定。

## 现状

- Linux 实测 GCC 14.2 + GDB 16.3。`engine.GDBEngine{}` 与默认的 LLDB 引擎跑同一组夹具，覆盖指定数组页、嵌套结构体、可靠的简单局部变量变化、回调、递归、全局和静态变量、`#line`、独立 stdout/stderr、非零退出、崩溃、采集上限和取消。
- `internal/engine/gdb.go` 管 MI 会话，`gdb_capture.py` 是嵌入二进制的只读采集代码，由 **GDB 内置 Python** 执行。无需另找系统 Python。GDB 的可靠身份对递归帧和可重复进入的嵌套局部变量仍会降级。
- Windows 目前被 `GDBEngine.Run` 的平台检查明确挡住。MI 运行使用 POSIX shell 重定向，清理使用 Linux 进程组。Windows 的现有备用实现只能杀一个进程，不能据此标记支持。正式入口仍指向 LLDB 当前路径。

## Windows 实机顺序

1. 固定 MSYS2 UCRT64 的 **原生 Windows** `gcc.exe` 和 `gdb.exe`。在 PowerShell 里记录 `Get-Command gcc,gdb`、`gcc --version`、`gdb --version` 和 `gdb -nx -batch -ex "python import sys; print(sys.version)"`。有多套工具链时，用同一 UCRT64 目录的两个绝对路径作为引擎覆盖值，确认不是调到另一套 GDB。
2. 先独立验证 MI 启动和分流。用小程序读取 stdin，分别写 stdout 与 stderr，从纯 ASCII 路径试起，再测源码、工作目录和临时目录含空格与中文。验证 `-exec-arguments` 重定向在该 GDB 上的实际行为。现有 POSIX 单引号和 Linux 专用初始化设置不能直接照搬。若该版本不能稳定重定向，再用最小的 Windows 输入输出桥接方案验证。不要拼接用户输入作命令，也不要把两条输出流混在一起。
3. 正常结束先验证 GDB 和被调试程序都退出。死循环走 MI 中断和正常退出，再验证强制清理路径。评估用 Windows Job Object 管理本次启动的编译器、GDB 和被调试程序。若采用 Job Object，要验证进程在生成子进程之前已纳入 job，以及 IDE 或 CI 已有 job 时的失败表现。按真实 PID 或进程句柄检查清理结果。保留取消前的快照，把清理失败写回现有字段。
4. 把现有真实测试的可移植部分带到 Windows。`engine_test.go` 目前仅支持 darwin/linux，其中取消测试用 Unix PID 探测，C 夹具用 `getpid`。将平台相关检查分开，确保 Windows 真的执行测试体。用相同 C 夹具对照 Linux，尤其检查数组指定页、进入 `qsort` 的用户比较函数并返回 `main`、全局和静态对象，以及 `#line` 的原始位置。Windows C 运行库可能产生 CRLF 或不同的异常原因，验收时明确展示实际字节和结束原因，保留引擎的原始输出语义。

## 验收

在 UCRT64 可用的 PowerShell 中运行下列命令。第二条的 Windows 版测试需在移植后实际执行，失败或跳过都要保留在记录里。

```powershell
go test ./...
$env:C_OBSERVER_REAL_GDB = '1'
go test -count=1 -v ./internal/engine
go vet ./...
```

首轮完成以这些结果为准：真实程序能编译、单步采集、返回独立输出，非零退出可辨识，取消死循环保留已有历史且进程消失，带空格和中文的路径可用。记录 Windows 版本、GCC/GDB/Python 版本、实际运行的测试和剩余限制。可选交付是一条 `-verify` 风格的自检命令，明确报出失败阶段。新版 Windows 真机跑通后再评估旧版 Dev-C++。
