package app

import (
	"Q-Solver/pkg/config"
	"Q-Solver/pkg/solution"
	"path/filepath"
	"testing"
)

func TestWorkflowSettingsAreAnIndependentSnapshot(t *testing.T) {
	manager := config.NewConfigManagerAt(filepath.Join(t.TempDir(), "config"))
	cfg := config.NewDefaultConfig()
	cfg.Model = "before"
	cfg.APIKey = "fixture"
	cfg.CompressionQuality = 72
	if err := manager.Replace(cfg); err != nil {
		t.Fatal(err)
	}
	snapshot := solution.SettingsSource{Manager: manager}.Snapshot()
	cfg.Model = "after"
	cfg.CompressionQuality = 90
	if err := manager.Replace(cfg); err != nil {
		t.Fatal(err)
	}
	if snapshot.Model != "before" || snapshot.Capture.Quality != 72 || (solution.SettingsSource{Manager: manager}).Snapshot().Model != "after" {
		t.Fatal(snapshot)
	}
}

func TestUnstartedWorkflowMethodsAreSafe(t *testing.T) {
	a := &App{}
	a.TriggerScreenshot()
	a.TriggerSend()
	a.RemoveScreenshot(0)
	a.RemoveLastScreenshot()
	a.ClearScreenshots()
	if a.CancelRequest(1) || a.CancelRunningTask() || a.GetWorkflowStatus().Phase != "idle" {
		t.Fatal("unstarted workflow")
	}
}
