package config

import (
	"Q-Solver/pkg/common"
	"Q-Solver/pkg/logger"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type ConfigManager struct {
	config      Config
	mu          sync.RWMutex
	configPath  string
	oldConfig   Config // 这是老配置
	subscribers []func(NewConfig Config, oldConfig Config)
}

func NewConfigManager() *ConfigManager {
	cm := &ConfigManager{
		config:      NewDefaultConfig(),
		oldConfig:   NewDefaultConfig(),
		subscribers: make([]func(NewConfig Config, oldConfig Config), 0),
	}
	cm.configPath = cm.getConfigPath()
	return cm
}

func (cm *ConfigManager) getConfigPath() string {
	var appDir string

	sysConfigDir, err := os.UserConfigDir()
	if err != nil {
		// 如果获取系统目录失败（极少情况），回退到当前目录
		sysConfigDir = "."
	}
	// 拼接项目名称目录
	appDir = filepath.Join(sysConfigDir, common.AppName)

	if err := os.MkdirAll(appDir, 0755); err != nil {
	}
	fullPath := filepath.Join(appDir, "config")
	logger.Println("配置文件路径", fullPath)

	return fullPath
}

func (cm *ConfigManager) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 先设置默认值
	cm.config = NewDefaultConfig()
	// 从文件加载（AES 加密存储）
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Printf("加载配置文件失败 (使用默认配置): %v", err)
		}
	} else {
		plain, err := decrypt(data)
		if err != nil {
			logger.Printf("解密配置文件失败 (使用默认配置): %v", err)
		} else if err := json.Unmarshal(plain, &cm.config); err != nil {
			logger.Printf("解析配置文件失败: %v", err)
		}
	}

	// 补齐新增的快捷键动作（如 panic）——老配置里没有这些 key，需要从默认值补。
	// 已存在的快捷键完全保留用户自定义，不覆盖。
	migrateShortcuts(&cm.config)

	logger.Println("配置已加载")
	return nil
}

// migrateShortcuts 处理快捷键从老版本到当前版本的迁移。
//
// 迁移策略：
//  1. 全量迁移：检测到任意一个核心 action 的 ComboID 仍是过去任何版本的旧值，
//     说明用户从未自定义过当前版本的快捷键，**强制**整套覆盖为新默认值。
//     这样设置页显示的（来自 config）和实际注册的（来自 shortcut_darwin.go 硬编码）
//     才能对齐——这俩不一致用户会看到 ⌘3 但按了没反应。
//  2. 增量补齐：对已是当前版本的配置，只补齐缺失的 action（如未来新增动作）。
//
// **新增/修改快捷键默认值时，必须把上一版本的 ComboID 加入下方 legacyComboIDs，
//
//	否则升级用户会卡在老快捷键。**
func migrateShortcuts(cfg *Config) {
	defaults := GetDefaultShortcuts()
	if cfg.Shortcuts == nil {
		cfg.Shortcuts = defaults
		return
	}

	legacy := false

	// 老版本独有的 action 名（只在 1.0.x macOS 出现过）
	if _, hasSolve := cfg.Shortcuts["solve"]; hasSolve {
		legacy = true
	}

	// 历史版本核心快捷键的 ComboID 黑名单
	// 每次升级默认值时把上一版的 ComboID 追加进来
	legacyComboIDs := map[string]bool{
		// === 1.0.x 老版本（项目原始默认值） ===
		// macOS
		"Cmd+J": true, // 老版本"send"
		// Windows
		"119":    true, // F8: screenshot
		"120":    true, // F9: toggle
		"121":    true, // F10: clickthrough
		"74+162": true, // Ctrl+J: send
		"68+162": true, // Ctrl+D: delete

		// === 中间版本（三修饰符方案）===
		// macOS
		"Ctrl+Option+Shift+1": true,
		"Ctrl+Option+Shift+2": true,
		"Ctrl+Option+Shift+3": true,
		"Ctrl+Option+Shift+4": true,
		"Ctrl+Option+Shift+5": true,
		// Windows
		"16+17+18+49": true, // Ctrl+Alt+Shift+1
		"16+17+18+50": true, // Ctrl+Alt+Shift+2
		"16+17+18+51": true, // Ctrl+Alt+Shift+3
		"16+17+18+52": true, // Ctrl+Alt+Shift+4
		"16+17+18+53": true, // Ctrl+Alt+Shift+5
	}
	for _, action := range []string{"screenshot", "send", "toggle", "clickthrough", "delete"} {
		if sc, ok := cfg.Shortcuts[action]; ok && legacyComboIDs[sc.ComboID] {
			legacy = true
			break
		}
	}

	if legacy {
		logger.Println("检测到老版本快捷键配置，自动升级到新版本默认值")
		cfg.Shortcuts = defaults
		return
	}

	// 增量补齐：保留用户已自定义的 key，仅补齐新增的 action
	for action, binding := range defaults {
		if _, ok := cfg.Shortcuts[action]; !ok {
			cfg.Shortcuts[action] = binding
		}
	}
}

func (cm *ConfigManager) Save() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	plain, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	data, err := encrypt(plain)
	if err != nil {
		return fmt.Errorf("加密配置失败: %w", err)
	}
	if err := os.WriteFile(cm.configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	logger.Printf("配置已保存到: %s", cm.configPath)
	return nil
}

func (cm *ConfigManager) Get() Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

// UpdateFromJSON 从前端 JSON 全量更新配置
func (cm *ConfigManager) UpdateFromJSON(jsonStr string) error {
	var newConfig Config
	if err := json.Unmarshal([]byte(jsonStr), &newConfig); err != nil {
		return fmt.Errorf("解析配置 JSON 失败: %w", err)
	}

	cm.mu.Lock()
	cm.oldConfig = cm.config //保存当前配置为之前的配置
	cm.config = newConfig
	configCopy := cm.config
	oldConfigCopy := cm.oldConfig
	subscribers := cm.subscribers
	cm.mu.Unlock()

	// 通知订阅者
	for _, sub := range subscribers {
		sub(configCopy, oldConfigCopy)
	}

	return cm.Save()
}

// Patch 部分更新配置字段（避免全量序列化/反序列化的开销）
// patchFn 接收当前配置指针，直接修改需要变更的字段
func (cm *ConfigManager) Patch(patchFn func(cfg *Config)) error {
	cm.mu.Lock()
	cm.oldConfig = cm.config
	patchFn(&cm.config)
	configCopy := cm.config
	oldConfigCopy := cm.oldConfig
	subscribers := cm.subscribers
	cm.mu.Unlock()

	// 通知订阅者
	for _, sub := range subscribers {
		sub(configCopy, oldConfigCopy)
	}

	return cm.Save()
}

func (cm *ConfigManager) Subscribe(callback func(NewConfig Config, oldConfig Config)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.subscribers = append(cm.subscribers, callback)
}
