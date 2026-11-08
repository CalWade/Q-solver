package app

import "Q-Solver/internal/window/domain"

type WindowStatus domain.Status

func (a *App) GetWindowStatus() WindowStatus { return WindowStatus(a.stateManager.GetWindowState()) }
func (a *App) SetCaptureProtection(enabled bool) string {
	return windowError(a.stateManager.SetCaptureProtection(enabled))
}
func (a *App) SetLocalVisible(visible bool) string {
	return windowError(a.stateManager.SetVisible(visible))
}
func (a *App) RetryWindowPolicy() string { return windowError(a.stateManager.RetryWindowPolicy()) }
func windowError(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

// ToggleVisibility 切换可见性
func (a *App) ToggleVisibility() {
	a.stateManager.ToggleVisibility()
}

// ToggleClickThrough 切换鼠标穿透
func (a *App) ToggleClickThrough() {
	a.stateManager.ToggleClickThrough()
}

// MoveWindow 移动窗口
func (a *App) MoveWindow(dx, dy int) {
	a.stateManager.MoveWindow(dx, dy)
}

// RestoreFocus 恢复焦点
func (a *App) RestoreFocus() string {
	return windowError(a.stateManager.RestoreFocus())
}

// RemoveFocus 移除焦点
func (a *App) RemoveFocus() string {
	return windowError(a.stateManager.RemoveFocus())
}

// SetWindowAlwaysOnTop 设置窗口是否置顶
func (a *App) SetWindowAlwaysOnTop(alwaysOnTop bool) string {
	return windowError(a.stateManager.SetAlwaysOnTop(alwaysOnTop))
}
