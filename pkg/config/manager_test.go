package config

import (
	"Q-Solver/internal/input/domain"
	"Q-Solver/pkg/shortcut"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestProtectionAndCustomBindingsPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	cm := NewConfigManagerAt(path)
	if err := cm.Load(); err != nil {
		t.Fatal(err)
	}
	if err := cm.Patch(func(cfg *Config) {
		cfg.KeyboardProtection = domain.Protection{Enabled: true, BlockRight: true}
		cfg.Shortcuts["screenshot"] = shortcut.KeyBinding{Keys: []domain.Key{domain.Meta, "KeyB"}}
	}); err != nil {
		t.Fatal(err)
	}
	next := NewConfigManagerAt(path)
	if err := next.Load(); err != nil {
		t.Fatal(err)
	}
	cfg := next.Get()
	if !cfg.KeyboardProtection.BlockRight || cfg.KeyboardProtection.BlockLeft || !cfg.KeyboardProtection.Enabled {
		t.Fatal(cfg.KeyboardProtection)
	}
	keys, _ := shortcut.ParseBinding(cfg.Shortcuts["screenshot"])
	found := false
	for _, key := range keys {
		found = found || key == "KeyB"
	}
	if !found {
		t.Fatal("custom key not persisted")
	}
	data, _ := os.ReadFile(path)
	if string(data) == cfg.ToJSON() {
		t.Fatal("plaintext config written")
	}
}

func TestConfigSnapshotsDoNotAlias(t *testing.T) {
	cm := NewConfigManagerAt(filepath.Join(t.TempDir(), "config"))
	cm.Load()
	cfg := cm.Get()
	binding := cfg.Shortcuts["screenshot"]
	binding.Keys[0] = "KeyZ"
	cfg.Shortcuts["screenshot"] = binding
	current := cm.Get()
	if current.Shortcuts["screenshot"].Keys[0] == "KeyZ" {
		t.Fatal("snapshot mutated stored settings")
	}
}

func TestFailedSaveLeavesMemoryAndSubscribersUntouched(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	os.WriteFile(file, []byte("x"), 0600)
	cm := NewConfigManagerAt(filepath.Join(file, "config"))
	called := false
	cm.Subscribe(func(Config, Config) { called = true })
	if err := cm.Patch(func(cfg *Config) { cfg.Model = "changed" }); err == nil {
		t.Fatal("expected failure")
	}
	if cm.Get().Model == "changed" || called {
		t.Fatal("failed save became visible")
	}
}

func TestInvalidPolicyDoesNotReplaceFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	cm := NewConfigManagerAt(path)
	if err := cm.Save(); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if err := cm.Patch(func(cfg *Config) { cfg.KeyboardProtection = domain.Protection{Enabled: true} }); err == nil {
		t.Fatal("invalid protection accepted")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("invalid settings replaced valid file")
	}
}

func TestMigrationPreservesOldCustomCombinations(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.Shortcuts = map[string]shortcut.KeyBinding{"send": {ComboID: "74+162"}, "solve": {ComboID: "119"}}
	migrateShortcuts(&cfg)
	if cfg.Shortcuts["send"].ComboID != "74+162" || cfg.Shortcuts["screenshot"].ComboID != "119" {
		t.Fatal("migration overwrote user bindings")
	}
	if _, exists := cfg.Shortcuts["solve"]; exists {
		t.Fatal("legacy action not renamed")
	}
}

func TestCorruptConfigIsNotOverwrittenOnShutdown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	os.WriteFile(path, []byte("corrupt-original"), 0600)
	cm := NewConfigManagerAt(path)
	if cm.Load() == nil {
		t.Fatal("corruption ignored")
	}
	if cm.Save() == nil {
		t.Fatal("corrupt original overwritten")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "corrupt-original" {
		t.Fatal("original changed")
	}
}

func TestConcurrentPatchesAreSerialized(t *testing.T) {
	cm := NewConfigManagerAt(filepath.Join(t.TempDir(), "config"))
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := cm.Patch(func(cfg *Config) { cfg.WindowWidth++ }); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if cm.Get().WindowWidth != 20 {
		t.Fatal("lost concurrent updates")
	}
}
