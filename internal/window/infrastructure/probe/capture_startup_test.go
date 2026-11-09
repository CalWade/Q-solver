package probe

import "testing"

func protectedStartupFixture() LiveCaptureReport {
	r := displayStreamFixture()
	r.DisplaySelection.WindowCount = 2
	r.DisplaySelection.Background = &CaptureBackground{ExpectedWindowID: 11, ActualWindowID: 11, OwnerPID: r.DisplaySelection.ExpectedOwnerPID, Window: r.DisplaySelection.Window}
	const shift = 2_200_000_000
	for i := range r.Transitions {
		r.Transitions[i].AppliedNanos += shift
	}
	for i := range r.Frames {
		r.Frames[i].PresentationNanos += shift
		r.Frames[i].DisplayNanos += shift
		r.Frames[i].ReceivedNanos += shift
	}
	for i := range r.Outputs {
		r.Outputs[i].ReceivedNanos += shift
		if r.Outputs[i].Status == "complete" {
			r.Outputs[i].PresentationNanos += shift
			r.Outputs[i].DisplayNanos += shift
		}
		r.Outputs[i].FrameIndex++
	}
	r.StoppedNanos += shift
	r.DrainedNanos += shift
	r.InitialProtection = &CaptureTransition{Stage: "initial-sharing-none", AppliedNanos: 1_000_000_000, PropertySet: true, LocalVisible: true, Alpha: 1}
	raw := 0.0
	first := CaptureFrame{CapturePixels: CapturePixels{Width: 390, Height: 170, TransparentRatio: 1},
		PresentationNanos: 1_204_000_000, DisplayNanos: 1_204_000_000, ReceivedNanos: 1_200_000_000, RawColorRatio: &raw, File: "initial.png"}
	r.Frames = append([]CaptureFrame{first}, r.Frames...)
	var initial []CaptureOutput
	for i := 0; i < 20; i++ {
		output := CaptureOutput{Status: "idle", FrameIndex: 0, ReceivedNanos: first.ReceivedNanos + int64(i)*100_000_000}
		if i == 0 {
			output.Status, output.PresentationNanos, output.DisplayNanos = "complete", first.PresentationNanos, first.DisplayNanos
		}
		initial = append(initial, output)
	}
	r.Outputs = append(initial, r.Outputs...)
	r.FrameStatuses["complete"]++
	r.FrameStatuses["idle"] += 19
	r.OutputCount += 20
	return r
}

func TestProtectionBeforeDisplayCaptureHasItsOwnObservedOutcome(t *testing.T) {
	r := protectedStartupFixture()
	if !r.Evaluate() || r.InitialOutcome != "transparent" || r.Outcome != "transparent" {
		t.Fatal(r)
	}
	if r.InitialPhase.Frames != 1 || r.InitialPhase.IdleOutputs != 19 || r.InitialPhase.Outcomes["transparent"] != 20 {
		t.Fatal("initial retained image was not observed", r.InitialPhase)
	}
	r.Frames[0].TransparentRatio, r.Frames[0].BlackRatio = 0, 1
	if !r.Evaluate() || r.InitialOutcome != "blank" {
		t.Fatal("opaque background was confused with transparency", r)
	}
	// Initial protection and a subsequent toggle can have different results.
	r.Frames[0].CapturePixels, r.Frames[0].RawColorRatio = r.Frames[1].CapturePixels, r.Frames[1].RawColorRatio
	if !r.Evaluate() || r.InitialOutcome != "visible" || r.Outcome != "transparent" {
		t.Fatal("startup visibility was confused with the later policy toggle", r)
	}
}

func TestProtectedStartupRequiresAcknowledgementCoverageAndOwnedSource(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*LiveCaptureReport)
	}{
		{"property off", func(r *LiveCaptureReport) { r.InitialProtection.PropertySet = false }},
		{"local hidden", func(r *LiveCaptureReport) { r.InitialProtection.LocalVisible = false }},
		{"local transparent", func(r *LiveCaptureReport) { r.InitialProtection.Alpha = 0 }},
		{"wrong stage", func(r *LiveCaptureReport) { r.InitialProtection.Stage = "other" }},
		{"missing initial acknowledgement", func(r *LiveCaptureReport) { r.InitialProtection.AppliedNanos = 0 }},
		{"protection after stream start", func(r *LiveCaptureReport) { r.InitialProtection.AppliedNanos = r.StartedNanos + 1 }},
		{"no owned source", func(r *LiveCaptureReport) { r.DisplaySelection = nil }},
		{"background control missing", func(r *LiveCaptureReport) { r.DisplaySelection.Background = nil }},
		{"wrong capture API", func(r *LiveCaptureReport) { r.API = StreamAPI }},
		{"initial observation missing", func(r *LiveCaptureReport) { r.Outputs = r.Outputs[20:] }},
		{"initial output gap", func(r *LiveCaptureReport) {
			for i := 1; i < 5; i++ {
				r.Outputs[i].ReceivedNanos = r.Outputs[0].ReceivedNanos
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := protectedStartupFixture()
			test.change(&r)
			if r.Evaluate() || r.Outcome != "inconclusive" || r.InitialOutcome != "inconclusive" {
				t.Fatal("unreliable protected startup accepted", r)
			}
		})
	}
}

func TestProtectedStartupWithoutInitialImageStaysInconclusive(t *testing.T) {
	r := protectedStartupFixture()
	r.Frames, r.Outputs = r.Frames[1:], r.Outputs[20:]
	for i := range r.Outputs {
		r.Outputs[i].FrameIndex--
	}
	r.OutputCount -= 20
	r.FrameStatuses["complete"]--
	r.FrameStatuses["idle"] -= 19
	if r.Evaluate() || r.InitialOutcome != "inconclusive" || r.InitialPhase.Outputs != 0 {
		t.Fatal("a stream with no initial image certified protection", r)
	}
}

func TestProtectedStartupRetainsRawColorAndClearsDerivedResults(t *testing.T) {
	r := protectedStartupFixture()
	raw := 1.0
	r.Frames[0].RawColorRatio = &raw
	if !r.Evaluate() || r.InitialOutcome != "transparent-with-data" {
		t.Fatal("initial RGB data was hidden by alpha", r)
	}
	r.InitialProtection = nil
	if r.Evaluate() || r.InitialPhase != nil || r.InitialOutcome != "" {
		t.Fatal("derived initial result survived reevaluation", r)
	}
}
