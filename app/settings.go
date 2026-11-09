package app

import (
	"Q-Solver/pkg/config"
	"Q-Solver/pkg/logger"
	"Q-Solver/pkg/shortcut"
)

// GetSettings 返回当前配置
func (a *App) GetSettings() config.Config {
	return a.configManager.Get()
}

func (a *App) GetProtectionShortcutPreset() map[string]shortcut.KeyBinding {
	return config.GetProtectionShortcuts()
}

// UpdateSettings 更新配置（从前端 JSON）
func (a *App) UpdateSettings(configJson string) string {
	cfg, err := a.configManager.Decode(configJson)
	if err != nil {
		return err.Error()
	}
	if err := a.applySettings(cfg); err != nil {
		return err.Error()
	}
	return ""
}

// ResetShortcutsToDefault 强制把快捷键重置为当前版本的默认值。
// 设置页在用户点击"重置快捷键"时调用。
// 调用后立即保存到磁盘，配置变更事件会自动通知 shortcut service。
func (a *App) ResetShortcutsToDefault() string {
	defaults := config.GetDefaultShortcuts()
	cfg := a.configManager.Get()
	cfg.Shortcuts = defaults
	err := a.applySettings(cfg)
	if err != nil {
		logger.Printf("重置快捷键失败: %v", err)
		return err.Error()
	}
	logger.Println("快捷键已重置为默认值")
	return ""
}

func (a *App) applySettings(cfg config.Config) error {
	commit := func() error { return a.configManager.Replace(cfg) }
	if a.shortcutService == nil {
		return commit()
	}
	return a.shortcutService.Configure(cfg.Shortcuts, cfg.KeyboardProtection, commit)
}
