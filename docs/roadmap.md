# 里程碑与验收

## 0：共同接口（已确认）
共享 Go 模型与阻塞 Engine.Run / RunResult。所有采集完成后回放，不引入流式协议或真实逆向执行。

## 1：LLDB 首版
维护者决定首版只支持 LLDB，macOS ARM64 优先；macOS x64 与 Linux x64 需分别记录实际 LLDB 运行验收。保留 Codex 的 GDB 与 Windows 开发分支，不纳入首版发行门槛。
- 同步已实现的 LLDB 身份、全局及静态变量、用户函数采集。
- TUI 类型化观察、历史索引、自适应范围、调用栈、输入输出与真实终端验收。
- 课程案例：数组求和、排序、二分、结构体错下标、qsort 比较器、递归、全局/静态变量、异常及取消。区分可靠身份和不可确认变化，区分实际运行与交叉构建。
- 验证 Companion diagnosticCode / #line、辅助函数、链接参数、冻结样例和真实终端交接。
- 保留现有 Engine 接口，不将 GDB 后端或 Windows 未验证能力写入首版正式入口。

## 2：首版分发
MIT 许可及实际发行依赖声明、独立原生归档和 npm tarball 同源构建。旧启动器 --ignore-scripts 真实安装升级、离线使用；Observer 为可选依赖，不支持的平台保留 Companion 原功能。
先发布 0.1.0-beta.1 到 next，配置并验证 Trusted Publishing；首版 LLDB 达到验收标准即可发布 0.1.0，不等待 Windows/GDB。维护者审核后发布。详见 docs/release.md、docs/integration.md。

## 3：后续平台支持
GDB/MI、Windows 原生进程管理、Dev-C++ 旧工具链兼容、共享工具链发现和跨平台实测。保留独立工作，不以其进展决定首版 LLDB 发行。新增平台需更新能力、分发、许可和实际运行验收。
