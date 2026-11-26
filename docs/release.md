# Observer 分发（预览基础设施）

独立原生程序与 npm 数据包共享同一次构建二进制。当前仅预览，不发布 registry 或 GitHub Release。scripts/package-preview.mjs 生成 macOS arm64/x64、Linux x64 产物及清单；首版仅 LLDB；macOS ARM64 优先，macOS x64 / Linux x64 按实际 LLDB 验收支持。Windows 与 GDB 留待后续独立版本，不阻塞 0.1.x 首发。npm tarball 直接包含二进制，不含 postinstall，采用旧启动器 --ignore-scripts 验证权限和 --capabilities。默认预览包 private:true，预防误发。CI artifact 并非正式发行。

正式发行前：同步并验收最新 LLDB 后端、链接参数、macOS/Linux 实际运行及 Companion 联调，确认 npm 包权限、许可文件和版本清单，设置 Trusted Publishing 或经批准的发布授权；人工触发 main 的明确版本发布，禁止把预览包当稳定版。先发布 Observer 并确认 registry 可获取，再发布依赖该版本的 Companion。旧 launcher 无需重新下载。

## npm Trusted Publishing 准备

新发行使用 `@routilet/c-observer`；原 `@annouimo/c-observer` 预览包保留历史版本。新包首次公开发布需由维护者手动使用已验收 tarball 完成，然后配置新包的 Trusted Publishing。旧包不撤回。

包名 @routilet/c-observer。npm 侧配置 GitHub Actions：Organization/user Routilet；Repository c-observer；Workflow filename publish-npm.yml；Environment 留空；允许 npm publish（仅 stage 权限不能运行此工作流）。不创建 token。新授权必须在两天内首次成功发布，建议首发可用且 workflow 已在 main 时再配置。

publish-npm.yml 支持 DRY_RUN（默认，private:true，不联网发行）、BOOTSTRAP_PACK（main 手动触发，生成可供维护者首次本地 npm publish 的公开预览 tarball）、PUBLISH_NEXT（main 手动触发，使用 OIDC 发布 next）。项目采用 MIT License；后两项必须在 main 上运行并检查 LICENSE。当前只接受明确预发行版本，不提供 latest 发布模式；首版 LLDB 验收后另行开放 latest 正式发行；不以 GDB/Windows 为前置条件。首次创建可通过维护者登录 npm，下载 BOOTSTRAP_PACK 的 tarball、校验 SHA256SUMS，然后 npm publish ./<tarball>.tgz --access public --tag next（交互 2FA）。也可使用 npm stage publish 后审核。待包创建后配置 Trusted Publisher，再用新版本触发 PUBLISH_NEXT 验证授权，避免重复发布同版本。预览包不写入正式 Companion 依赖。


## 发行内容与验证

所有归档保留 MIT LICENSE 与由实际发行目标的 Go 包依赖并集生成的 THIRD_PARTY_NOTICES.txt（包含 Go 工具链及上游许可文件）。DRY_RUN 同时验证私有包、公开包元数据和 --ignore-scripts 安装，但只上传 private:true 的产物，不执行 npm publish。原生安装验证使用 Linux 和 macOS GitHub-hosted runner；这仅检查本机二进制可运行、版本、协议与哈希，并不代表真实调试器或 Windows 验收。发布作业必须等待这些检查完成。首次 BOOTSTRAP_PACK / PUBLISH_NEXT 仍需等待合并与维护者批准，且仅限 next。

## 首版范围（维护者确认）

0.1.0-beta.1 使用 LLDB，经过真实终端、真实程序、Companion 装配源码与旧启动器兼容性检查后再发布。0.1.0 稳定版同样不等待 Windows/GDB。保持已开发的 GDB 分支及后续计划，不合入首版入口或标记为受支持。Companion 依赖为可选，Windows 用户仍可正常使用其原有功能。不得仅凭交叉构建或 --capabilities 判断调试器可用。

许可证检查依据实际发行目标的 go list -deps 依赖集合，不把仅用于未发行平台或测试的模块混入二进制分发声明；若实际依赖缺少上游许可证，仍停止打包并审查。新增平台必须自动纳入检查。

## 首次公开预览包（维护者确认）

PR #1（同仓库 feat/tui）的一次性 BOOTSTRAP_PR 模式生成 0.1.0-beta.1 的可公开发布 tarball；执行完整包检查和三平台原生安装检查，保留 SHA256SUMS。此模式不拥有 id-token 写权限、不调用 npm publish、不合入 main。维护者仅在该 CI 成功后下载并手动发布 `next`，用于最终审核。后续自动发布仍限定 main，待工作流合入后配置 Trusted Publisher 并使用不同版本号。不要重新发布已占用的 beta.1 版本；后续 PR 构建产物不是 registry 中已发行版本。

## 第二次预览

首发 beta.1 的固定历史产物：https://github.com/Routilet/c-observer/actions/runs/45738301452/artifacts/15079363315 。合并映射身份修复后，PR #1 的手动发布产物改为 0.1.0-beta.2，避免不同内容重用 beta.1 版本。自动 OIDC 发布仍限定 main；待维护者完成 npm 首次发布及授权后再走自动流程。稳定版不在本轮发布范围。

## 更新后的预览范围

新 scope 首次迁移发行已由维护者确认完成，移除 BOOTSTRAP_PR / BOOTSTRAP_PACK，仅保留 DRY_RUN 与 main 人工触发 PUBLISH_NEXT。新增 win32-x64 构建、zip、清单和 Windows 原生安装测试；GDB 真机引擎已有记录，但正式入口/TUI/分发仍需联合验收。Windows 仅 x64，macOS LLDB，Linux 双后端。旧段落中的 LLDB-only、旧首发步骤为历史记录，不再作为计划。正式 latest 和独立 Release 尚待验收后启用。

Windows 新增实际依赖 go-localereader v0.0.1：固定版本上游 README 明确声明 MIT 并署名 Yasuhiro Matsumoto，但无独立 LICENSE。收集器仅为此精确版本校验原 README、保留原声明并补充标准 MIT 条款，不伪造上游许可文件或年份；其他缺失许可证仍阻止构建。正式发行前维护者审核此处理。

## 2026-11-26：跨平台候选验收与下一次 next

修复提交 2b9e6a3 已合入 feat/tui。真实调试器 CI 45758778049 全部通过（Windows GDB 16 快照、Linux GDB 17 快照、Linux LLDB 装配源码回归），四平台 npm 原生安装 CI 45758778175 通过。英文 Windows 的不可表示路径明确拒绝；不宣称所有 Windows Unicode 路径可用。新 scope npm dist-tags 已通过 Companion CI 45758886077 确认为 latest/next=0.1.0-beta.2；建议下次 next 为 0.1.0-beta.3，但须核对完整已发布版本列表，不能覆盖已发布版本。正式 latest 不在此轮预览中修改。

预览发行仍走既有 main 人工触发 PUBLISH_NEXT 流程，确认 CI、许可及合并审批后由维护者触发，不为 PR 添加发布权限。Companion 预览精确依赖实际发布的 Observer 版本；不要把本地私有 tarball 误当已发布包。
