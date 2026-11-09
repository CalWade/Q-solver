package platform

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
)

// Win32 orchestration stays independent of the DLL adapter so its state and
// failure paths can be tested without opening a Windows desktop.
const (
	windowsStyleIndex         int32  = -16
	windowsExtendedStyleIndex int32  = -20
	windowsNoActivate         uint32 = 0x08000000
	windowsToolWindow         uint32 = 0x00000080
	windowsAppWindow          uint32 = 0x00040000
	windowsTransparent        uint32 = 0x00000020
	windowsLayered            uint32 = 0x00080000
	windowsTopmost            uint32 = 0x00000008
	windowsFrameFlags         uint32 = 0x0001 | 0x0002 | 0x0004 | 0x0010 | 0x0020
	windowsPositionFlags      uint32 = 0x0001 | 0x0002 | 0x0010
	windowsFrameDecoration    uint32 = 0x00C00000 | 0x00040000 | 0x00020000 | 0x00010000 | 0x00080000
)

type windowsWindowInfo struct {
	Handle    uintptr
	ProcessID uint32
	Visible   bool
	Minimized bool
}

type windowsWindowAPI interface {
	Window(uintptr) (windowsWindowInfo, error)
	Foreground() windowsWindowInfo
	ReadStyle(uintptr, int32) (uint32, error)
	WriteStyle(uintptr, int32, uint32) error
	Position(uintptr, uintptr, uint32) error
	Show(uintptr, bool) error
	SetAlpha(uintptr, byte) error
	Alpha(uintptr) (byte, error)
	SetAffinity(uintptr, uint32) error
	Affinity(uintptr) (uint32, error)
	Activate(windowsWindowInfo) error
}

type windowsEditingSession struct {
	previous windowsWindowInfo
	editing  bool
}

type windowsWindowController struct {
	mu       sync.Mutex
	api      windowsWindowAPI
	process  uint32
	sessions map[uintptr]*windowsEditingSession
}

func newWindowsWindowController(api windowsWindowAPI, process uint32) *windowsWindowController {
	return &windowsWindowController{api: api, process: process, sessions: map[uintptr]*windowsEditingSession{}}
}

func (c *windowsWindowController) owned(hwnd uintptr) (windowsWindowInfo, error) {
	window, err := c.api.Window(hwnd)
	if err != nil {
		return window, err
	}
	if window.Handle != hwnd || hwnd == 0 || window.ProcessID != c.process {
		return window, errors.New("窗口不属于当前进程")
	}
	return window, nil
}

func (c *windowsWindowController) changeStyle(hwnd uintptr, index int32, clear, add uint32) error {
	style, err := c.api.ReadStyle(hwnd, index)
	if err != nil {
		return err
	}
	next := style&^clear | add
	if next != style {
		if err := c.api.WriteStyle(hwnd, index, next); err != nil {
			return err
		}
	}
	if err := c.api.Position(hwnd, 0, windowsFrameFlags); err != nil {
		return err
	}
	actual, err := c.api.ReadStyle(hwnd, index)
	if err != nil {
		return err
	}
	if actual != next {
		return errors.New("窗口样式读回与请求不一致")
	}
	return nil
}

func (c *windowsWindowController) Ghost(hwnd uintptr) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := c.owned(hwnd); err != nil {
		return err
	}
	if err := c.changeStyle(hwnd, windowsStyleIndex, windowsFrameDecoration, 0); err != nil {
		return err
	}
	if err := c.changeStyle(hwnd, windowsExtendedStyleIndex, windowsAppWindow, windowsToolWindow|windowsNoActivate); err != nil {
		return err
	}
	return c.topmost(hwnd, true)
}

func (c *windowsWindowController) ToolWindow(hwnd uintptr) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := c.owned(hwnd); err != nil {
		return err
	}
	return c.changeStyle(hwnd, windowsExtendedStyleIndex, windowsAppWindow, windowsToolWindow)
}

func (c *windowsWindowController) Capture(hwnd uintptr, build uint32, enabled bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := c.owned(hwnd); err != nil {
		return "unavailable", err
	}
	// GetWindowDisplayAffinity requires a layered window. Initialise a newly
	// layered window before reading back protection, preserving existing alpha.
	if err := c.ensureLayered(hwnd); err != nil {
		return "unavailable", err
	}
	affinity, _ := captureAffinity(build, enabled)
	mode, err := c.capture(hwnd, affinity)
	if err != nil && affinity == 0x11 {
		fallback, fallbackErr := c.capture(hwnd, 1)
		if fallbackErr != nil {
			return "unavailable", errors.Join(err, fallbackErr)
		}
		return fallback, nil
	}
	return mode, err
}

func (c *windowsWindowController) capture(hwnd uintptr, requested uint32) (string, error) {
	if err := c.api.SetAffinity(hwnd, requested); err != nil {
		return "unavailable", err
	}
	actual, err := c.api.Affinity(hwnd)
	if err != nil {
		return "unavailable", err
	}
	switch {
	case requested == 0 && actual == 0:
		return "off", nil
	case requested == 0x11 && actual == 0x11:
		return "exclude-from-capture", nil
	case (requested == 1 || requested == 0x11) && actual == 1:
		return "blank-content", nil
	default:
		return "unavailable", fmt.Errorf("捕获属性读回不一致，请求%d，实际%d", requested, actual)
	}
}

