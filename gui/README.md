# GUI 离线原型

开发分支验收入口，不是正式发行。独立 Wails v3 Go 模块；后续接入引擎时整合。内置明确标识的 wrong-index 合成记录，也可打开本地 v1 JSON；不执行 C。

主界面：少量源码上下文、全宽数据、底部拖动条。点击数据模块、成员或元素关注其已知变化；再次点击取消。拖动或左右键切换相邻观察，变化跳转只依据实际相邻可比较值。— 为当前根无对应值，? 为未记录成员；不会用其他快照的值补齐。布局利用历史坐标，不拼接历史状态。完整源码与输入输出使用临时查看入口。

开发：frontend 内 npm install、npm test、npm run build；gui 内 go mod tidy、go build -tags production。npx vite 可浏览器预览。CI Ubuntu 只准备一次前端和 Go 依赖，默认原生 Mac arm64 打包；手动启用 windows 时额外构建 Windows x64。归档保留 7 天。发行前提交锁文件并双平台验收，不运行 npm 发布。

当前仍未接引擎或 Companion；输入输出为解码文本对照，不是 OJ 判定。复杂数据、长记录、原生窗口拖动性能仍需实测。
