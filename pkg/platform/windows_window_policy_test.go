package platform

import (
	windowapp "Q-Solver/internal/window/application"
	"Q-Solver/internal/window/domain"
	"errors"
	"fmt"
	"syscall"
	"testing"
)

var errWindowsAPI = errors.New("synthetic Win32 failure")

type fakeWindowsWindow struct {
	info     windowsWindowInfo
	styles   map[int32]uint32
	alpha    byte
	affinity uint32
}

type fakeWindowsAPI struct {
	windows             map[uintptr]*fakeWindowsWindow
	foreground          uintptr
	counts              map[string]int
	failAt              map[string]int
	noEffect            string
	downgrade           bool
	activateBeforeError bool
	activations         []uintptr
	positions           []uint32
}

func newFakeWindowsAPI() *fakeWindowsAPI {
	api := &fakeWindowsAPI{windows: map[uintptr]*fakeWindowsWindow{}, foreground: 2, counts: map[string]int{}, failAt: map[string]int{}}
	for handle, process := range map[uintptr]uint32{1: 100, 2: 200, 3: 300} {
		api.windows[handle] = &fakeWindowsWindow{info: windowsWindowInfo{Handle: handle, ProcessID: process, Visible: true},
			styles: map[int32]uint32{windowsStyleIndex: windowsFrameDecoration, windowsExtendedStyleIndex: windowsAppWindow | windowsNoActivate}, alpha: 255}
	}
	return api
}

func (api *fakeWindowsAPI) operation(name string) error {
	api.counts[name]++
	if api.failAt[name] == api.counts[name] {
		return errWindowsAPI
	}
	return nil
}
func (api *fakeWindowsAPI) Window(hwnd uintptr) (windowsWindowInfo, error) {
	if err := api.operation("window"); err != nil {
		return windowsWindowInfo{}, err
	}
	window := api.windows[hwnd]
	if window == nil {
		return windowsWindowInfo{}, errors.New("window closed")
	}
	return window.info, nil
}
func (api *fakeWindowsAPI) Foreground() windowsWindowInfo {
	window := api.windows[api.foreground]
	if window == nil {
		return windowsWindowInfo{}
	}
	return window.info
}
func (api *fakeWindowsAPI) ReadStyle(hwnd uintptr, index int32) (uint32, error) {
	return api.windows[hwnd].styles[index], api.operation("read-style")
}
func (api *fakeWindowsAPI) WriteStyle(hwnd uintptr, index int32, value uint32) error {
	if err := api.operation("write-style"); err != nil {
		return err
	}
	if api.noEffect != "style" {
		api.windows[hwnd].styles[index] = value
	}
	return nil
}
func (api *fakeWindowsAPI) Position(hwnd, after uintptr, flags uint32) error {
	api.positions = append(api.positions, flags)
	if err := api.operation("position"); err != nil {
		return err
	}
	if flags&0x4 == 0 && api.noEffect != "topmost" {
		style := api.windows[hwnd].styles[windowsExtendedStyleIndex]
		if after == ^uintptr(0) {
			style |= windowsTopmost
		} else if after == ^uintptr(1) {
			style &^= windowsTopmost
		}
		api.windows[hwnd].styles[windowsExtendedStyleIndex] = style
	}
	return nil
}
func (api *fakeWindowsAPI) Show(hwnd uintptr, visible bool) error {
	if err := api.operation("show"); err != nil {
		return err
	}
	if api.noEffect != "visibility" {
		api.windows[hwnd].info.Visible = visible
	}
	return nil
}
func (api *fakeWindowsAPI) SetAlpha(hwnd uintptr, alpha byte) error {
	if err := api.operation("set-alpha"); err != nil {
		return err
	}
	if api.noEffect != "alpha" {
		api.windows[hwnd].alpha = alpha
	}
	return nil
}
func (api *fakeWindowsAPI) Alpha(hwnd uintptr) (byte, error) {
	return api.windows[hwnd].alpha, api.operation("read-alpha")
}
func (api *fakeWindowsAPI) SetAffinity(hwnd uintptr, affinity uint32) error {
	if err := api.operation(fmt.Sprintf("set-affinity-%d", affinity)); err != nil {
		return err
	}
	if api.noEffect != "affinity" {
		if affinity == 0x11 && api.downgrade {
			affinity = 1
		}
		api.windows[hwnd].affinity = affinity
	}
	return nil
}
func (api *fakeWindowsAPI) Affinity(hwnd uintptr) (uint32, error) {
	if api.windows[hwnd].styles[windowsExtendedStyleIndex]&windowsLayered == 0 {
		return 0, errors.New("GetWindowDisplayAffinity requires a layered window")
	}
	return api.windows[hwnd].affinity, api.operation("read-affinity")
}
func (api *fakeWindowsAPI) Activate(window windowsWindowInfo) error {
	api.activations = append(api.activations, window.Handle)
	err := api.operation("activate")
	if err == nil || api.activateBeforeError {
		api.foreground = window.Handle
	}
	return err
}

