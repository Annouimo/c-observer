# GUI 分发与发行

目标 0.2.0-beta.1；PREVIEW_VERSION 声明预发行，VERSION 保持已发行的 0.1.2。仅 Windows 11 x64 与 Apple Silicon macOS；独立 ZIP 和 npm 包来自同一原生构建。Mac APP ad-hoc 签名（非公证），Windows GUI EXE；使用已有工具链及系统 WebView。不提供其他平台、额外安装器、运行时自动安装。

publish-npm.yml 支持手动 DRY_RUN（私有 tarball）；main 上 PREVIEW_VERSION 变更显式触发 PUBLISH_NEXT。统一准备前端与 Go 依赖，原生 runner 执行真实引擎测试并构建，收集实际 Go/前端依赖许可，组合 npm manifest 和 ZIP，双平台从准确 tarball 安装并执行 --version/--capabilities，校验 SHA256 和 Mac 签名。Go 和前端完整锁文件已经从首次预检提交，所有模式均要求锁文件不变。Mac 大数组真实测试仅在唯一失败为 variables 请求超时时重试一次；其他失败或重试失败阻止发行。beta 人工真机验收由维护者授权后置，CI 不代替真机验收。

预发行通过 main 的 PREVIEW_VERSION 变更触发，或手动选择 PUBLISH_NEXT 并明确输入 PREVIEW_VERSION；不自动改 latest，禁止覆盖已发布版本。发布 job 在全部测试成功后通过现有 npm Trusted Publishing（workflow publish-npm.yml）发布 next，并从同一产物创建 GitHub prerelease。若 npm 成功但 GitHub Release 失败，人工从该次已验证 artifact 恢复 Release，不重复 npm publish。正式 stable 需另行批准、更新 VERSION 与发行说明，再 PUBLISH_LATEST。

旧 TUI 分发、四平台历史支持见 Git 历史；不回退旧用户的已发行版本。Companion GUI 完整联调留待稳定版。引擎精简、采集预算优化留待 beta 之后。