func (c *windowsWindowController) ClickThrough(hwnd uintptr, enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := c.owned(hwnd); err != nil {
		return err
	}
	if enabled {
		if err := c.ensureLayered(hwnd); err != nil {
			return err
		}
		return c.changeStyle(hwnd, windowsExtendedStyleIndex, 0, windowsTransparent)
	}
	return c.changeStyle(hwnd, windowsExtendedStyleIndex, windowsTransparent, 0)
}

func (c *windowsWindowController) Suppress(hwnd uintptr, suppressed bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := c.owned(hwnd); err != nil {
		return err
	}
	if err := c.changeStyle(hwnd, windowsExtendedStyleIndex, 0, windowsLayered); err != nil {
		return err
	}
	alpha := byte(255)
	if suppressed {
		alpha = 0
	}
	return c.alpha(hwnd, alpha)
}

func (c *windowsWindowController) ensureLayered(hwnd uintptr) error {
	style, err := c.api.ReadStyle(hwnd, windowsExtendedStyleIndex)
	if err != nil {
		return err
	}
	if style&windowsLayered != 0 {
		return nil
	}
	if err := c.changeStyle(hwnd, windowsExtendedStyleIndex, 0, windowsLayered); err != nil {
		return err
	}
	return c.alpha(hwnd, 255)
}

func (c *windowsWindowController) alpha(hwnd uintptr, alpha byte) error {
	if err := c.api.SetAlpha(hwnd, alpha); err != nil {
		return err
	}
	actual, err := c.api.Alpha(hwnd)
	if err != nil {
		return err
	}
	if actual != alpha {
		return errors.New("窗口透明度读回与请求不一致")
	}
	return nil
}

func (c *windowsWindowController) Topmost(hwnd uintptr, enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := c.owned(hwnd); err != nil {
		return err
	}
	return c.topmost(hwnd, enabled)
}

func (c *windowsWindowController) topmost(hwnd uintptr, enabled bool) error {
	position := ^uintptr(1)
	if enabled {
		position = ^uintptr(0)
	}
	if err := c.api.Position(hwnd, position, windowsPositionFlags); err != nil {
		return err
	}
	style, err := c.api.ReadStyle(hwnd, windowsExtendedStyleIndex)
	if err != nil {
		return err
	}
	if (style&windowsTopmost != 0) != enabled {
		return errors.New("窗口置顶状态未生效")
	}
	return nil
}

func (c *windowsWindowController) EnterEditing(hwnd uintptr) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	window, err := c.owned(hwnd)
	if err != nil {
		return err
	}
	session := c.sessions[hwnd]
	if session != nil && session.editing {
		return c.changeStyle(hwnd, windowsExtendedStyleIndex, windowsNoActivate, 0)
	}
	previous := c.api.Foreground()
	if session == nil {
		session = &windowsEditingSession{}
	}
	if previous.Handle != 0 && previous.ProcessID != c.process {
		session.previous = previous
	}
	c.sessions[hwnd] = session
	if err := c.changeStyle(hwnd, windowsExtendedStyleIndex, windowsNoActivate, 0); err != nil {
		return errors.Join(err, c.exitEditing(hwnd))
	}
	if window.Visible {
		if err := c.api.Activate(window); err != nil {
			return errors.Join(err, c.exitEditing(hwnd))
		}
	}
	session.editing = true
	return nil
}

func (c *windowsWindowController) ExitEditing(hwnd uintptr) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := c.owned(hwnd); err != nil {
		return err
	}
	return c.exitEditing(hwnd)
}

func (c *windowsWindowController) exitEditing(hwnd uintptr) error {
	session := c.sessions[hwnd]
	if session != nil {
		session.editing = false
	}
	if err := c.changeStyle(hwnd, windowsExtendedStyleIndex, 0, windowsNoActivate); err != nil {
		return err
	}
	if err := c.returnFocus(session); err != nil {
		return err
	}
	delete(c.sessions, hwnd)
	return nil
}

func (c *windowsWindowController) returnFocus(session *windowsEditingSession) error {
	if session == nil || session.previous.Handle == 0 {
		return nil
	}
	previous, err := c.api.Window(session.previous.Handle)
	if err != nil || previous.ProcessID != session.previous.ProcessID || !previous.Visible || previous.Minimized {
		return nil
	}
	if c.api.Foreground().ProcessID != c.process {
		return nil
	}
	return c.api.Activate(previous)
}

func (c *windowsWindowController) Visible(hwnd uintptr, visible bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := c.owned(hwnd); err != nil {
		return err
	}
	var focusErr error
	if !visible {
		focusErr = c.returnFocus(c.sessions[hwnd])
	}
	if err := c.api.Show(hwnd, visible); err != nil {
		return errors.Join(focusErr, err)
	}
	actual, err := c.owned(hwnd)
	if err != nil {
		return errors.Join(focusErr, err)
	}
	if actual.Visible != visible {
		return errors.Join(focusErr, errors.New("窗口显示状态未生效"))
	}
	return focusErr
}

func (c *windowsWindowController) Release(hwnd uintptr) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	session := c.sessions[hwnd]
	defer delete(c.sessions, hwnd)
	if session == nil {
		return nil
	}
	if _, err := c.owned(hwnd); err != nil {
		return nil
	}
	return c.returnFocus(session)
}

func windowsStyleCallError(result uintptr, err error) error {
	if result == 0 && err != nil && !errors.Is(err, syscall.Errno(0)) {
		return err
	}
	return nil
}

func windowsCallFailure(operation string, err error) error {
	if err == nil || errors.Is(err, syscall.Errno(0)) {
		return fmt.Errorf("%s未生效", operation)
	}
	return fmt.Errorf("%s失败: %w", operation, err)
}