func TestWindowsWindowModesUseReadbackAndNeverImplicitlyActivate(t *testing.T) {
	api := newFakeWindowsAPI()
	c := newWindowsWindowController(api, 100)
	if err := c.ToolWindow(1); err != nil {
		t.Fatal(err)
	}
	if err := c.Ghost(1); err != nil {
		t.Fatal(err)
	}
	style := api.windows[1].styles
	if style[windowsStyleIndex]&windowsFrameDecoration != 0 || style[windowsExtendedStyleIndex]&windowsAppWindow != 0 || style[windowsExtendedStyleIndex]&(windowsToolWindow|windowsNoActivate|windowsTopmost) != windowsToolWindow|windowsNoActivate|windowsTopmost {
		t.Fatal(style)
	}
	for _, enabled := range []bool{true, false} {
		if err := c.ClickThrough(1, enabled); err != nil {
			t.Fatal(err)
		}
		if (style[windowsExtendedStyleIndex]&windowsTransparent != 0) != enabled {
			t.Fatal(style)
		}
		if err := c.Topmost(1, enabled); err != nil {
			t.Fatal(err)
		}
		if err := c.Visible(1, enabled); err != nil {
			t.Fatal(err)
		}
		if err := c.Suppress(1, enabled); err != nil {
			t.Fatal(err)
		}
	}
	if api.windows[1].alpha != 255 || api.foreground != 2 || len(api.activations) != 0 {
		t.Fatal(api)
	}
	for _, flags := range api.positions {
		if flags&0x10 == 0 {
			t.Fatal("style or visibility update could activate implicitly", flags)
		}
	}
}

func TestWindowsCaptureReportsActualAffinityAndFallback(t *testing.T) {
	for _, test := range []struct {
		name      string
		build     uint32
		enabled   bool
		downgrade bool
		failAt    map[string]int
		want      string
	}{
		{"new Windows", 26100, true, false, nil, "exclude-from-capture"},
		{"old Windows", 18363, true, false, nil, "blank-content"},
		{"unknown build", 0, true, false, nil, "blank-content"},
		{"explicit off", 26100, false, false, nil, "off"},
		{"OS downgrades exclusion", 26100, true, true, nil, "blank-content"},
		{"excluded call fails", 26100, true, false, map[string]int{"set-affinity-17": 1}, "blank-content"},
		{"excluded readback fails", 26100, true, false, map[string]int{"read-affinity": 1}, "blank-content"},
	} {
		t.Run(test.name, func(t *testing.T) {
			api := newFakeWindowsAPI()
			api.downgrade = test.downgrade
			if test.failAt != nil {
				api.failAt = test.failAt
			}
			mode, err := newWindowsWindowController(api, 100).Capture(1, test.build, test.enabled)
			if err != nil || mode != test.want {
				t.Fatal(mode, err)
			}
		})
	}
	api := newFakeWindowsAPI()
	api.failAt = map[string]int{"set-affinity-17": 1, "set-affinity-1": 1}
	if mode, err := newWindowsWindowController(api, 100).Capture(1, 26100, true); mode != "unavailable" || !errors.Is(err, errWindowsAPI) {
		t.Fatal(mode, err)
	}
	api = newFakeWindowsAPI()
	api.noEffect = "affinity"
	if mode, err := newWindowsWindowController(api, 100).Capture(1, 26100, true); mode != "unavailable" || err == nil {
		t.Fatal("success return hid a failed property change", mode, err)
	}
}

