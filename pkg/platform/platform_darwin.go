//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework AppKit -framework CoreGraphics -framework AVFoundation -framework QuartzCore

#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>
#import <QuartzCore/QuartzCore.h>
#import <AVFoundation/AVFoundation.h>

// Implemented by the pinned native host; ordinary NSWindow cannot provide this gate.
@protocol QSEditingWindow
- (void)setQSEditable:(BOOL)editable;
@end

static NSRunningApplication *qsPreviousApplication;
static void QSWindowOnMain(dispatch_block_t block) {
    if ([NSThread isMainThread]) block();
    else dispatch_sync(dispatch_get_main_queue(), block);
}
static bool QSWindowValid(NSWindow *window) {
    return window && [NSApp.windows indexOfObjectIdenticalTo:window] != NSNotFound;
}

// 检查截图权限 (macOS 10.15+)
bool CheckScreenCaptureAccessC() {
    if (@available(macOS 10.15, *)) {
        return CGPreflightScreenCaptureAccess();
    }
    return true; // macOS 10.15 以下不需要权限
}

// 请求截图权限 (macOS 10.15+)
bool RequestScreenCaptureAccessC() {
    if (@available(macOS 10.15, *)) {
        return CGRequestScreenCaptureAccess();
    }
    return true; // macOS 10.15 以下不需要权限
}

// 获取应用主窗口
void* GetMainWindowC() {
    __block NSWindow *window = nil;
    QSWindowOnMain(^{
        NSApplication *app = NSApplication.sharedApplication;
        window = app.mainWindow;
        if (!window && app.windows.count) window = app.windows[0];
    });
    return (__bridge void*)window;
}

// 设置窗口忽略鼠标事件（鼠标穿透）
void SetWindowIgnoresMouseEventsC(void* nsWindow, bool ignores) {
    if (nsWindow == NULL) return;
    NSWindow* window = (__bridge NSWindow*)nsWindow;
    QSWindowOnMain(^{
        [window setIgnoresMouseEvents:ignores];
    });
}

// 设置窗口级别（置顶）
void SetWindowLevelC(void* nsWindow, int level) {
    if (nsWindow == NULL) return;
    NSWindow* window = (__bridge NSWindow*)nsWindow;
    QSWindowOnMain(^{
        [window setLevel:level];
    });
}

// 设置窗口样式（无边框 + 圆角）
void SetWindowStyleMaskBorderlessC(void* nsWindow) {
    if (nsWindow == NULL) return;
    NSWindow* window = (__bridge NSWindow*)nsWindow;
    QSWindowOnMain(^{
        // 保留 titled 样式以维持输入法上下文（IMK），同时隐藏标题栏
        NSWindowStyleMask panelMode = window.styleMask & NSWindowStyleMaskNonactivatingPanel;
        [window setStyleMask:(NSWindowStyleMaskTitled |
                              NSWindowStyleMaskFullSizeContentView | panelMode)];
        [window setTitlebarAppearsTransparent:YES];
        [window setTitleVisibility:NSWindowTitleHidden];
        // 隐藏标题栏按钮
        [[window standardWindowButton:NSWindowCloseButton] setHidden:YES];
        [[window standardWindowButton:NSWindowMiniaturizeButton] setHidden:YES];
        [[window standardWindowButton:NSWindowZoomButton] setHidden:YES];
        // 透明背景
        [window setBackgroundColor:[NSColor clearColor]];
        [window setOpaque:NO];
        // 允许透明
        [window setHasShadow:YES];
    });
}

// Applies the legacy sharing attribute; capture-path compatibility is separate.
bool SetWindowSharingTypeC(void* nsWindow, int sharingType) {
    __block bool applied = false;
    NSWindow *window = (__bridge NSWindow*)nsWindow;
    QSWindowOnMain(^{
        if (!QSWindowValid(window)) return;
        window.sharingType = (NSWindowSharingType)sharingType;
        applied = window.sharingType == sharingType;
    });
    return applied;
}

