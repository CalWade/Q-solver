package probe

const LivePhaseMillis = 2000

type CaptureTransition struct {
	Stage        string  `json:"stage"`
	AppliedNanos int64   `json:"appliedNanos"`
	PropertySet  bool    `json:"propertySet"`
	LocalVisible bool    `json:"localVisible"`
	Alpha        float64 `json:"alpha"`
}

type LivePhase struct {
	Stage                  string         `json:"stage"`
	Frames                 int            `json:"frames"`
	Outputs                int            `json:"outputs,omitempty"`
	IdleOutputs            int            `json:"idleOutputs,omitempty"`
	Outcomes               map[string]int `json:"outcomes"`
	FirstVisibleMillis     *float64       `json:"firstVisibleMillis,omitempty"`
	LastVisibleMillis      *float64       `json:"lastVisibleMillis,omitempty"`
	FirstTransparentMillis *float64       `json:"firstTransparentMillis,omitempty"`
}

type CaptureOutput struct {
	Status            string `json:"status"`
	FrameIndex        int    `json:"frameIndex"`
	PresentationNanos int64  `json:"presentationNanos"`
	DisplayNanos      int64  `json:"displayNanos"`
	ReceivedNanos     int64  `json:"receivedNanos"`
}

type LiveCaptureReport struct {
	API              string                   `json:"api"`
	WindowCount      int                      `json:"registeredWindows"`
	DisplaySelection *DisplayCaptureSelection `json:"displaySelection,omitempty"`
	Starts           int                      `json:"starts"`
	Stops            int                      `json:"stops"`
	StartedNanos     int64                    `json:"startedNanos"`
	StoppedNanos     int64                    `json:"stoppedNanos"`
	DrainedNanos     int64                    `json:"drainedNanos,omitempty"`
	Transitions      []CaptureTransition      `json:"transitions"`
	OutputCount      int                      `json:"outputCount"`
	FrameStatuses    map[string]int           `json:"frameStatuses"`
	Frames           []CaptureFrame           `json:"frames"`
	Outputs          []CaptureOutput          `json:"outputs,omitempty"`
	ErrorCode        int                      `json:"errorCode,omitempty"`
	StartupFrames    int                      `json:"startupFrames"`
	Phases           []LivePhase              `json:"phases"`
	Completed        bool                     `json:"completed"`
	Outcome          string                   `json:"protectedWindowOutcome"`
	Checks           []Check                  `json:"checks"`
}

// Every complete frame stays in the trace. Frame display time and policy
// acknowledgements share the CoreMedia host clock, including queued frames.
func (r *LiveCaptureReport) Evaluate() bool {
	if r.API == DisplayWindowStreamAPI {
		return r.evaluateDisplayStream()
	}
	if !r.beginEvaluation(r.API == StreamAPI) {
		return false
	}
	stages := []string{"baseline-before", "sharing-none", "baseline-after"}
	groups := make([][]CaptureFrame, len(stages))
	framesValid := true
	var previousPresentation, previousDisplay, previousReceipt int64
	for _, frame := range r.Frames {
		pixelsValid := validLiveFramePixels(frame)
		clocksOrdered := frame.PresentationNanos > previousPresentation && frame.DisplayNanos > previousDisplay && frame.ReceivedNanos >= previousReceipt
		clocksComparable := frame.ReceivedNanos >= frame.DisplayNanos && frame.DisplayNanos <= r.StoppedNanos
		framesValid = framesValid && pixelsValid && clocksOrdered && clocksComparable
		previousPresentation, previousDisplay, previousReceipt = frame.PresentationNanos, frame.DisplayNanos, frame.ReceivedNanos
		phase := r.phaseAt(frame.DisplayNanos)
		if phase < 0 {
			r.StartupFrames++
			continue
		}
		groups[phase] = append(groups[phase], frame)
	}
	r.Checks = append(r.Checks, Check{"frame clocks are ordered and comparable with policy acknowledgements", framesValid})
	for i, frames := range groups {
		phase := summarizeLivePhase(stages[i], r.Transitions[i].AppliedNanos, frames)
		r.Phases = append(r.Phases, phase)
		start, end := r.phaseInterval(i, r.StoppedNanos)
		valid := end-start >= LivePhaseMillis*1_000_000 && len(frames) >= 3
		if len(frames) >= 3 {
			valid = valid && frames[len(frames)-1].DisplayNanos-frames[0].DisplayNanos >= 1_500_000_000
			if i != 1 {
				for _, frame := range frames[len(frames)-3:] {
					valid = valid && frame.outcome() == "visible"
				}
			}
		}
		r.Checks = append(r.Checks, Check{stages[i] + " has sustained complete frames and valid controls", valid})
	}
	return r.finishEvaluation(r.Phases[1].Frames)
}