func TestWindowsCaptureInitialisesLayeredReadbackWithoutChangingExistingSuppression(t *testing.T) {
	api := newFakeWindowsAPI()
	c := newWindowsWindowController(api, 100)
	if _, err := c.Capture(1, 26100, true); err != nil || api.windows[1].alpha != 255 || api.windows[1].styles[windowsExtendedStyleIndex]&windowsLayered == 0 {
		t.Fatal("capture readback was attempted before layered setup", err)
	}
	if err := c.Suppress(1, true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Capture(1, 26100, true); err != nil || api.windows[1].alpha != 0 {
		t.Fatal("capture policy update exposed a suppressed window", err)
	}
}

func TestWindowsEditingReturnsToPreviousWindowAndRetryKeepsOriginalTarget(t *testing.T) {
	api := newFakeWindowsAPI()
	c := newWindowsWindowController(api, 100)
	if err := c.EnterEditing(1); err != nil {
		t.Fatal(err)
	}
	if api.foreground != 1 || api.windows[1].styles[windowsExtendedStyleIndex]&windowsNoActivate != 0 {
		t.Fatal(api)
	}
	api.foreground = 3
	if err := c.EnterEditing(1); err != nil || api.foreground != 3 || len(api.activations) != 1 {
		t.Fatal("retry stole focus from another application", err, api.activations)
	}
	api.foreground = 1
	if err := c.ExitEditing(1); err != nil || api.foreground != 2 || len(c.sessions) != 0 {
		t.Fatal("editor did not return to the original application", err, api.foreground)
	}
	if api.windows[1].styles[windowsExtendedStyleIndex]&windowsNoActivate == 0 {
		t.Fatal("passive style was not restored")
	}
}

func TestWindowsEditingDoesNotRestoreClosedReusedHiddenOrUserChangedWindows(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*fakeWindowsAPI)
	}{
		{"closed previous", func(api *fakeWindowsAPI) { delete(api.windows, 2) }},
		{"reused previous handle", func(api *fakeWindowsAPI) { api.windows[2].info.ProcessID = 400 }},
		{"hidden previous", func(api *fakeWindowsAPI) { api.windows[2].info.Visible = false }},
		{"minimized previous", func(api *fakeWindowsAPI) { api.windows[2].info.Minimized = true }},
		{"user changed applications", func(api *fakeWindowsAPI) { api.foreground = 3 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			api := newFakeWindowsAPI()
			c := newWindowsWindowController(api, 100)
			if err := c.EnterEditing(1); err != nil {
				t.Fatal(err)
			}
			test.change(api)
			before := api.foreground
			if err := c.ExitEditing(1); err != nil || api.foreground != before || len(api.activations) != 1 {
				t.Fatal("an unrelated window was activated", err, api.activations)
			}
		})
	}
}

func TestWindowsFailedActivationRollsBackAndCanRetryPassiveCleanup(t *testing.T) {
	api := newFakeWindowsAPI()
	api.failAt["activate"] = 1
	api.activateBeforeError = true
	c := newWindowsWindowController(api, 100)
	if err := c.EnterEditing(1); !errors.Is(err, errWindowsAPI) || api.foreground != 2 || len(c.sessions) != 0 {
		t.Fatal("partial activation was not rolled back", err, api.foreground)
	}
	api = newFakeWindowsAPI()
	api.failAt = map[string]int{"activate": 1, "write-style": 2}
	c = newWindowsWindowController(api, 100)
	if err := c.EnterEditing(1); err == nil || len(c.sessions) != 1 {
		t.Fatal("failed rollback lost its recovery state", err, c.sessions)
	}
	if err := c.ExitEditing(1); err != nil || len(c.sessions) != 0 || api.windows[1].styles[windowsExtendedStyleIndex]&windowsNoActivate == 0 {
		t.Fatal("cancel did not reconcile a partial focus transition", err)
	}
}

