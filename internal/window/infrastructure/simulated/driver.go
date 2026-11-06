package simulated

import (
	"Q-Solver/internal/window/domain"
	"fmt"
	"sync"
)

type Driver struct {
	mu      sync.Mutex
	failure string
}

func (d *Driver) Fail(operation string) { d.mu.Lock(); d.failure = operation; d.mu.Unlock() }
func (d *Driver) check(operation string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.failure == operation {
		return fmt.Errorf("simulated %s failure", operation)
	}
	return nil
}
func (d *Driver) SetCaptureProtection(_ domain.ID, value bool) (domain.CaptureResult, error) {
	err := d.check("capture")
	return domain.CaptureResult{Applied: value && err == nil, Mechanism: "simulated"}, err
}
func (d *Driver) SetInputMode(domain.ID, domain.InputMode) error { return d.check("input") }
func (d *Driver) SetClickThrough(domain.ID, bool) error          { return d.check("mouse") }
func (d *Driver) SetVisible(domain.ID, bool) error               { return d.check("visible") }
func (d *Driver) SetAlwaysOnTop(domain.ID, bool) error           { return d.check("top") }
func (d *Driver) SetCaptureSuppressed(domain.ID, bool) error     { return d.check("suppression") }
