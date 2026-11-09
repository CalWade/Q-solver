package application

import (
	"Q-Solver/internal/input/domain"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeDriver struct {
	callbacks Callbacks
	plan      NativePlan
	filter    bool
	register  bool
	failApply bool
	stopped   bool
}

func (d *fakeDriver) Start(c Callbacks) (Capabilities, error) {
	d.callbacks = c
	return Capabilities{Filtering: d.filter, Backend: "test", Reason: "permission missing"}, nil
}
func (d *fakeDriver) CanRegister(domain.Binding) bool { return d.register }
func (d *fakeDriver) Apply(p NativePlan) error {
	if d.failApply {
		return errors.New("registration failed")
	}
	d.plan = p
	return nil
}
func (d *fakeDriver) Stop() error { d.stopped = true; return nil }

type commands chan string

func (c commands) Execute(action string) { c <- action }

func profile() domain.Policy {
	return domain.Policy{Modifier: domain.Control, Protection: domain.Protection{Enabled: true, BlockLeft: true}, Bindings: []domain.Binding{{Action: "capture", Keys: []domain.Key{domain.Control, "KeyA"}}}}
}
func start(t *testing.T, d *fakeDriver, p domain.Policy) (*Service, commands) {
	t.Helper()
	c := make(commands, 20)
	s := NewService(d, c, nil)
	if err := s.Start(p); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})
	return s, c
}

func TestServiceProtectionDispatchAndRelease(t *testing.T) {
	d := &fakeDriver{filter: true, register: true}
	s, c := start(t, d, profile())
	if len(d.plan.Bindings) != 0 {
		t.Fatal("protected binding also registered as a generic system hotkey")
	}
	for _, e := range []domain.Event{{Key: "ControlLeft", Down: true}, {Key: "KeyA", Down: true}, {Key: "ControlLeft"}, {Key: "KeyA"}} {
		if !d.callbacks.Event(e).Suppress {
			t.Fatalf("event leaked: %+v", e)
		}
	}
	select {
	case a := <-c:
		if a != "capture" {
			t.Fatal(a)
		}
	case <-time.After(time.Second):
		t.Fatal("action not dispatched")
	}
	if s.Status().State != "active" {
		t.Fatal(s.Status())
	}
}

func TestConfigurationRollbackOnSaveFailure(t *testing.T) {
	p := profile()
	p.Protection.Enabled = false
	d := &fakeDriver{filter: true, register: true}
	s, _ := start(t, d, p)
	previous := d.plan.Revision
	p.Bindings[0].Keys = []domain.Key{domain.Control, "KeyB"}
	if err := s.Configure(p, func() error { return errors.New("disk full") }); err == nil {
		t.Fatal("save failure ignored")
	}
	if d.plan.Revision != previous || s.Status().Revision != previous {
		t.Fatal("failed save changed active revision")
	}
	if d.plan.Bindings[0].Keys[1] != "KeyA" {
		t.Fatal("old registration not restored")
	}
}

func TestNativeFailureDoesNotPersist(t *testing.T) {
	d := &fakeDriver{filter: true, register: true}
	s, _ := start(t, d, profile())
	d.failApply = true
	called := false
	if err := s.Configure(profile(), func() error { called = true; return nil }); err == nil {
		t.Fatal("driver failure ignored")
	}
	if called {
		t.Fatal("persisted a failed native update")
	}
}

func TestEditingSuspendsAndStaleEditorCannotDisableOtherApps(t *testing.T) {
	d := &fakeDriver{filter: true}
	s, _ := start(t, d, profile())
	if err := s.SetEditing(true); err != nil {
		t.Fatal(err)
	}
	for _, e := range []domain.Event{{Key: "ControlLeft", Down: true, OwnWindow: true}, {Key: "KeyA", Down: true, OwnWindow: true}, {Key: "KeyA", OwnWindow: true}, {Key: "ControlLeft", OwnWindow: true}} {
		if d.callbacks.Event(e).Suppress {
			t.Fatal("editing was intercepted")
		}
	}
	if !d.callbacks.Event(domain.Event{Key: "ControlLeft", Down: true, OwnWindow: false}).Suppress {
		t.Fatal("stale editing flag disabled protection outside app")
	}
	d.callbacks.Event(domain.Event{Key: "ControlLeft"})
	if err := s.SetEditing(false); err != nil {
		t.Fatal(err)
	}
	if s.Status().State != "active" {
		t.Fatal(s.Status())
	}
}

func TestNativeShortcutsUseRevisionAndPause(t *testing.T) {
	p := profile()
	p.Protection.Enabled = false
	d := &fakeDriver{filter: false, register: true}
	s, c := start(t, d, p)
	old := d.plan.Revision
	if err := s.SetEditing(true); err != nil {
		t.Fatal(err)
	}
	if len(d.plan.Bindings) != 0 {
		t.Fatal("editing retained native hotkeys")
	}
	d.callbacks.Shortcut(old, "capture")
	if err := s.SetEditing(false); err != nil {
		t.Fatal(err)
	}
	d.callbacks.Shortcut(d.plan.Revision, "capture")
	select {
	case a := <-c:
		if a != "capture" {
			t.Fatal(a)
		}
	case <-time.After(time.Second):
		t.Fatal("native shortcut lost")
	}
	select {
	case <-c:
		t.Fatal("stale callback dispatched")
	default:
	}
}

func TestMissingFilterPermissionIsNotReportedActive(t *testing.T) {
	d := &fakeDriver{filter: false, register: true}
	s := NewService(d, nil, nil)
	if s.Start(profile()) == nil {
		t.Fatal("protection accepted without filter permission")
	}
	defer s.Stop()
	if s.Status().State != "unavailable" {
		t.Fatal(s.Status())
	}
}

func TestConcurrentInputAndConfiguration(t *testing.T) {
	d := &fakeDriver{filter: true}
	s, _ := start(t, d, profile())
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			d.callbacks.Event(domain.Event{Key: "KeyZ", Down: true})
			d.callbacks.Event(domain.Event{Key: "KeyZ"})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			if err := s.Configure(profile(), nil); err != nil {
				t.Error(err)
			}
			s.Status()
		}
	}()
	wg.Wait()
}

func TestListenerFailureNeedsRestartAndKeepsEditing(t *testing.T) {
	d := &fakeDriver{filter: true, register: true}
	s, _ := start(t, d, profile())
	d.callbacks.Failed(errors.New("tap interrupted"))
	if s.Status().Capabilities.Filtering || s.Status().State != "unavailable" {
		t.Fatal(s.Status())
	}
	if err := s.SetEditing(true); err != nil {
		t.Fatal(err)
	}
	if err := s.Configure(profile(), nil); err != nil {
		t.Fatal(err)
	}
	if s.Status().State != "unavailable" {
		t.Fatal("saving hid the driver failure", s.Status())
	}
	if err := s.Restart(profile()); err != nil {
		t.Fatal(err)
	}
	if s.Status().State != "editing" {
		t.Fatal("restart lost editing intent", s.Status())
	}
	if err := s.SetEditing(false); err != nil {
		t.Fatal(err)
	}
	if s.Status().State != "active" {
		t.Fatal(s.Status())
	}
}

func TestStoppedStatusWinsOverPreviousFailure(t *testing.T) {
	d := &fakeDriver{filter: true, register: true}
	s, _ := start(t, d, profile())
	d.callbacks.Failed(errors.New("tap interrupted"))
	s.Stop()
	if s.Status().State != "stopped" {
		t.Fatal(s.Status())
	}
}