func TestWindowsHiddenEditingAndDeniedFocusReturnStayRecoverable(t *testing.T) {
	api := newFakeWindowsAPI()
	api.windows[1].info.Visible = false
	c := newWindowsWindowController(api, 100)
	if err := c.EnterEditing(1); err != nil || len(api.activations) != 0 {
		t.Fatal("hidden editor tried to activate", err)
	}
	if err := c.Visible(1, true); err != nil || len(api.activations) != 0 {
		t.Fatal(err)
	}
	api.foreground = 1
	api.failAt["activate"] = 1
	if err := c.ExitEditing(1); !errors.Is(err, errWindowsAPI) || len(c.sessions) != 1 {
		t.Fatal("denied return discarded recovery state", err)
	}
	if err := c.ExitEditing(1); err != nil || api.foreground != 2 || len(c.sessions) != 0 {
		t.Fatal("return could not be retried", err)
	}
}

func TestWindowsHideAndReleaseReturnFocusAndDiscardOldHandles(t *testing.T) {
	api := newFakeWindowsAPI()
	c := newWindowsWindowController(api, 100)
	if err := c.EnterEditing(1); err != nil {
		t.Fatal(err)
	}
	if err := c.Visible(1, false); err != nil || api.foreground != 2 {
		t.Fatal(err, api.foreground)
	}
	if err := c.Visible(1, true); err != nil || api.foreground != 2 {
		t.Fatal("show reactivated an editor", err)
	}
	api.foreground = 1
	if err := c.Release(1); err != nil || api.foreground != 2 || len(c.sessions) != 0 {
		t.Fatal(err)
	}
	api.foreground = 3
	if err := c.EnterEditing(1); err != nil {
		t.Fatal(err)
	}
	delete(api.windows, 1)
	if err := c.Release(1); err != nil || len(c.sessions) != 0 {
		t.Fatal("closed window retained focus state", err)
	}
}

func TestWindowsFailuresAndReadbackMismatchesRemainErrors(t *testing.T) {
	for _, test := range []struct {
		name      string
		operation func(*windowsWindowController) error
		fail      string
		noEffect  string
	}{
		{"style read fails", func(c *windowsWindowController) error { return c.ToolWindow(1) }, "read-style", ""},
		{"style write fails", func(c *windowsWindowController) error { return c.ToolWindow(1) }, "write-style", ""},
		{"style refresh fails", func(c *windowsWindowController) error { return c.ToolWindow(1) }, "position", ""},
		{"style mismatch", func(c *windowsWindowController) error { return c.ToolWindow(1) }, "", "style"},
		{"topmost mismatch", func(c *windowsWindowController) error { return c.Topmost(1, true) }, "", "topmost"},
		{"visibility mismatch", func(c *windowsWindowController) error { return c.Visible(1, false) }, "", "visibility"},
		{"show fails", func(c *windowsWindowController) error { return c.Visible(1, true) }, "show", ""},
		{"alpha write fails", func(c *windowsWindowController) error { return c.Suppress(1, true) }, "set-alpha", ""},
		{"alpha read fails", func(c *windowsWindowController) error { return c.Suppress(1, true) }, "read-alpha", ""},
		{"alpha mismatch", func(c *windowsWindowController) error { return c.Suppress(1, true) }, "", "alpha"},
	} {
		t.Run(test.name, func(t *testing.T) {
			api := newFakeWindowsAPI()
			api.noEffect = test.noEffect
			if test.fail != "" {
				api.failAt[test.fail] = 1
			}
			if err := test.operation(newWindowsWindowController(api, 100)); err == nil {
				t.Fatal("failed native effect was acknowledged")
			}
		})
	}
	api := newFakeWindowsAPI()
	api.windows[1].info.ProcessID = 999
	c := newWindowsWindowController(api, 100)
	if err := c.EnterEditing(1); err == nil || len(api.activations) != 0 {
		t.Fatal("foreign target accepted", err)
	}
}

func TestWindowsZeroStyleReturnIsValidOnlyWithoutLastError(t *testing.T) {
	if err := windowsStyleCallError(0, syscall.Errno(0)); err != nil {
		t.Fatal(err)
	}
	if err := windowsStyleCallError(0, errWindowsAPI); !errors.Is(err, errWindowsAPI) {
		t.Fatal(err)
	}
	if err := windowsStyleCallError(1, errWindowsAPI); err != nil {
		t.Fatal("successful return used stale error", err)
	}
	if err := windowsCallFailure("native call", syscall.Errno(0)); err == nil {
		t.Fatal("failed BOOL call accepted a zero last-error value")
	}
}

