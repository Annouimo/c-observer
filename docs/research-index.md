# 研究成果归档

当前决策：0.1.2 只纳入 #15 与 #13 的已验证改进；恢复实际课程使用。此目录索引历史研究，保留可复现代码与数据的固定提交链接，不把实验生产代码合入 main。

| 主题 | 交接 | 研究报告 / 数据 | 决策 |
| --- | --- | --- | --- |
| 输入消费进度 | [#10](https://github.com/Routilet/c-observer/pull/10) | [输入进度实测](https://github.com/Routilet/c-observer/blob/8d012d7620ccaa6b6ea7ad7e19bea977a3546bc3/docs/input-progress-research.md) | 停止本轮研究；性能与目标程序副作用不足以支持默认启用。 |
| 主要变量身份、输出缓冲 | [#12](https://github.com/Routilet/c-observer/pull/12) | [报告](https://github.com/Routilet/c-observer/blob/d1b14382d0401d4ba6c643eecaf345fc00840817/docs/identity-output-research.md) / [真实脱敏记录](https://github.com/Routilet/c-observer/tree/d1b14382d0401d4ba6c643eecaf345fc00840817/research/identity-output) | #15 纳入最外层变量与 AST 修复；保持现有 stdout 缓冲行为。 |
| 循环身份及紧凑编码 | [#14](https://github.com/Routilet/c-observer/pull/14) | [#16](https://github.com/Routilet/c-observer/pull/16) / [报告](https://github.com/Routilet/c-observer/blob/ab834c5343135a3cecca7f29a0810ff7645abada/docs/loop-identity-encoding-research.md) / [实验与数据](https://github.com/Routilet/c-observer/tree/ab834c5343135a3cecca7f29a0810ff7645abada/research/loop-identity-encoding) | 原型保留在研究分支，不进入 0.1.2；ID 编码保持 string。 |

## 已知边界

- #16 的普通 for 识别只覆盖部分 LLDB 独立源码，未覆盖 Companion #line 装配，且非复合语句的 while/do 嵌套、直接声明证据路径仍需反例验证。循环体生命周期、普通调用、递归不作为本轮发行要求。GDB 词法块信息不足以单独证明生命周期。
- 1000 快照压力实验约 8.9 MiB，先达快照上限；整数编码约节省一成原始 JSON。匹配基准在整数路径包含每次转换成本，不能据此断言预编码整数比较更慢；短数字文本方案也未包含反向字典。现无足够实际收益支撑共享契约改造。
- LLVM 21 缺失全局声明扩展是已报告的兼容性缺口，未纳入本轮修复。不要将单平台夹具结论泛化到所有工具链。
- 维持静态输入、已捕获输出偏移、区间归因。低频不确定场景如实降级。

仅在实际使用出现显著问题时重新评估相应方向。归档保留历史文档、分支和提交，不删除研究者的工作。
