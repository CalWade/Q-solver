package config

import (
	"Q-Solver/pkg/common"
	"Q-Solver/pkg/shortcut"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type ConfigManager struct {
	updateMu    sync.Mutex
	mu          sync.RWMutex
	config      Config
	configPath  string
	loadFailed  bool
	subscribers []func(Config, Config)
}

func NewConfigManager() *ConfigManager {
	dir := os.Getenv("Q_SOLVER_CONFIG_DIR")
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			base = "."
		}
		dir = filepath.Join(base, common.AppName)
	}
	return NewConfigManagerAt(filepath.Join(dir, "config"))
}

// NewConfigManagerAt is the filesystem adapter seam used by isolated services
// and integration tests. Tests never read the user's application settings.
func NewConfigManagerAt(path string) *ConfigManager {
	return &ConfigManager{config: NewDefaultConfig(), configPath: path}
}

func (cm *ConfigManager) Load() error {
	cm.updateMu.Lock()
	defer cm.updateMu.Unlock()
	cfg := NewDefaultConfig()
	data, err := os.ReadFile(cm.configPath)
	if err != nil && !os.IsNotExist(err) {
		return cm.loadError(err)
	}
	if err == nil {
		plain, err := decrypt(data)
		if err != nil {
			return cm.loadError(err)
		}
		if err = json.Unmarshal(plain, &cfg); err != nil {
			return cm.loadError(err)
		}
	}
	migrateShortcuts(&cfg)
	if err := normalize(&cfg); err != nil {
		return cm.loadError(err)
	}
	cm.mu.Lock()
	cm.config = cfg
	cm.loadFailed = false
	cm.mu.Unlock()
	return nil
}

func (cm *ConfigManager) loadError(err error) error {
	cm.mu.Lock()
	cm.loadFailed = true
	cm.mu.Unlock()
	return fmt.Errorf("加载配置失败，原文件保留：%w", err)
}

func migrateShortcuts(cfg *Config) {
	if cfg.Shortcuts == nil {
		cfg.Shortcuts = map[string]shortcut.KeyBinding{}
	}
	if old, ok := cfg.Shortcuts["solve"]; ok {
		if _, exists := cfg.Shortcuts["screenshot"]; !exists {
			cfg.Shortcuts["screenshot"] = old
		}
		delete(cfg.Shortcuts, "solve")
	}
	for action, binding := range GetDefaultShortcuts() {
		if _, exists := cfg.Shortcuts[action]; !exists {
			cfg.Shortcuts[action] = binding
		}
	}
}

func normalize(cfg *Config) error {
	bindings, err := shortcut.NormalizeBindings(cfg.Shortcuts)
	if err != nil {
		return err
	}
	cfg.Shortcuts = bindings
	return cfg.Validate()
}

func (cm *ConfigManager) Get() Config { cm.mu.RLock(); defer cm.mu.RUnlock(); return cm.config.Clone() }

func (cm *ConfigManager) Decode(jsonStr string) (Config, error) {
	cfg := cm.Get()
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		return cfg, fmt.Errorf("解析配置 JSON 失败：%w", err)
	}
	if err := normalize(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (cm *ConfigManager) UpdateFromJSON(jsonStr string) error {
	cfg, err := cm.Decode(jsonStr)
	if err != nil {
		return err
	}
	return cm.Replace(cfg)
}

func (cm *ConfigManager) Replace(cfg Config) error {
	cm.updateMu.Lock()
	defer cm.updateMu.Unlock()
	return cm.replaceLocked(cfg)
}

func (cm *ConfigManager) replaceLocked(cfg Config) error {
	cfg = cfg.Clone()
	if err := normalize(&cfg); err != nil {
		return err
	}
	if err := cm.write(cfg); err != nil {
		return err
	}
	cm.mu.Lock()
	old := cm.config.Clone()
	cm.config = cfg
	cm.loadFailed = false
	subscribers := append([]func(Config, Config){}, cm.subscribers...)
	cm.mu.Unlock()
	for _, sub := range subscribers {
		sub(cfg.Clone(), old.Clone())
	}
	return nil
}

func (cm *ConfigManager) Patch(update func(*Config)) error {
	cm.updateMu.Lock()
	defer cm.updateMu.Unlock()
	cfg := cm.Get()
	update(&cfg)
	return cm.replaceLocked(cfg)
}

func (cm *ConfigManager) Save() error {
	cm.updateMu.Lock()
	defer cm.updateMu.Unlock()
	cm.mu.RLock()
	failed := cm.loadFailed
	cfg := cm.config.Clone()
	cm.mu.RUnlock()
	if failed {
		return fmt.Errorf("配置未成功加载，拒绝覆盖原文件")
	}
	return cm.write(cfg)
}

func (cm *ConfigManager) write(cfg Config) error {
	plain, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	data, err := encrypt(plain)
	if err != nil {
		return err
	}
	dir := filepath.Dir(cm.configPath)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(name, cm.configPath); err != nil {
		return fmt.Errorf("保存配置失败：%w", err)
	}
	return nil
}

func (cm *ConfigManager) Subscribe(callback func(Config, Config)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.subscribers = append(cm.subscribers, callback)
}