type windowsPolicyTestDriver struct{ controller *windowsWindowController }

func (d windowsPolicyTestDriver) SetCaptureProtection(_ domain.ID, enabled bool) (domain.CaptureResult, error) {
	mode, err := d.controller.Capture(1, 26100, enabled)
	return domain.CaptureResult{Applied: enabled && err == nil, Mechanism: mode}, err
}
func (d windowsPolicyTestDriver) SetInputMode(_ domain.ID, mode domain.InputMode) error {
	if mode == domain.Editing {
		return d.controller.EnterEditing(1)
	}
	return d.controller.ExitEditing(1)
}
func (d windowsPolicyTestDriver) SetClickThrough(_ domain.ID, enabled bool) error {
	return d.controller.ClickThrough(1, enabled)
}
func (d windowsPolicyTestDriver) SetVisible(_ domain.ID, visible bool) error {
	return d.controller.Visible(1, visible)
}
func (d windowsPolicyTestDriver) SetAlwaysOnTop(_ domain.ID, enabled bool) error {
	return d.controller.Topmost(1, enabled)
}
func (d windowsPolicyTestDriver) SetCaptureSuppressed(_ domain.ID, suppressed bool) error {
	return d.controller.Suppress(1, suppressed)
}

func TestWindowServiceReconcilesCancelledPartialWindowsEditing(t *testing.T) {
	api := newFakeWindowsAPI()
	api.windows[1].info.Visible = false
	c := newWindowsWindowController(api, 100)
	service := windowapp.New(windowsPolicyTestDriver{c})
	if err := service.Register("main"); err != nil {
		t.Fatal(err)
	}
	api.counts = map[string]int{}
	api.failAt = map[string]int{"activate": 1, "write-style": 2}
	if err := service.Update(func(p *domain.Policy) { p.InputMode = domain.Editing }); err == nil {
		t.Fatal("partial focus failure was accepted")
	}
	failed := service.Status().Windows[0]
	if failed.Ready || !failed.InputModePending || failed.Applied.InputMode != domain.Passive {
		t.Fatal(failed)
	}
	if err := service.Update(func(p *domain.Policy) { p.InputMode = domain.Passive }); err != nil {
		t.Fatal(err)
	}
	actual := service.Status().Windows[0]
	if !actual.Ready || actual.InputModePending || api.windows[1].styles[windowsExtendedStyleIndex]&windowsNoActivate == 0 || len(c.sessions) != 0 {
		t.Fatal("cancel left an editable window with passive status", actual, api.windows[1])
	}
	if !actual.Applied.CaptureProtection || api.windows[1].alpha != 255 {
		t.Fatal(actual)
	}
}

func TestWindowServiceCanReshowAfterHidePartlySucceeded(t *testing.T) {
	api := newFakeWindowsAPI()
	api.windows[1].info.Visible = false
	c := newWindowsWindowController(api, 100)
	service := windowapp.New(windowsPolicyTestDriver{c})
	if err := service.Register("main"); err != nil {
		t.Fatal(err)
	}
	if err := service.Update(func(p *domain.Policy) { p.InputMode = domain.Editing }); err != nil {
		t.Fatal(err)
	}
	api.failAt["activate"] = 2
	if err := service.Update(func(p *domain.Policy) { p.Visible = false }); err == nil || api.windows[1].info.Visible {
		t.Fatal("expected a hidden window with a reported focus return failure", err)
	}
	if !service.Status().Windows[0].VisibilityPending {
		t.Fatal("partial visibility change was forgotten")
	}
	if err := service.Update(func(p *domain.Policy) { p.Visible = true }); err != nil {
		t.Fatal(err)
	}
	actual := service.Status().Windows[0]
	if !actual.Ready || actual.VisibilityPending || !api.windows[1].info.Visible || !actual.Applied.Visible {
		t.Fatal("reshow trusted the stale pre-failure visibility", actual)
	}
}
