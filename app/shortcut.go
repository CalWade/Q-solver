package app

import (
	"Q-Solver/internal/input/application"
	"Q-Solver/internal/input/infrastructure/native"
)

func (a *App) GetKeyboardProtectionStatus() application.Status {
	if a.shortcutService == nil {
		return application.Status{State: "stopped"}
	}
	return a.shortcutService.Status()
}

func (a *App) SetInputEditing(editing bool) string {
	if a.shortcutService == nil {
		return "键盘服务尚未启动"
	}
	if err := a.shortcutService.SetEditing(editing); err != nil {
		return err.Error()
	}
	return ""
}

func (a *App) RequestKeyboardPermission() bool { return native.RequestPermission() }

func (a *App) RetryKeyboardService() string {
	if a.shortcutService == nil {
		return "键盘服务尚未启动"
	}
	cfg := a.configManager.Get()
	if err := a.shortcutService.Restart(cfg.Shortcuts, cfg.KeyboardProtection); err != nil {
		return err.Error()
	}
	return ""
}

// StartRecordingKey 开始录制快捷键
func (a *App) StartRecordingKey(action string) {
	a.shortcutService.StartRecording(action)
}

// StopRecordingKey 停止录制快捷键
func (a *App) StopRecordingKey() {
	if a.shortcutService != nil {
		a.shortcutService.StopRecording()
	}
}

// ScrollContent 滚动内容
func (a *App) ScrollContent(direction string) {
	a.EmitEvent("scroll-content", direction)
}

// CopyCode 复制代码
func (a *App) CopyCode() {
	a.EmitEvent("copy-code")
}
