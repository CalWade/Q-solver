//go:build darwin && cgo

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <ImageIO/ImageIO.h>
#import "native_darwin.h"
#import "capture_darwin.h"

extern void panelProbeCreated(uintptr_t handle);
extern void panelProbeReady(void);
extern void panelProbeTimeout(void);
extern void panelProbeCaptured(int index, int width, int height, double red, double green, double black, double transparent, int errorCode);

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

static void captureResultWithMetrics(int index, int width, int height, double red, double green, int errorCode, double black, double transparent) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (probeDelegate && !probeDelegate.finished) panelProbeCaptured(index, width, height, red, green, black, transparent, errorCode);
    });
}
static void captureResult(int index, int width, int height, double red, double green, int errorCode) {
    captureResultWithMetrics(index, width, height, red, green, errorCode, 0, 0);
}

QSProbePixels QSProbeReadImage(CGImageRef image, NSString *filename) {
    QSProbePixels result = {0};
    size_t width = CGImageGetWidth(image), height = CGImageGetHeight(image);
    if (!width || !height) { result.error = -10004; return result; }
    NSString *directory = @".local/reports";
    [[NSFileManager defaultManager] createDirectoryAtPath:directory withIntermediateDirectories:YES attributes:nil error:nil];
    NSURL *url = [NSURL fileURLWithPath:[directory stringByAppendingPathComponent:filename]];
    CGImageDestinationRef destination = CGImageDestinationCreateWithURL((CFURLRef)url, CFSTR("public.png"), 1, NULL);
    if (!destination) { result.error = -10006; return result; }
    CGImageDestinationAddImage(destination, image, NULL);
    BOOL saved = CGImageDestinationFinalize(destination);
    CFRelease(destination);
    if (!saved) { result.error = -10006; return result; }
    CGColorSpaceRef colors = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
    CGContextRef bitmap = CGBitmapContextCreate(NULL, width, height, 8, width * 4, colors, kCGBitmapByteOrder32Big | kCGImageAlphaPremultipliedLast);
    CGColorSpaceRelease(colors);
    if (!bitmap) { result.error = -10004; return result; }
    CGContextDrawImage(bitmap, CGRectMake(0, 0, width, height), image);
    const unsigned char *pixels = CGBitmapContextGetData(bitmap);
    size_t red = 0, green = 0, black = 0, transparent = 0, count = width * height;
    for (size_t p = 0; p < count; p++) {
        const unsigned char *rgb = pixels + p * 4;
        if (rgb[3] < 10) transparent++;
        if (rgb[3] > 245 && rgb[0] < 10 && rgb[1] < 10 && rgb[2] < 10) black++;
        if (rgb[3] < 180) continue;
        if (rgb[0] > 180 && rgb[1] < 80 && rgb[2] < 80) red++;
        if (rgb[1] > 180 && rgb[0] < 80 && rgb[2] < 80) green++;
    }
    result.width = (int)width; result.height = (int)height;
    result.red = (double)red / count; result.green = (double)green / count;
    result.black = (double)black / count; result.transparent = (double)transparent / count;
    CGContextRelease(bitmap);
    return result;
}

int QSProbeRunning(void) { return probeDelegate && !probeDelegate.finished; }

void QSProbeCapture(uintptr_t handle, int index, int stream) {
    if (!CGPreflightScreenCaptureAccess()) { captureResult(index, 0, 0, 0, 0, -10001); return; }
    if (@available(macOS 14.0, *)) {
        CGWindowID ownedID = (CGWindowID)[(NSWindow *)handle windowNumber];
        // Give the compositor a turn after the acknowledged sharing property.
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 200 * NSEC_PER_MSEC), dispatch_get_main_queue(), ^{
            if (!probeDelegate || probeDelegate.finished) return;
            [SCShareableContent getShareableContentExcludingDesktopWindows:YES onScreenWindowsOnly:YES completionHandler:^(SCShareableContent *content, NSError *error) {
                if (error) { captureResult(index, 0, 0, 0, 0, (int)error.code); return; }
                SCWindow *ownedWindow = nil;
                for (SCWindow *candidate in content.windows) {
                    if (candidate.windowID == ownedID) { ownedWindow = candidate; break; }
                }
                // Never fall back to a display capture or another application's window.
                if (!ownedWindow) { captureResult(index, 0, 0, 0, 0, -10002); return; }
                SCContentFilter *filter = [[SCContentFilter alloc] initWithDesktopIndependentWindow:ownedWindow];
                SCStreamConfiguration *config = [[SCStreamConfiguration alloc] init];
                config.width = 390; config.height = 170;
                config.showsCursor = NO;
                config.ignoreShadowsSingleWindow = YES;
                if (stream) {
                    dispatch_async(dispatch_get_main_queue(), ^{
                        if (QSProbeRunning()) QSProbeStartStream(filter, config, index);
                    });
                } else [SCScreenshotManager captureImageWithFilter:filter configuration:config completionHandler:^(CGImageRef image, NSError *captureError) {
                    if (captureError || !image) { captureResult(index, 0, 0, 0, 0, captureError ? (int)captureError.code : -10003); return; }
                    QSProbePixels pixels = QSProbeReadImage(image, [NSString stringWithFormat:@"capture-sample-%d.png", index]);
                    captureResultWithMetrics(index, pixels.width, pixels.height, pixels.red, pixels.green, pixels.error, pixels.black, pixels.transparent);
                }];
                [config release]; [filter release];
            }];
        });
    } else { captureResult(index, 0, 0, 0, 0, -10005); }
}

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
void QSProbeEndRun(void) {
    [NSApp stop:nil];
    NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSZeroPoint
        modifierFlags:0 timestamp:0 windowNumber:0 context:nil subtype:0 data1:0 data2:0];
    [NSApp postEvent:event atStart:NO];
}
void QSProbeStop(void) {
    probeDelegate.finished = YES;
    if (!QSProbeCancelStream()) { QSProbeEndRun(); return; }
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC), dispatch_get_main_queue(), ^{
        if (NSApp.running) QSProbeEndRun();
    });
}

@implementation QSProbeDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    uintptr_t handle = QSProbeCreate();
    NSPanel *panel = (NSPanel *)handle;
    WKWebViewConfiguration *configuration = [[WKWebViewConfiguration alloc] init];
    [configuration.userContentController addScriptMessageHandler:self name:@"fixture"];
    QSProbeWebView *view = [[QSProbeWebView alloc] initWithFrame:panel.contentView.bounds configuration:configuration];
    panel.contentView = view;
    NSString *html = @"<!doctype html><meta charset='utf-8'><style>"
        @"html,body{margin:0;width:100%;height:100%;background:linear-gradient(90deg,#ff0000 50%,#00ff00 50%);font:14px -apple-system}"
        @"div{padding:14px}i{position:absolute;bottom:4px;left:20px;width:4px;height:4px;background:white;animation:pulse .2s infinite alternate}"
        @"@keyframes pulse{from{opacity:0}to{opacity:1}}</style>"
        @"<div><b>原生面板隔离自检</b><p>仅用合成色块验证，完成后自动关闭。</p></div><i></i>"
        @"<script>window.webkit.messageHandlers.fixture.postMessage('ready')</script>";
    [view loadHTMLString:html baseURL:nil];
    [view release]; [configuration release];
    panelProbeCreated(handle);
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 20 * NSEC_PER_SEC), dispatch_get_main_queue(), ^{
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