bool SetWindowVisibleC(void* nsWindow, bool visible) {
    __block bool applied = false;
    NSWindow *window = (__bridge NSWindow*)nsWindow;
    QSWindowOnMain(^{
        if (!QSWindowValid(window)) return;
        if (visible) [window orderFrontRegardless]; else [window orderOut:nil];
        applied = window.visible == visible;
    });
    return applied;
}

bool SetCaptureSuppressedC(void* nsWindow, bool suppressed) {
    __block bool applied = false;
    NSWindow *window = (__bridge NSWindow*)nsWindow;
    QSWindowOnMain(^{
        if (!QSWindowValid(window)) return;
        window.alphaValue = suppressed ? 0.0 : 1.0;
        [CATransaction flush];
        applied = window.alphaValue == (suppressed ? 0.0 : 1.0);
    });
    return applied;
}

bool WindowValidC(void *nsWindow) {
    __block bool valid = false;
    QSWindowOnMain(^{ valid = QSWindowValid((__bridge NSWindow*)nsWindow); });
    return valid;
}

// 设置窗口不激活
bool SetWindowNotActivatingC(void* nsWindow, bool noActivate) {
    if (nsWindow == NULL) return false;
    NSWindow* window = (__bridge NSWindow*)nsWindow;
    __block bool applied = false;
    QSWindowOnMain(^{
        if (!QSWindowValid(window)) return;
        if (noActivate) {
            if ([window respondsToSelector:@selector(setQSEditable:)]) {
                [(id<QSEditingWindow>)window setQSEditable:NO];
            }
            [window setCollectionBehavior:(window.collectionBehavior | NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorFullScreenAuxiliary)];
            BOOL ownedKey = window.keyWindow;
            if (ownedKey) [window resignKeyWindow];
            if (ownedKey && NSApp.active && qsPreviousApplication && !qsPreviousApplication.terminated) {
                [qsPreviousApplication activateWithOptions:0];
            }
            [qsPreviousApplication release];
            qsPreviousApplication = nil;
        }
        applied = !noActivate || (!window.canBecomeKeyWindow && !window.keyWindow);
    });
    return applied;
}

// 让窗口可以接收键盘焦点
bool SetWindowCanBecomeKeyC(void* nsWindow) {
    if (nsWindow == NULL) return false;
    NSWindow* window = (__bridge NSWindow*)nsWindow;
    __block bool applied = false;
    QSWindowOnMain(^{
        if (!QSWindowValid(window)) return;
        if ([window respondsToSelector:@selector(setQSEditable:)]) {
            [(id<QSEditingWindow>)window setQSEditable:YES];
        }
        NSRunningApplication *front = NSWorkspace.sharedWorkspace.frontmostApplication;
        if (front.processIdentifier != NSProcessInfo.processInfo.processIdentifier) {
            [qsPreviousApplication release];
            qsPreviousApplication = [front retain];
        }
        [NSApp activateIgnoringOtherApps:YES];
        [window makeKeyAndOrderFront:nil];
        applied = window.canBecomeKeyWindow;
    });
    return applied;
}

// 设置窗口接受鼠标事件时自动成为 key window
void SetWindowAcceptsMouseMovedC(void* nsWindow) {
    if (nsWindow == NULL) return;
    NSWindow* window = (__bridge NSWindow*)nsWindow;
    QSWindowOnMain(^{
        [window setAcceptsMouseMovedEvents:YES];
    });
}

// 设置 contentView 圆角
void SetWindowCornerRadiusC(void* nsWindow, float radius) {
    if (nsWindow == NULL) return;
    NSWindow* window = (__bridge NSWindow*)nsWindow;
    QSWindowOnMain(^{
        // 设置 contentView 层的圆角
        NSView* contentView = [window contentView];
        if (contentView) {
            [contentView setWantsLayer:YES];
            contentView.layer.cornerRadius = radius;
            contentView.layer.masksToBounds = YES;
        }
    });
}

