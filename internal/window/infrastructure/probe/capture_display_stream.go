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
	framesValid := true
	var lastPresentation, lastDisplay, lastReceipt int64
	for _, frame := range r.Frames {
		ordered := frame.PresentationNanos > lastPresentation && frame.DisplayNanos > lastDisplay && frame.ReceivedNanos >= lastReceipt
		framesValid = framesValid && validLiveFramePixels(frame) && ordered
		lastPresentation, lastDisplay, lastReceipt = frame.PresentationNanos, frame.DisplayNanos, frame.ReceivedNanos
	}
	r.Checks = append(r.Checks, Check{"complete images have valid pixels and ordered media timestamps", framesValid})
	groups := make([][]displayObservation, 3)
	traceValid := len(r.Outputs) == r.OutputCount && r.DrainedNanos >= r.StoppedNanos
	counts := map[string]int{}
	nextFrame, retained := 0, -1
	lastReceipt = 0
	for _, output := range r.Outputs {
		counts[output.Status]++
		traceValid = traceValid && output.ReceivedNanos >= lastReceipt && output.ReceivedNanos > 0 && output.ReceivedNanos <= r.DrainedNanos
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
		if output.FrameIndex < 0 || output.FrameIndex >= len(r.Frames) {
			traceValid = false
			continue
		}
		frame := r.Frames[output.FrameIndex]
		if output.Status == "complete" {
			traceValid = traceValid && output.ReceivedNanos == frame.ReceivedNanos && output.PresentationNanos == frame.PresentationNanos && output.DisplayNanos == frame.DisplayNanos
		}
		phase := r.phaseAt(output.ReceivedNanos)
		if phase < 0 {
			if output.Status == "complete" {
				r.StartupFrames++
			}
			continue
		}
		groups[phase] = append(groups[phase], displayObservation{output, frame})
	}
	traceValid = traceValid && nextFrame == len(r.Frames)
	for status, count := range r.FrameStatuses {
		traceValid = traceValid && counts[status] == count
	}
	r.Checks = append(r.Checks, Check{"every SDK output records its complete image or retained idle image", traceValid})
	for i, observations := range groups {
		phase := LivePhase{Stage: r.Transitions[i].Stage, Outputs: len(observations), Outcomes: map[string]int{}}
		for _, observation := range observations {
			if observation.output.Status == "complete" {
				phase.Frames++
			} else {
				phase.IdleOutputs++
			}
			recordLiveOutcome(&phase, liveFrameOutcome(observation.frame), observation.output.ReceivedNanos-r.Transitions[i].AppliedNanos)
		}
		r.Phases = append(r.Phases, phase)
		start, end := r.phaseInterval(i, r.DrainedNanos)
		valid := end-start >= LivePhaseMillis*1_000_000 && len(observations) >= 3
		if len(observations) >= 3 {
			first, last := observations[0].output.ReceivedNanos, observations[len(observations)-1].output.ReceivedNanos
			valid = valid && last-first >= 1_500_000_000 && first-start <= maxDisplayOutputGapNanos && end-last <= maxDisplayOutputGapNanos
			for j := 1; j < len(observations); j++ {
				valid = valid && observations[j].output.ReceivedNanos-observations[j-1].output.ReceivedNanos <= maxDisplayOutputGapNanos
			}
			if i != 1 {
				for _, observation := range observations[len(observations)-3:] {
					valid = valid && liveFrameOutcome(observation.frame) == "visible"
				}
			}
		}
		r.Checks = append(r.Checks, Check{phase.Stage + " has continuous observed output and valid controls", valid})
	}
	return r.finishEvaluation(r.Phases[1].Outputs)
}
