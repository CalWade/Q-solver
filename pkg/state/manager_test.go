package state

import (
	windowapp "Q-Solver/internal/window/application"
	"Q-Solver/internal/window/domain"
	"Q-Solver/internal/window/infrastructure/simulated"
	"testing"
)

func TestWindowAdapterReportsFailedProtectionWithoutChangingLocalVisibility(t *testing.T) {
	driver := &simulated.Driver{}
	service := windowapp.New(driver)
	service.Register("main")
	events := map[string]int{}
	sm := &StateManager{windows: service, emitEvent: func(name string, _ ...interface{}) { events[name]++ }}
	driver.Fail("capture")
	sm.ToggleVisibility()
	status := sm.GetWindowState()
	if !status.Windows[0].Applied.CaptureProtection || !sm.IsVisible() {
		t.Fatal(status)
	}
	if events["capture-protection-state"] != 0 || events["window-state"] != 1 || events["toast"] != 1 {
		t.Fatal(events)
	}
	driver.Fail("")
	if err := sm.RetryWindowPolicy(); err != nil {
		t.Fatal(err)
	}
	if sm.GetWindowState().Windows[0].Applied.CaptureProtection || !sm.IsVisible() {
		t.Fatal(sm.GetWindowState())
	}
	if !sm.ToggleClickThrough() {
		t.Fatal("mouse preference not applied")
	}
	if err := sm.RestoreFocus(); err != nil {
		t.Fatal(err)
	}
	if sm.IsClickThrough() || sm.GetWindowState().Desired.InputMode != domain.Editing {
		t.Fatal(sm.GetWindowState())
	}
	sm.RemoveFocus()
	if !sm.IsClickThrough() {
		t.Fatal("mouse preference lost")
	}
}
