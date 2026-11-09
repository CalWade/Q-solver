//go:build windows

package platform

import (
	"errors"
	"os"
	"runtime"
	"unsafe"
)

var (
	procSetLastError               = kernel32.NewProc("SetLastError")
	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
	procIsIconic                   = user32.NewProc("IsIconic")
	procGetWindowDisplayAffinity   = user32.NewProc("GetWindowDisplayAffinity")
	procGetLayeredWindowAttributes = user32.NewProc("GetLayeredWindowAttributes")
	windowsWindowState             = newWindowsWindowController(win32WindowAPI{}, uint32(os.Getpid()))
)

type win32WindowAPI struct{}

func (win32WindowAPI) Window(hwnd uintptr) (windowsWindowInfo, error) {
	if valid, _, _ := procIsWindow.Call(hwnd); valid == 0 {
		return windowsWindowInfo{}, errors.New("窗口句柄已失效")
	}
	var process uint32
	if thread, _, err := procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&process))); thread == 0 {
		return windowsWindowInfo{}, windowsCallFailure("查询窗口进程", err)
	}
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	minimized, _, _ := procIsIconic.Call(hwnd)
	return windowsWindowInfo{Handle: hwnd, ProcessID: process, Visible: visible != 0, Minimized: minimized != 0}, nil
}

func (api win32WindowAPI) owned(hwnd uintptr) error {
	window, err := api.Window(hwnd)
	if err != nil {
		return err
	}
	if window.ProcessID != uint32(os.Getpid()) {
		return errors.New("窗口不属于当前进程")
	}
	return nil
}

func (api win32WindowAPI) Foreground() windowsWindowInfo {
	hwnd, _, _ := procGetForegroundWindow.Call()
	window, _ := api.Window(hwnd)
	return window
}

func (api win32WindowAPI) ReadStyle(hwnd uintptr, index int32) (uint32, error) {
	if err := api.owned(hwnd); err != nil {
		return 0, err
	}
	if err := procGetWindowLongW.Find(); err != nil {
		return 0, err
	}
	// Last-error belongs to the OS thread, including a successful zero result.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	procSetLastError.Call(0)
	result, _, err := procGetWindowLongW.Call(hwnd, uintptr(index))
	if err := windowsStyleCallError(result, err); err != nil {
		return 0, windowsCallFailure("读取窗口样式", err)
	}
	return uint32(result), nil
}

func (api win32WindowAPI) WriteStyle(hwnd uintptr, index int32, style uint32) error {
	if err := api.owned(hwnd); err != nil {
		return err
	}
	if err := procSetWindowLongW.Find(); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	procSetLastError.Call(0)
	previous, _, err := procSetWindowLongW.Call(hwnd, uintptr(index), uintptr(style))
	if err := windowsStyleCallError(previous, err); err != nil {
		return windowsCallFailure("更新窗口样式", err)
	}
	return nil
}

func (api win32WindowAPI) Position(hwnd, after uintptr, flags uint32) error {
	if err := api.owned(hwnd); err != nil {
		return err
	}
	if ok, _, err := procSetWindowPos.Call(hwnd, after, 0, 0, 0, 0, uintptr(flags)); ok == 0 {
		return windowsCallFailure("更新窗口位置与样式", err)
	}
	return nil
}

func (api win32WindowAPI) Show(hwnd uintptr, visible bool) error {
	if err := api.owned(hwnd); err != nil {
		return err
	}
	var command uintptr
	if visible {
		command = 4 // SW_SHOWNOACTIVATE
	}
	// ShowWindow returns the previous visibility, not a success indicator.
	procShowWindow.Call(hwnd, command)
	return nil
}

func (api win32WindowAPI) SetAlpha(hwnd uintptr, alpha byte) error {
	if err := api.owned(hwnd); err != nil {
		return err
	}
	var color, flags uint32
	var previous byte
	if ok, _, _ := procGetLayeredWindowAttributes.Call(hwnd, uintptr(unsafe.Pointer(&color)), uintptr(unsafe.Pointer(&previous)), uintptr(unsafe.Pointer(&flags))); ok == 0 {
		color, flags = 0, 0
	}
	if ok, _, err := procSetLayeredWindowAttributes.Call(hwnd, uintptr(color), uintptr(alpha), uintptr(flags|LWA_ALPHA)); ok == 0 {
		return windowsCallFailure("设置窗口透明度", err)
	}
	return nil
}

func (api win32WindowAPI) Alpha(hwnd uintptr) (byte, error) {
	if err := api.owned(hwnd); err != nil {
		return 0, err
	}
	var color, flags uint32
	var alpha byte
	if ok, _, err := procGetLayeredWindowAttributes.Call(hwnd, uintptr(unsafe.Pointer(&color)), uintptr(unsafe.Pointer(&alpha)), uintptr(unsafe.Pointer(&flags))); ok == 0 {
		return 0, windowsCallFailure("读取窗口透明度", err)
	}
	if flags&LWA_ALPHA == 0 {
		return 0, errors.New("窗口透明度属性未生效")
	}
	return alpha, nil
}

func (api win32WindowAPI) SetAffinity(hwnd uintptr, affinity uint32) error {
	if err := api.owned(hwnd); err != nil {
		return err
	}
	if ok, _, err := procSetWindowDisplayAffinity.Call(hwnd, uintptr(affinity)); ok == 0 {
		return windowsCallFailure("设置窗口捕获属性", err)
	}
	return nil
}

func (api win32WindowAPI) Affinity(hwnd uintptr) (uint32, error) {
	if err := api.owned(hwnd); err != nil {
		return 0, err
	}
	var affinity uint32
	if ok, _, err := procGetWindowDisplayAffinity.Call(hwnd, uintptr(unsafe.Pointer(&affinity))); ok == 0 {
		return 0, windowsCallFailure("读取窗口捕获属性", err)
	}
	return affinity, nil
}

func (api win32WindowAPI) Activate(expected windowsWindowInfo) error {
	actual, err := api.Window(expected.Handle)
	if err != nil {
		return err
	}
	if actual.ProcessID != expected.ProcessID || !actual.Visible || actual.Minimized {
		return errors.New("原窗口已关闭、隐藏或变化")
	}
	if ok, _, err := procSetForegroundWindow.Call(expected.Handle); ok == 0 {
		return windowsCallFailure("切换窗口焦点", err)
	}
	if api.Foreground().Handle != expected.Handle {
		return errors.New("系统未确认目标窗口已获得焦点")
	}
	return nil
}

func ReleaseWindowState(hwnd uintptr) error { return windowsWindowState.Release(hwnd) }
