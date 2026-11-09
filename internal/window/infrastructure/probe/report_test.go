package probe

import "testing"

func completeReport() Report {
	r := Report{WebViewReady: true}
	for stage, alpha := range map[string]float64{"shown": 1, "capture-first": 0, "capture-nested": 0, "secondary-during-capture": 0, "first-release": 0, "last-release": 1, "secondary-restored": 1, "recreated": 1} {
		r.Observations = append(r.Observations, Observation{Stage: stage, Panel: true, Visible: true, NonActivating: true, CaptureProperty: true, Alpha: alpha, ForegroundUnchanged: true})
	}
	return r
}
func TestCompleteEvidencePassesButDoesNotCertifyCapture(t *testing.T) {
	r := completeReport()
	if !r.Evaluate() || len(r.Limitations) < 3 {
		t.Fatal(r)
	}
}
func TestMissingOrAdverseEvidenceFails(t *testing.T) {
	for _, change := range []func(*Report){
		func(r *Report) { r.WebViewReady = false },
		func(r *Report) { r.Observations = r.Observations[1:] },
		func(r *Report) { r.Observations[0].CanBecomeKey = true },
		func(r *Report) { r.Observations[0].ForegroundUnchanged = false },
		func(r *Report) { r.Observations[0].Alpha = 0.5 },
		func(r *Report) { r.Errors = []string{"native error"} },
	} {
		r := completeReport()
		change(&r)
		if r.Evaluate() {
			t.Fatal("incomplete/adverse native evidence passed", r)
		}
	}
}

func TestWholeReportIncludesLiveCaptureCleanupAndEvidence(t *testing.T) {
	r := completeReport()
	live := liveCaptureFixture()
	r.LiveCapture = &live
	if !r.Evaluate() {
		t.Fatal(r)
	}
	live.Stops = 0
	if r.Evaluate() {
		t.Fatal("window observations masked incomplete live capture", r)
	}
}
