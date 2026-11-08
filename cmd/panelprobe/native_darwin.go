//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c -fblocks
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#include "native_darwin.h"
*/
import "C"

import (
	"Q-Solver/internal/window/application"
	"Q-Solver/internal/window/domain"
	"Q-Solver/internal/window/infrastructure/native"
	"Q-Solver/internal/window/infrastructure/probe"
	"Q-Solver/pkg/platform"
	"fmt"
	"runtime"
)

// Owned only by this standalone process and accessed on AppKit's main thread.
var currentProbe *nativeProbe

func init() { runtime.LockOSThread() }

type nativeProbe struct {
	report  probe.Report
	before  int
	service *application.Service
	driver  *native.Driver
	handles map[domain.ID]platform.WindowHandle
}

func runNative() (probe.Report, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	driver := native.New()
	currentProbe = &nativeProbe{driver: driver, service: application.New(driver), handles: map[domain.ID]platform.WindowHandle{}, before: int(C.QSProbeForeground())}
	currentProbe.report.System = C.GoString(C.QSProbeSystem())
	C.QSProbeRun()
	result := currentProbe.report
	currentProbe = nil
	return result, nil
}
func (p *nativeProbe) check(err error) bool {
	if err != nil {
		p.report.Errors = append(p.report.Errors, err.Error())
		return false
	}
	return true
}
func (p *nativeProbe) register(id domain.ID, handle platform.WindowHandle) bool {
	p.handles[id] = handle
	return p.check(p.driver.Bind(id, handle)) && p.check(p.service.Register(id))
}
func (p *nativeProbe) close(id domain.ID) {
	handle := p.handles[id]
	if handle == 0 {
		return
	}
	p.service.Unregister(id)
	p.driver.Unbind(id)
	delete(p.handles, id)
	C.QSProbeClose(C.uintptr_t(handle))
}
func (p *nativeProbe) observe(stage string, id domain.ID) {
	o := C.QSProbeObserve(C.uintptr_t(p.handles[id]))
	p.report.Observations = append(p.report.Observations, probe.Observation{Stage: stage, Panel: o.panel != 0, Visible: o.visible != 0, NonActivating: o.nonactivating != 0, CanBecomeKey: o.canKey != 0, Key: o.key != 0, Main: o.main != 0, CaptureProperty: o.protectedContent != 0, Alpha: float64(o.alpha), ForegroundUnchanged: int(o.foreground) == p.before})
}
func (p *nativeProbe) stop() {
	for id := range p.handles {
		p.close(id)
	}
	C.QSProbeStop()
}

//export panelProbeCreated
func panelProbeCreated(handle C.uintptr_t) {
	if !currentProbe.register("main", platform.WindowHandle(handle)) {
		currentProbe.stop()
	}
}

//export panelProbeTimeout
func panelProbeTimeout() {
	currentProbe.report.Errors = append(currentProbe.report.Errors, "WKWebView fixture did not become ready within 10 seconds")
	currentProbe.stop()
}

//export panelProbeReady
func panelProbeReady() {
	p := currentProbe
	p.report.WebViewReady = true
	defer p.stop()
	// Even a direct request must not give the passive window keyboard focus.
	C.QSProbeTryKey(C.uintptr_t(p.handles["main"]))
	p.observe("shown", "main")
	first, err := p.service.BeginCapture()
	if !p.check(err) {
		return
	}
	p.observe("capture-first", "main")
	second, err := p.service.BeginCapture()
	if !p.check(err) {
		p.check(first())
		return
	}
	p.observe("capture-nested", "main")
	if !p.register("secondary", platform.WindowHandle(C.QSProbeCreate())) {
		p.check(first())
		p.check(second())
		return
	}
	p.observe("secondary-during-capture", "secondary")
	p.check(first())
	p.observe("first-release", "main")
	p.check(second())
	p.observe("last-release", "main")
	p.observe("secondary-restored", "secondary")
	p.close("secondary")
	if !p.register("secondary", platform.WindowHandle(C.QSProbeCreate())) {
		return
	}
	p.observe("recreated", "secondary")
	if p.service.Status().Captures != 0 {
		p.check(fmt.Errorf("capture lease remained active"))
	}
}
