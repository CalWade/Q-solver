package native

import (
	"Q-Solver/internal/input/application"
	"testing"
)

func TestWindowsDriverRejectsRulesWithoutALiveWorker(t *testing.T) {
	driver := &Driver{}
	if err := driver.Apply(application.NativePlan{}); err == nil {
		t.Fatal("unstarted driver accepted a plan")
	}
	if err := driver.Stop(); err != nil {
		t.Fatal(err)
	}
	driver.done = make(chan struct{})
	close(driver.done)
	if err := driver.Apply(application.NativePlan{}); err == nil {
		t.Fatal("stopped driver accepted a plan")
	}
}

func TestWindowsDriverRejectsDuplicateStartBeforeInstallingHooks(t *testing.T) {
	done := make(chan struct{})
	driver := &Driver{done: done}
	if _, err := driver.Start(application.Callbacks{}); err == nil {
		t.Fatal("running worker was replaced")
	}
	if driver.done != done {
		t.Fatal("duplicate start changed the running lifecycle")
	}
}
