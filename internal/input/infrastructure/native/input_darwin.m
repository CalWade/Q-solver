#import <AppKit/AppKit.h>
#import <ApplicationServices/ApplicationServices.h>
#import <IOKit/hidsystem/IOLLEvent.h>
#include <stdatomic.h>
#include "input_darwin.h"

extern uint64_t goQSInput(uintptr_t, uint32_t, int, int, int, int);
extern void goQSInputReady(uintptr_t);
extern void goQSInputFailed(uintptr_t);
extern void goQSInputSeed(uintptr_t, uint32_t);

typedef struct {
    CFMachPortRef tap;
    CFRunLoopRef loop;
    uintptr_t handle;
} QSInputContext;

static atomic_bool qsOwnWindow;
static NSMutableArray *qsFocusObservers;

static void qsOnMain(dispatch_block_t block) {
    if ([NSThread isMainThread]) block();
    else dispatch_sync(dispatch_get_main_queue(), block);
}

bool qs_input_permission(bool prompt) {
    if (!prompt) return AXIsProcessTrusted();
    NSDictionary *options = @{(__bridge NSString *)kAXTrustedCheckOptionPrompt: @YES};
    return AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options);
}

static uint64_t qsSideFlag(CGKeyCode key) {
    switch (key) {
        case 54: return NX_DEVICERCMDKEYMASK;
        case 55: return NX_DEVICELCMDKEYMASK;
        case 56: return NX_DEVICELSHIFTKEYMASK;
        case 58: return NX_DEVICELALTKEYMASK;
        case 59: return NX_DEVICELCTLKEYMASK;
        case 60: return NX_DEVICERSHIFTKEYMASK;
        case 61: return NX_DEVICERALTKEYMASK;
        case 62: return NX_DEVICERCTLKEYMASK;
        default: return 0;
    }
}

static CGEventRef qsFilter(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *userInfo) {
    QSInputContext *context = userInfo;
    if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
        goQSInputFailed(context->handle);
        CGEventTapEnable(context->tap, false);
        CFRunLoopStop(context->loop);
        return event;
    }
    uint32_t key = (uint32_t)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
    bool down = type == kCGEventKeyDown;
    bool mouse = type == kCGEventOtherMouseDown || type == kCGEventOtherMouseUp;
    CGEventFlags flags = CGEventGetFlags(event);
    if (mouse) {
        int64_t button = CGEventGetIntegerValueField(event, kCGMouseEventButtonNumber);
        if (button != 3 && button != 4) return event;
        key = button == 3 ? 1000 : 1001;
        down = type == kCGEventOtherMouseDown;
    } else if (type == kCGEventFlagsChanged) {
        uint64_t side = qsSideFlag((CGKeyCode)key);
        if (!side) return event;
        down = (flags & side) != 0;
    }
    uint64_t result = goQSInput(context->handle, key, down,
        CGEventGetIntegerValueField(event, kCGKeyboardEventAutorepeat) != 0,
        CGEventGetIntegerValueField(event, kCGEventSourceUnixProcessID) != 0,
        atomic_load(&qsOwnWindow));
    if (result & 1) return NULL;
    if (!mouse && (result & 2)) {
        uint8_t visible = (result >> 8) & 0xff;
        uint8_t sides = (result >> 16) & 0xff;
        flags &= ~(kCGEventFlagMaskControl | kCGEventFlagMaskCommand | kCGEventFlagMaskAlternate | kCGEventFlagMaskShift |
            NX_DEVICELCTLKEYMASK | NX_DEVICERCTLKEYMASK | NX_DEVICELCMDKEYMASK | NX_DEVICERCMDKEYMASK |
            NX_DEVICELALTKEYMASK | NX_DEVICERALTKEYMASK | NX_DEVICELSHIFTKEYMASK | NX_DEVICERSHIFTKEYMASK);
        if (visible & 1) flags |= kCGEventFlagMaskControl;
        if (visible & 2) flags |= kCGEventFlagMaskCommand;
        if (visible & 4) flags |= kCGEventFlagMaskAlternate;
        if (visible & 8) flags |= kCGEventFlagMaskShift;
        uint64_t masks[] = {NX_DEVICELCTLKEYMASK, NX_DEVICERCTLKEYMASK, NX_DEVICELCMDKEYMASK, NX_DEVICERCMDKEYMASK,
            NX_DEVICELALTKEYMASK, NX_DEVICERALTKEYMASK, NX_DEVICELSHIFTKEYMASK, NX_DEVICERSHIFTKEYMASK};
        for (int i = 0; i < 8; i++) if (sides & (1 << i)) flags |= masks[i];
        CGEventSetFlags(event, flags);
    }
    return event;
}

void *qs_input_create(uintptr_t handle) {
    if (!qs_input_permission(false)) return NULL;
    QSInputContext *context = calloc(1, sizeof(QSInputContext));
    if (!context) return NULL;
    context->handle = handle;
    CGEventMask mask = CGEventMaskBit(kCGEventKeyDown) | CGEventMaskBit(kCGEventKeyUp) |
        CGEventMaskBit(kCGEventFlagsChanged) | CGEventMaskBit(kCGEventOtherMouseDown) | CGEventMaskBit(kCGEventOtherMouseUp);
    context->tap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap, kCGEventTapOptionDefault, mask, qsFilter, context);
    if (!context->tap) { free(context); return NULL; }
    qsOnMain(^{
        qsFocusObservers = [[NSMutableArray alloc] init];
        NSNotificationCenter *center = NSNotificationCenter.defaultCenter;
        for (NSString *name in @[NSWindowDidBecomeKeyNotification, NSWindowDidResignKeyNotification, NSApplicationDidResignActiveNotification]) {
            id observer = [center addObserverForName:name object:nil queue:NSOperationQueue.mainQueue usingBlock:^(NSNotification *note) {
                atomic_store(&qsOwnWindow, NSApp.keyWindow != nil);
            }];
            [qsFocusObservers addObject:observer];
        }
        atomic_store(&qsOwnWindow, NSApp.keyWindow != nil);
    });
    return context;
}

void qs_input_run(void *value) {
    QSInputContext *context = value;
    context->loop = CFRunLoopGetCurrent();
    CFRetain(context->loop);
    CFRunLoopSourceRef source = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, context->tap, 0);
    CFRunLoopAddSource(context->loop, source, kCFRunLoopCommonModes);
    for (uint32_t key = 0; key < 128; key++) {
        if (CGEventSourceKeyState(kCGEventSourceStateCombinedSessionState, key)) goQSInputSeed(context->handle, key);
    }
    CGEventTapEnable(context->tap, true);
    goQSInputReady(context->handle);
    CFRunLoopRun();
    CGEventTapEnable(context->tap, false);
    CFRunLoopRemoveSource(context->loop, source, kCFRunLoopCommonModes);
    CFRelease(source);
}

void qs_input_stop(void *value) {
    QSInputContext *context = value;
    if (context && context->loop) { CFRunLoopStop(context->loop); CFRunLoopWakeUp(context->loop); }
}

void qs_input_destroy(void *value) {
    QSInputContext *context = value;
    if (!context) return;
    qsOnMain(^{
        for (id observer in qsFocusObservers) [NSNotificationCenter.defaultCenter removeObserver:observer];
        [qsFocusObservers release];
        qsFocusObservers = nil;
    });
    CFMachPortInvalidate(context->tap);
    CFRelease(context->tap);
    if (context->loop) CFRelease(context->loop);
    free(context);
}
