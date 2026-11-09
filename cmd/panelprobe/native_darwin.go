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
	"os"
	"runtime"
)

// Owned only by this standalone process and accessed on AppKit's main thread.
var currentProbe *nativeProbe

func init() { runtime.LockOSThread() }

type nativeProbe struct {
	report           probe.Report
	before           int
	service          *application.Service
	driver           *native.Driver
	handles          map[domain.ID]platform.WindowHandle
	captureStarted   bool
	mode             captureMode
	displayWindow    bool
	startProtected   bool
	streamFrames     []probe.CaptureFrame
	streamStatuses   map[string]int
	streamOutputs    []probe.CaptureOutput
	protectedSession *probe.ProtectedCaptureSession
	stopped          bool
}

func runNative(mode captureMode, display, startProtected bool) (probe.Report, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	state := C.QSProbeCaptureEnvironment()
	environment := &probe.CaptureEnvironment{
		SessionKnown: state.sessionKnown != 0, SessionLocked: state.locked != 0,
		OnConsole: state.onConsole != 0, DisplaysKnown: state.displaysKnown != 0,
		ActiveDisplays: int(state.activeDisplays), MainDisplayAsleep: state.asleep != 0,
	}
	report := probe.Report{System: C.GoString(C.QSProbeSystem()), Environment: environment}
	if !environment.Available() {
		return report, fmt.Errorf("native capture needs an unlocked, awake desktop; locked=%t activeDisplays=%d asleep=%t", environment.SessionLocked, environment.ActiveDisplays, environment.MainDisplayAsleep)
	}
	driver := native.New()
	currentProbe = &nativeProbe{driver: driver, service: application.New(driver), handles: map[domain.ID]platform.WindowHandle{}, before: int(C.QSProbeForeground()), mode: mode, displayWindow: display, startProtected: startProtected}
	currentProbe.report = report
	if mode == captureScreenshot {
		currentProbe.report.Capture = &probe.CaptureReport{API: probe.ScreenshotAPI}
	}
	if mode == captureFreshStream {
		currentProbe.report.Capture = &probe.CaptureReport{API: probe.StreamAPI}
	}
	if mode == captureLiveStream {
		currentProbe.report.LiveCapture = &probe.LiveCaptureReport{API: probe.StreamAPI}
		if display {
			currentProbe.report.LiveCapture.API = probe.DisplayWindowStreamAPI
		}
	}
	timeoutMillis := 20_000
	if mode == captureProtectedSession {
		currentProbe.report.ProtectedCapture = &probe.ProtectedCaptureReport{API: probe.DisplayWindowStreamAPI}
		timeoutMillis = 90_000
	}
	C.QSProbeRun(C.int(timeoutMillis))
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
	currentProbe.report.Errors = append(currentProbe.report.Errors, "native fixture/capture exceeded the diagnostic deadline")
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
	if p.report.ProtectedCapture != nil {
		p.close("secondary")
		p.report.ProtectedCapture.WindowCount = len(p.service.Status().Windows)
		C.QSProbeCreateCaptureBackground(C.uintptr_t(p.handles["main"]), 1)
		p.nextProtectedCapture(0)
		return
	}
	if p.report.Capture != nil || p.report.LiveCapture != nil {
		if p.report.LiveCapture != nil {
			p.close("secondary")
			p.report.LiveCapture.WindowCount = len(p.service.Status().Windows)
		}
		index := 0
		if p.startProtected {
			C.QSProbeCreateCaptureBackground(C.uintptr_t(p.handles["main"]), 0)
			index = -1
		}
		p.nextCapture(index)
	}
}

func (p *nativeProbe) nextCapture(index int) {
	if !p.setCapturePolicy(index) {
		p.stop()
		return
	}
	p.captureStarted = true
	p.streamFrames = nil
	p.streamStatuses = map[string]int{}
	p.streamOutputs = nil
	stream, liveMillis := 0, 0
	if p.mode == captureFreshStream || p.mode == captureLiveStream {
		stream = 1
	}
	if p.mode == captureLiveStream {
		liveMillis = probe.LivePhaseMillis
	}
	display := 0
	if p.displayWindow {
		display = 1
	}
	C.QSProbeCapture(C.uintptr_t(p.handles["main"]), C.int(index), C.int(stream), C.int(liveMillis), C.int(display), 0)
}

