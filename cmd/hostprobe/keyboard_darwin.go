//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c -fblocks
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#import <IOKit/hidsystem/IOLLEvent.h>
#include <stdint.h>
#include <unistd.h>
static int QSKeyboardOwnTaps(void) {
    uint32_t count = 0;
    if (CGGetEventTapList(0, NULL, &count) != kCGErrorSuccess) return -1;
    if (!count) return 0;
    CGEventTapInformation *list = calloc(count, sizeof(CGEventTapInformation));
    if (!list) return -1;
    uint32_t actual = 0;
    if (CGGetEventTapList(count, list, &actual) != kCGErrorSuccess) { free(list); return -1; }
    int owned = 0;
    for (uint32_t i = 0; i < actual; i++) if (list[i].tappingProcess == getpid()) owned++;
    free(list);
    return owned;
}
static int QSKeyboardForeground(void) {
    __block int pid = 0;
    dispatch_sync(dispatch_get_main_queue(), ^{ pid = NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier; });
    return pid;
}
typedef struct { int active, appKey, windowKeys, foreground; } QSKeyboardFocus;
static QSKeyboardFocus QSKeyboardFocusState(void) {
    __block QSKeyboardFocus state = {0};
    dispatch_sync(dispatch_get_main_queue(), ^{
        state.active = NSApp.active;
        state.appKey = NSApp.keyWindow != nil;
        for (NSWindow *window in NSApp.windows) if (window.keyWindow) state.windowKeys++;
        state.foreground = NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier == getpid();
    });
    return state;
}
static bool QSKeyboardReady(void) {
    if (!AXIsProcessTrusted() || !CGPreflightPostEventAccess()) return false;
    for (CGKeyCode code = 0; code < 128; code++) {
        if (CGEventSourceKeyState(kCGEventSourceStateHIDSystemState, code)) return false;
    }
    return true;
}
static CGEventFlags QSKeyboardFlags(bool left, bool right) {
    CGEventFlags flags = 0;
    if (left || right) flags |= kCGEventFlagMaskCommand;
    if (left) flags |= NX_DEVICELCMDKEYMASK;
    if (right) flags |= NX_DEVICERCMDKEYMASK;
    return flags;
}
static CGEventSourceRef QSKeyboardSourceCreate(void) {
    CGEventSourceRef source = CGEventSourceCreate(kCGEventSourceStatePrivate);
    if (source) CGEventSourceSetLocalEventsSuppressionInterval(source, 0);
    return source;
}
static void QSKeyboardSourceDestroy(CGEventSourceRef source) {
    if (source) CFRelease(source);
}
static bool QSKeyboardPost(CGEventSourceRef source, int pid, int code, bool down, bool repeat, uint64_t tag, CGEventFlags flags) {
    if (QSKeyboardForeground() != pid) return false;
    CGEventRef event = CGEventCreateKeyboardEvent(source, (CGKeyCode)code, down);
    if (!event) return false;
    if (code == 54 || code == 55) CGEventSetType(event, kCGEventFlagsChanged);
    CGEventSetFlags(event, flags);
    CGEventSetIntegerValueField(event, kCGKeyboardEventAutorepeat, repeat);
    CGEventSetIntegerValueField(event, kCGEventSourceUserData, (int64_t)tag);
    CGEventPost(kCGSessionEventTap, event);
    CFRelease(event);
    return true;
}
static void QSKeyboardReleaseToTarget(CGEventSourceRef source, int pid, int code) {
    CGEventRef event = CGEventCreateKeyboardEvent(source, (CGKeyCode)code, false);
    if (event) {
        if (code == 54 || code == 55) CGEventSetType(event, kCGEventFlagsChanged);
        CGEventSetFlags(event, 0);
        CGEventPostToPid(pid, event);
        CFRelease(event);
    }
}
*/
import "C"

import (
	"Q-Solver/internal/input/application"
	"Q-Solver/internal/input/domain"
	"Q-Solver/internal/input/infrastructure/keymap"
	"Q-Solver/internal/input/infrastructure/native"
	inputprobe "Q-Solver/internal/input/infrastructure/probe"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"
)

