# 变量身份可靠性与紧凑编码实测

本报告承接 [PR #14 交接](https://github.com/Routilet/c-observer/pull/14)、[PR #12 交接](https://github.com/Routilet/c-observer/pull/12) 与 `research/identity-output` 的实测结果。实验分支为 `research/loop-identity-encoding`，基于 `fix/lldb-entry-local-identity`（PR #15，`f9d5197`）；主分支保持 `fbc651f`（0.1.1）不变。没有修改 `internal/observer`、前端、记录格式、Companion 协议或 `VERSION`。

## 结论摘要

**A. 循环变量身份**

- LLDB 可以在现有证据上可靠识别 **entry `main` 帧、未被其他循环嵌套的 `for` 初始化变量**。在包含 12 个循环控制声明、9 个符合条件的真实夹具上，9/9 全部获得稳定 ID；`for (int i...)` 的三个相邻同名循环得到三个不同 ID，值域分别为 `0..2`、`0..1`、`10..11`。
- 1000 快照压力运行中，同一个 `i` 只产生一个 ID `entry-main:L10:i`，覆盖 856 次可靠观察、值 `0..142`；重复三次的 ID 序列完全一致。
- 未观察到误合并：不嵌套的 `for` 控制变量、不同声明、同名但不同作用域的变量都保持区分；嵌套循环、循环体重入局部、`while` 内层 `for`、`goto` 重入和函数内循环仍全部保守降级。
- GDB 没有获得任何循环控制变量的可靠 ID。GDB 的 `frame.block()` 层级无法区分“顶层 `for` 块”“`while` 循环体块”和“外层循环内的 `for` 块”；实测 `z`（`while` 内层 `for`）的块链与顶层 `for` 完全同形。若按块深度标记会误合并，因此本轮不实现 GDB 循环身份。
- 最小实现建议：**只合入 LLDB 的顶层 `for` 控制变量规则**，不新增接口字段，不增加调试器往返。GDB 侧保持现状，等待源码级索引或 DWARF 块标签证据；循环体重入局部和普通函数调用实例不在本轮范围。是否接受“LLDB 比 GDB 多识别一类变化”的产品语义，由维护者决定。

**B. 紧凑编码**

- 在 1000 快照真实压力记录中，ID 字符串（值 + `"id"` 键）占紧凑 JSON 的 **14.3%**；改成整数 ID 加字典可减少 **9.6%** 原始 JSON（LLDB）/ **10.8%**（GDB），但 gzip 后 LLDB **变差 10.8%**，GDB 改善 8.5%。在真实生日记录中原始 JSON 可减少 16%，gzip 改善不稳定。
- 默认 1000 快照压力运行实际只用了 **8.9 MiB** 的快照历史（预算 32 MiB），先触发的是快照数量上限，不是字节预算。当前没有证据表明编码是瓶颈。
- `map[string]` 匹配已经是测得的四种方案中最快的：45,711 个 ID 约 **0.82 ms**，整数 ID 需要额外一次字符串驻留查找（1.44 ms），64 位 Hash 1.68 ms，结构化键 3.89 ms。
- 纯 Hash 不能作为可靠身份：8 位截断在生日扩展语料上 169 个 ID 碰撞 40 个；16 位也出现 1 个碰撞。保留精确原键做碰撞校验后，原始字符串仍然存在，尺寸收益消失。
- 结论：**本轮不建议修改 ID 类型、记录格式或共享接口**。如果以后确实需要降低保留堆，可在采集器内部做 capture-local 字符串驻留实验；目前没有证据表明它值得复杂度。`fmt.Sprintf` 构造 ID 比字符串拼接慢约 4 倍，但 45,711 个 ID 也只占压力运行总时间的约 0.16%–0.38%，不值得单独变更。

## 1. 环境与基线

| 项 | 值 |
|---|---|
| 平台 | Ubuntu 26.04 LTS, x86_64, Linux 7.0.0 |
| Go | go1.26.8 |
| LLDB 后端 | Clang 21.1.8 + lldb-dap 21.1.8 |
| GDB 后端 | GCC 15.2.0 + GDB 17.1（内置 Python 3.14.4） |
| 项目 CI 基线 | Go 1.23.x、LLVM 19.1.7（未在本机复跑） |
| 记录/UI | 0.1.1，observation record v1，未改变 |

真实运行使用 `internal/engine` 的现有 `Engine.Run` / `GDBEngine.Run`，由 `TestLoopIdentityAudit` 导出完整 `RunResult`。测试程序是本轮新增的 C 夹具；ID、值、成员关系均来自真实调试器，不是合成夹具。`DEBUGINFOD_URLS=` 用于避开本机 debuginfod 网络挂起，是环境设置，不属于引擎改动。

## 2. A：循环变量身份

### 2.1 证据模型

现有 `IdentityReliable` 只表示动态对象身份，不表示初始化、赋值或“值一定可比较”。本轮把证据拆成两层：

- **声明身份**：Clang AST 给出同一编译单元内的词法声明位置（文件、行、作用域范围）。
- **动态身份**：只有当循环在 entry `main` 的最外层控制流中、不在其他循环内部，并且 `main` 没有 `goto` 时，才认为该 `for` 初始化变量在本次 `main` 调用中只有一个动态实例。`main` 只被调用一次，因此无需额外的调用实例证据。

规则刻意保守：

1. 只处理 `for` 头部的 `DeclStmt` 变量（`for (int i = 0; ...)`），不处理循环体局部，也不处理在循环外声明的计数器（后者已由现有外层局部回退覆盖）。
2. 只处理 **没有被其他循环嵌套** 的循环。`if`/`switch` 下的循环仍可处理，因为它们在单次 `main` 调用中最多进入一次。
3. 暂停行必须在循环范围内且 **晚于声明行**；声明行本身不标记，避免把未初始化的可读内存当作已初始化。
4. 同名声明只要在暂停点有多个可见候选（外层同名、内层同名、循环体同名），就降级为不可靠。
5. `main` 内任意位置出现 `goto` 时，禁用全部循环推断；`#line` / SourceMap 装配源码也不索引循环，因为可见行范围可能不连续。
6. 不新增调试器请求。DAP 声明存在时优先使用声明；声明缺失时才用 AST 的唯一候选回退。

原型代码位于 `internal/engine/loop_identity.go`，接入点在 `identity.go`、`snapshot.go` 和 `engine.go`。`COBS_LOOP_DISABLE=1` 仅用于复现实验前基线。

### 2.2 正例

`testdata/programs/loop_identity.c` 覆盖了相邻同名循环、并列声明、外层计数器、`while`、`if` 内循环、`continue`、嵌套循环、同名遮蔽、`while` 内层 `for` 和辅助函数内循环。

| 场景 | 期望 | LLDB 实测 | GDB 实测 |
|---|---|---|---|
| 三个相邻 `for (int i...)` | 3 个不同 ID，值域 `0..2`/`0..1`/`10..11` | 3 个 ID：`entry-main:L14:i`、`L18:i`、`L21:i`，各 6/2/2 次可靠观察 | 0 个可靠循环 ID |
| 顶层 `for (int outer...)` | 1 个 ID | `entry-main:L25:outer`，值 `0,1` | 0 |
| `for (int a=0, b=0; ...)` | 两个独立 ID | `L37:a`、`L37:b` | 0 |
| `if` 内 `for (int q...)` | 1 个 ID | `L54:q` | 0 |
| 带 `continue` 的 `for (int c...)` | 1 个 ID | `L67:c` | 0 |
| 外层 `while` 计数器 `k` / `w` | 已有外层局部身份 | 可靠 | 可靠 |
| 循环外声明、循环内使用的 `m` | 已有外层局部身份 | 可靠 | 可靠 |

LLDB 前基线（`COBS_LOOP_DISABLE=1`）只有 `answer`、`m`、`k`、`w` 四个可靠局部；开启后新增 9 个循环控制 ID。GDB 前后一致：没有任何循环控制 ID。

### 2.3 反例与保守降级

| 场景 | 期望 | LLDB 实测 | GDB 实测 |
|---|---|---|---|
| 内层 `for (int inner...)`（外层还有 `for`） | 不可靠 | 0 个可靠 ID | 0 |
| 同名嵌套 `shadow`（外层/内层同名） | 外层可识别、内层不可识别 | 外层 `L31:shadow` 仅在外层循环收尾处出现；内层 0 | 0 |
| `while` 内层 `for (int z...)` | 不可靠 | 0 | 0 |
| 循环体局部 `x`、`y` | 不可靠（每次重入是新实例） | 0 | 0 |
| 辅助函数内 `for (int h...)` | 不在 entry main，不可靠 | 0 | 0 |
| `goto` 重入同一 `for` | 不可靠 | 0（整个 main 禁用循环推断） | 0 |
| 外层 `int i` 遮蔽循环 `i` | 不可靠 | 0 | 0 |
| 内层块再次遮蔽循环 `j` | 外层循环 `j` 可识别、内层不可识别 | `L11:j` 可靠；内层块 0 | 0 |

### 2.4 覆盖与误合并

- 夹具中循环控制声明共 12 个：9 个符合“entry main、非嵌套、无 goto、无同名冲突”，LLDB **9/9** 至少出现一次唯一 ID；3 个嵌套/内层循环保持不可靠。
- 循环体局部 2 个、辅助函数循环 1 个，全部保持不可靠。
- 逐 ID 检查未发现跨对象合并：三个 `i` 的值域互不相交；`a`/`b` ID 不同；`shadow` 只有外层 ID；`j` 的内层块没有可靠 ID；`z`/`inner` 没有可靠 ID。
- 1000 快照压力夹具中，单个顶层 `for (int i...)` 的 **856 次**可靠观察共享 `entry-main:L10:i`，值从 0 到 142；没有第二个 ID，也没有跨迭代合并。
- 关闭/开启循环推断的同一压力运行分别为 7.47 s / 8.42 s（开启时三次重复范围 7.44–8.42 s），差异落在波动内；ID 平均长度从 22.32 降到 22.27，没有观察到额外调试器开销或尺寸恶化。
- 三次重复的完整运行 ID 序列哈希完全一致（LLDB loop、GDB loop、LLDB stress、GDB stress 各自一致），说明当前观测下的 ID 生成是确定性的。

“0 次误合并”只代表这组夹具和这些暂停序列；它不能证明所有 C 程序都正确。复杂宏、单行循环、`setjmp/longjmp`、信号处理、`goto` 之外的非常规控制流仍未覆盖。

### 2.5 为什么 GDB 本轮不实现

GDB 的 Python 块信息实测如下（`research/loop-identity-encoding/gdb-blockdump.py`）：

| 断点 | 块链（由内到外） | 结论 |
|---|---|---|
| 第 16 行，循环体 `x` | `x` 块 → `i` 循环块 → `main` 函数块 | 控制变量 `i` 在第二层块 |
| 第 48 行，`while` 体 `y` | `y` 块 → `main` 函数块 | 与顶层循环体局部同形 |
| 第 61 行，`while` 内层 `z` | `z` 循环块 → `main` 函数块 | 外层 `while` 块未出现在链中 |
| 第 54 行，`if` 内 `q` | `q` 循环块 → `main` 函数块 | 与 `z` 同形 |

`block.function` 只在函数最外层块非空；`block.start`/`block.end` 地址范围也不能区分 `for` 块和 `while` 体块。若简单按“嵌套块变量”标记，`z`、`y` 等会被错误合并。可靠实现需要额外的源码级循环索引或 DWARF 块标签解析，复杂度和误判风险明显高于本轮收益，因此建议 GDB 维持保守降级，等待后续单独验证。

### 2.6 最小实现建议

建议把 LLDB 的顶层 `for` 控制变量规则作为独立后端改动提交，满足以下条件再合并：

- 不新增 `observer` 字段，不改变 `ID`、`IdentityReliable`、记录格式或前端匹配逻辑；TUI 仍把 ID 当作不透明字符串。
- 不增加 DAP 请求；AST 仍只解析一次，循环索引与现有入口局部索引共用同一次 Clang AST。
- 默认行为对无法证明的场景保持 `IdentityReliable=false`；生产版本去掉 `COBS_LOOP_DISABLE` 实验开关，测试可直接构造 `mainIdentity`。
- 在 LLVM 19 CI 基线和 macOS 真实 LLDB 上复跑本夹具；本轮只在 LLVM 21 + Linux 实测。
- GDB、循环体重入局部、普通函数调用实例、嵌套循环动态实例不纳入本次改动。

## 3. B：标识符编码

### 3.1 基线

所有语料来自真实运行；生日记录来自上一轮 Linux LLDB/GDB 真实记录，循环/压力语料由本轮实际调试器产生。JSON 以紧凑格式（`json.Marshal` 风格）计，避免缩进影响。

| 语料 | 快照 | ID 次数 | 唯一 ID | 平均/最大长度 | 紧凑 JSON | gzip | ID 值+键占比 |
|---|---:|---:|---:|---:|---:|---:|---:|
| birthday-native | 20 | 1,680 | 86 | 35.6 / 38 | 339,636 | 4,723 | 21.1% |
| birthday-expanded | 20 | 1,680 | 169 | 34.5 / 38 | 429,940 | 22,885 | 16.2% |
| birthday-gdb | 21 | 1,763 | 86 | 32.9 / 35 | 352,862 | 4,660 | 19.9% |
| loop-lldb | 111 | 565 | 31 | 17.9 / 21 | 220,170 | 4,432 | 6.4% |
| loop-gdb | 107 | 546 | 9 | 15.7 / 20 | 215,420 | 3,297 | 5.8% |
| stress-lldb | 1,000 | 45,711 | 90 | 22.3 / 25 | 9,329,800 | 85,546 | 14.3% |
| stress-gdb | 1,000 | 45,708 | 46 | 20.8 / 23 | 9,131,164 | 73,846 | 13.9% |

压力记录触发 `snapshot limit reached`，快照历史的紧凑 JSON 为 **8.9 MiB（LLDB）/ 8.71 MiB（GDB）**，低于 32 MiB 字节预算；默认 1000 快照上限先到。

### 3.2 编码方案对比

`int` 表示把 ID 改写为整数并在记录顶层加字典；`strint` 表示数字字符串；`hash` 表示 16 位十六进制 BLAKE2b-64（仅作尺寸对照，不作为可靠身份）；`none` 表示删除 ID 字段，是下界。百分比相对同语料紧凑 JSON。

| 语料 | int 原始 | int gzip | strint 原始 | strint gzip | hash 原始 | none 原始 |
|---|---:|---:|---:|---:|---:|---:|
| birthday-native | −16.1% | +5.4% | −16.4% | −11.5% | −9.4% | −21.9% |
| birthday-expanded | −11.7% | +2.7% | −12.5% | −2.6% | −7.0% | −16.8% |
| birthday-gdb | −15.6% | −0.3% | −15.6% | −12.1% | −8.2% | −20.8% |
| loop-lldb | −3.0% | +39.3% | −4.4% | −1.3% | +1.7% | −9.4% |
| loop-gdb | −5.7% | +0.9% | −4.9% | −1.8% | +1.8% | −9.2% |
| stress-lldb | −9.6% | +10.8% | −9.1% | −6.0% | −2.6% | −15.4% |
| stress-gdb | −10.8% | −8.5% | −9.8% | −7.6% | −2.0% | −15.1% |

关键观察：

- 原始 JSON 的节省主要来自把重复长 ID 换成短数字；但在压力语料上 gzip 可能反而变差（LLDB +10.8%），因为长 ID 的重复模式被字典/唯一数字破坏了。
- 小记录（20 快照）原始节省看似很大（16%），但绝对量只有约 55 KB，gzip 后往往只剩几 KB 差异；磁盘/分享体积不是当前痛点。
- `hash` 的尺寸收益很小（2%–9%），且 **不能直接作为可靠身份**：8 位截断在 birthday-expanded 上 169 个唯一 ID 碰撞 40 个，16 位仍有 1 个碰撞；32/64 位在本语料没有碰撞，但“没观察到”不等于碰撞概率为零。若用 Hash 索引并保留精确原键校验，原字符串仍需存储，尺寸收益消失。
- 记录 v1 的 `ID` 是字符串，改成整数需要字典、版本升级、旧记录兼容、前端匹配和预算统计一起改；收益不足以为此承担跨端契约变更。

### 3.3 运行时间与内存

微基准使用真实 ID 序列，`testing.Benchmark` 自动选择 N，`-repeats 3` 取中位数；匹配和构造都复用同一语料。

**匹配（stress-lldb，45,711 个 ID）：**

| 方案 | 中位 ns/op | 相对 string |
|---|---:|---:|
| `map[string]`（现状） | 818,042 | 1.00× |
| `map[uint32]` + 驻留查找 | 1,442,335 | 1.76× |
| `map[uint64]`（FNV-1a） | 1,681,122 | 2.06× |
| 结构化键 `{owner,middle,path}` | 3,886,461 | 4.75× |

**构造（stress-lldb）：**

| 方式 | 中位 ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `fmt.Sprintf("%s:%s:%s")` | 13,233,518 | 3,404,695 | 182,846 |
| 字符串拼接 | 3,135,180 | 1,210,096 | 45,711 |
| 拼接 + 驻留 | 4,309,343 | 1,213,592 | 45,714 |
| `strings.Clone`（保留每项） | 1,514,345 | 1,210,096 | 45,711 |

**保留堆（45,711 个 ID）：** 字符串副本约 **1.90 MiB**；uint32 + 驻留表约 **187.5 KiB**；64 位 Hash 约 **360 KiB**。整数/驻留确实能把 ID 保留堆降低约 1.7 MiB，但匹配反而更慢，且会改变模型/记录。

`fmt.Sprintf` 构造 ID 的单次成本是字符串拼接的约 4 倍，但在压力运行中全部 ID 构造的上界约 **13.2 ms**，占总运行（LLDB 约 8.4 s、GDB 约 3.4 s）的 **0.16%–0.38%**。当前代码中 `fmt.Sprintf` 用于帧/成员/静态 ID；如果今后因其他原因修改这些位置，可以顺手改为 `+`/`strconv.Itoa`，但不建议为此单独发一个 PR。

### 3.4 结论

**不建议本轮实施紧凑 ID。** 理由按优先级：

1. **没有触发预算瓶颈。** 1000 快照压力运行只用了 8.9 MiB/32 MiB，先到快照上限；在当前默认预算下，编码不决定能保存多少历史。
2. **gzip 收益不稳定。** 原始 JSON 的 9%–11% 节省在 LLDB 压力语料上变成 gzip 变差；记录分享/归档通常还会压缩。
3. **匹配成本反而上升。** 现状 `map[string]` 最快；uint32 需要额外驻留查找，结构化键更慢。
4. **Hash 不安全，且保留原键后不省空间。** 只有接受概率身份才能省掉原键，这与 `IdentityReliable` 的语义冲突。
5. **契约成本高。** ID 类型、记录格式、前端匹配和预算统计需要一起改，不符合“最小实现”的标准。

如果未来出现真实的内存压力，建议另开实验：在采集器内部维护 `map[string]uint32` 的 capture-local 驻留表，只减少保留字符串，不改变导出格式和前端；先用真实课程记录测量峰值 RSS 和 GC，再决定是否值得。不要在没有内存数据的情况下先改共享模型。

## 4. 复现

```sh
cd <repo>
./research/loop-identity-encoding/run.sh lldb loop_identity.c loop-lldb
./research/loop-identity-encoding/run.sh gdb  loop_identity.c loop-gdb
./research/loop-identity-encoding/run.sh lldb loop_identity_shadow.c shadow-lldb
./research/loop-identity-encoding/run.sh lldb loop_identity_goto.c goto-lldb
./research/loop-identity-encoding/run.sh lldb loop_identity_stress.c stress-lldb
./research/loop-identity-encoding/run.sh gdb  loop_identity_stress.c stress-gdb
```

基线对比：

```sh
COBS_LOOP_DISABLE=1 ./research/loop-identity-encoding/run.sh lldb loop_identity.c loop-lldb-before
```

全量验证：

```sh
gofmt -l internal/engine
go test ./...
go vet ./...
C_OBSERVER_REAL_DEBUGGER=1 DEBUGINFOD_URLS= go test -count=1 -v ./internal/engine
C_OBSERVER_REAL_GDB=1 DEBUGINFOD_URLS= go test -count=1 -v ./internal/engine
```

三次重复的 ID 序列 SHA-256 前缀：

- loop-lldb: `1214fa62c07eb102`（3/3 一致）
- loop-gdb: `eb93368c5553964c`（3/3 一致）
- stress-lldb: `17392ede09f91a78`（3/3 一致）
- stress-gdb: `15b053c2edb0e52e`（3/3 一致）

## 4.1 本轮验证状态

- `gofmt -l internal/engine` 干净，`go test ./...` 与 `go vet ./...` 通过。
- 真实 GDB 全套 `C_OBSERVER_REAL_GDB=1 go test -count=1 -v ./internal/engine` 通过；真实 TUI 回放测试通过。
- 真实 LLDB 全套除 **`TestRealGlobalsAndStatics`** 外通过。该失败是 LLVM 21 的 DAP 不为全局变量提供声明扩展导致的既有缺口（PR #15 只恢复了入口局部），与本轮循环规则无关；`TestRealObservation`、`TestRealFailureAndStop`、`TestRealIdentityBoundaries`、`TestRealMappedSessionAndNonzeroExit`、`TestRealMappedCallbackAndIdentity` 均通过。
- 本机没有复跑 CI 的 LLVM 19 / macOS Xcode LLDB / Windows GDB；这些平台仍是发行前必须补的验收项。

## 5. 边界与未验证

- 本轮是 **Linux x86-64、LLVM 21.1.8、GDB 17.1**；项目 CI 基线 LLVM 19 / GDB 16.3、macOS Xcode LLDB、Windows MinGW GDB 未复跑。循环索引只依赖 Clang AST 的声明/范围形状，但仍需在 LLVM 19 和 macOS 上验收。
- 只覆盖独立 C 源文件；`#line` / Companion 装配源码不索引循环，仍保持原有保守行为。
- 单行 `for`、同一行多个循环、宏生成声明、`setjmp/longjmp`、信号、异常控制流未覆盖；`goto` 出现时整体禁用循环推断。
- 循环体局部、嵌套循环实例、普通函数调用实例、递归帧仍未可靠；不要把本轮的“顶层 for 控制变量”扩展成“循环内所有变量都可靠”。
- `IdentityReliable` 仍是对象身份，不是初始化/赋值证据。GDB 在外层局部首次暂停时可能显示未初始化值（例如压力夹具的 `answer`），这是既有行为，本轮未改变。
- 真实测试只证明这些夹具/暂停序列没有误合并，不构成所有 C 程序的正确性证明。
