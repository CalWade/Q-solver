# 固定原生宿主补丁

`wails-v2.12.0-panel.patch` 只修改 Wails v2.12.0 的四个 Darwin 文件：

- 构造 nonactivating NSPanel，默认拒绝键盘焦点；`QSEditable` 由平台适配器显式切换。
- 启动使用 accessory 激活策略，不主动激活应用。
- WebView 允许首击，是否允许面板获得键盘焦点服从编辑状态。

`wails-v2.12.0-panel.json` 记录完整上游模块的 Go 校验值，以及四个文件补丁前后的 SHA-256。由于 `go.mod` 使用本地替换，`go mod tidy` 会从 `go.sum` 移除上游模块校验值，准备脚本因此直接与该清单核对。`node scripts/run.mjs prepare` 从固定模块复制到 `.local/framework/wails-v2.12.0`，校验后应用补丁；共享模块缓存不修改。未受管理的目标目录或被编辑的补丁文件会保留并报错，须先保存自己的修改再重建。

升级流程：先审阅新版本宿主行为，再更新补丁和前后哈希，运行脚本测试、`check`、`build`、`panel-probe` 与 `host-probe`。不得仅更新版本号或绕开校验。补丁保留上游许可与来源；项目私有副本属于可重建依赖，不提交 Git。