const diagnosticBarrier = "diagnostic-drain"

type inputTrace struct {
	application.Driver
	mu        sync.Mutex
	events    []inputprobe.EventRecord
	callbacks application.Callbacks
}

func (t *inputTrace) Start(callbacks application.Callbacks) (application.Capabilities, error) {
	t.callbacks = callbacks
	forward := callbacks.Event
	callbacks.Event = func(event domain.Event) domain.Decision {
		decision := forward(event)
		t.mu.Lock()
		t.events = append(t.events, inputprobe.EventRecord{Event: event, Decision: decision})
		t.mu.Unlock()
		return decision
	}
	return t.Driver.Start(callbacks)
}

func (t *inputTrace) snapshot() []inputprobe.EventRecord {
	t.mu.Lock()
	defer t.mu.Unlock()
	return slices.Clone(t.events)
}

type inputCommands struct {
	mu      sync.Mutex
	actions []string
	drained chan struct{}
}

func (c *inputCommands) Execute(action string) {
	if action == diagnosticBarrier {
		c.drained <- struct{}{}
		return
	}
	c.mu.Lock()
	c.actions = append(c.actions, action)
	c.mu.Unlock()
}

func (c *inputCommands) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.actions)
}

type keyboardRunner struct {
	host     *hostProbe
	service  *application.Service
	trace    *inputTrace
	commands *inputCommands
	report   *inputprobe.Report
	tag      uint64
	held     map[domain.Key]bool
	target   int
	source   C.CGEventSourceRef
}

func newKeyboardRunner(host *hostProbe) (*keyboardRunner, error) {
	report := &inputprobe.Report{Scope: "tagged CGEvent through session event tap"}
	host.report.Keyboard = report
	if int(C.QSKeyboardForeground()) != host.before {
		return nil, fmt.Errorf("the owned browser receiver is not foreground")
	}
	if !bool(C.QSKeyboardReady()) {
		return nil, fmt.Errorf("keyboard diagnostic needs existing input permissions and all physical keys released")
	}
	report.NativeTapsBefore = int(C.QSKeyboardOwnTaps())
	if report.NativeTapsBefore < 0 {
		return nil, fmt.Errorf("cannot inspect owned native event taps")
	}
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return nil, err
	}
	tag := binary.LittleEndian.Uint64(data[:])&((1<<63)-1) | 1
	driver, err := native.NewTaggedDiagnostic(tag)
	if err != nil {
		return nil, err
	}
	trace := &inputTrace{Driver: driver}
	commands := &inputCommands{drained: make(chan struct{}, 1)}
	r := &keyboardRunner{host: host, trace: trace, commands: commands, report: report, tag: tag, held: map[domain.Key]bool{}}
	r.service = application.NewService(trace, commands, nil)
	r.source = C.QSKeyboardSourceCreate()
	if r.source == 0 {
		return nil, fmt.Errorf("cannot create diagnostic event source")
	}
	if err := r.service.Start(inputprobe.PassiveScenarios()[0].Policy()); err != nil {
		return r, err
	}
	report.Filtering = r.service.Status().Capabilities.Filtering
	if !report.Filtering {
		return r, fmt.Errorf("native event filtering unavailable: %s", r.service.Status().Error)
	}
	return r, nil
}

