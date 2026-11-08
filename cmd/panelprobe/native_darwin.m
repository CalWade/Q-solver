//go:build darwin && cgo

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import "native_darwin.h"

extern void panelProbeCreated(uintptr_t handle);
extern void panelProbeReady(void);
extern void panelProbeTimeout(void);

// Passive output has a strict key-window gate, independent of activation.
// A production host must put editors in a separate explicit input mode/window.
@interface QSProbePanel : NSPanel
@end
@implementation QSProbePanel
- (BOOL)canBecomeKeyWindow { return NO; }
- (BOOL)canBecomeMainWindow { return NO; }
@end

@interface QSProbeWebView : WKWebView
@end
@implementation QSProbeWebView
- (BOOL)acceptsFirstMouse:(NSEvent *)event { return YES; }
- (BOOL)needsPanelToBecomeKey { return NO; }
@end

@interface QSProbeDelegate : NSObject <NSApplicationDelegate, WKScriptMessageHandler>
@property BOOL finished;
@end
static QSProbeDelegate *probeDelegate;

uintptr_t QSProbeCreate(void) {
    QSProbePanel *panel = [[QSProbePanel alloc] initWithContentRect:NSMakeRect(50, 80, 390, 170)
        styleMask:(NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel)
        backing:NSBackingStoreBuffered defer:NO];
    panel.title = @"Q-solver isolated panel probe";
    panel.hidesOnDeactivate = NO;
    panel.releasedWhenClosed = NO;
    panel.floatingPanel = YES;
    panel.becomesKeyOnlyIfNeeded = YES;
    panel.backgroundColor = [NSColor colorWithRed:0.08 green:0.11 blue:0.16 alpha:1];
    panel.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorFullScreenAuxiliary;
    return (uintptr_t)panel;
}
void QSProbeClose(uintptr_t handle) {
    NSWindow *window = (NSWindow *)handle;
    [window close]; [window release];
}
QSProbeObservation QSProbeObserve(uintptr_t handle) {
    NSWindow *window = (NSWindow *)handle;
    QSProbeObservation result = {
        [window isKindOfClass:[NSPanel class]], window.visible,
        (window.styleMask & NSWindowStyleMaskNonactivatingPanel) != 0,
        window.canBecomeKeyWindow, window.keyWindow, window.mainWindow,
        window.sharingType == NSWindowSharingNone, window.alphaValue,
        NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier
    };
    return result;
}
int QSProbeForeground(void) { return NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier; }
const char *QSProbeSystem(void) { return NSProcessInfo.processInfo.operatingSystemVersionString.UTF8String; }
void QSProbeTryKey(uintptr_t handle) { [(NSWindow *)handle makeKeyWindow]; }
void QSProbeStop(void) {
    probeDelegate.finished = YES;
    [NSApp stop:nil];
    NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSZeroPoint
        modifierFlags:0 timestamp:0 windowNumber:0 context:nil subtype:0 data1:0 data2:0];
    [NSApp postEvent:event atStart:NO];
}

@implementation QSProbeDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    uintptr_t handle = QSProbeCreate();
    NSPanel *panel = (NSPanel *)handle;
    WKWebViewConfiguration *configuration = [[WKWebViewConfiguration alloc] init];
    [configuration.userContentController addScriptMessageHandler:self name:@"fixture"];
    QSProbeWebView *view = [[QSProbeWebView alloc] initWithFrame:panel.contentView.bounds configuration:configuration];
    panel.contentView = view;
    [view loadHTMLString:@"<!doctype html><meta charset='utf-8'><style>body{font:16px -apple-system;background:#141d2b;color:white;padding:16px}</style><b>原生面板隔离自检</b><p>正在验证窗口生命周期，完成后自动关闭。</p><script>window.webkit.messageHandlers.fixture.postMessage('ready')</script>" baseURL:nil];
    [view release]; [configuration release];
    panelProbeCreated(handle);
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC), dispatch_get_main_queue(), ^{
        if (!self.finished) panelProbeTimeout();
    });
}
- (void)userContentController:(WKUserContentController *)controller didReceiveScriptMessage:(WKScriptMessage *)message {
    if (self.finished || ![message.body isEqual:@"ready"]) return;
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 250 * NSEC_PER_MSEC), dispatch_get_main_queue(), ^{
        if (!self.finished) panelProbeReady();
    });
}
@end

void QSProbeRun(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
        probeDelegate = [[QSProbeDelegate alloc] init];
        NSApp.delegate = probeDelegate;
        [NSApp run];
        NSApp.delegate = nil;
        [probeDelegate release]; probeDelegate = nil;
    }
}
