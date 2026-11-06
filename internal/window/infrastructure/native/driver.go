package native

import (
	"Q-Solver/internal/window/domain"
	"Q-Solver/pkg/platform"
	"fmt"
	"sync"
)

type Driver struct {
	mu      sync.RWMutex
	handles map[domain.ID]platform.WindowHandle
}

func New() *Driver { return &Driver{handles: map[domain.ID]platform.WindowHandle{}} }
func (d *Driver) Bind(id domain.ID, handle platform.WindowHandle) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if id == "" || handle == 0 {
		return fmt.Errorf("invalid window binding")
	}
	if _, exists := d.handles[id]; exists {
		return fmt.Errorf("window %s is already bound", id)
	}
	d.handles[id] = handle
	return nil
}
func (d *Driver) Unbind(id domain.ID) { d.mu.Lock(); delete(d.handles, id); d.mu.Unlock() }
func (d *Driver) handle(id domain.ID) (platform.WindowHandle, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	h := d.handles[id]
	if h == 0 {
		return 0, fmt.Errorf("window %s is not registered", id)
	}
	return h, nil
}
func (d *Driver) SetCaptureProtection(id domain.ID, value bool) (domain.CaptureResult, error) {
	h, err := d.handle(id)
	if err != nil {
		return domain.CaptureResult{}, err
	}
	method, err := platform.ApplyCaptureProtection(h, value)
	return domain.CaptureResult{Applied: value && err == nil, Mechanism: method, Limitation: "系统属性已设置；实际共享与录屏效果需按捕获路径验证"}, err
}
func (d *Driver) SetInputMode(id domain.ID, mode domain.InputMode) error {
	h, err := d.handle(id)
	if err != nil {
		return err
	}
	if mode == domain.Editing {
		return platform.RestoreFocus(h)
	}
	return platform.RemoveFocus(h)
}
func (d *Driver) SetClickThrough(id domain.ID, value bool) error {
	h, err := d.handle(id)
	if err != nil {
		return err
	}
	return platform.SetClickThrough(h, value)
}
func (d *Driver) SetVisible(id domain.ID, value bool) error {
	h, err := d.handle(id)
	if err != nil {
		return err
	}
	return platform.SetVisibleWithoutActivation(h, value)
}
func (d *Driver) SetAlwaysOnTop(id domain.ID, value bool) error {
	h, err := d.handle(id)
	if err != nil {
		return err
	}
	level := platform.WindowLevelNormal
	if value {
		level = platform.WindowLevelFloating
	}
	return platform.SetWindowLevel(h, level)
}
func (d *Driver) SetCaptureSuppressed(id domain.ID, value bool) error {
	h, err := d.handle(id)
	if err != nil {
		return err
	}
	return platform.SetCaptureSuppressed(h, value)
}
