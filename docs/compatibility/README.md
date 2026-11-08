# 兼容性记录与验收

这里分别记录原生宿主证据和第三方平台的实际结果。未执行的组合保持 `not_run`，不得从系统属性、网页模拟测试、构建成功或其他产品宣传填成通过。

## 已取得的本机证据

2026-11-08，在 macOS 27.0（26A428）、Apple Silicon 上运行 `node scripts/run.mjs panel-probe`。独立 AppKit NSPanel + WKWebView 加载本地合成页面，复用生产窗口应用服务和原生驱动。报告见 [原始 JSON](evidence/macos-27-panel-2026-11-08.json)。

- WKWebView 完成页面加载，面板显示后没有改变前台应用。
- 面板创建时设置 nonactivating 标记，明确拒绝成为 key/main window；直接请求 makeKeyWindow 后仍未取得键盘焦点。
- 两个重叠截图租约期间窗口透明度为零；第一个释放后仍保持零，最后一个释放后恢复一。
- 截图期间新建的第二窗口继承透明状态，最后恢复；关闭、注销和重新创建后重新应用策略。
- 八个窗口阶段的系统属性及焦点观察均通过。

这个原型没有捕获桌面、没有注入物理点击和按键、没有测试中文输入法，没有启动任何真实会议或笔试平台。`sharingType == none` 的读回只证明属性设置成功。生产程序目前仍是 Wails 2 宿主，不能引用原型结果宣称生产窗口已具备相同行为。

## 平台矩阵

[matrix.json](matrix.json) 当前有 60 个待测组合：macOS/Windows 各覆盖腾讯会议、飞书会议、钉钉会议、Zoom、Google Meet、Microsoft Teams、企业微信的整屏与单窗口共享；牛客、赛码、HackerRank、Codility 的屏幕共享、被动输出焦点、显式编辑焦点和键盘传播场景。

这些名称定义验证范围，不表示每个平台必定提供某种检测能力、共享模式或相同产品版本。入口不支持的场景应记录原因和证据，调整需求并重新审阅，不能随意填成通过。

每条已测记录必须有系统版本、平台客户端/浏览器版本、应用修订、时间和证据；共享结果还必须有独立接收端。证据只保留本地合成题面、自己的测试会议和受控窗口，避免记录账号、公司聊天或真实考试内容。记录共享选择是整屏、窗口还是其他来源；能够确认底层捕获 API 时再补充，无法确认时写明未知。

原生侧与网页侧至少联合观察：实际收到的按下/释放事件、快捷键执行次数、当前前台进程、key window、`document.hasFocus()`、`focus/blur`、`visibilitychange`、全屏状态以及独立接收画面。浏览器里的合成 `dispatchEvent` 不代表物理键盘路径；本机截图不代表会议远端画面。

`node scripts/compatibility.mjs` 只校验记录结构并统计缺口。`--require-complete` 要求全部组合有足够证据且结果符合该场景的预期；当前应失败，因为 60 项尚未执行。

## 生产宿主接入仍需完成

1. 将经过原型验证的被动面板构造规则接入生产窗口创建入口，避免 Wails 2 启动主动激活以及普通 NSWindow 点击取焦点。
2. 明确独立编辑窗口/编辑模式，验证录制、中文输入法、输入暂停及返回原应用。不能为了声称“无失焦”而让文本编辑失效。
3. 核对 Windows 的不激活显示、WM_MOUSEACTIVATE、编辑恢复和窗口重建；当前 Windows 只有交叉编译证据。
4. 使用上述矩阵完成实际共享与输入观察后，再决定哪些能力可作为默认功能，哪些仅能标为实验或不支持。

参考：[Wails 3.1.0 WebviewPanel](https://github.com/wailsapp/wails/blob/v3.1.0/v3/pkg/application/webview_panel_darwin.m) 的 `canBecomeKeyWindow` 返回 YES；[官方面板选项](https://wails.io/features/windows/options/) 提供 NonActivating 和 BecomesKeyOnlyIfNeeded。因此“换到支持 panel 的框架”本身还不能证明被动面板不改变键盘焦点。
