//go:build darwin

package native

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit -framework ApplicationServices
#include "input_darwin.h"
*/
import "C"

import (
	"Q-Solver/internal/input/application"
	"Q-Solver/internal/input/domain"
	"Q-Solver/internal/input/infrastructure/keymap"
	"fmt"
	"runtime"
	"runtime/cgo"
	"strings"
	"sync"
	"unsafe"

	"golang.design/x/hotkey"
)

type registration struct {
	hotkey   *hotkey.Hotkey
	revision uint64
	action   string
	repeat   bool
	done     chan struct{}
}

type Driver struct {
	mu            sync.Mutex
	callbacks     application.Callbacks
	context       unsafe.Pointer
	handle        cgo.Handle
	ready         chan struct{}
	done          chan struct{}
	seed          []domain.Key
	registrations map[string]*registration
	listeners     sync.WaitGroup
	diagnosticTag uint64
}

func New() application.Driver { return &Driver{} }

// NewTaggedDiagnostic scopes a native integration check to its own synthetic
// events. Production callers use New, which handles the full input stream.
func NewTaggedDiagnostic(tag uint64) (application.Driver, error) {
	if tag == 0 {
		return nil, fmt.Errorf("diagnostic input requires a nonzero event tag")
	}
	return &Driver{diagnosticTag: tag}, nil
}
func RequestPermission() bool { return bool(C.qs_input_permission(C.bool(true))) }

func (d *Driver) Start(callbacks application.Callbacks) (application.Capabilities, error) {
	d.callbacks = callbacks
	d.registrations = map[string]*registration{}
	d.ready, d.done = make(chan struct{}), make(chan struct{})
	d.seed = nil
	d.handle = cgo.NewHandle(d)
	d.context = C.qs_input_create(C.uintptr_t(d.handle), C.uint64_t(d.diagnosticTag))
	if d.context == nil {
		d.handle.Delete()
		close(d.done)
		return application.Capabilities{Filtering: false, Backend: "macos-system-hotkeys", Reason: "请在系统设置允许辅助功能访问，然后重试键盘服务"}, nil
	}
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(d.done)
		C.qs_input_run(d.context)
	}()
	<-d.ready
	return application.Capabilities{Filtering: true, Backend: "macos-event-tap-and-hotkeys"}, nil
}

func (d *Driver) CanRegister(binding domain.Binding) bool {
	_, key, ok := keymap.SystemChord(binding)
	if !ok {
		return false
	}
	_, ok = keymap.MacCode(key)
	return ok
}

func (d *Driver) Apply(plan application.NativePlan) error {
	if d.diagnosticTag != 0 && len(plan.Bindings) != 0 {
		return fmt.Errorf("diagnostic input must use scoped event filtering without system hotkey registration")
	}
	if err := validatePlan(plan, d.CanRegister); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	next := map[string]*registration{}
	var created []*registration
	for _, binding := range plan.Bindings {
		parts := make([]string, len(binding.Keys))
		for i, k := range binding.Keys {
			parts[i] = string(k)
		}
		signature := strings.Join(parts, "+")
		if existing, ok := d.registrations[signature]; ok {
			next[signature] = existing
			continue
		}
		mods, key, ok := keymap.SystemChord(binding)
		if !ok {
			return fmt.Errorf("无效的系统快捷键 %s", binding.Action)
		}
		var nativeMods []hotkey.Modifier
		for _, mod := range mods {
			switch mod {
			case domain.Control:
				nativeMods = append(nativeMods, hotkey.ModCtrl)
			case domain.Meta:
				nativeMods = append(nativeMods, hotkey.ModCmd)
			case domain.Alt:
				nativeMods = append(nativeMods, hotkey.ModOption)
			case domain.Shift:
				nativeMods = append(nativeMods, hotkey.ModShift)
			}
		}
		code, _ := keymap.MacCode(key)
		h := hotkey.New(nativeMods, hotkey.Key(code))
		if err := h.Register(); err != nil {
			for _, entry := range created {
				entry.hotkey.Unregister()
			}
			return fmt.Errorf("快捷键 %s 注册失败，可能已被占用：%w", binding.Action, err)
		}
		entry := &registration{hotkey: h, done: make(chan struct{})}
		created = append(created, entry)
		next[signature] = entry
	}
	for _, binding := range plan.Bindings {
		parts := make([]string, len(binding.Keys))
		for i, k := range binding.Keys {
			parts[i] = string(k)
		}
		entry := next[strings.Join(parts, "+")]
		entry.revision, entry.action, entry.repeat = plan.Revision, binding.Action, binding.Repeat
	}
	for signature, entry := range d.registrations {
		if _, keep := next[signature]; !keep {
			entry.hotkey.Unregister()
			close(entry.done)
		}
	}
	d.registrations = next
	for _, entry := range created {
		d.listeners.Add(1)
		go d.listen(entry)
	}
	return nil
}

func (d *Driver) listen(entry *registration) {
	defer d.listeners.Done()
	pressed := false
	for {
		select {
		case <-entry.done:
			return
		case <-entry.hotkey.Keyup():
			pressed = false
		case <-entry.hotkey.Keydown():
			d.mu.Lock()
			revision, action, repeat := entry.revision, entry.action, entry.repeat
			d.mu.Unlock()
			if !pressed || repeat {
				d.callbacks.Shortcut(revision, action)
			}
			pressed = true
		}
	}
}

func (d *Driver) Stop() error {
	d.mu.Lock()
	for _, entry := range d.registrations {
		entry.hotkey.Unregister()
		close(entry.done)
	}
	d.registrations = map[string]*registration{}
	d.mu.Unlock()
	d.listeners.Wait()
	if d.context != nil {
		C.qs_input_stop(d.context)
		<-d.done
		C.qs_input_destroy(d.context)
		d.handle.Delete()
		d.context = nil
	}
	return nil
}

//export goQSInput
func goQSInput(handle C.uintptr_t, code C.uint32_t, down, repeat, injected, own C.int) C.uint64_t {
	d := cgo.Handle(handle).Value().(*Driver)
	key := keymap.Mac(uint32(code))
	if key == "" {
		key = domain.Key(fmt.Sprintf("NativeKey%d", uint32(code)))
	}
	decision := d.callbacks.Event(domain.Event{Key: key, Down: down != 0, Repeat: repeat != 0, Injected: injected != 0, OwnWindow: own != 0})
	result := uint64(decision.VisibleModifiers)<<8 | uint64(decision.VisibleSides)<<16
	if decision.Suppress {
		result |= 1
	}
	if decision.RewriteModifiers {
		result |= 2
	}
	return C.uint64_t(result)
}

//export goQSInputReady
func goQSInputReady(handle C.uintptr_t) {
	d := cgo.Handle(handle).Value().(*Driver)
	d.callbacks.Seed(d.seed)
	close(d.ready)
}

//export goQSInputSeed
func goQSInputSeed(handle C.uintptr_t, code C.uint32_t) {
	d := cgo.Handle(handle).Value().(*Driver)
	if key := keymap.Mac(uint32(code)); key != "" {
		d.seed = append(d.seed, key)
	}
}

//export goQSInputFailed
func goQSInputFailed(handle C.uintptr_t) {
	d := cgo.Handle(handle).Value().(*Driver)
	d.callbacks.Failed(fmt.Errorf("系统键盘过滤已中断，请重试键盘服务"))
}