func (p *nativeProbe) protectedObservation(stage string) probe.CaptureTransition {
	state := C.QSProbeObserve(C.uintptr_t(p.handles["main"]))
	return probe.CaptureTransition{Stage: stage, AppliedNanos: int64(C.QSProbeHostTime()), PropertySet: state.protectedContent != 0, LocalVisible: state.visible != 0, Alpha: float64(state.alpha)}
}

func (p *nativeProbe) nextProtectedCapture(index int) {
	// This path retains the default protection. It never calls setCapturePolicy.
	r := p.report.ProtectedCapture
	r.Sessions = append(r.Sessions, probe.ProtectedCaptureSession{Before: p.protectedObservation("protected-session-start")})
	p.protectedSession = &r.Sessions[len(r.Sessions)-1]
	p.captureStarted = true
	p.streamFrames, p.streamOutputs = nil, nil
	p.streamStatuses = map[string]int{}
	C.QSProbeCapture(C.uintptr_t(p.handles["main"]), C.int(index), 1, 0, 1, probe.ProtectedSessionMillis)
}

//export panelProbeDisplaySelected
func panelProbeDisplaySelected(s *C.QSProbeDisplaySelection) C.int {
	p := currentProbe
	if p == nil || p.stopped || !p.displayWindow || (p.report.LiveCapture == nil && p.protectedSession == nil) || s == nil {
		return 0
	}
	selection := &probe.DisplayCaptureSelection{
		ExpectedWindowID: uint32(s.expectedWindow), ActualWindowID: uint32(s.actualWindow),
		ExpectedOwnerPID: os.Getpid(), ActualOwnerPID: int(s.ownerPID),
		ExpectedDisplay: uint32(s.expectedDisplay), ActualDisplay: uint32(s.actualDisplay),
		WindowCount: int(s.windows), DisplayCount: int(s.displays),
		Window:  probe.CaptureRect{X: float64(s.windowX), Y: float64(s.windowY), Width: float64(s.windowWidth), Height: float64(s.windowHeight)},
		Display: probe.CaptureRect{X: float64(s.displayX), Y: float64(s.displayY), Width: float64(s.displayWidth), Height: float64(s.displayHeight)},
		Crop:    probe.CaptureRect{X: float64(s.cropX), Y: float64(s.cropY), Width: float64(s.cropWidth), Height: float64(s.cropHeight)},
	}
	if s.expectedBackground != 0 || s.actualBackground != 0 || s.backgroundOwnerPID != 0 {
		selection.Background = &probe.CaptureBackground{
			ExpectedWindowID: uint32(s.expectedBackground), ActualWindowID: uint32(s.actualBackground), OwnerPID: int(s.backgroundOwnerPID),
			Window: probe.CaptureRect{X: float64(s.backgroundX), Y: float64(s.backgroundY), Width: float64(s.backgroundWidth), Height: float64(s.backgroundHeight)},
		}
	}
	if p.protectedSession != nil {
		p.protectedSession.Selection = selection
	} else {
		p.report.LiveCapture.DisplaySelection = selection
	}
	if !selection.Valid() {
		return 0
	}
	return 1
}

func (p *nativeProbe) setCapturePolicy(index int) bool {
	if !p.check(p.service.Update(func(policy *domain.Policy) { policy.CaptureProtection = index == 1 || index == -1 })) {
		return false
	}
	if p.report.LiveCapture != nil {
		state := C.QSProbeObserve(C.uintptr_t(p.handles["main"]))
		stages := []string{"baseline-before", "sharing-none", "baseline-after"}
		transition := probe.CaptureTransition{
			AppliedNanos: int64(C.QSProbeHostTime()),
			PropertySet:  state.protectedContent != 0, LocalVisible: state.visible != 0,
			Alpha: float64(state.alpha),
		}
		if index == -1 {
			transition.Stage = "initial-sharing-none"
			p.report.LiveCapture.InitialProtection = &transition
		} else {
			transition.Stage = stages[index]
			p.report.LiveCapture.Transitions = append(p.report.LiveCapture.Transitions, transition)
		}
	}
	return true
}