// 设置窗口透明度 (0.0 - 1.0)
void SetWindowAlphaC(void* nsWindow, float alpha) {
    if (nsWindow == NULL) return;
    NSWindow* window = (__bridge NSWindow*)nsWindow;
    QSWindowOnMain(^{
        [window setAlphaValue:alpha];
    });
}

// 打开系统偏好设置的屏幕录制权限页面
void OpenScreenCaptureSettingsC() {
    QSWindowOnMain(^{
        NSURL *url = [NSURL URLWithString:@"x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture"];
        [[NSWorkspace sharedWorkspace] openURL:url];
    });
}

// 打开系统偏好设置的麦克风权限页面
void OpenMicrophoneSettingsC() {
    QSWindowOnMain(^{
        NSURL *url = [NSURL URLWithString:@"x-apple.systempreferences:com.apple.preference.security?Privacy_Microphone"];
        [[NSWorkspace sharedWorkspace] openURL:url];
    });
}

// 检查麦克风权限状态
// 返回: 0=未决定, 1=已授权, 2=已拒绝
int CheckMicrophoneAccessC() {
    if (@available(macOS 10.14, *)) {
        AVAuthorizationStatus status = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
        switch (status) {
            case AVAuthorizationStatusNotDetermined:
                return 0;
            case AVAuthorizationStatusAuthorized:
                return 1;
            case AVAuthorizationStatusDenied:
            case AVAuthorizationStatusRestricted:
                return 2;
        }
    }
    return 1; // macOS 10.14 以下不需要权限
}

// 请求麦克风权限
void RequestMicrophoneAccessC(void (*callback)(bool granted)) {
    if (@available(macOS 10.14, *)) {
        [AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio completionHandler:^(BOOL granted) {
            if (callback != NULL) {
                callback(granted);
            }
        }];
    } else {
        if (callback != NULL) {
            callback(true);
        }
    }
}
*/
import "C"
import (
	"Q-Solver/pkg/logger"
	"errors"
	"unsafe"
)

// WindowHandle 窗口句柄类型（macOS 为 NSWindow 指针）
type WindowHandle uintptr

// 窗口级别常量
const (
	WindowLevelNormal   = 0 // 正常窗口级别
	WindowLevelFloating = 3 // 置顶窗口级别

	// NSWindow sharingType 常量
	nsWindowSharingNone     = 0
	nsWindowSharingReadOnly = 1
)

// GetWindowHandle 获取当前进程主窗口句柄
func GetWindowHandle() (WindowHandle, error) {
	window := C.GetMainWindowC()
	if window == nil {
		return 0, errors.New("无法获取主窗口")
	}
	return WindowHandle(uintptr(window)), nil
}

// ApplyGhostMode 应用幽灵模式（无边框、置顶、防录屏、不抢焦点）
func ApplyGhostMode(hwnd WindowHandle) error {
	if !bool(C.WindowValidC(unsafe.Pointer(uintptr(hwnd)))) {
		return errors.New("窗口句柄已失效")
	}
	window := unsafe.Pointer(uintptr(hwnd))

	// 无边框 + 透明背景
	C.SetWindowStyleMaskBorderlessC(window)

	// 设置圆角
	C.SetWindowCornerRadiusC(window, 12.0)

	// 置顶
	C.SetWindowLevelC(window, C.int(WindowLevelFloating))

	// 不抢焦点
	C.SetWindowNotActivatingC(window, C.bool(true))

	// 允许窗口在点击时自动接收鼠标事件（修复失焦后无法拖动的问题）
	C.SetWindowAcceptsMouseMovedC(window)

	logger.Println("[macOS] 窗口外观已初始化")
	return nil
}

// SetClickThrough 设置鼠标穿透
func SetClickThrough(hwnd WindowHandle, enabled bool) error {
	if !bool(C.WindowValidC(unsafe.Pointer(uintptr(hwnd)))) {
		return errors.New("窗口句柄已失效")
	}
	window := unsafe.Pointer(uintptr(hwnd))
	C.SetWindowIgnoresMouseEventsC(window, C.bool(enabled))
	return nil
}

