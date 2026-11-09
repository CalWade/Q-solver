package probe

type CaptureSample struct {
	Stage            string  `json:"stage"`
	PropertySet      bool    `json:"propertySet"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	RedRatio         float64 `json:"redRatio"`
	GreenRatio       float64 `json:"greenRatio"`
	BlackRatio       float64 `json:"opaqueBlackRatio"`
	TransparentRatio float64 `json:"transparentRatio"`
	ErrorCode        int     `json:"errorCode,omitempty"`
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
	r.Checks = nil
	expected := []string{"baseline-before", "sharing-none", "baseline-after"}
	complete := len(r.Samples) == len(expected)
	for i, s := range r.Samples {
		valid := i < len(expected) && s.Stage == expected[i] && s.Width > 0 && s.Height > 0 && s.ErrorCode == 0 && s.RedRatio >= 0 && s.RedRatio <= 1 && s.GreenRatio >= 0 && s.GreenRatio <= 1
		valid = valid && s.PropertySet == (i == 1)
		valid = valid && s.BlackRatio >= 0 && s.BlackRatio <= 1 && s.TransparentRatio >= 0 && s.TransparentRatio <= 1
		if i != 1 {
			valid = valid && s.RedRatio >= .15 && s.GreenRatio >= .15
		}
		r.Checks = append(r.Checks, Check{s.Stage, valid})
		complete = complete && valid
	}
	if complete {
		s := r.Samples[1]
		switch {
		case s.RedRatio >= .15 && s.GreenRatio >= .15:
			r.Outcome = "visible"
		case s.RedRatio <= .01 && s.GreenRatio <= .01:
			r.Outcome = "marker_absent"
			if s.BlackRatio >= .98 {
				r.Outcome = "blank"
			}
			if s.TransparentRatio >= .98 {
				r.Outcome = "transparent"
			}
		}
	}
	r.Completed = complete && r.Outcome != "inconclusive"
	r.Checks = append(r.Checks, Check{"valid baseline/protected/baseline comparison", r.Completed})
	return r.Completed
}
