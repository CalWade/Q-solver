//go:build windows

package native

import (
	"Q-Solver/internal/input/application"
	"Q-Solver/internal/input/domain"
	"Q-Solver/internal/input/infrastructure/keymap"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var user32 = syscall.NewLazyDLL("user32.dll")
var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var setHook = user32.NewProc("SetWindowsHookExW")
var unhook = user32.NewProc("UnhookWindowsHookEx")
var nextHook = user32.NewProc("CallNextHookEx")
var getMessage = user32.NewProc("GetMessageW")
var peekMessage = user32.NewProc("PeekMessageW")
var postMessage = user32.NewProc("PostThreadMessageW")
var registerHotKey = user32.NewProc("RegisterHotKey")
var unregisterHotKey = user32.NewProc("UnregisterHotKey")
var getThreadID = kernel32.NewProc("GetCurrentThreadId")
var getModule = kernel32.NewProc("GetModuleHandleW")
var getForeground = user32.NewProc("GetForegroundWindow")
var getWindowPID = user32.NewProc("GetWindowThreadProcessId")
var getAsyncKey = user32.NewProc("GetAsyncKeyState")
var mapVirtualKey = user32.NewProc("MapVirtualKeyW")

const applyMessage = 0x8001

type message struct {
	Window         uintptr
	ID             uint32
	WParam, LParam uintptr
	Time           uint32
	X, Y           int32
	Private        uint32
}
type keyboardEvent struct {
	VK, Scan, Flags, Time uint32
	Extra                 uintptr
}
type mouseEvent struct {
	X, Y              int32
	Data, Flags, Time uint32
	Extra             uintptr
}
type registration struct {
	id        int
	revision  uint64
	action    string
	flags, vk uint32
}
type applyRequest struct {
	plan   application.NativePlan
	result chan error
}

type Driver struct {
	mu            sync.Mutex
	callbacks     application.Callbacks
	threadID      uintptr
	done          chan struct{}
	requests      chan applyRequest
	registrations map[string]registration
	nextID        int
}

func New() application.Driver { return &Driver{} }

func RequestPermission() bool { return true }

func (d *Driver) Start(callbacks application.Callbacks) (application.Capabilities, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.done != nil {
		select {
		case <-d.done:
		default:
			return application.Capabilities{}, fmt.Errorf("键盘监听已启动")
		}
	}
	d.callbacks = callbacks
	d.done = make(chan struct{})
	d.requests = make(chan applyRequest, 1)
	d.registrations = map[string]registration{}
	d.nextID = 1
	ready := make(chan error, 1)
	go d.run(ready)
	err := <-ready
	if err != nil {
		<-d.done
	}
	return application.Capabilities{Filtering: err == nil, Backend: "windows-hook-and-hotkeys"}, err
}

func (d *Driver) CanRegister(binding domain.Binding) bool {
	_, key, ok := keymap.SystemChord(binding)
	if !ok || key == "F12" {
		return false
	}
	_, ok = keymap.WindowsVK(key)
	return ok
}

func (d *Driver) Apply(plan application.NativePlan) error {
	if err := validatePlan(plan, d.CanRegister); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.done == nil {
		return fmt.Errorf("键盘监听尚未启动")
	}
	select {
	case <-d.done:
		return fmt.Errorf("键盘监听线程已停止")
	default:
	}
	req := applyRequest{plan: plan, result: make(chan error, 1)}
	select {
	case <-d.done:
		return fmt.Errorf("键盘监听线程已停止")
	case d.requests <- req:
	}
	if ok, _, err := postMessage.Call(d.threadID, applyMessage, 0, 0); ok == 0 {
		select {
		case <-d.requests:
		default:
		}
		return fmt.Errorf("更新键盘规则失败：%w", err)
	}
	select {
	case err := <-req.result:
		return err
	case <-d.done:
		return fmt.Errorf("键盘监听线程已停止")
	}
}

func (d *Driver) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.done == nil {
		return nil
	}
	select {
	case <-d.done:
		return nil
	default:
	}
	if ok, _, err := postMessage.Call(d.threadID, 0x12, 0, 0); ok == 0 {
		return fmt.Errorf("停止键盘线程失败：%w", err)
	}
	<-d.done
	return nil
}

func ownWindow() bool {
	h, _, _ := getForeground.Call()
	var pid uint32
	getWindowPID.Call(h, uintptr(unsafe.Pointer(&pid)))
	return pid == uint32(os.Getpid())
}

