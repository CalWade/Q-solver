package shortcut

// ServiceDelegate 定义了 Shortcut Service 需要 App 配合做的事情
type ServiceDelegate interface {
	TriggerSolve()
	TriggerScreenshot()
	TriggerSend()
	TriggerDeleteScreenshot()
	ToggleVisibility()
	ToggleClickThrough()
	MoveWindow(dx, dy int)
	ScrollContent(direction string)
	EmitEvent(eventName string, data ...interface{})
	// PanicExit 紧急退出：立刻隐藏窗口、取消 LLM 请求、清缓存、os.Exit(0)
	PanicExit()
}
