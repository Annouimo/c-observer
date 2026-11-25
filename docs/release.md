# Observer 分发（预览基础设施）

独立原生程序与 npm 数据包共享同一次构建二进制。当前仅预览，不发布 registry 或 GitHub Release。scripts/package-preview.mjs 生成 macOS arm64/x64、Linux x64 产物及清单；Windows 等 GDB 真机验收后添加。npm tarball 直接包含二进制，不含 postinstall，采用旧启动器 --ignore-scripts 验证权限和 --capabilities。包 private:true，预防误发。CI artifact 并非正式发行。

正式发行前：协调后端选取和链接参数，Linux/macOS/Windows 实机验收，确认 npm 包权限、许可文件和版本清单，设置 Trusted Publishing 或经批准的发布授权；人工触发 main 的明确版本发布，禁止把预览包当稳定版。先发布 Observer 并确认 registry 可获取，再发布依赖该版本的 Companion。旧 launcher 无需重新下载。