func (d *Driver) run(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(d.done)
	d.threadID, _, _ = getThreadID.Call()
	var msg message
	peekMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 0)
	module, _, _ := getModule.Call(0)
	keyboardCallback := syscall.NewCallback(func(code int32, w, l uintptr) uintptr {
		if code >= 0 {
			e := (*keyboardEvent)(unsafe.Pointer(l))
			key := keymap.Windows(e.VK, e.Scan, e.Flags&1 != 0)
			if key == "" {
				key = domain.Key(fmt.Sprintf("NativeKey%d", e.VK))
			}
			decision := d.callbacks.Event(domain.Event{Key: key, Down: w == 0x100 || w == 0x104, Injected: e.Flags&0x10 != 0, OwnWindow: ownWindow()})
			if decision.Suppress {
				return 1
			}
		}
		result, _, _ := nextHook.Call(0, uintptr(code), w, l)
		return result
	})
	hook, _, err := setHook.Call(13, keyboardCallback, module, 0)
	if hook == 0 {
		ready <- fmt.Errorf("安装键盘过滤器失败：%w", err)
		return
	}
	defer unhook.Call(hook)
	mouseCallback := syscall.NewCallback(func(code int32, w, l uintptr) uintptr {
		if code >= 0 && (w == 0x20b || w == 0x20c) {
			e := (*mouseEvent)(unsafe.Pointer(l))
			key := domain.Key("MouseBack")
			if e.Data>>16 == 2 {
				key = "MouseForward"
			}
			if d.callbacks.Event(domain.Event{Key: key, Down: w == 0x20b, Injected: e.Flags&1 != 0, OwnWindow: ownWindow()}).Suppress {
				return 1
			}
		}
		result, _, _ := nextHook.Call(0, uintptr(code), w, l)
		return result
	})
	mouse, _, err := setHook.Call(14, mouseCallback, module, 0)
	if mouse == 0 {
		ready <- fmt.Errorf("安装鼠标快捷键监听失败：%w", err)
		return
	}
	defer unhook.Call(mouse)
	var held []domain.Key
	for vk := uint32(1); vk < 255; vk++ {
		if vk == 16 || vk == 17 || vk == 18 {
			continue
		}
		state, _, _ := getAsyncKey.Call(uintptr(vk))
		if state&0x8000 != 0 {
			scan, _, _ := mapVirtualKey.Call(uintptr(vk), 0)
			if key := keymap.Windows(vk, uint32(scan), vk == 163 || vk == 165); key != "" {
				held = append(held, key)
			}
		}
	}
	d.callbacks.Seed(held)
	defer func() {
		for _, r := range d.registrations {
			unregisterHotKey.Call(0, uintptr(r.id))
		}
	}()
	ready <- nil
	for {
		result, _, err := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(result) == -1 {
			d.callbacks.Failed(fmt.Errorf("键盘消息循环失败：%w", err))
			return
		}
		if result == 0 {
			return
		}
		switch msg.ID {
		case applyMessage:
			select {
			case req := <-d.requests:
				req.result <- d.apply(req.plan)
			default:
			}
		case 0x312:
			for _, r := range d.registrations {
				if r.id == int(msg.WParam) {
					d.callbacks.Shortcut(r.revision, r.action)
					break
				}
			}
		}
	}
}

func (d *Driver) apply(plan application.NativePlan) error {
	next := map[string]registration{}
	var created []int
	var changed []registration
	rollback := func() {
		for _, id := range created {
			unregisterHotKey.Call(0, uintptr(id))
		}
		for _, old := range changed {
			unregisterHotKey.Call(0, uintptr(old.id))
			if ok, _, err := registerHotKey.Call(0, uintptr(old.id), uintptr(old.flags), uintptr(old.vk)); ok == 0 {
				d.callbacks.Failed(fmt.Errorf("快捷键恢复失败：%w", err))
			}
		}
	}
	for _, binding := range plan.Bindings {
		mods, key, ok := keymap.SystemChord(binding)
		if !ok {
			return fmt.Errorf("无效的系统快捷键 %s", binding.Action)
		}
		parts := make([]string, len(binding.Keys))
		for i, k := range binding.Keys {
			parts[i] = string(k)
		}
		signature := strings.Join(parts, "+")
		var flags uint32
		for _, mod := range mods {
			switch mod {
			case domain.Control:
				flags |= 2
			case domain.Alt:
				flags |= 1
			case domain.Shift:
				flags |= 4
			case domain.Meta:
				flags |= 8
			}
		}
		if !binding.Repeat {
			flags |= 0x4000
		}
		vk, _ := keymap.WindowsVK(key)
		if scan, ok := keymap.WindowsScan(key); ok {
			mapped, _, _ := mapVirtualKey.Call(uintptr(scan), 3)
			if mapped != 0 {
				vk = uint32(mapped)
			}
		}
		id := d.nextID
		existing, reused := d.registrations[signature]
		if reused {
			id = existing.id
			if existing.flags == flags && existing.vk == vk {
				existing.revision, existing.action = plan.Revision, binding.Action
				next[signature] = existing
				continue
			}
			unregisterHotKey.Call(0, uintptr(id))
			changed = append(changed, existing)
		} else {
			d.nextID++
		}
		if success, _, err := registerHotKey.Call(0, uintptr(id), uintptr(flags), uintptr(vk)); success == 0 {
			rollback()
			return fmt.Errorf("快捷键 %s 注册失败，可能已被占用：%w", binding.Action, err)
		}
		if !reused {
			created = append(created, id)
		}
		next[signature] = registration{id: id, revision: plan.Revision, action: binding.Action, flags: flags, vk: vk}
	}
	for signature, r := range d.registrations {
		if _, keep := next[signature]; !keep {
			unregisterHotKey.Call(0, uintptr(r.id))
		}
	}
	d.registrations = next
	return nil
}
