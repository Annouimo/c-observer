# Observer 分发与发行

## 正式发行 0.1.0

维护者已批准直接进入正式 npm 发行，通过正常安装和课程调试流程继续体验验收。精确单行归因与布局微调不阻塞本轮发行；当前回放按相邻暂停区间如实展示。

- npm 包：`@routilet/c-observer`（MIT），包含原生二进制、manifest、SHA256 及许可证；不包含生命周期安装脚本。独立归档和 npm 包共享同一次构建。
- 支持 macOS arm64/x64（LLDB）、Linux x64（GDB/LLDB）、Windows x64（GDB）。GDB 需 Python 支持，实际工具链需已安装；不扫描 IDE、不改 PATH、不承诺所有 Windows Unicode 路径。
- GitHub Actions `publish-npm.yml`：`DRY_RUN` 默认生成私有包并验证公开元数据；main 人工触发 `PUBLISH_LATEST` + 正式版本发布 latest；`PUBLISH_NEXT` 只接受显式预发行版本并发布 next。版本已占用时拒绝发布。发布需通过打包、Go 测试/静态检查、四平台原生安装和 SHA256 校验。
- Trusted Publishing：npm 包配置 Organization Routilet、Repository c-observer、Workflow publish-npm.yml、Environment 留空；无需长期 token。若授权失败，由维护者检查 npm 设置，不绕过权限。
- 发布顺序：先 Observer 0.1.0，确认 npm registry 可安装，再发布绑定该正式依赖的 Companion 0.2.8，最后发布 Launcher 0.3.0。
- CI 不等于实机视觉验收。数据优先重构 5a5cd2c 的真实 Windows/Linux GDB、Linux LLDB、Companion 装配回放及四平台原生安装均通过（45778124229、45778124197、45778124219）。稳定版工作流改动须重新测试。

## 许可

保留实际发行目标的 Go 依赖并集生成的 THIRD_PARTY_NOTICES.txt；实际依赖缺失许可证时阻止打包。Windows 依赖 go-localereader v0.0.1 的固定上游 README 明确声明 MIT 并署名 Yasuhiro Matsumoto，收集器保留原声明并补充标准 MIT 条款，不伪造上游许可文件或年份。

## 历史

旧 scope @annouimo/c-observer 预览包保留历史版本。新 scope 已发行 beta.1/beta.2；不撤回或覆盖旧版本。历史预览流程以 Git 历史为准。
