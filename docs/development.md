# 自主开发与测试

依赖 Go 1.24.1 或更新版本、Node 22 或更新版本和 npm。复用本机已有运行时；前端依赖由 `frontend/package-lock.json` 锁定，Wails 命令固定为 v2.12.0，不要求全局安装 CLI。

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
```

Windows 无需安装 make，可直接运行 `node scripts/run.mjs check`，或使用 Makefile 中对应的命令。

macOS 的 E2E 默认复用已安装的 Chrome，使用独立临时浏览器配置。其他平台首次运行：`cd frontend && npx --no-install playwright install chromium`。可通过 `PLAYWRIGHT_CHANNEL` 明确指定浏览器渠道。CI 安装自己的 Chromium，不使用个人登录状态。

端口 4173、4317 被占用会报错，不复用未知进程，也不结束其他应用。HTML 报告位于 `frontend/playwright-report`，失败轨迹位于 `frontend/test-results`；两者不提交 Git。生产构建不包含测试桥接代码。

## 分层验收

| 层级 | 验证内容 | 不包含 |
| --- | --- | --- |
| 单元 | 左右键、先后释放、重复、配置切换、编辑恢复、冲突校验 | 系统是否真正投递事件 |
| 集成 | Go 服务、持久化、驱动替换、命令分发、保存失败恢复 | 实际全局热键占用与系统权限 |
| E2E | 页面录制/保存/取消/重试连接真实 Go 逻辑 | 会议远端画面、真实监考系统 |
| 原生实测 | 物理键盘、独立接收窗口、焦点归属及实际捕获路径 | 未记录的平台/版本组合 |

CI 的 Windows/macOS 测试编译并测试对应适配器，但不会安装全局钩子。一次构建成功不能当作两个系统的行为实测。真实会议测试必须另建个人测试会议、独立接收端，记录版本和共享方式，不使用公司群。
