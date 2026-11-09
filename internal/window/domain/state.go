package domain

import "fmt"

type ID string
type InputMode string

const (
	Passive InputMode = "passive"
	Editing InputMode = "editing"
)

// Policy separates local appearance from capture policy and input ownership.
type Policy struct {
	Visible           bool      `json:"visible"`
	CaptureProtection bool      `json:"captureProtection"`
	ClickThrough      bool      `json:"clickThrough"`
	InputMode         InputMode `json:"inputMode"`
	AlwaysOnTop       bool      `json:"alwaysOnTop"`
}

func DefaultPolicy() Policy {
	return Policy{Visible: true, CaptureProtection: true, InputMode: Passive, AlwaysOnTop: true}
}
func (p Policy) Validate() error {
	if p.InputMode != Passive && p.InputMode != Editing {
		return fmt.Errorf("invalid input mode %q", p.InputMode)
	}
	return nil
}
func (p Policy) Effective(captures int) Policy {
	if captures > 0 {
		p.CaptureProtection = true
	}
	if p.InputMode == Editing {
		p.ClickThrough = false
	}
	return p
}

type CaptureResult struct {
	Applied    bool   `json:"applied"`
	Mechanism  string `json:"mechanism"`
	Limitation string `json:"limitation,omitempty"`
}
type Window struct {
	ID                ID            `json:"id"`
	Applied           Policy        `json:"applied"`
	Capture           CaptureResult `json:"capture"`
	CaptureSuppressed bool          `json:"captureSuppressed"`
	// An interrupted native mode change must be reconciled even if cancelled.
	InputModePending  bool   `json:"-"`
	VisibilityPending bool   `json:"-"`
	Ready             bool   `json:"ready"`
	Error             string `json:"error,omitempty"`
}
type Status struct {
	Desired  Policy   `json:"desired"`
	Windows  []Window `json:"windows"`
	Captures int      `json:"captures"`
}
