package state

import (
	"Q-Solver/internal/window/application"
	"Q-Solver/internal/window/domain"
	"Q-Solver/internal/window/infrastructure/native"
	"Q-Solver/pkg/logger"
	"Q-Solver/pkg/platform"
	"context"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"sync"
	"time"
)

type InitStatus string

const (
	StatusInitializing InitStatus = "initializing"
	StatusLoadingModel InitStatus = "loading-model"
	StatusReady        InitStatus = "ready"
	StatusError        InitStatus = "error"
)

type WindowState = domain.Status

// StateManager is a Wails adapter. Window rules and capture leases live in the
// application service, which knows nothing about Wails or native handles.
type StateManager struct {
	ctx          context.Context
	windows      *application.Service
	driver       *native.Driver
	windowMu     sync.RWMutex
	hwnd         platform.WindowHandle
	initStatus   InitStatus
	initStatusMu sync.RWMutex
	emitEvent    func(string, ...interface{})
}

func NewStateManager() *StateManager {
	driver := native.New()
	return &StateManager{driver: driver, windows: application.New(driver), initStatus: StatusInitializing}
}
func (sm *StateManager) Startup(ctx context.Context, emit func(string, ...interface{})) {
	sm.ctx, sm.emitEvent = ctx, emit
	for attempt := 0; attempt < 10; attempt++ {
		hwnd, err := platform.GetWindowHandle()
		if err == nil && hwnd != 0 {
			if err = platform.ApplyGhostMode(hwnd); err != nil {
				logger.Printf("窗口初始化失败: %v", err)
				sm.UpdateInitStatus(StatusError)
				return
			}
			if err = sm.RegisterWindow("main", hwnd); err != nil {
				logger.Printf("窗口策略未完全生效: %v", err)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	logger.Println("窗口句柄未就绪")
	sm.UpdateInitStatus(StatusError)
}
func (sm *StateManager) RegisterWindow(id domain.ID, handle platform.WindowHandle) error {
	if err := sm.driver.Bind(id, handle); err != nil {
		return err
	}
	if id == "main" {
		sm.windowMu.Lock()
		sm.hwnd = handle
		sm.windowMu.Unlock()
	}
	err := sm.windows.Register(id)
	sm.publish(err)
	return err
}
func (sm *StateManager) UnregisterWindow(id domain.ID) {
	sm.windows.Unregister(id)
	sm.driver.Unbind(id)
	if id == "main" {
		sm.windowMu.Lock()
		sm.hwnd = 0
		sm.windowMu.Unlock()
	}
	sm.publish(nil)
}
func (sm *StateManager) GetHwnd() platform.WindowHandle {
	sm.windowMu.RLock()
	defer sm.windowMu.RUnlock()
	return sm.hwnd
}
func (sm *StateManager) GetWindowState() WindowState { return sm.windows.Status() }
func (sm *StateManager) GetInitStatus() InitStatus {
	sm.initStatusMu.RLock()
	defer sm.initStatusMu.RUnlock()
	return sm.initStatus
}
func (sm *StateManager) GetInitStatusString() string { return string(sm.GetInitStatus()) }
func (sm *StateManager) UpdateInitStatus(status InitStatus) {
	sm.initStatusMu.Lock()
	sm.initStatus = status
	sm.initStatusMu.Unlock()
	if sm.emitEvent != nil {
		sm.emitEvent("init-status", string(status))
	}
}
func (sm *StateManager) IsReady() bool { return sm.GetInitStatus() == StatusReady }
func (sm *StateManager) IsVisible() bool {
	for _, w := range sm.windows.Status().Windows {
		if w.ID == "main" {
			return w.Applied.Visible
		}
	}
	return false
}
func (sm *StateManager) IsClickThrough() bool {
	for _, w := range sm.windows.Status().Windows {
		if w.ID == "main" {
			return w.Applied.ClickThrough
		}
	}
	return false
}
func (sm *StateManager) publish(err error) {
	if sm.emitEvent == nil {
		return
	}
	sm.emitEvent("window-state", sm.windows.Status())
	if err != nil {
		sm.emitEvent("toast", "窗口策略未生效: "+err.Error())
	}
}
func (sm *StateManager) change(update func(*domain.Policy)) error {
	err := sm.windows.Update(update)
	sm.publish(err)
	return err
}

// ToggleVisibility is retained for existing bindings; its historic action was
// capture protection. Local visibility now has an independent explicit method.
func (sm *StateManager) ToggleVisibility() bool {
	err := sm.change(func(p *domain.Policy) { p.CaptureProtection = !p.CaptureProtection })
	status := sm.windows.Status()
	if err == nil && sm.emitEvent != nil {
		sm.emitEvent("capture-protection-state", status.Desired.CaptureProtection)
	}
	return status.Desired.CaptureProtection
}
func (sm *StateManager) SetCaptureProtection(value bool) error {
	return sm.change(func(p *domain.Policy) { p.CaptureProtection = value })
}
func (sm *StateManager) SetVisible(value bool) error {
	return sm.change(func(p *domain.Policy) { p.Visible = value })
}
func (sm *StateManager) ToggleClickThrough() bool {
	err := sm.change(func(p *domain.Policy) { p.ClickThrough = !p.ClickThrough })
	value := sm.IsClickThrough()
	if err == nil && sm.emitEvent != nil {
		sm.emitEvent("click-through-state", value)
	}
	return value
}
func (sm *StateManager) RestoreFocus() error {
	return sm.change(func(p *domain.Policy) { p.InputMode = domain.Editing })
}
func (sm *StateManager) RemoveFocus() error {
	return sm.change(func(p *domain.Policy) { p.InputMode = domain.Passive })
}
func (sm *StateManager) SetAlwaysOnTop(value bool) error {
	return sm.change(func(p *domain.Policy) { p.AlwaysOnTop = value })
}
func (sm *StateManager) RetryWindowPolicy() error {
	err := sm.windows.Retry()
	sm.publish(err)
	return err
}
func (sm *StateManager) BeginCapture() (func() error, error) {
	release, err := sm.windows.BeginCapture()
	sm.publish(err)
	if err != nil {
		return nil, err
	}
	return func() error { err := release(); sm.publish(err); return err }, nil
}
func (sm *StateManager) MoveWindow(dx, dy int) {
	if sm.ctx == nil {
		return
	}
	x, y := runtime.WindowGetPosition(sm.ctx)
	runtime.WindowSetPosition(sm.ctx, x+dx, y+dy)
}