//export panelProbeLivePolicy
func panelProbeLivePolicy(index C.int) C.int {
	p := currentProbe
	if p == nil || p.stopped || index < 0 || index > 2 || (index == 0 && !p.startProtected) || !p.setCapturePolicy(int(index)) {
		return 0
	}
	return 1
}

//export panelProbeCaptured
func panelProbeCaptured(index, width, height C.int, red, green, black, transparent C.double, errorCode C.int) {
	p := currentProbe
	if p == nil {
		return
	}
	if p.protectedSession != nil {
		p.protectedSession.ErrorCode = int(errorCode)
		p.protectedSession.After = p.protectedObservation("protected-session-end")
		p.stop()
		return
	}
	if p.report.LiveCapture != nil {
		p.report.LiveCapture.ErrorCode = int(errorCode)
		p.stop()
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
func panelProbeStreamFrame(width, height C.int, red, green, blue, black, transparent, rawColor C.double, presentation, display, received C.int64_t, file *C.char) {
	p := currentProbe
	if p == nil || p.stopped {
		return
	}
	rawRatio := float64(rawColor)
	p.streamFrames = append(p.streamFrames, probe.CaptureFrame{
		CapturePixels: probe.CapturePixels{
			Width: int(width), Height: int(height),
			RedRatio: float64(red), GreenRatio: float64(green),
			BlueRatio:  float64(blue),
			BlackRatio: float64(black), TransparentRatio: float64(transparent),
		},
		PresentationNanos: int64(presentation), DisplayNanos: int64(display),
		ReceivedNanos: int64(received), File: C.GoString(file),
		RawColorRatio: &rawRatio,
	})
}

//export panelProbeStreamFinished
func panelProbeStreamFinished(index, duration, outputs, errorCode C.int) {
	p := currentProbe
	if p == nil || p.stopped {
		return
	}
	if session := p.protectedSession; session != nil {
		session.StreamObservation = probe.StreamObservation{DurationMillis: int(duration), OutputCount: int(outputs), FrameStatuses: p.streamStatuses, Frames: p.streamFrames}
		session.Outputs, session.ErrorCode = p.streamOutputs, int(errorCode)
		session.After = p.protectedObservation("protected-session-end")
		if errorCode != 0 || index == 1 {
			p.stop()
		} else {
			p.nextProtectedCapture(int(index) + 1)
		}
		return
	}
	if live := p.report.LiveCapture; live != nil {
		live.Frames, live.FrameStatuses = p.streamFrames, p.streamStatuses
		live.Outputs = p.streamOutputs
		live.OutputCount, live.ErrorCode = int(outputs), int(errorCode)
		p.stop()
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

//export panelProbeStreamLifecycle
func panelProbeStreamLifecycle(phase C.int, at C.int64_t) {
	p := currentProbe
	if p == nil || p.stopped {
		return
	}
	if session := p.protectedSession; session != nil {
		switch phase {
		case 0:
			session.Starts++
			session.StartedNanos = int64(at)
		case 1:
			session.Stops++
			session.StoppedNanos = int64(at)
		case 2:
			session.DrainedNanos = int64(at)
		}
		return
	}
	live := p.report.LiveCapture
	if live == nil {
		return
	}
	if phase == 0 {
		live.Starts++
		live.StartedNanos = int64(at)
	} else if phase == 1 {
		live.Stops++
		live.StoppedNanos = int64(at)
	} else if phase == 2 {
		live.DrainedNanos = int64(at)
	}
}

//export panelProbeStreamOutput
func panelProbeStreamOutput(status, frame C.int, presentation, display, received C.int64_t) {
	p := currentProbe
	if p == nil || p.stopped || !p.displayWindow {
		return
	}
	names := []string{"complete", "idle", "blank", "suspended", "started", "stopped", "unknown"}
	name := "unknown"
	if status >= 0 && int(status) < len(names) {
		name = names[int(status)]
	}
	p.streamOutputs = append(p.streamOutputs, probe.CaptureOutput{
		Status: name, FrameIndex: int(frame), PresentationNanos: int64(presentation),
		DisplayNanos: int64(display), ReceivedNanos: int64(received),
	})
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