// SetDisplayAffinity 设置防录屏状态
func SetDisplayAffinity(hwnd WindowHandle, hidden bool) error {
	_, err := ApplyCaptureProtection(hwnd, hidden)
	return err
}

func ApplyCaptureProtection(hwnd WindowHandle, hidden bool) (string, error) {
	sharing := nsWindowSharingReadOnly
	if hidden {
		sharing = nsWindowSharingNone
	}
	if !bool(C.SetWindowSharingTypeC(unsafe.Pointer(uintptr(hwnd)), C.int(sharing))) {
		return "unavailable", errors.New("系统未确认窗口捕获属性")
	}
	if !hidden {
		return "off", nil
	}
	return "nswindow-sharing-none", nil
}

func SetVisibleWithoutActivation(hwnd WindowHandle, visible bool) error {
	if !bool(C.SetWindowVisibleC(unsafe.Pointer(uintptr(hwnd)), C.bool(visible))) {
		return errors.New("窗口显示状态未生效")
	}
	return nil
}

func SetCaptureSuppressed(hwnd WindowHandle, suppressed bool) error {
	if !bool(C.SetCaptureSuppressedC(unsafe.Pointer(uintptr(hwnd)), C.bool(suppressed))) {
		return errors.New("截图期间窗口透明状态未生效")
	}
	return nil
}

// RestoreFocus 恢复焦点
func RestoreFocus(hwnd WindowHandle) error {
	if !bool(C.WindowValidC(unsafe.Pointer(uintptr(hwnd)))) {
		return errors.New("窗口句柄已失效")
	}
	window := unsafe.Pointer(uintptr(hwnd))
	if !bool(C.SetWindowCanBecomeKeyC(window)) {
		return errors.New("窗口未允许编辑焦点")
	}
	return nil
}

// RemoveFocus 移除焦点
func RemoveFocus(hwnd WindowHandle) error {
	if !bool(C.WindowValidC(unsafe.Pointer(uintptr(hwnd)))) {
		return errors.New("窗口句柄已失效")
	}
	window := unsafe.Pointer(uintptr(hwnd))
	if !bool(C.SetWindowNotActivatingC(window, C.bool(true))) {
		return errors.New("当前宿主未提供被动面板，请使用项目构建入口")
	}
	return nil
}

// CheckScreenCaptureAccess 检查截图权限 (macOS 10.15+)
func CheckScreenCaptureAccess() bool {
	return bool(C.CheckScreenCaptureAccessC())
}

// RequestScreenCaptureAccess 请求截图权限 (macOS 10.15+)
func RequestScreenCaptureAccess() bool {
	return bool(C.RequestScreenCaptureAccessC())
}

// OpenScreenCaptureSettings 打开系统设置的屏幕录制权限页面
func OpenScreenCaptureSettings() {
	C.OpenScreenCaptureSettingsC()
}

// SetWindowLevel 设置窗口层级 (用于临时取消/恢复置顶)
func SetWindowLevel(hwnd WindowHandle, level int) error {
	if !bool(C.WindowValidC(unsafe.Pointer(uintptr(hwnd)))) {
		return errors.New("窗口句柄已失效")
	}
	window := unsafe.Pointer(uintptr(hwnd))
	C.SetWindowLevelC(window, C.int(level))
	return nil
}

// CheckMicrophoneAccess 检查麦克风权限状态 (macOS 专用)
// 返回: 0=未决定, 1=已授权, 2=已拒绝
func CheckMicrophoneAccess() int {
	return int(C.CheckMicrophoneAccessC())
}

// RequestMicrophoneAccess 请求麦克风权限 (macOS 专用)
func RequestMicrophoneAccess() {
	C.RequestMicrophoneAccessC(nil)
}

// OpenMicrophoneSettings 打开系统设置的麦克风权限页面 (macOS 专用)
func OpenMicrophoneSettings() {
	C.OpenMicrophoneSettingsC()
}
