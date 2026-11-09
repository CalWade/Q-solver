package probe

import "fmt"

const ProtectedSessionMillis = 30_000

type ProtectedCaptureSession struct {
	StreamObservation
	Selection    *DisplayCaptureSelection `json:"selection"`
	Before       CaptureTransition        `json:"before"`
	After        CaptureTransition        `json:"after"`
	Starts       int                      `json:"starts"`
	Stops        int                      `json:"stops"`
	StartedNanos int64                    `json:"startedNanos"`
	StoppedNanos int64                    `json:"stoppedNanos"`
	DrainedNanos int64                    `json:"drainedNanos"`
	Outputs      []CaptureOutput          `json:"outputs"`
	ErrorCode    int                      `json:"errorCode,omitempty"`
	Phase        LivePhase                `json:"phase"`
	Outcome      string                   `json:"protectedWindowOutcome"`
	Checks       []Check                  `json:"checks"`
}

type ProtectedCaptureReport struct {
	API         string                    `json:"api"`
	WindowCount int                       `json:"registeredWindows"`
	Sessions    []ProtectedCaptureSession `json:"sessions"`
	Completed   bool                      `json:"completed"`
	Outcome     string                    `json:"protectedWindowOutcome"`
	Checks      []Check                   `json:"checks"`
}

func (s *ProtectedCaptureSession) evaluate() bool {
	s.Outcome = "inconclusive"
	observations, framesValid, traceValid := observeDisplayStream(s.Frames, s.Outputs, s.OutputCount, s.FrameStatuses, s.StoppedNanos, s.DrainedNanos)
	phase, coverage := evaluateDisplayPhase(s.Before, observations, s.StartedNanos, s.DrainedNanos, false)
	s.Phase = phase
	policy := s.Before.PropertySet && s.After.PropertySet && s.Before.LocalVisible && s.After.LocalVisible && s.Before.Alpha == 1 && s.After.Alpha == 1
	policy = policy && s.Before.Stage == "protected-session-start" && s.After.Stage == "protected-session-end"
	policy = policy && s.Before.AppliedNanos > 0 && s.Before.AppliedNanos <= s.StartedNanos && s.After.AppliedNanos >= s.DrainedNanos
	duration := s.Starts == 1 && s.Stops == 1 && s.StartedNanos > 0 && s.StoppedNanos-s.StartedNanos >= ProtectedSessionMillis*1_000_000
	duration = duration && s.DurationMillis >= ProtectedSessionMillis && s.ErrorCode == 0
	if len(observations) > 0 {
		span := observations[len(observations)-1].output.ReceivedNanos - observations[0].output.ReceivedNanos
		coverage = coverage && span >= ProtectedSessionMillis*1_000_000-2*maxDisplayOutputGapNanos
	}
	// Blue with no red/green markers proves an actual control image arrived.
	// Visible target markers are also an interpretable failure of exclusion.
	interpretable := phase.Outputs > 0 && phase.Outcomes["visible"]+phase.Outcomes["background-only"] == phase.Outputs
	s.Checks = []Check{
		{"explicitly owned target and background with an exact crop", s.Selection != nil && s.Selection.Valid() && s.Selection.Background != nil},
		{"protection and local visibility acknowledged before and after capture", policy},
		{"one uninterrupted capture session of at least thirty seconds", duration},
		{"complete image pixels and media clocks are valid", framesValid},
		{"every SDK output refers to its actual image", traceValid},
		{"continuous observed output covers the session", coverage},
		{"target or positive control image is observed throughout", interpretable},
	}
	valid := true
	for _, check := range s.Checks {
		valid = valid && check.Passed
	}
	if valid {
		s.Outcome = protectedPhaseOutcome(phase.Outcomes, phase.Outputs)
	}
	return valid && s.Outcome != "inconclusive"
}

func (r *ProtectedCaptureReport) Evaluate() bool {
	r.Outcome, r.Completed = "inconclusive", false
	r.Checks = []Check{
		{"known owned-window display capture API", r.API == DisplayWindowStreamAPI},
		{"one managed target window", r.WindowCount == 1},
		{"initial capture and a fresh reconnection are both recorded", len(r.Sessions) == 2},
	}
	counts := map[string]int{}
	files := map[string]bool{}
	total := 0
	var previous *ProtectedCaptureSession
	for i := range r.Sessions {
		session := &r.Sessions[i]
		valid := session.evaluate()
		for _, frame := range session.Frames {
			valid = valid && !files[frame.File]
			files[frame.File] = true
		}
		if previous != nil {
			valid = valid && session.Before.AppliedNanos >= previous.After.AppliedNanos
			valid = valid && sameCaptureSource(previous.Selection, session.Selection)
		}
		r.Checks = append(r.Checks, Check{fmt.Sprintf("protected session %d capture and source continuity", i+1), valid})
		for outcome, count := range session.Phase.Outcomes {
			counts[outcome] += count
			total += count
		}
		previous = session
	}
	valid := true
	for _, check := range r.Checks {
		valid = valid && check.Passed
	}
	if valid {
		r.Outcome = protectedPhaseOutcome(counts, total)
	}
	r.Completed = valid && r.Outcome != "inconclusive"
	return r.Completed
}

func sameCaptureSource(a, b *DisplayCaptureSelection) bool {
	if a == nil || b == nil || a.Background == nil || b.Background == nil {
		return false
	}
	return a.ExpectedWindowID == b.ExpectedWindowID && a.ExpectedOwnerPID == b.ExpectedOwnerPID && a.ExpectedDisplay == b.ExpectedDisplay && a.Display == b.Display && a.Window == b.Window && a.Crop == b.Crop && *a.Background == *b.Background
}
