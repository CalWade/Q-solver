package app

import (
	"Q-Solver/pkg/logger"
	"os"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// PanicExit 是紧急退出（Panic Button）的实现。
//
// 触发场景：面试官要求切换屏幕共享查看"活动监视器 / 任务管理器 / 运行进程"时，
// 用户按下 panic 快捷键（macOS: Cmd+Shift+Esc，Windows: Ctrl+Alt+Shift+Esc），
// 程序立刻：
//  1. 隐藏窗口（屏幕上瞬间消失）
//  2. 取消正在进行的 LLM 请求
//  3. 清空运行态缓冲（截图 + 对话上下文）
//  4. 清空局域网广播的当前答案（iPad 端收到 clear 事件，避免在副机上残留可见答案）
//  5. **保留磁盘上的 history.dat**——下次启动可恢复（数据不应因 panic 而灭失）
//  6. 给前端 ~250ms 时间把内存里的 history flush 到磁盘（如果 watcher 还在 debounce）
//  7. os.Exit(0) 立即退出进程
//
// 隐身机制不受影响：此函数不修改任何 platform 级窗口属性，只做应用层 cleanup。
// 进程退出后，OS 会回收所有窗口 + 热键 + hook，安全。
func (a *App) PanicExit() {
	logger.Println("[PANIC] 紧急退出已触发")

	// 1. 立即隐藏窗口（在屏幕上消失；ctx 可能极早期未就绪）
	if a.ctx != nil {
		func() {
			defer func() { _ = recover() }()
			runtime.WindowHide(a.ctx)
			// 通知前端立即把 history flush 到磁盘
			runtime.EventsEmit(a.ctx, "panic-flush")
		}()
	}

	// 2. 取消当前 LLM 任务
	if a.workflow != nil {
		a.workflow.CancelCurrent()
		a.workflow.ClearContext()
	}

	// 3. 清理运行态（注意：不动磁盘上的 history.dat，那是用户的资产）
	// Cancellation prevents late output. Process exit discards the in-memory
	// screenshot queue without waiting on an in-progress native capture.

	// 4. 清空局域网广播
	if a.broadcaster != nil {
		a.broadcaster.Clear()
	}

	// 5. 等一会儿让前端的 panic-flush 事件被处理 + SSE clear 事件 flush 完
	//    250ms 是经验值——足够 EventsEmit + 前端 sync 落盘往返，但用户感知不到延迟
	time.Sleep(250 * time.Millisecond)

	// 6. 立即退出（跳过 OnShutdown，不保存最近 API Key 变更——符合 panic 语义）
	logger.Println("[PANIC] 进程即将退出")
	os.Exit(0)
}
