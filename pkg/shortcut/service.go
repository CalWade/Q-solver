package shortcut

import (
	"Q-Solver/internal/input/application"
	"Q-Solver/internal/input/domain"
	"Q-Solver/internal/input/infrastructure/native"
	"runtime"
)

// Service adapts existing application commands to the input bounded context.
// It owns neither native key state nor a second mutable shortcut map.
type Service struct {
	input    *application.Service
	delegate ServiceDelegate
}

func NewService(delegate ServiceDelegate) *Service {
	return NewServiceWithDriver(delegate, native.New())
}

// NewServiceWithDriver lets integration tests replace only OS effects.
func NewServiceWithDriver(delegate ServiceDelegate, driver application.Driver) *Service {
	s := &Service{delegate: delegate}
	s.input = application.NewService(driver, s, s)
	return s
}

func (s *Service) Start(bindings map[string]KeyBinding, protection domain.Protection) error {
	p, err := Profile(bindings, protection, runtime.GOOS)
	if err != nil {
		return err
	}
	return s.input.Start(p)
}
func (s *Service) Configure(bindings map[string]KeyBinding, protection domain.Protection, commit func() error) error {
	p, err := Profile(bindings, protection, runtime.GOOS)
	if err != nil {
		return err
	}
	return s.input.Configure(p, commit)
}
func (s *Service) Stop() error { return s.input.Stop() }
func (s *Service) Restart(bindings map[string]KeyBinding, protection domain.Protection) error {
	p, err := Profile(bindings, protection, runtime.GOOS)
	if err != nil {
		return err
	}
	return s.input.Restart(p)
}
func (s *Service) SetEditing(editing bool) error { return s.input.SetEditing(editing) }
func (s *Service) Status() application.Status    { return s.input.Status() }
func (s *Service) StartRecording(action string)  { s.SetEditing(true) }
func (s *Service) StopRecording()                { s.SetEditing(false) }

func (s *Service) InputStatusChanged(status application.Status) {
	s.delegate.EmitEvent("keyboard-protection-status", status)
}
func (s *Service) Execute(action string) {
	switch action {
	case "screenshot":
		s.delegate.TriggerScreenshot()
	case "send":
		s.delegate.TriggerSend()
	case "delete":
		s.delegate.TriggerDeleteScreenshot()
	case "toggle":
		s.delegate.ToggleVisibility()
	case "clickthrough":
		s.delegate.ToggleClickThrough()
	case "move_up":
		s.delegate.MoveWindow(0, -10)
	case "move_down":
		s.delegate.MoveWindow(0, 10)
	case "move_left":
		s.delegate.MoveWindow(-10, 0)
	case "move_right":
		s.delegate.MoveWindow(10, 0)
	case "scroll_up":
		s.delegate.ScrollContent("up")
	case "scroll_down":
		s.delegate.ScrollContent("down")
	case "panic":
		s.delegate.PanicExit()
	}
}
