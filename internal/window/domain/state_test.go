package domain

import "testing"

func TestEffectivePolicyDoesNotChangeUserPreferences(t *testing.T) {
	p := DefaultPolicy()
	p.CaptureProtection = false
	p.ClickThrough = true
	p.InputMode = Editing
	effective := p.Effective(1)
	if !effective.CaptureProtection || effective.ClickThrough || !p.ClickThrough || p.CaptureProtection {
		t.Fatal(p, effective)
	}
	if p.Validate() != nil {
		t.Fatal("valid editing mode rejected")
	}
	p.InputMode = "unknown"
	if p.Validate() == nil {
		t.Fatal("invalid input mode accepted")
	}
}