func (r *LiveCaptureReport) beginEvaluation(apiValid bool) bool {
	r.Outcome, r.Completed = "inconclusive", false
	r.StartupFrames, r.Phases = 0, nil
	r.Checks = []Check{
		{"single registered window during live capture", r.WindowCount == 1},
		{"one stream started and stopped", apiValid && r.Starts == 1 && r.Stops == 1 && r.ErrorCode == 0 && r.StartedNanos > 0 && r.StoppedNanos > r.StartedNanos},
		{"every complete output recorded", len(r.Frames) > 0 && validFrameStatuses(r.FrameStatuses, r.OutputCount, len(r.Frames))},
	}
	if r.API == DisplayWindowStreamAPI {
		r.Checks = append(r.Checks, Check{"display filter includes only the owned window with an exact crop", r.DisplaySelection != nil && r.DisplaySelection.Valid()})
	}
	stages := []string{"baseline-before", "sharing-none", "baseline-after"}
	transitionsValid := len(r.Transitions) == len(stages)
	var previous int64
	for i, transition := range r.Transitions {
		valid := i < len(stages) && transition.Stage == stages[i] && transition.AppliedNanos > previous && transition.LocalVisible && transition.Alpha == 1
		valid = valid && transition.PropertySet == (i == 1)
		transitionsValid = transitionsValid && valid
		previous = transition.AppliedNanos
	}
	if transitionsValid {
		transitionsValid = r.Transitions[0].AppliedNanos <= r.StartedNanos && r.StartedNanos < r.Transitions[1].AppliedNanos && r.Transitions[2].AppliedNanos < r.StoppedNanos
	}
	r.Checks = append(r.Checks, Check{"three ordered policy acknowledgements with local window visible", transitionsValid})
	if !transitionsValid {
		return false
	}
	return true
}

func (r *LiveCaptureReport) phaseAt(at int64) int {
	phase := -1
	for i, transition := range r.Transitions {
		if at >= transition.AppliedNanos {
			phase = i
		}
	}
	return phase
}

func (r *LiveCaptureReport) phaseInterval(i int, stop int64) (int64, int64) {
	start, end := r.Transitions[i].AppliedNanos, stop
	if i == 0 {
		start = r.StartedNanos
	}
	if i+1 < len(r.Transitions) {
		end = r.Transitions[i+1].AppliedNanos
	}
	return start, end
}

func (r *LiveCaptureReport) finishEvaluation(total int) bool {
	valid := true
	for _, check := range r.Checks {
		valid = valid && check.Passed
	}
	if valid {
		counts := r.Phases[1].Outcomes
		for _, outcome := range []string{"visible", "transparent", "transparent-with-data", "blank", "marker_absent"} {
			if counts[outcome] == total {
				r.Outcome = outcome
			}
		}
		if counts["visible"] > 0 && counts["visible"] < total {
			r.Outcome = "transition-leak"
		}
	}
	r.Completed = valid && r.Outcome != "inconclusive"
	r.Checks = append(r.Checks, Check{"live comparison has an interpretable protected outcome", r.Completed})
	return r.Completed
}

func summarizeLivePhase(stage string, applied int64, frames []CaptureFrame) LivePhase {
	phase := LivePhase{Stage: stage, Frames: len(frames), Outcomes: map[string]int{}}
	for _, frame := range frames {
		outcome := liveFrameOutcome(frame)
		recordLiveOutcome(&phase, outcome, frame.DisplayNanos-applied)
	}
	return phase
}

func validLiveFramePixels(frame CaptureFrame) bool {
	return frame.CapturePixels.valid() && frame.File != "" && frame.RawColorRatio != nil && *frame.RawColorRatio >= 0 && *frame.RawColorRatio <= 1
}

func recordLiveOutcome(phase *LivePhase, outcome string, elapsedNanos int64) {
	phase.Outcomes[outcome]++
	elapsed := float64(elapsedNanos) / 1_000_000
	if outcome == "visible" {
		if phase.FirstVisibleMillis == nil {
			phase.FirstVisibleMillis = &elapsed
		}
		phase.LastVisibleMillis = &elapsed
	}
	if outcome == "transparent" && phase.FirstTransparentMillis == nil {
		phase.FirstTransparentMillis = &elapsed
	}
}

func liveFrameOutcome(frame CaptureFrame) string {
	outcome := frame.outcome()
	if outcome == "transparent" && frame.RawColorRatio != nil && *frame.RawColorRatio > 0 {
		return "transparent-with-data"
	}
	return outcome
}
