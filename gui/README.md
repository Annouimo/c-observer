# GUI 产品整合预览

开发分支，尚未发行。与原项目统一 Go 模块，保留 gui/ 入口直到正式迁移，旧 cmd/c-observer TUI 暂不变。原生启动默认选择 .c 源文件和样例，预期输出通过勾选区分未提供与空输出；运行后进入已验收的数据观察工作区。源码选择使用系统文件对话框，允许输入路径。运行期间可以取消；关闭应用取消并等待引擎清理。重跑复用配置并重新读取源码，保存记录显式写入主目录、唯一文件名。离线记录不执行代码。

原生预览二进制支持 [source.c]、--session FILE、--replay FILE、--version、--capabilities 及旧工具覆盖参数；session 自动采集，不修改 Companion。旧入口尚未替换，尚未验证正式 npm 调用。浏览器 Vite 开发预览使用标识为合成的夹具，不接本机引擎。

开发：gui/frontend 内 npm install、npm test、npm run build；仓库根 go mod tidy、go test ./internal/... ./cmd/...、go vet ./internal/... ./cmd/...；gui 内 go build -tags production。CI 默认 Mac arm64，Windows 按需，准备阶段验证共享包。可在目标机器设置 C_OBSERVER_REAL_APP=1 后运行 go test -run TestRealApplication ./internal/app（需要本机编译器及调试器）。真机运行、窗口关闭清理及 Companion 联调仍需验收。

主界面仍为数据优先、时间线驱动：点击数据标记变化，拖动/点击轨道、点击标记带、滚轮、左右键、Shift+左右键、Home/End、编号精确定位；源码和样例整体点击查看详情。遵循减少动效设置。
