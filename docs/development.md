# 自主开发与测试

依赖 Go 1.24.1 或更新版本、Node 22 或更新版本和 npm。复用本机已有运行时；前端依赖由 `frontend/package-lock.json` 锁定，Wails 命令固定为 v2.12.0，不要求全局安装 CLI。

Wails CLI 首次使用时从该精确版本自己的依赖清单构建到 `.local/bin/wails-v2.12.0`，后续复用。它避开 `go run 包@版本` 的额外子模块/废弃版本在线查询；已缓存的工具不因这些查询不可达而无法运行。工具源码仍归属 Go 模块缓存，不修改全局安装。

开发与构建命令会先准备 `.local/framework/wails-v2.12.0`：从固定模块复制源码，只在项目副本上应用 `patches/` 中的 macOS 面板补丁，并校验前后哈希。Windows 也使用这份固定依赖，其平台代码没有补丁。首次使用需要能取得锁定依赖；以后可复用缓存。直接运行 Go 命令前先执行 `node scripts/run.mjs prepare`，否则本地依赖尚不存在。四个补丁文件有人工修改时会报错并保留，不自动覆盖。

补丁命令通过命令级 Git 参数固定 LF 字节，使用相同清单校验各系统输出。回归测试将 Git 默认配置设为 CRLF，核对副本补丁结果与原始模块内容。

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
make protected-session-probe # macOS 15.2+，常驻保护主验收，初次采集及重连各30秒
make capture-probe # macOS 14+：只捕获合成窗口，对照保护关闭/开启/再次关闭
make stream-capture-probe # macOS 14+：分别新建短时视频流，核对所有完整帧
make live-capture-probe # macOS 14+，同一视频流运行时开启、关闭保护并保留全部帧
make display-capture-probe # macOS 15.2+，显示器过滤器仅包含自建窗口并裁剪其区域
make display-startup-capture-probe # 采集开始前已保护的目标与自建背景对照，再切换保护
make keyboard-probe # macOS：真实事件过滤、独立 Chrome 与内部编辑器联合测试
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

`stream-capture-probe` 使用相同的自建窗口过滤器，在三个保护状态分别创建短时 `SCStream`。每个流停止完成后输出完整帧的时间戳、像素统计和原始 PNG，记录位于 `.local/reports/stream-capture-probe.json`，图像为 `capture-stream-<阶段>-<帧>.png`。每阶段至少需要 3 个完整帧且覆盖 200 毫秒，所有帧的结果都须一致。流启动后另外等待图像就绪，最多 3 秒。只收到 idle、暂停、未知状态或没有完整帧会失败，任何已采样可见帧都不能被最后一张透明图掩盖。该入口不捕获显示器、其他应用或音频。

捕获输出与像素解释都固定 sRGB，避免显示器色彩空间导致基线误判。此项单独验证新建短时单窗口流。

`live-capture-probe` 在生命周期检查后关闭额外测试窗口，按生产的单窗口形态只启动、停止一次 SCStream，保持原有过滤器，三个阶段各至少持续 2 秒。报告位于 `.local/reports/live-capture-probe.json`，原始帧为 `capture-live-<序号>.png`。每次生产窗口属性确认都记录本地可见性和 CoreMedia host time，每个完整帧保留 WindowServer 显示时间、接收时间及原始缓冲内非零 RGB 像素比例。窗口保护切换中途保持采集，全部帧按生成时间归类，排队中的旧帧仍保留在原始时间线内。

每个阶段至少有 3 个完整帧且时间跨度达到 1.5 秒。前后基线必须以连续三帧可见内容结束。`liveCapture.completed` 表示实验完整，`protectedWindowOutcome=transition-leak` 表示保护开启后仍采到可见帧，`visible` 表示该阶段全部可见，`transparent-with-data` 表示 Alpha 透明但原始 RGB 非零。正常退出与实验完成都须结合结果字段读取。该入口使用短时单窗口流，长时运行、显示器与第三方远端分别验收。

`display-capture-probe` 使用 `SCContentFilter(display:includingWindows:)`，包含名单仅有本进程的合成窗口，采集前核对窗口、进程和显示器身份，并验证裁剪范围与窗口区域一致。该命令要求 macOS 15.2+ 的过滤器名单读回能力。报告位于 `.local/reports/display-capture-probe.json`，完整图像为 `capture-display-<序号>.png`。

