package application

import (
	"Q-Solver/internal/window/domain"
	"errors"
	"fmt"
	"sort"
	"sync"
)

type Driver interface {
	SetCaptureProtection(domain.ID, bool) (domain.CaptureResult, error)
	SetClickThrough(domain.ID, bool) error
	SetInputMode(domain.ID, domain.InputMode) error
	SetVisible(domain.ID, bool) error
	SetAlwaysOnTop(domain.ID, bool) error
	SetCaptureSuppressed(domain.ID, bool) error
}
type Service struct {
	mu       sync.Mutex
	driver   Driver
	desired  domain.Policy
	windows  map[domain.ID]domain.Window
	captures int
}

func New(driver Driver) *Service {
	return &Service{driver: driver, desired: domain.DefaultPolicy(), windows: map[domain.ID]domain.Window{}}
}

// Register applies policy before making a new/recreated window visible. The
// native host must create it hidden and must unregister it before destruction.
func (s *Service) Register(id domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		return errors.New("window id is empty")
	}
	if _, exists := s.windows[id]; exists {
		return fmt.Errorf("window %s already registered", id)
	}
	s.windows[id] = domain.Window{ID: id}
	return s.apply(id)
}
func (s *Service) Unregister(id domain.ID) { s.mu.Lock(); defer s.mu.Unlock(); delete(s.windows, id) }
func (s *Service) Update(change func(*domain.Policy)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	desired := s.desired
	change(&desired)
	if err := desired.Validate(); err != nil {
		return err
	}
	s.desired = desired
	return s.applyAll()
}
func (s *Service) Retry() error { s.mu.Lock(); defer s.mu.Unlock(); return s.applyAll() }
func (s *Service) Status() domain.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := domain.Status{Desired: s.desired, Captures: s.captures, Windows: make([]domain.Window, 0, len(s.windows))}
	for _, window := range s.windows {
		result.Windows = append(result.Windows, window)
	}
	sort.Slice(result.Windows, func(i, j int) bool { return result.Windows[i].ID < result.Windows[j].ID })
	return result
}
func (s *Service) applyAll() error {
	var failures []error
	for id := range s.windows {
		if err := s.apply(id); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", id, err))
		}
	}
	return errors.Join(failures...)
}
func (s *Service) apply(id domain.ID) (err error) {
	w := s.windows[id]
	next := s.desired.Effective(s.captures)
	defer func() {
		if err != nil {
			w.Error = err.Error()
			w.Ready = false
		} else {
			w.Error = ""
			w.Ready = true
		}
		s.windows[id] = w
	}()
	// Failed effects are retried in full; a successful field is acknowledged only
	// after the driver completed it. In particular no optimistic "hidden" flag.
	if !next.Visible && (w.Applied.Visible || !w.Ready) {
		if err = s.driver.SetVisible(id, false); err != nil {
			return
		}
		w.Applied.Visible = false
	}
	if !w.Ready || next.CaptureProtection != w.Applied.CaptureProtection {
		var result domain.CaptureResult
		result, err = s.driver.SetCaptureProtection(id, next.CaptureProtection)
		if err != nil {
			return
		}
		w.Capture = result
		w.Applied.CaptureProtection = next.CaptureProtection
	}
	// Retrying an unrelated property must not reacquire keyboard focus.
	if next.InputMode != w.Applied.InputMode {
		if err = s.driver.SetInputMode(id, next.InputMode); err != nil {
			return
		}
		w.Applied.InputMode = next.InputMode
	}
	if !w.Ready || next.ClickThrough != w.Applied.ClickThrough {
		if err = s.driver.SetClickThrough(id, next.ClickThrough); err != nil {
			return
		}
		w.Applied.ClickThrough = next.ClickThrough
	}
	if !w.Ready || next.AlwaysOnTop != w.Applied.AlwaysOnTop {
		if err = s.driver.SetAlwaysOnTop(id, next.AlwaysOnTop); err != nil {
			return
		}
		w.Applied.AlwaysOnTop = next.AlwaysOnTop
	}
	suppressed := s.captures > 0
	if !w.Ready || suppressed != w.CaptureSuppressed {
		if err = s.driver.SetCaptureSuppressed(id, suppressed); err != nil {
			return
		}
		w.CaptureSuppressed = suppressed
	}
	if next.Visible && !w.Applied.Visible {
		if err = s.driver.SetVisible(id, true); err != nil {
			return
		}
		w.Applied.Visible = true
	}
	return
}

// BeginCapture owns a reference-counted temporary suppression of every managed
// window. Closing a lease restores the latest policy, never a stale snapshot.
func (s *Service) BeginCapture() (func() error, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.windows) == 0 {
		return nil, errors.New("no managed window is ready for capture")
	}
	s.captures++
	if err := s.applyAll(); err != nil {
		s.captures--
		restore := s.applyAll()
		return nil, errors.Join(err, restore)
	}
	var once sync.Once
	var releaseError error
	return func() error {
		once.Do(func() { s.mu.Lock(); defer s.mu.Unlock(); s.captures--; releaseError = s.applyAll() })
		return releaseError
	}, nil
}
