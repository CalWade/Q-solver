package probe

import "testing"

func captureFixture() CaptureReport {
	r := CaptureReport{}
	for i, stage := range []string{"baseline-before", "sharing-none", "baseline-after"} {
		r.Samples = append(r.Samples, CaptureSample{Stage: stage, PropertySet: i == 1, Width: 390, Height: 170, RedRatio: .4, GreenRatio: .4})
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
