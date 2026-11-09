package probe

const ScreenshotAPI = "ScreenCaptureKit.SCScreenshotManager / desktopIndependentWindow"
const StreamAPI = "ScreenCaptureKit.SCStream / desktopIndependentWindow"

type CapturePixels struct {
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	RedRatio         float64 `json:"redRatio"`
	GreenRatio       float64 `json:"greenRatio"`
	BlackRatio       float64 `json:"opaqueBlackRatio"`
	TransparentRatio float64 `json:"transparentRatio"`
}

func (p CapturePixels) valid() bool {
	return p.Width > 0 && p.Height > 0 && p.RedRatio >= 0 && p.RedRatio <= 1 && p.GreenRatio >= 0 && p.GreenRatio <= 1 && p.BlackRatio >= 0 && p.BlackRatio <= 1 && p.TransparentRatio >= 0 && p.TransparentRatio <= 1
}

func (p CapturePixels) outcome() string {
	if p.RedRatio >= .15 && p.GreenRatio >= .15 {
		return "visible"
	}
	if p.RedRatio > .01 || p.GreenRatio > .01 {
		return "inconclusive"
	}
	if p.TransparentRatio >= .98 {
		return "transparent"
	}
	if p.BlackRatio >= .98 {
		return "blank"
	}
	return "marker_absent"
}

type CaptureFrame struct {
	CapturePixels
	PresentationNanos int64    `json:"presentationNanos"`
	DisplayNanos      int64    `json:"displayNanos,omitempty"`
	ReceivedNanos     int64    `json:"receivedNanos,omitempty"`
	RawColorRatio     *float64 `json:"rawColorRatio,omitempty"`
	File              string   `json:"file"`
}

type StreamObservation struct {
	DurationMillis int            `json:"durationMillis"`
	OutputCount    int            `json:"outputCount"`
	FrameStatuses  map[string]int `json:"frameStatuses"`
	Frames         []CaptureFrame `json:"frames"`
}

type CaptureSample struct {
	CapturePixels
	Stage       string             `json:"stage"`
	PropertySet bool               `json:"propertySet"`
	ErrorCode   int                `json:"errorCode,omitempty"`
	Stream      *StreamObservation `json:"stream,omitempty"`
}

type CaptureReport struct {
	API       string          `json:"api"`
	Samples   []CaptureSample `json:"samples"`
	Completed bool            `json:"completed"`
	Outcome   string          `json:"protectedWindowOutcome"`
	Checks    []Check         `json:"checks"`
}

// Successful diagnostics can reveal that protection is ineffective. Completion
// and pixel outcome are separate: a missing/failed baseline never means hidden.
func (r *CaptureReport) Evaluate() bool {
	r.Outcome = "inconclusive"
	r.Checks = []Check{{"known capture API", r.API == ScreenshotAPI || r.API == StreamAPI}}
	expected := []string{"baseline-before", "sharing-none", "baseline-after"}
	complete := r.Checks[0].Passed && len(r.Samples) == len(expected)
	for i, s := range r.Samples {
		valid := i < len(expected) && s.Stage == expected[i] && s.CapturePixels.valid() && s.ErrorCode == 0
		valid = valid && s.PropertySet == (i == 1)
		if i != 1 {
			valid = valid && s.outcome() == "visible"
		}
		if r.API == StreamAPI {
			valid = valid && validStream(s)
		}
		r.Checks = append(r.Checks, Check{s.Stage, valid})
		complete = complete && valid
	}
	if complete {
		r.Outcome = r.Samples[1].outcome()
	}
	r.Completed = complete && r.Outcome != "inconclusive"
	r.Checks = append(r.Checks, Check{"valid baseline/protected/baseline comparison", r.Completed})
	return r.Completed
}

func validStream(s CaptureSample) bool {
	stream := s.Stream
	if stream == nil || stream.DurationMillis < 300 || len(stream.Frames) < 3 || stream.OutputCount < len(stream.Frames) {
		return false
	}
	if !validFrameStatuses(stream.FrameStatuses, stream.OutputCount, len(stream.Frames)) {
		return false
	}
	var previous int64
	for _, frame := range stream.Frames {
		if !frame.CapturePixels.valid() || frame.Width != s.Width || frame.Height != s.Height || frame.outcome() != s.outcome() || frame.PresentationNanos <= previous || frame.File == "" {
			return false
		}
		previous = frame.PresentationNanos
	}
	span := stream.Frames[len(stream.Frames)-1].PresentationNanos - stream.Frames[0].PresentationNanos
	return span >= 200_000_000 && s.CapturePixels == stream.Frames[len(stream.Frames)-1].CapturePixels
}

func validFrameStatuses(statuses map[string]int, outputs, frames int) bool {
	if statuses["complete"] != frames || statuses["blank"] != 0 || statuses["suspended"] != 0 || statuses["unknown"] != 0 {
		return false
	}
	total := 0
	for _, name := range []string{"complete", "idle", "blank", "suspended", "started", "stopped", "unknown"} {
		count, exists := statuses[name]
		if !exists || count < 0 {
			return false
		}
		total += count
	}
	return len(statuses) == 7 && total == outputs
}
