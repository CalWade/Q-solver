package application

import (
	"Q-Solver/internal/window/domain"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

type fakeDriver struct {
	calls []string
	fail  string
}

func (d *fakeDriver) call(id domain.ID, action string, value any) error {
	d.calls = append(d.calls, fmt.Sprintf("%s:%s:%v", id, action, value))
	if d.fail == action {
		return errors.New("native failure")
	}
	return nil
}
func (d *fakeDriver) SetCaptureProtection(id domain.ID, v bool) (domain.CaptureResult, error) {
	return domain.CaptureResult{Applied: v, Mechanism: "test"}, d.call(id, "capture", v)
}
func (d *fakeDriver) SetInputMode(id domain.ID, v domain.InputMode) error {
	return d.call(id, "input", v)
}
func (d *fakeDriver) SetClickThrough(id domain.ID, v bool) error { return d.call(id, "mouse", v) }
func (d *fakeDriver) SetVisible(id domain.ID, v bool) error      { return d.call(id, "visible", v) }
func (d *fakeDriver) SetAlwaysOnTop(id domain.ID, v bool) error  { return d.call(id, "top", v) }
func (d *fakeDriver) SetCaptureSuppressed(id domain.ID, v bool) error {
	return d.call(id, "suppressed", v)
}

func TestRegisterProtectsBeforeFirstShowAndRecreatedWindows(t *testing.T) {
	d := &fakeDriver{}
	s := New(d)
	for _, id := range []domain.ID{"main", "secondary"} {
		if err := s.Register(id); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(d.calls[0], "capture:true") || d.calls[5] != "main:visible:true" {
		t.Fatal(d.calls)
	}
	if s.Register("main") == nil {
		t.Fatal("duplicate handle accepted")
	}
	s.Unregister("main")
	d.calls = nil
	if err := s.Register("main"); err != nil {
		t.Fatal(err)
	}
	if d.calls[0] != "main:capture:true" {
		t.Fatal(d.calls)
	}
}
func TestFailureDoesNotClaimSuccessOrShowUnprotectedWindow(t *testing.T) {
	d := &fakeDriver{fail: "capture"}
	s := New(d)
	if s.Register("main") == nil {
		t.Fatal("failure hidden")
	}
	status := s.Status().Windows[0]
	if status.Ready || status.Applied.Visible || status.Capture.Applied || status.Error == "" {
		t.Fatal(status)
	}
	for _, call := range d.calls {
		if strings.Contains(call, "visible:true") {
			t.Fatal("showed before policy")
		}
	}
	d.fail = ""
	if err := s.Retry(); err != nil {
		t.Fatal(err)
	}
	if !s.Status().Windows[0].Ready {
		t.Fatal(s.Status())
	}
}
func TestVisibilityCaptureAndEditingHaveIndependentState(t *testing.T) {
	d := &fakeDriver{}
	s := New(d)
	s.Register("main")
	s.Update(func(p *domain.Policy) { p.ClickThrough = true; p.CaptureProtection = false; p.Visible = false })
	s.Update(func(p *domain.Policy) { p.InputMode = domain.Editing })
	status := s.Status()
	if status.Windows[0].Applied.ClickThrough || !status.Desired.ClickThrough || status.Windows[0].Applied.CaptureProtection || status.Windows[0].Applied.Visible {
		t.Fatal(status)
	}
	s.Update(func(p *domain.Policy) { p.InputMode = domain.Passive })
	if !s.Status().Windows[0].Applied.ClickThrough {
		t.Fatal("editing lost mouse preference")
	}
}
func TestCaptureLeasesRestoreLatestPolicyAfterLastRelease(t *testing.T) {
	d := &fakeDriver{}
	s := New(d)
	s.Register("main")
	one, err := s.BeginCapture()
	if err != nil {
		t.Fatal(err)
	}
	two, err := s.BeginCapture()
	if err != nil {
		t.Fatal(err)
	}
	s.Update(func(p *domain.Policy) { p.CaptureProtection = false; p.Visible = false })
	if err := s.Register("late"); err != nil {
		t.Fatal(err)
	}
	one()
	one()
	for _, w := range s.Status().Windows {
		if !w.CaptureSuppressed || !w.Applied.CaptureProtection {
			t.Fatal(w)
		}
	}
	two()
	status := s.Status()
	if status.Captures != 0 {
		t.Fatal(status)
	}
	for _, w := range status.Windows {
		if w.CaptureSuppressed || w.Applied.CaptureProtection || w.Applied.Visible {
			t.Fatal(w)
		}
	}
}
func TestFailedCaptureAcquisitionRestoresSuppressionAndCount(t *testing.T) {
	d := &fakeDriver{}
	s := New(d)
	s.Register("main")
	d.fail = "suppressed"
	if release, err := s.BeginCapture(); err == nil || release != nil {
		t.Fatal("failed capture allowed")
	}
	if s.Status().Captures != 0 {
		t.Fatal(s.Status())
	}
	d.fail = ""
	if err := s.Retry(); err != nil {
		t.Fatal(err)
	}
	if s.Status().Windows[0].CaptureSuppressed {
		t.Fatal("suppression remained")
	}
}
func TestConcurrentLeasesAndPolicyUpdates(t *testing.T) {
	s := New(&fakeDriver{})
	s.Register("main")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := s.BeginCapture()
			if err != nil {
				t.Error(err)
				return
			}
			s.Update(func(p *domain.Policy) { p.CaptureProtection = !p.CaptureProtection })
			if err := release(); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if s.Status().Captures != 0 {
		t.Fatal(s.Status())
	}
}

func TestRetryingCaptureDoesNotReacquireEditorFocus(t *testing.T) {
	d := &fakeDriver{}
	s := New(d)
	s.Register("main")
	s.Update(func(p *domain.Policy) { p.InputMode = domain.Editing })
	d.fail = "suppressed"
	_, _ = s.BeginCapture()
	d.fail = ""
	d.calls = nil
	if err := s.Retry(); err != nil {
		t.Fatal(err)
	}
	for _, call := range d.calls {
		if strings.Contains(call, ":input:") {
			t.Fatal("retry requested focus", d.calls)
		}
	}
}
