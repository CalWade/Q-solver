package probe

import (
	"math"
	"testing"
)

func displaySelectionFixture() DisplayCaptureSelection {
	return DisplayCaptureSelection{
		ExpectedWindowID: 10, ActualWindowID: 10, ExpectedOwnerPID: 20, ActualOwnerPID: 20,
		ExpectedDisplay: 30, ActualDisplay: 30, WindowCount: 1, DisplayCount: 1,
		Window:  CaptureRect{X: -800, Y: 50, Width: 390, Height: 170},
		Display: CaptureRect{X: -1000, Y: -100, Width: 1000, Height: 900},
		Crop:    CaptureRect{X: 200, Y: 150, Width: 390, Height: 170},
	}
}

func TestDisplaySelectionRequiresOwnedWindowAndExactCrop(t *testing.T) {
	selection := displaySelectionFixture()
	if !selection.Valid() {
		t.Fatal(selection)
	}
	for _, change := range []func(*DisplayCaptureSelection){
		func(s *DisplayCaptureSelection) { s.WindowCount = 2 },
		func(s *DisplayCaptureSelection) { s.DisplayCount = 2 },
		func(s *DisplayCaptureSelection) { s.ActualWindowID++ },
		func(s *DisplayCaptureSelection) { s.ExpectedWindowID = 0; s.ActualWindowID = 0 },
		func(s *DisplayCaptureSelection) { s.ActualOwnerPID++ },
		func(s *DisplayCaptureSelection) { s.ExpectedOwnerPID = 0; s.ActualOwnerPID = 0 },
		func(s *DisplayCaptureSelection) { s.ActualDisplay++ },
		func(s *DisplayCaptureSelection) { s.Crop.X++ },
		func(s *DisplayCaptureSelection) { s.Crop.Width++ },
		func(s *DisplayCaptureSelection) { s.Window.X = -1100 },
		func(s *DisplayCaptureSelection) { s.Window.Width = 1000 },
		func(s *DisplayCaptureSelection) { s.Display.Width = 0 },
		func(s *DisplayCaptureSelection) { s.Crop.Y = math.NaN() },
		func(s *DisplayCaptureSelection) { s.Display.Height = math.Inf(1) },
	} {
		selection = displaySelectionFixture()
		change(&selection)
		if selection.Valid() {
			t.Fatal("unexpected display content accepted", selection)
		}
	}
}

func TestDisplayCaptureUsesItsOwnAPIAndSelectionEvidence(t *testing.T) {
	r := displayStreamFixture()
	selection := displaySelectionFixture()
	r.DisplaySelection = &selection
	if !r.Evaluate() || r.Outcome != "transparent" {
		t.Fatal(r)
	}
	r.DisplaySelection = nil
	if r.Evaluate() {
		t.Fatal("missing display filter evidence accepted", r)
	}
	r.DisplaySelection = &selection
	selection.ActualOwnerPID++
	if r.Evaluate() {
		t.Fatal("foreign capture source accepted", r)
	}
}
