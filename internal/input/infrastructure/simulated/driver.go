// Package simulated replaces OS effects for integration and browser tests.
// It never installs hooks, captures a screen, or sends events to other apps.
package simulated

import (
	"Q-Solver/internal/input/application"
	"Q-Solver/internal/input/domain"
	"Q-Solver/internal/input/infrastructure/keymap"
	"errors"
	"sync"
)

type Driver struct {
	mu        sync.Mutex
	callbacks application.Callbacks
	plan      application.NativePlan
	native    *domain.Engine
	running   bool
}

func (d *Driver) Start(c application.Callbacks) (application.Capabilities, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.callbacks, d.running = c, true
	return application.Capabilities{Filtering: true, Backend: "simulated-no-os-hooks"}, nil
}
func (d *Driver) CanRegister(b domain.Binding) bool { _, _, ok := keymap.SystemChord(b); return ok }
func (d *Driver) Apply(p application.NativePlan) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		return errors.New("simulated driver stopped")
	}
	engine, err := domain.NewEngine(domain.Policy{Modifier: domain.Control, Bindings: p.Bindings})
	if err != nil {
		return err
	}
	d.plan, d.native = p, engine
	return nil
}
func (d *Driver) Stop() error { d.mu.Lock(); d.running = false; d.mu.Unlock(); return nil }
func (d *Driver) Input(event domain.Event) domain.Decision {
	d.mu.Lock()
	callback, running := d.callbacks, d.running
	d.mu.Unlock()
	if !running {
		return domain.Decision{}
	}
	result := callback.Event(event)
	d.mu.Lock()
	var native domain.Decision
	revision := d.plan.Revision
	if !result.Suppress && d.native != nil {
		native = d.native.Process(event)
	}
	d.mu.Unlock()
	if native.Action != "" {
		callback.Shortcut(revision, native.Action)
	}
	return result
}
func (d *Driver) Fail(err error) {
	d.mu.Lock()
	callback := d.callbacks
	d.mu.Unlock()
	callback.Failed(err)
}
