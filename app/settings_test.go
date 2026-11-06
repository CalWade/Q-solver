package app

import (
	"Q-Solver/internal/input/infrastructure/simulated"
	"Q-Solver/pkg/config"
	"Q-Solver/pkg/shortcut"
	"path/filepath"
	"testing"
)

type quietDelegate struct{ shortcut.ServiceDelegate }

func (quietDelegate) EmitEvent(string, ...interface{}) {}

func TestSettingsBindingCommitsOnlyValidInputConfiguration(t *testing.T) {
	cm := config.NewConfigManagerAt(filepath.Join(t.TempDir(), "config"))
	if err := cm.Load(); err != nil {
		t.Fatal(err)
	}
	input := shortcut.NewServiceWithDriver(quietDelegate{}, &simulated.Driver{})
	cfg := cm.Get()
	if err := input.Start(cfg.Shortcuts, cfg.KeyboardProtection); err != nil {
		t.Fatal(err)
	}
	defer input.Stop()
	a := &App{configManager: cm, shortcutService: input}
	if got := a.UpdateSettings(`{"keyboardProtection":{"enabled":true,"blockLeft":false,"blockRight":false}}`); got == "" {
		t.Fatal("invalid sides saved")
	}
	if cm.Get().KeyboardProtection.Enabled {
		t.Fatal("failed save changed config")
	}
	if got := a.UpdateSettings(`{"keyboardProtection":{"enabled":true,"blockLeft":true}}`); got != "" {
		t.Fatal(got)
	}
	if a.GetKeyboardProtectionStatus().State != "active" {
		t.Fatal(a.GetKeyboardProtectionStatus())
	}
	if err := a.SetInputEditing(true); err != "" {
		t.Fatal(err)
	}
	if err := a.RetryKeyboardService(); err != "" {
		t.Fatal(err)
	}
	if a.GetKeyboardProtectionStatus().State != "editing" {
		t.Fatal("retry armed shortcuts in editor")
	}
	if err := a.ResetShortcutsToDefault(); err != "" {
		t.Fatal(err)
	}
}
