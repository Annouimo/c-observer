# C Observer 前端：体验版预览

状态：合成回放和真实 LLDB 预览入口已经实现；本分支用于产品审核，不是正式发行。主程序入口由双方在合并时协调。

## 本地预览

从仓库根目录（Go 1.23 或兼容版本）：

```sh
go run -mod=mod ./cmd/c-observer-preview -fixture testdata/snapshots/adaptive-array.json
go run -mod=mod ./cmd/c-observer-preview -setup
go run -mod=mod ./cmd/c-observer-preview -fixture testdata/snapshots/wrong-index.json
go run -mod=mod ./cmd/c-observer-preview -fixture testdata/snapshots/compile-failed.json
```

默认仍是合成回放，界面明确标记，不运行 C。查看实际渲染的文本：
```sh
go run -mod=mod ./cmd/c-observer-preview -fixture testdata/snapshots/adaptive-array.json -dump -step 6 -width 100 -height 30
```
0 表示最终结果。Linux 和 macOS arm64 预览二进制由 CI 上传；后者仅交叉构建，真机视觉验收另做。

## 真实运行

显式加 -live 才启动真实程序：
```sh
go run -mod=mod ./cmd/c-observer-preview -live
go run -mod=mod ./cmd/c-observer-preview -live -source main.c -input sample.in -expected sample.out
```
可用 -compiler/-debugger 指定工具链。-live -setup 显示启动配置。依赖已安装的 Clang / LLDB DAP。正式 CLI 支持 macOS LLDB、Linux GDB/LLDB、Windows x64 GDB；该预览入口的 -live 仍使用 LLDB。真实记录不会标为合成；身份不可靠时不生成虚假变化或自动裁剪。

## 数据优先回放
- 默认全宽数据区；←/→：历史前后；↑/↓：对象选择；C：完整源码；I：输入输出；Tab/Shift+Tab：视图；Home/End：全部历史首尾。
- Enter：数据详情（数据区）、调用栈（源码区）、输入输出切换（控制台）。
- F：选择全部 / 任意可靠变化 / 当前变量变化 / 当前源码行的停靠记录。J：直接输入快照号。H：对象或源码行历史。
- Esc：返回数据或关闭详情；S：调用栈；O：当前对象操作。
- D/A/V：数据表 / 采集范围重跑 / 完整与自动展示切换。详情中 T 浏览全部已采集成员，Enter 看选中单元格原值。O 可访问手动展示区间（矩阵支持行、列）。
- [ / ]：源码横向滚动、详情表翻列。R：重跑（预览重置）；E：独立模式重新配置；?：帮助；Q：退出。

默认数据区优先保留所有根对象标题，按终端宽高选出可展示更多数据的布局；整段历史预估列宽。太多对象时随选择滚动。执行上下文来自前一个暂停点，变化为相邻快照区间差异，尚未确认单行归因。小数组、矩阵、结构体及结构体数组在空间允许时直接展示，声明容量不是折叠依据。详情表保留真实下标、缺失标记、可靠相邻变化。固定关注范围由完整已采集历史计算；一维两端各 2 项，二维各 1 行列；无可靠变化时完整显示采集范围。V 随时恢复完整范围，再次 V 恢复自动范围。展示区间和采集页分离，后者需重跑。未观察到变化不等于未初始化或未使用。

输入与实际输出按需查看，控制台可切换 stdin、stdout、stderr、预期和对照；最终状态为单独页面。对照按行号配对、逐字节比较，不做 OJ 判题，显示空格/回车/换行。长行折行。编译失败、运行异常、取消及截断保留历史或诊断。Esc 取消采集，Q 等待后端清理后退出。

启动配置：Ctrl+F 补全 C 文件路径，Ctrl+O 导入当前样例字段（普通文件，1 MiB 上限）；Tab 切换，Ctrl+R 开始。源码内容由当次 RunResult 冻结，修改需重跑。

## 验收
```sh
go test ./...
go vet ./...
C_OBSERVER_REAL_DEBUGGER=1 go test -v -run '^TestRealEngineReplay$' ./internal/tui
```
真实用例通过已发布 LLDB 后端运行大容量数组、含零样例、检查真实输出、将记录送入 TUI 并逐快照渲染；在真实记录上检查可靠变化及自适应范围。合成测试覆盖自适应范围、稀疏缺失、矩阵、结构体、栈帧、历史筛选、控制台、样例导入。最终需在真实终端审核布局和键位，并联调新引擎。详见 docs/frontend-next.md。

合成压力预览使用 `testdata/snapshots/data-first-stress.json`。用 J 跳至 2、4、6、8、10、12、14、16 分别检查不同课程题型。
