//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c -fblocks
#cgo LDFLAGS: -framework Cocoa -framework UniformTypeIdentifiers
#import <Cocoa/Cocoa.h>
#include <stdint.h>
typedef struct { int panel, nonactivating, gate, canKey, key, front, own; } QSHostObservation;
static void QSHostMain(dispatch_block_t block) {
    if (NSThread.isMainThread) block(); else dispatch_sync(dispatch_get_main_queue(), block);
}
static QSHostObservation QSHostObserve(uintptr_t handle) {
    __block QSHostObservation result = {0};
    QSHostMain(^{
        NSWindow *window = (NSWindow *)handle;
        if (window && [NSApp.windows indexOfObjectIdenticalTo:window] != NSNotFound) {
            result.panel = [window isKindOfClass:NSPanel.class];
            result.nonactivating = (window.styleMask & NSWindowStyleMaskNonactivatingPanel) != 0;
            result.gate = [window respondsToSelector:@selector(setQSEditable:)];
            result.canKey = window.canBecomeKeyWindow;
            result.key = window.keyWindow;
        }
        result.front = NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier;
        result.own = NSProcessInfo.processInfo.processIdentifier;
    });
    return result;
}
static int QSHostForeground(void) { return NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier; }
static const char *QSHostSystem(void) { return NSProcessInfo.processInfo.operatingSystemVersionString.UTF8String; }
*/
import "C"

import (
	"Q-Solver/internal/window/infrastructure/probe"
	"Q-Solver/pkg/platform"
	"Q-Solver/pkg/state"
	"context"
	"fmt"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"runtime"
	"testing/fstest"
	"time"
)

func init() { runtime.LockOSThread() }

type hostProbe struct {
	before     int
	report     probe.HostReport
	manager    *state.StateManager
	pageReady  chan struct{}
	textResult chan bool
	done       chan struct{}
	observer   func(string) error
}

func (p *hostProbe) acknowledge(stage string) bool {
	if p.observer != nil {
		if err := p.observer(stage); err != nil {
			p.report.Errors = append(p.report.Errors, err.Error())
			return false
		}
	}
	return true
}

func (p *hostProbe) observe(stage string) probe.HostObservation {
	handle, err := platform.GetWindowHandle()
	if err != nil {
		p.report.Errors = append(p.report.Errors, err.Error())
	}
	o := C.QSHostObserve(C.uintptr_t(handle))
	return probe.HostObservation{Stage: stage, Panel: o.panel != 0, NonActivating: o.nonactivating != 0, HasEditingGate: o.gate != 0, CanBecomeKey: o.canKey != 0, Key: o.key != 0, ForegroundIsApp: o.front == o.own, ForegroundRestored: int(o.front) == p.before}
}
func (p *hostProbe) poll(stage string, ready func(probe.HostObservation) bool) {
	deadline := time.Now().Add(2 * time.Second)
	for {
		o := p.observe(stage)
		if ready(o) || time.Now().After(deadline) {
			p.report.Observations = append(p.report.Observations, o)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func (p *hostProbe) exercise(ctx context.Context) {
	defer close(p.done)
	defer wailsruntime.Quit(ctx)
	select {
	case <-p.pageReady:
		p.report.PageReady = true
	case <-time.After(10 * time.Second):
		p.report.Errors = append(p.report.Errors, "local page did not initialize")
		return
	}
	p.manager.Startup(ctx, func(string, ...interface{}) {})
	for _, window := range p.manager.GetWindowState().Windows {
		if !window.Ready {
			p.report.Errors = append(p.report.Errors, window.Error)
		}
	}
	p.poll("startup-passive", func(o probe.HostObservation) bool {
		return o.Panel && o.NonActivating && !o.CanBecomeKey && !o.Key && o.ForegroundRestored
	})
	if !p.acknowledge("startup-passive") {
		return
	}
	if err := p.manager.RestoreFocus(); err != nil {
		p.report.Errors = append(p.report.Errors, err.Error())
	}
	// Always attempt to return focus even if the local page cannot finish.
	defer p.manager.RemoveFocus()
	p.poll("explicit-editor", func(o probe.HostObservation) bool { return o.CanBecomeKey && o.Key && o.ForegroundIsApp })
	wailsruntime.EventsEmit(ctx, "probe-edit")
	select {
	case ok := <-p.textResult:
		p.report.TextRoundTrip = ok
	case <-time.After(3 * time.Second):
		p.report.Errors = append(p.report.Errors, "editor bridge timed out")
	}
	if !p.acknowledge("explicit-editor") {
		return
	}
	if err := p.manager.RemoveFocus(); err != nil {
		p.report.Errors = append(p.report.Errors, err.Error())
	}
	p.poll("returned-passive", func(o probe.HostObservation) bool { return !o.CanBecomeKey && !o.Key && o.ForegroundRestored })
	p.acknowledge("returned-passive")
}
func runHost(observer func(string) error) (probe.HostReport, error) {
	p := &hostProbe{before: int(C.QSHostForeground()), manager: state.NewStateManager(), pageReady: make(chan struct{}, 1), textResult: make(chan bool, 1), done: make(chan struct{}), observer: observer}
	p.report.System = C.GoString(C.QSHostSystem())
	p.report.Host = "Wails v2.12.0 with pinned passive-panel patch"
	err := wails.Run(&options.App{Title: "Q-solver isolated host check", Width: 460, Height: 220, StartHidden: true,
		AssetServer: &assetserver.Options{Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte(hostHTML)}}},
		OnStartup: func(ctx context.Context) {
			wailsruntime.EventsOn(ctx, "probe-page-ready", func(...interface{}) {
				select {
				case p.pageReady <- struct{}{}:
				default:
				}
			})
			wailsruntime.EventsOn(ctx, "probe-text", func(values ...interface{}) {
				ok := len(values) == 2 && values[0] == "隔离输入 fixture" && values[1] == true
				select {
				case p.textResult <- ok:
				default:
				}
			})
			// Starting here also bounds a failed page load; OnDomReady would
			// never fire in that case and leave the isolated host running.
			go p.exercise(ctx)
		},
	})
	if err == nil {
		select {
		case <-p.done:
		case <-time.After(time.Second):
			err = fmt.Errorf("host exited before exercise finished")
		}
	}
	return p.report, err
}

const hostHTML = `<!doctype html><html><meta charset="utf-8"><style>body{font:15px -apple-system;background:#182332;color:white;padding:16px}input{padding:10px;width:90%}</style><b>原生宿主隔离自检</b><p>短暂进入编辑模式，然后恢复原应用。</p><input id="editor" aria-label="隔离编辑字段"><script>
function setup(){if(!window.runtime?.EventsOn){setTimeout(setup,10);return;}
window.runtime.EventsOn('probe-edit',()=>{const e=document.getElementById('editor');const end=performance.now()+1500;function check(){e.focus();e.value='隔离输入 fixture';const focused=document.activeElement===e&&document.hasFocus();if(!focused&&performance.now()<end){setTimeout(check,10);return;}window.runtime.EventsEmit('probe-text',e.value,focused);}check();});
window.runtime.EventsEmit('probe-page-ready');}setup();
</script></html>`
