package probe

const maxDisplayOutputGapNanos = 250_000_000

type displayObservation struct {
	output CaptureOutput
	frame  CaptureFrame
}

// Display output can become idle after clearing the included window. An idle
// notification retains its last complete image. It never creates a new image.
// Callback receipt time defines when the receiver observed that image because
// media presentation timestamps may be scheduled ahead of the callback.
func (r *LiveCaptureReport) evaluateDisplayStream() bool {
	if !r.beginEvaluation(r.API == DisplayWindowStreamAPI) {
		return false
	}
	observations, framesValid, traceValid := observeDisplayStream(r.Frames, r.Outputs, r.OutputCount, r.FrameStatuses, r.StoppedNanos, r.DrainedNanos)
	r.Checks = append(r.Checks, Check{"complete images have valid pixels and ordered media timestamps", framesValid})
	groups := make([][]displayObservation, 3)
	var initialObservations []displayObservation
	for _, observation := range observations {
		phase := r.phaseAt(observation.output.ReceivedNanos)
		if phase < 0 {
			if observation.output.Status == "complete" {
				r.StartupFrames++
			}
			if initial := r.InitialProtection; initial != nil {
				traceValid = traceValid && observation.output.ReceivedNanos >= initial.AppliedNanos
				initialObservations = append(initialObservations, observation)
			}
			continue
		}
		groups[phase] = append(groups[phase], observation)
	}
	r.Checks = append(r.Checks, Check{"every SDK output records its complete image or retained idle image", traceValid})
	for i, observations := range groups {
		start, end := r.phaseInterval(i, r.DrainedNanos)
		phase, valid := evaluateDisplayPhase(r.Transitions[i], observations, start, end, i != 1)
		r.Phases = append(r.Phases, phase)
		r.Checks = append(r.Checks, Check{phase.Stage + " has continuous observed output and valid controls", valid})
	}
	if initial := r.InitialProtection; initial != nil {
		phase, valid := evaluateDisplayPhase(*initial, initialObservations, r.StartedNanos, r.Transitions[0].AppliedNanos, false)
		r.InitialPhase = &phase
		r.InitialOutcome = protectedPhaseOutcome(phase.Outcomes, phase.Outputs)
		r.Checks = append(r.Checks, Check{"protection was active before capture with continuous initial output", valid && r.InitialOutcome != "inconclusive"})
	}
	completed := r.finishEvaluation(r.Phases[1].Outputs)
	if !completed && r.InitialProtection != nil {
		r.InitialOutcome = "inconclusive"
	}
	return completed
}

func evaluateDisplayPhase(transition CaptureTransition, observations []displayObservation, start, end int64, visibleTail bool) (LivePhase, bool) {
	phase := LivePhase{Stage: transition.Stage, Outputs: len(observations), Outcomes: map[string]int{}}
	for _, observation := range observations {
		if observation.output.Status == "complete" {
			phase.Frames++
		} else {
			phase.IdleOutputs++
		}
		recordLiveOutcome(&phase, liveFrameOutcome(observation.frame), observation.output.ReceivedNanos-transition.AppliedNanos)
	}
	valid := end-start >= LivePhaseMillis*1_000_000 && len(observations) >= 3
	if len(observations) < 3 {
		return phase, false
	}
	first, last := observations[0].output.ReceivedNanos, observations[len(observations)-1].output.ReceivedNanos
	valid = valid && last-first >= 1_500_000_000 && first-start <= maxDisplayOutputGapNanos && end-last <= maxDisplayOutputGapNanos
	for j := 1; j < len(observations); j++ {
		valid = valid && observations[j].output.ReceivedNanos-observations[j-1].output.ReceivedNanos <= maxDisplayOutputGapNanos
	}
	if visibleTail {
		for _, observation := range observations[len(observations)-3:] {
			valid = valid && liveFrameOutcome(observation.frame) == "visible"
		}
	}
	return phase, valid
}

// Both steady protection and policy-toggle diagnostics use the same raw trace
// validation. Every idle notification must reference an actual retained image.
func observeDisplayStream(frames []CaptureFrame, outputs []CaptureOutput, outputCount int, statuses map[string]int, stopped, drained int64) ([]displayObservation, bool, bool) {
	framesValid := len(frames) > 0
	var lastPresentation, lastDisplay, lastReceipt int64
	for _, frame := range frames {
		ordered := frame.PresentationNanos > lastPresentation && frame.DisplayNanos > lastDisplay && frame.ReceivedNanos >= lastReceipt
		framesValid = framesValid && validLiveFramePixels(frame) && ordered
		if frame.BlueRatio >= .98 && frame.RawColorRatio != nil {
			framesValid = framesValid && *frame.RawColorRatio >= frame.BlueRatio
		}
		lastPresentation, lastDisplay, lastReceipt = frame.PresentationNanos, frame.DisplayNanos, frame.ReceivedNanos
	}
	traceValid := len(outputs) == outputCount && drained >= stopped && validFrameStatuses(statuses, outputCount, len(frames))
	counts := map[string]int{}
	nextFrame, retained := 0, -1
	lastReceipt = 0
	var observations []displayObservation
	for _, output := range outputs {
		counts[output.Status]++
		traceValid = traceValid && output.ReceivedNanos >= lastReceipt && output.ReceivedNanos > 0 && output.ReceivedNanos <= drained
		lastReceipt = output.ReceivedNanos
		switch output.Status {
		case "complete":
			traceValid = traceValid && output.FrameIndex == nextFrame
			nextFrame++
			retained = output.FrameIndex
		case "idle":
			traceValid = traceValid && retained >= 0 && output.FrameIndex == retained
		default:
			traceValid = false
			continue
		}
		if output.FrameIndex < 0 || output.FrameIndex >= len(frames) {
			traceValid = false
			continue
		}
		frame := frames[output.FrameIndex]
		if output.Status == "complete" {
			traceValid = traceValid && output.ReceivedNanos == frame.ReceivedNanos && output.PresentationNanos == frame.PresentationNanos && output.DisplayNanos == frame.DisplayNanos
		}
		observations = append(observations, displayObservation{output, frame})
	}
	traceValid = traceValid && nextFrame == len(frames)
	for status, count := range statuses {
		traceValid = traceValid && counts[status] == count
	}
	return observations, framesValid, traceValid
}
