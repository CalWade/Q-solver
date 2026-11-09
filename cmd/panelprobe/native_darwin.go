//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c -fblocks
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework ScreenCaptureKit -framework ImageIO -framework CoreMedia -framework CoreVideo
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
	report         probe.Report
	before         int
	service        *application.Service
	driver         *native.Driver
	handles        map[domain.ID]platform.WindowHandle
	captureStarted bool
	streaming      bool
	streamFrames   []probe.CaptureFrame
	streamStatuses map[string]int
	stopped        bool
}

func runNative(capture, stream bool) (probe.Report, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	driver := native.New()
	currentProbe = &nativeProbe{driver: driver, service: application.New(driver), handles: map[domain.ID]platform.WindowHandle{}, before: int(C.QSProbeForeground()), streaming: stream}
	currentProbe.report.System = C.GoString(C.QSProbeSystem())
	if capture {
		currentProbe.report.Capture = &probe.CaptureReport{API: probe.ScreenshotAPI}
	}
	if stream {
		currentProbe.report.Capture = &probe.CaptureReport{API: probe.StreamAPI}
	}
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
	if p.stopped {
		return
	}
	p.stopped = true
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
	currentProbe.report.Errors = append(currentProbe.report.Errors, "native fixture/capture did not finish within 20 seconds")
	currentProbe.stop()
}

//export panelProbeReady
func panelProbeReady() {
	p := currentProbe
	p.report.WebViewReady = true
	defer func() {
		if !p.captureStarted {
			p.stop()
		}
	}()
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
	if p.report.Capture != nil {
		p.nextCapture(0)
	}
}

func (p *nativeProbe) nextCapture(index int) {
	if !p.check(p.service.Update(func(policy *domain.Policy) { policy.CaptureProtection = index == 1 })) {
		p.stop()
		return
	}
	p.captureStarted = true
	p.streamFrames = nil
	p.streamStatuses = map[string]int{}
	stream := 0
	if p.streaming {
		stream = 1
	}
	C.QSProbeCapture(C.uintptr_t(p.handles["main"]), C.int(index), C.int(stream))
}

//export panelProbeCaptured
func panelProbeCaptured(index, width, height C.int, red, green, black, transparent C.double, errorCode C.int) {
	p := currentProbe
	if p == nil {
		return
	}
	stages := []string{"baseline-before", "sharing-none", "baseline-after"}
	i := int(index)
	if i < 0 || i >= len(stages) {
		p.check(fmt.Errorf("unexpected capture sample"))
		p.stop()
		return
	}
	state := C.QSProbeObserve(C.uintptr_t(p.handles["main"]))
	p.report.Capture.Samples = append(p.report.Capture.Samples, probe.CaptureSample{Stage: stages[i], PropertySet: state.protectedContent != 0, CapturePixels: probe.CapturePixels{Width: int(width), Height: int(height), RedRatio: float64(red), GreenRatio: float64(green), BlackRatio: float64(black), TransparentRatio: float64(transparent)}, ErrorCode: int(errorCode)})
	if errorCode != 0 || i == len(stages)-1 {
		p.stop()
		return
	}
	p.nextCapture(i + 1)
}

//export panelProbeStreamFrame
func panelProbeStreamFrame(width, height C.int, red, green, black, transparent C.double, presentation C.int64_t, file *C.char) {
	p := currentProbe
	if p == nil || p.stopped {
		return
	}
	p.streamFrames = append(p.streamFrames, probe.CaptureFrame{CapturePixels: probe.CapturePixels{Width: int(width), Height: int(height), RedRatio: float64(red), GreenRatio: float64(green), BlackRatio: float64(black), TransparentRatio: float64(transparent)}, PresentationNanos: int64(presentation), File: C.GoString(file)})
}

//export panelProbeStreamFinished
func panelProbeStreamFinished(index, duration, outputs, errorCode C.int) {
	p := currentProbe
	if p == nil || p.stopped {
		return
	}
	observation := &probe.StreamObservation{DurationMillis: int(duration), OutputCount: int(outputs), FrameStatuses: p.streamStatuses, Frames: p.streamFrames}
	var pixels probe.CapturePixels
	if len(p.streamFrames) > 0 {
		pixels = p.streamFrames[len(p.streamFrames)-1].CapturePixels
	}
	// The shared callback advances the next stage, so retain this observation
	// before it resets the accumulator or closes the final window.
	sampleIndex := len(p.report.Capture.Samples)
	panelProbeCaptured(index, C.int(pixels.Width), C.int(pixels.Height), C.double(pixels.RedRatio), C.double(pixels.GreenRatio), C.double(pixels.BlackRatio), C.double(pixels.TransparentRatio), errorCode)
	if len(p.report.Capture.Samples) > sampleIndex {
		p.report.Capture.Samples[sampleIndex].Stream = observation
	}
}

//export panelProbeStreamStatus
func panelProbeStreamStatus(status, count C.int) {
	p := currentProbe
	if p == nil || p.stopped {
		return
	}
	names := []string{"complete", "idle", "blank", "suspended", "started", "stopped", "unknown"}
	if status >= 0 && int(status) < len(names) {
		p.streamStatuses[names[int(status)]] = int(count)
	}
}
