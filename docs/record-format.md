# 观察记录格式 v1

0.1.1 新增观察记录。实际运行完成（含取消、超时、编译错误、部分采集）后，在主界面或结果页按 W，确认后在用户主目录创建 c-observer-日期时间-随机后缀.json。默认不保存；不覆盖已有文件；失败移除不完整文件。界面显示保存路径。记录含源码、样例、诊断和本地路径，分享前应检查敏感信息。

独立原生程序入口：

```sh
c-observer --replay /path/to/c-observer-record.json
```

Windows 使用 c-observer.exe。npm 分发包内选择 manifest.json 对应的本机原生程序；无全局 shim。离线回放不查找或调用工具链、不打开原始源码路径、不重新运行代码，R 仅重置回放。其他原有导航、详情、历史、输入输出和结果对照保持不变；调整采集范围需要实际运行，不可在离线回放补采。合成开发夹具继续使用 c-observer-preview，不等同正式记录。

## JSON

- format: c-observer-record；schema_version: 1；observer_version: 实际 CLI 构建版本；recorded_at: 记录创建时刻（UTC）；platform、arch: 采集程序所在系统。
- backend_request、compiler_request、debugger_request: 原运行配置中的请求/覆盖（非实际探测出的工具版本）。run.session.backend 为实际后端标识。当前引擎没有统一提供实际编译器、调试器版本，故不伪造；保留后续补充空间。
- stdin、expected_stdout: 原始字节以 JSON base64 编码；expected_stdout 为 null 表示未提供，为空字符串表示提供了空输出；comparison 保留原样例比较规则。
- run: 原始 RunResult，含冻结源码、快照、身份可靠性、采集完整性、值与输出偏移、输出字节、退出信息及诊断。不保存前端展示裁剪或派生变化。不保存可执行文件、完整进程内存或凭据；源码/输入本身可能含敏感内容。
- 旧开发夹具、未知格式版本、非法输出偏移及非连续 ID 拒绝导入；空历史、正常结束和不完整运行合法。单文件上限 128 MiB。记录不是签名或真实性证明；对导入记录只称离线记录。

记录和输入会话协议是不同概念，不修改 Engine.Run、RunResult 或 Companion session v1。实际调试器版本探测、输入位置采集、变量身份修复及界面重构不在此次版本范围内。
