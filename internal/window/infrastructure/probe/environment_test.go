package probe

import "testing"

func TestGraphicalCaptureRequiresAnAvailableDesktop(t *testing.T) {
	ready := CaptureEnvironment{SessionKnown: true, OnConsole: true, DisplaysKnown: true, ActiveDisplays: 1}
	if !ready.Available() {
		t.Fatal(ready)
	}
	for _, change := range []func(*CaptureEnvironment){
		func(e *CaptureEnvironment) { e.SessionKnown = false },
		func(e *CaptureEnvironment) { e.SessionLocked = true },
		func(e *CaptureEnvironment) { e.OnConsole = false },
		func(e *CaptureEnvironment) { e.DisplaysKnown = false },
		func(e *CaptureEnvironment) { e.ActiveDisplays = 0 },
		func(e *CaptureEnvironment) { e.MainDisplayAsleep = true },
	} {
		environment := ready
		change(&environment)
		if environment.Available() {
			t.Fatal("unavailable desktop accepted", environment)
		}
	}
}
