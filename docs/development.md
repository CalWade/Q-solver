# 自主开发与测试

依赖 Go 1.24.1 或更新版本、Node 22 或更新版本和 npm。复用本机已有运行时；前端依赖由 `frontend/package-lock.json` 锁定，Wails 命令固定为 v2.12.0，不要求全局安装 CLI。

Wails CLI 首次使用时从该精确版本自己的依赖清单构建到 `.local/bin/wails-v2.12.0`，后续复用。它避开 `go run 包@版本` 的额外子模块/废弃版本在线查询；已缓存的工具不因这些查询不可达而无法运行。工具源码仍归属 Go 模块缓存，不修改全局安装。

开发与构建命令会先准备 `.local/framework/wails-v2.12.0`：从固定模块复制源码，只在项目副本上应用 `patches/` 中的 macOS 面板补丁，并校验前后哈希。Windows 也使用这份固定依赖，其平台代码没有补丁。首次使用需要能取得锁定依赖；以后可复用缓存。直接运行 Go 命令前先执行 `node scripts/run.mjs prepare`，否则本地依赖尚不存在。四个补丁文件有人工修改时会报错并保留，不自动覆盖。

```sh
npm --prefix frontend ci
node scripts/run.mjs doctor
node scripts/run.mjs dev
```

启动后访问 http://127.0.0.1:4173。该入口启动真实 Vue 页面与 Go 规则/配置服务；仅将系统输入、窗口和外部服务替换为模拟实现。测试配置每次放在新的临时目录，退出后清理。Ctrl+C 同时停止前后端。默认不连接真实模型、会议、笔试账号或局域网广播。

```sh
make unit         # 领域规则、应用编排、配置、前端逻辑
make integration  # HTTP → 配置事务 → 输入领域 → 命令端口
make e2e          # 自动启动两个服务、运行浏览器、保留失败截图与 trace、停止服务
make check        # 前端测试/构建、全量 Go race 测试、E2E
make build        # 原生应用打包，不启动应用
make panel-probe  # macOS：短暂显示隔离原生面板，自动验证并关闭
make host-probe   # macOS：真实 Wails 宿主进入编辑，再恢复原应用并关闭
make focus-probe  # macOS：独立可见 Chrome 与真实宿主联合观察焦点；不访问外网
make capture-probe # macOS 14+：只捕获合成窗口，对照保护关闭/开启/再次关闭
make compatibility # 校验兼容性记录结构，明确输出未测试数量
```

Windows 无需安装 make，可直接运行 `node scripts/run.mjs check`，或使用 Makefile 中对应的命令。

macOS 的 E2E 默认复用已安装的 Chrome，使用独立临时浏览器配置。其他平台首次运行：`cd frontend && npx --no-install playwright install chromium`。可通过 `PLAYWRIGHT_CHANNEL` 明确指定浏览器渠道。CI 安装自己的 Chromium，不使用个人登录状态。

端口 4173、4317 被占用会报错，不复用未知进程，也不结束其他应用。HTML 报告位于 `frontend/playwright-report`，失败轨迹位于 `frontend/test-results`；两者不提交 Git。生产构建不包含测试桥接代码。

## 分层验收

| 层级 | 验证内容 | 不包含 |
| --- | --- | --- |
| 单元 | 左右键、先后释放、重复、配置切换、编辑恢复、冲突校验 | 系统是否真正投递事件 |
| 集成 | Go 服务、持久化、驱动替换、命令分发、保存失败恢复、截图管道、模型适配、请求取消与 SSE | 实际全局热键占用与系统权限 |
| E2E | 页面录制/保存/取消/重试，截图→生成→主界面与副机同步，旧请求拒绝与刷新恢复 | 会议远端画面、真实监考系统 |
| 原生实测 | 物理键盘、独立接收窗口、焦点归属及实际捕获路径 | 未记录的平台/版本组合 |

CI 的 Windows/macOS 测试编译并测试对应适配器，但不会安装全局钩子。一次构建成功不能当作两个系统的行为实测。真实会议测试必须另建个人测试会议、独立接收端，记录版本和共享方式，不使用公司群。

隔离环境的副机页面在 `http://127.0.0.1:4173/broadcast/`。测试用模型可逐段输出、延迟、报错及故意发送旧回调；它复用生产请求服务和模型端口，不读取真实密钥。原生报告写入 `.local/reports/panel-probe.json` 和 `host-probe.json`。普通 `check` 不启动原生窗口；两个 probe 是独立、可自主运行的原生验收入口，`host-probe` 会短暂获取编辑焦点。直接编译该宿主须使用 Wails 的 `production` 标签；项目自有的 `prod` 标签不能代替它，统一脚本已处理这一区别。

`node scripts/compatibility.mjs --require-complete` 用于真正的平台验收门槛：有未测试项、缺失版本/证据、缺失独立接收端或不确定结果都会失败。普通结构校验通过不等于兼容性验证通过。

`focus-probe` 使用项目锁定的 Playwright 和临时 Chrome 配置，显式关闭工具默认开启的焦点模拟。原生宿主每到一个阶段会等待浏览器观察完成，再继续下一步；原生属性与网页 `document.hasFocus()`、真实 `focus/blur`、可见性和全屏状态一同记录。它不注入网页事件，也不使用个人浏览器配置。第二个用例在编辑中断开控制端，确认焦点归还和进程退出。任一端失败会让完整报告失败；观察器断开或超时也不会无限等待。

联合报告位于 `.local/reports/focus-browser.json`，异常清理报告位于 `focus-disconnected.json`。普通 CI/check 运行证据判定与阶段协议的单元测试；需要图形桌面的原生浏览器测试显式运行 `focus-probe`，不把无图形环境的编译结果当作验收。

`capture-probe` 需要已有屏幕录制权限，仅预检，不自动请求授权。它复用生产窗口服务设置捕获属性，再使用 `SCScreenshotManager` 的单窗口过滤器只捕获自己的合成面板；找不到该窗口就终止，不回退到显示器或其他窗口。报告位于 `.local/reports/capture-probe.json`，三张原始图像为 `capture-sample-0.png` 至 `capture-sample-2.png`。

该命令成功退出只表示对照完成，必须另外读取 `protectedWindowOutcome`：`visible` 是仍可见，`transparent` 是全透明，`blank` 是近乎全黑，`marker_absent` 只说明标记缺失，`inconclusive` 不能下结论。缺失或失败的基线不能作为保护成功的证据。它未运行持续屏幕流、整屏共享或第三方客户端。
