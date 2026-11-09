package probe

import (
	"fmt"
	"testing"
)

func displayStreamFixture() LiveCaptureReport {
	r := liveCaptureFixture()
	r.API, r.DrainedNanos = DisplayWindowStreamAPI, r.StoppedNanos+10_000_000
	selection := displaySelectionFixture()
	r.DisplaySelection = &selection
	r.Frames, r.Outputs = nil, nil
	r.FrameStatuses["complete"], r.FrameStatuses["idle"] = 0, 0
	for phase := range r.Transitions {
		for step := 1; step <= 20; step++ {
			received := r.Transitions[phase].AppliedNanos + int64(step)*100_000_000
			output := CaptureOutput{Status: "idle", ReceivedNanos: received}
			if step == 1 || phase != 1 && step%4 == 1 {
				pixels, raw := CapturePixels{Width: 390, Height: 170, RedRatio: .4, GreenRatio: .4}, 1.0
				if phase == 1 {
					pixels.RedRatio, pixels.GreenRatio, pixels.TransparentRatio, raw = 0, 0, 1, 0
				}
				// Display presentation can be scheduled after the observer callback.
				presentation := received + 4_000_000
				r.Frames = append(r.Frames, CaptureFrame{CapturePixels: pixels, RawColorRatio: &raw,
					PresentationNanos: presentation, DisplayNanos: presentation, ReceivedNanos: received,
					File: fmt.Sprintf("display-%d.png", len(r.Frames))})
				output.Status, output.PresentationNanos, output.DisplayNanos = "complete", presentation, presentation
			}
			output.FrameIndex = len(r.Frames) - 1
			r.Outputs = append(r.Outputs, output)
			r.FrameStatuses[output.Status]++
		}
	}
	r.OutputCount = len(r.Outputs)
	return r
}

func TestDisplayIdleRetainsTheLastRealImageWithoutInventingFrames(t *testing.T) {
	r := displayStreamFixture()
	if !r.Evaluate() || r.Outcome != "transparent" {
		t.Fatal(r)
	}
	phase := r.Phases[1]
	if phase.Frames != 1 || phase.Outputs != 20 || phase.IdleOutputs != 19 || phase.Outcomes["transparent"] != 20 {
		t.Fatal("idle output was confused with a fresh image", phase)
	}
	// A complete visible image followed by idle stays visible to the receiver.
	r.Frames[5].CapturePixels, r.Frames[5].RawColorRatio = r.Frames[0].CapturePixels, r.Frames[0].RawColorRatio
	if !r.Evaluate() || r.Outcome != "visible" {
		t.Fatal("idle made a visible image disappear", r)
	}
}

func TestDisplayObserverReportsVisibleOutputBeforeTheClearImage(t *testing.T) {
	r := displayStreamFixture()
	// Move a queued pre-change visible image's callback across the policy ack.
	queued := r.Outputs[19]
	queued.ReceivedNanos = r.Transitions[1].AppliedNanos + 1_000_000
	r.Outputs[19] = queued
	if !r.Evaluate() || r.Outcome != "transition-leak" || r.Phases[1].Outcomes["visible"] != 1 {
		t.Fatal("late visible output was omitted from the receiver timeline", r)
	}
}

func TestMissingIdleEvidenceOrSparseOutputCannotCertifyDisplayProtection(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*LiveCaptureReport)
	}{
		{"trace missing", func(r *LiveCaptureReport) { r.Outputs = nil }},
		{"retained image wrong", func(r *LiveCaptureReport) { r.Outputs[21].FrameIndex = 0 }},
		{"retained image unavailable", func(r *LiveCaptureReport) { r.Outputs[21].FrameIndex = -1 }},
		{"complete image mismatch", func(r *LiveCaptureReport) { r.Outputs[20].FrameIndex = 0 }},
		{"complete metadata mismatch", func(r *LiveCaptureReport) { r.Outputs[20].PresentationNanos++ }},
		{"timestamp order", func(r *LiveCaptureReport) { r.Outputs[21].ReceivedNanos = 1 }},
		{"unknown output", func(r *LiveCaptureReport) { r.Outputs[21].Status = "unknown" }},
		{"drain missing", func(r *LiveCaptureReport) { r.DrainedNanos = 0 }},
		{"output after drain", func(r *LiveCaptureReport) { r.DrainedNanos = r.Outputs[59].ReceivedNanos - 1 }},
		{"raw data missing", func(r *LiveCaptureReport) { r.Frames[5].RawColorRatio = nil }},
		{"coverage gap", func(r *LiveCaptureReport) {
			for i := 25; i < 29; i++ {
				r.Outputs[i].ReceivedNanos = r.Outputs[24].ReceivedNanos
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := displayStreamFixture()
			test.change(&r)
			if r.Evaluate() || r.Outcome != "inconclusive" {
				t.Fatal("unreliable output trace accepted", r)
			}
		})
	}
}
