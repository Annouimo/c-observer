# GUI 分发与发行

正式版本由 VERSION 声明，预发行由 PREVIEW_VERSION 声明。维护者已批准 0.2.0 正式发行；只发布 Observer，不变更 Companion、Launcher 或 C Exam Kit。支持 Windows 11 x64 和 Apple Silicon macOS；原生 ZIP 与 npm 包使用同一批构建产物，Mac ad-hoc 签名，Windows GUI EXE。旧 TUI 不再发行，0.1.x 历史版本继续可安装。

main 的 VERSION 变更触发 PUBLISH_LATEST，PREVIEW_VERSION 变更触发 PUBLISH_NEXT；两者同时改动时正式版优先。手动 DRY_RUN 只构建，手动 PUBLISH_LATEST 可重试 VERSION，PUBLISH_NEXT 需显式输入 PREVIEW_VERSION。已发行版本跳过，查询失败停止，不覆盖或降级。全部模式固定依赖锁文件，统一前端构建，双平台真实引擎测试、原生安装、CLI 能力、SHA256 与签名校验。Mac 大数组用例仅在唯一失败为 variables 请求超时时重试一次；其他失败阻止发行。

npm 发布沿用 Trusted Publishing，独立 github-release 任务尝试在本次验证源码上建标签并创建 Release，不需要额外长期令牌。若标签或 Release 权限拒绝，npm 成功不受影响；从本次 observer-release-VERSION artifact 下载 ZIP、tarball 与 SHA256SUMS，维护者可手工创建对应标签和 Release。不要重复发布 npm。beta.1 的旧一次性 artifact 恢复分支已删除。

Companion 0.2.8 声明 ^0.1.0，仍使用旧 Observer；新版 Companion 需修改兼容范围并验证 GUI 调用、真实装配、安装与返回工作流。Launcher 0.3.0 通过安装信息协议发现组件，不需要为此重新发行；C Exam Kit 不变。正式版不表示 Windows 完整 GUI 或 Companion 联调已经通过；采集性能与工程精简留待后续。