func (r *keyboardRunner) close() {
	for _, key := range []domain.Key{"KeyA", "KeyB", "MetaLeft", "MetaRight"} {
		if r.held[key] {
			code, _ := keymap.MacCode(key)
			C.QSKeyboardReleaseToTarget(r.source, C.int(r.target), C.int(code))
		}
	}
	if err := r.service.Stop(); err != nil {
		r.report.Errors = append(r.report.Errors, err.Error())
	}
	r.report.Stopped = r.service.Status().State == "stopped"
	deadline := time.Now().Add(time.Second)
	for {
		r.report.NativeTapsAfter = int(C.QSKeyboardOwnTaps())
		if r.report.NativeTapsAfter == r.report.NativeTapsBefore || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.report.ListenerReleased = r.report.NativeTapsAfter == r.report.NativeTapsBefore
	C.QSKeyboardSourceDestroy(r.source)
	r.source = 0
}

func (r *keyboardRunner) run(scenario inputprobe.Scenario) (err error) {
	defer func() {
		if err != nil {
			r.report.Errors = append(r.report.Errors, err.Error())
		}
	}()
	if err = r.service.Configure(scenario.Policy(), nil); err != nil {
		return err
	}
	if err = r.service.SetEditing(scenario.EditingIntent); err != nil {
		return err
	}
	r.target = r.host.before
	if scenario.OwnEditor {
		r.target = os.Getpid()
	}
	if !r.host.acknowledge("keys-ready/" + scenario.ID) {
		return fmt.Errorf("keyboard observer did not prepare %s", scenario.ID)
	}
	startEvent, startAction := len(r.trace.snapshot()), len(r.commands.snapshot())
	matched := startEvent
	for _, packet := range scenario.Packets {
		code, ok := keymap.MacCode(packet.Key)
		if !ok {
			return fmt.Errorf("unmapped diagnostic key")
		}
		previous := r.held[packet.Key]
		r.held[packet.Key] = packet.Down
		tag := r.tag
		if !packet.Tagged {
			tag ^= 0xff
		}
		flags := C.QSKeyboardFlags(C.bool(r.held["MetaLeft"]), C.bool(r.held["MetaRight"]))
		if !bool(C.QSKeyboardPost(r.source, C.int(r.target), C.int(code), C.bool(packet.Down), C.bool(packet.Repeat), C.uint64_t(tag), flags)) {
			r.held[packet.Key] = previous
			return fmt.Errorf("foreground changed before injecting %s: expected process %d, observed %d", scenario.ID, r.target, int(C.QSKeyboardForeground()))
		}
		if packet.Tagged {
			matched++
			deadline := time.Now().Add(time.Second)
			for len(r.trace.snapshot()) < matched {
				if time.Now().After(deadline) {
					return fmt.Errorf("native event not observed in %s: %s", scenario.ID, r.service.Status().Error)
				}
				time.Sleep(2 * time.Millisecond)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	focus := C.QSKeyboardFocusState()
	actual := inputprobe.Case{ID: scenario.ID, ForegroundRetained: int(C.QSKeyboardForeground()) == r.target, Focus: inputprobe.FocusFacts{AppActive: focus.active != 0, AppKeyWindow: focus.appKey != 0, KeyWindows: int(focus.windowKeys), Foreground: focus.foreground != 0}}
	if scenario.OwnEditor {
		select {
		case observation := <-r.host.keyResult:
			actual.Editor = &observation
		case <-time.After(3 * time.Second):
			return fmt.Errorf("native editor did not receive its keyboard barrier")
		}
	} else if scenario.EditingIntent {
		deadline := time.Now().Add(time.Second)
		for len(r.commands.snapshot()) < startAction+scenario.Actions {
			if time.Now().After(deadline) {
				return fmt.Errorf("native command did not complete in %s", scenario.ID)
			}
			time.Sleep(2 * time.Millisecond)
		}
	} else {
		// All native callbacks through the barrier have returned. Draining the
		// same FIFO also waits for every previously enqueued business command.
		r.trace.callbacks.Shortcut(r.service.Status().Revision, diagnosticBarrier)
		select {
		case <-r.commands.drained:
		case <-time.After(time.Second):
			return fmt.Errorf("native command queue did not drain in %s", scenario.ID)
		}
	}
	actual.Events = r.trace.snapshot()[startEvent:]
	actual.Actions = r.commands.snapshot()[startAction:]
	r.report.Cases = append(r.report.Cases, actual)
	if !r.host.acknowledge("keys-done/" + scenario.ID) {
		return fmt.Errorf("keyboard receiver did not confirm %s", scenario.ID)
	}
	return nil
}
