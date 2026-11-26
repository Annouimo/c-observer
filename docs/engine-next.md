# Codex 当前任务：0.1.2 后端集成

维护者已决定收束研究，优先恢复实际课程使用。#15 的最外层变量身份及大 AST 解析修复是本轮正式范围；#16 的循环实验原型保留，不合入。

- 配合 #13 联合回归，保持 Engine.Run / RunResult、IdentityReliable 和 observation v1 语义不变。
- 核实完整 preset 装配的真实生日程序主要对象变化（birthdays[2].date.day 1→3）以及既有 GDB/LLDB 回归；未知身份继续降级，不将未初始化的观测值称作赋值。
- 仅处理明确阻断本轮的后端问题；循环/函数生命周期、Hash/整数 ID、输入进度、stdout 缓冲不启动新研究。LLVM 21 全局声明缺口作为已知限制记录，若实用反馈表明严重再处理。
- 保持原有预算、只读行为及工具链要求，不修改 Companion、共享接口或版本。真实平台与模拟测试分别报告。
- 依 AGENTS.md 分工。运行 gofmt、go test ./...、go vet ./... 和适用真实调试器测试。

已完成研究及未解决边界见 [research-index.md](research-index.md)。后续由实际使用反馈决定最小有效修复，不按研究列表逐项追求完备。
