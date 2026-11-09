package probe

import (
	"encoding/json"
	"testing"
)

func captureFixture() CaptureReport {
	r := CaptureReport{API: ScreenshotAPI}
	for i, stage := range []string{"baseline-before", "sharing-none", "baseline-after"} {
		r.Samples = append(r.Samples, CaptureSample{Stage: stage, PropertySet: i == 1, CapturePixels: CapturePixels{Width: 390, Height: 170, RedRatio: .4, GreenRatio: .4}})
	}
	return r
}

func TestCaptureDiagnosticsSeparatesCompletionFromProtection(t *testing.T) {
	r := captureFixture()
	if !r.Evaluate() || r.Outcome != "visible" {
		t.Fatal(r)
	}
	r.Samples[1].RedRatio, r.Samples[1].GreenRatio = 0, 0
	if !r.Evaluate() || r.Outcome != "marker_absent" {
		t.Fatal(r)
	}
	r.Samples[1].BlackRatio = 1
	if !r.Evaluate() || r.Outcome != "blank" {
		t.Fatal(r)
	}
	r.Samples[1].BlackRatio, r.Samples[1].TransparentRatio = 0, 1
	if !r.Evaluate() || r.Outcome != "transparent" {
		t.Fatal(r)
	}
}

func TestInvalidControlsCannotClaimCaptureProtection(t *testing.T) {
	for _, change := range []func(*CaptureReport){
		func(r *CaptureReport) { r.API = "unknown" },
		func(r *CaptureReport) { r.Samples = r.Samples[:2] },
		func(r *CaptureReport) { r.Samples[0].RedRatio = 0 },
		func(r *CaptureReport) { r.Samples[2].GreenRatio = 0 },
		func(r *CaptureReport) { r.Samples[1].ErrorCode = -1 },
		func(r *CaptureReport) { r.Samples[1].PropertySet = false },
		func(r *CaptureReport) { r.Samples[1].Width = 0 },
		func(r *CaptureReport) { r.Samples[1].RedRatio = .05 },
		func(r *CaptureReport) { r.Samples[1].TransparentRatio = -1 },
	} {
		r := captureFixture()
		change(&r)
		if r.Evaluate() || r.Outcome != "inconclusive" {
			t.Fatal("invalid comparison accepted", r)
		}
	}
}

func streamFixture() CaptureReport {
	r := captureFixture()
	r.API = StreamAPI
	for i := range r.Samples {
		s := &r.Samples[i]
		s.Stream = &StreamObservation{DurationMillis: 500, OutputCount: 3, FrameStatuses: map[string]int{"complete": 3, "idle": 0, "blank": 0, "suspended": 0, "started": 0, "stopped": 0, "unknown": 0}, Frames: []CaptureFrame{
			{CapturePixels: s.CapturePixels, PresentationNanos: 100_000_000, File: "frame-0.png"},
			{CapturePixels: s.CapturePixels, PresentationNanos: 200_000_000, File: "frame-1.png"},
			{CapturePixels: s.CapturePixels, PresentationNanos: 400_000_000, File: "frame-2.png"},
		}}
	}
	return r
}

func TestStreamEvidenceRequiresFreshFramesAndConsistentPixelOutcomes(t *testing.T) {
	r := streamFixture()
	if !r.Evaluate() || r.Outcome != "visible" {
		t.Fatal(r)
	}
	for _, change := range []func(*CaptureReport){
		func(r *CaptureReport) { r.Samples[1].Stream = nil },
		func(r *CaptureReport) { r.Samples[1].Stream.Frames = nil },
		func(r *CaptureReport) { r.Samples[1].Stream.Frames = r.Samples[1].Stream.Frames[:2] },
		func(r *CaptureReport) { r.Samples[1].Stream.DurationMillis = 0 },
		func(r *CaptureReport) { r.Samples[1].Stream.OutputCount = 1 },
		func(r *CaptureReport) { r.Samples[1].Stream.FrameStatuses = nil },
		func(r *CaptureReport) { delete(r.Samples[1].Stream.FrameStatuses, "idle") },
		func(r *CaptureReport) { r.Samples[1].Stream.FrameStatuses["complete"] = 2 },
		func(r *CaptureReport) {
			r.Samples[1].Stream.FrameStatuses["blank"] = 1
			r.Samples[1].Stream.OutputCount++
		},
		func(r *CaptureReport) {
			r.Samples[1].Stream.FrameStatuses["suspended"] = 1
			r.Samples[1].Stream.OutputCount++
		},
		func(r *CaptureReport) {
			r.Samples[1].Stream.FrameStatuses["unknown"] = 1
			r.Samples[1].Stream.OutputCount++
		},
		func(r *CaptureReport) { r.Samples[1].Stream.Frames[1].PresentationNanos = 100_000_000 },
		func(r *CaptureReport) { r.Samples[1].Stream.Frames[2].PresentationNanos = 200_000_001 },
		func(r *CaptureReport) { r.Samples[1].Stream.Frames[0].File = "" },
		func(r *CaptureReport) { r.Samples[1].Stream.Frames[0].RedRatio = 0 },
		func(r *CaptureReport) { r.Samples[1].Stream.Frames[1].Width = 200 },
	} {
		r = streamFixture()
		change(&r)
		if r.Evaluate() || r.Outcome != "inconclusive" {
			t.Fatal("invalid stream accepted", r)
		}
	}
	r = streamFixture()
	r.Samples[1].RedRatio, r.Samples[1].GreenRatio, r.Samples[1].TransparentRatio = 0, 0, 1
	for i := range r.Samples[1].Stream.Frames {
		r.Samples[1].Stream.Frames[i].CapturePixels = r.Samples[1].CapturePixels
	}
	if !r.Evaluate() || r.Outcome != "transparent" {
		t.Fatal(r)
	}
	// A visible frame anywhere in the protected sample invalidates the result.
	r.Samples[1].Stream.Frames[0].CapturePixels = r.Samples[0].CapturePixels
	if r.Evaluate() || r.Outcome != "inconclusive" {
		t.Fatal("transient visible frame ignored", r)
	}
}

func TestCapturePixelFieldsKeepTheirFlatJSONRepresentation(t *testing.T) {
	fixture := captureFixture()
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var shape struct {
		Samples []map[string]interface{} `json:"samples"`
	}
	if err := json.Unmarshal(data, &shape); err != nil || shape.Samples[0]["width"] != float64(390) {
		t.Fatal("pixel fields changed their wire representation", string(data), err)
	}
	var restored CaptureReport
	if err := json.Unmarshal(data, &restored); err != nil || !restored.Evaluate() {
		t.Fatal("capture report did not restore", restored, err)
	}
}