`display-startup-capture-probe` 在目标首次显示时保留保护属性，再开始同一个显示器流。初始保护观察至少两秒，之后连续执行关闭、开启、再关闭保护。包含名单仅有目标及同进程、相同位置尺寸的纯黑背景窗口，名单和范围在开始前读回核对。背景属于诊断装置，用于提供独立画面对照。报告位于 `.local/reports/display-startup-capture-probe.json`，`initialProtectedWindowOutcome` 与中途切换的 `protectedWindowOutcome` 分开记录。初始阶段没有图像或输出回调时报告无法确定，不能仅凭流启动确认通过。

`protected-session-probe` 是常驻保护的主验收入口。目标从首次显示前到结束始终保留默认捕获属性，两次采集各至少30秒，第二次重新创建流及过滤器并读回相同来源。包含名单仅有目标及同进程、相同范围的蓝色动态背景。背景提供实际可见的画面对照，目标使用红绿标记。`protectedCapture.completed` 表示诊断完整，`background-only` 表示全部观察只含对照背景，任何已采样红绿像素均报告可见或泄漏。缺图、纯黑、透明、暂停、错误引用和回调空缺均无法通过主验收。

报告位于 `.local/reports/protected-session-probe.json`，原始图像命名为 `capture-protected-<会话>-<序号>.png`。每个会话记录开始及结束时的本地可见性与属性、启动/停止/回调排空时间，实际名单、输出引用及原始RGB。此项复用生产窗口服务及原生适配器，独立AppKit宿主与真实生产Wails页面、会议远端分别验收。

该路径另存每次 SDK 输出。完整图像与 idle 分开计数，idle 继续引用上一张实际收到的图像。显示器的呈现时间可能晚于接收回调，观察端时间线使用实际接收时刻，原始呈现时间仍保留。每阶段输出跨度至少1.5秒，连续通知间隔和边界空缺最多250毫秒，前后基线最后三次观察须可见。缺失透明图像时，idle 仍保持之前的可见图像，判定不会将它视为隐藏成功。该实验范围为只含自建窗口的显示器过滤器。

原生面板测试启动前记录会话锁定、控制台状态、活动显示器数量及休眠状态。桌面锁定、休眠或不可用会在创建测试窗口前退出并给出环境原因。中途出现 suspended 或缺失输出仍使本次实验失败。图形实测需要持续可用的桌面。

`keyboard-probe` 需要当前身份已有辅助功能与事件投递权限。合成按键经过实际 macOS session event tap、生产输入服务和独立 Chrome/WKWebView。诊断监听只处理本轮随机标记的事件，服务使用本地动作计数器。原生侧和独立接收端共同验证十一组场景，另有异常断开及主动切换自有浏览器的用例，确认系统监听被移除、前台变化会终止投递。正向场景需要测试浏览器持续处于前台。

完整键盘报告位于 `.local/reports/keyboard-browser.json`，原生原始报告位于 `keyboard-host.json`，断开报告位于 `keyboard-disconnected.json`。浏览器进程 ID 从本轮 Playwright 浏览器连接取得，每次系统事件投递前核对目标。普通 check 运行协议与证据判定的单元测试，原生 UI 用例由显式 probe 命令启动。

拥有启动脚本子进程的 Node 控制器可用 IPC 消息 `{ "type": "stop" }` 请求停止。终端使用 Ctrl+C。准备依赖、构建和等待服务就绪期间均处理停止请求，正常停止返回零退出码。已有服务意外退出会停止同组服务并返回失败。

IPC 控制端可观察 `control-ready`、`building`、`waiting-ready` 和 `ready` 消息。后两项携带本轮服务进程 ID，分别表示服务已启动并等待健康状态，以及健康检查完成。控制端断开会结束所拥有的服务。命令完成或停止后关闭 IPC 通道，方便父进程等待最终退出。

自主启动集成测试每次创建独立的空 Go 编译缓存，实际执行冷编译，并等待 `ready` 消息后再次核对两个健康端点。测试总期限为180秒，涵盖首次依赖下载和编译。就绪期间停止用例由 `waiting-ready` 阶段触发，分别核对退出码和监听释放。
