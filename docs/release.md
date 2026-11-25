# Observer 分发（预览基础设施）

独立原生程序与 npm 数据包共享同一次构建二进制。当前仅预览，不发布 registry 或 GitHub Release。scripts/package-preview.mjs 生成 macOS arm64/x64、Linux x64 产物及清单；Windows 等 GDB 真机验收后添加。npm tarball 直接包含二进制，不含 postinstall，采用旧启动器 --ignore-scripts 验证权限和 --capabilities。默认预览包 private:true，预防误发。CI artifact 并非正式发行。

正式发行前：协调后端选取和链接参数，Linux/macOS/Windows 实机验收，确认 npm 包权限、许可文件和版本清单，设置 Trusted Publishing 或经批准的发布授权；人工触发 main 的明确版本发布，禁止把预览包当稳定版。先发布 Observer 并确认 registry 可获取，再发布依赖该版本的 Companion。旧 launcher 无需重新下载。

## npm Trusted Publishing 准备

包名 @annouimo/c-observer。npm 侧配置 GitHub Actions：Organization/user Annouimo；Repository c-observer；Workflow filename publish-npm.yml；Environment 留空；允许 npm publish（仅 stage 权限不能运行此工作流）。不创建 token。新授权必须在两天内首次成功发布，建议首发可用且 workflow 已在 main 时再配置。

publish-npm.yml 支持 DRY_RUN（默认，private:true，不联网发行）、BOOTSTRAP_PACK（main 手动触发，生成可供维护者首次本地 npm publish 的公开预览 tarball）、PUBLISH_NEXT（main 手动触发，使用 OIDC 发布 next）。项目采用 MIT License；后两项必须在 main 上运行并检查 LICENSE。当前只接受明确预发行版本，不提供 latest 发布模式；GDB/Windows 验收后另行完善正式发行。首次创建可通过维护者登录 npm，下载 BOOTSTRAP_PACK 的 tarball、校验 SHA256SUMS，然后 npm publish ./<tarball>.tgz --access public --tag next（交互 2FA）。也可使用 npm stage publish 后审核。待包创建后配置 Trusted Publisher，再用新版本触发 PUBLISH_NEXT 验证授权，避免重复发布同版本。预览包不写入正式 Companion 依赖。


## 发行内容与验证

所有归档保留 MIT LICENSE 与由 Go 模块依赖树生成的 THIRD_PARTY_NOTICES.txt（包含 Go 工具链及上游许可文件）。DRY_RUN 同时验证私有包、公开包元数据和 --ignore-scripts 安装，但只上传 private:true 的产物，不执行 npm publish。原生安装验证使用 Linux 和 macOS GitHub-hosted runner；这仅检查本机二进制可运行、版本、协议与哈希，并不代表真实调试器或 Windows 验收。发布作业必须等待这些检查完成。首次 BOOTSTRAP_PACK / PUBLISH_NEXT 仍需等待合并与维护者批准，且仅限 next。
