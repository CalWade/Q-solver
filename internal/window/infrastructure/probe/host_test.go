package probe

import "testing"

func hostFixture() HostReport {
	r := HostReport{PageReady: true, TextRoundTrip: true}
	for _, stage := range []string{"startup-passive", "explicit-editor", "returned-passive"} {
		r.Observations = append(r.Observations, HostObservation{Stage: stage, Panel: true, NonActivating: true, HasEditingGate: true, CanBecomeKey: stage == "explicit-editor", Key: stage == "explicit-editor", ForegroundIsApp: stage == "explicit-editor", ForegroundRestored: stage != "explicit-editor"})
	}
	return r
}
func TestHostEvidenceRequiresFocusGateAndCompleteReturn(t *testing.T) {
	r := hostFixture()
	if !r.Evaluate() {
		t.Fatal(r)
	}
	for _, change := range []func(*HostReport){func(r *HostReport) { r.Observations[0].HasEditingGate = false }, func(r *HostReport) { r.Observations[1].Key = false }, func(r *HostReport) { r.Observations[2].ForegroundRestored = false }, func(r *HostReport) { r.TextRoundTrip = false }, func(r *HostReport) { r.Observations = r.Observations[:2] }} {
		r = hostFixture()
		change(&r)
		if r.Evaluate() {
			t.Fatal("false host success", r)
		}
	}
}
