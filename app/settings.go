package app

import (
	"Q-Solver/pkg/config"
	"Q-Solver/pkg/logger"
)

// GetSettings 返回当前配置
func (a *App) GetSettings() config.Config {
	return a.configManager.Get()
}

// UpdateSettings 更新配置（从前端 JSON）
func (a *App) UpdateSettings(configJson string) string {
	if err := a.configManager.UpdateFromJSON(configJson); err != nil {
		return err.Error()
	}
	return ""
}

// ResetShortcutsToDefault 强制把快捷键重置为当前版本的默认值。
// 设置页在用户点击"重置快捷键"时调用。
// 调用后立即保存到磁盘，配置变更事件会自动通知 shortcut service。
func (a *App) ResetShortcutsToDefault() string {
	defaults := config.GetDefaultShortcuts()
	err := a.configManager.Patch(func(cfg *config.Config) {
		cfg.Shortcuts = defaults
	})
	if err != nil {
		logger.Printf("重置快捷键失败: %v", err)
		return err.Error()
	}
	logger.Println("快捷键已重置为默认值")
	return ""
}
