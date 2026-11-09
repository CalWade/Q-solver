//go:build darwin && cgo

#import "capture_darwin.h"
#import "native_darwin.h"
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>
#include <time.h>

extern void panelProbeStreamFrame(int width, int height, double red, double green, double black, double transparent, double rawColor, int64_t presentation, int64_t display, int64_t received, const char *file);
extern void panelProbeStreamFinished(int index, int duration, int outputs, int errorCode);
extern void panelProbeStreamStatus(int status, int count);
extern void panelProbeStreamLifecycle(int phase, int64_t at);
extern void panelProbeStreamOutput(int status, int frame, int64_t presentation, int64_t display, int64_t received);
extern int panelProbeLivePolicy(int index);

typedef struct {
    QSProbePixels pixels;
    int64_t presentation, display, received;
    double rawColor;
    char file[64];
} QSStreamFrame;

typedef struct {
    int status, frame;
    int64_t presentation, display, received;
} QSStreamOutput;

static int64_t timeNanos(CMTime time) {
    if (!CMTIME_IS_NUMERIC(time)) return 0;
    return CMTimeConvertScale(time, NSEC_PER_SEC, kCMTimeRoundingMethod_Default).value;
}

int64_t QSProbeHostTime(void) { return timeNanos(CMClockGetTime(CMClockGetHostTimeClock())); }

static int64_t monotonicNanos(void) {
    struct timespec now;
    clock_gettime(CLOCK_MONOTONIC, &now);
    return (int64_t)now.tv_sec * NSEC_PER_SEC + now.tv_nsec;
}

@interface QSProbeStream : NSObject <SCStreamOutput, SCStreamDelegate> {
    SCStream *_stream;
    dispatch_queue_t _sampleQueue;
    NSMutableArray *_frames;
    NSMutableArray *_outputEvents;
    int _index, _outputs, _sampleError, _cancelError, _liveMillis;
    int _statuses[7];
    int64_t _startedAt;
    BOOL _starting, _started, _finishing, _displayFilter;
}
- (instancetype)initWithFilter:(SCContentFilter *)filter configuration:(SCStreamConfiguration *)config index:(int)index liveMillis:(int)liveMillis;
- (void)start;
- (void)finish:(int)errorCode;
- (void)checkSamples;
- (void)advanceLivePhase;
- (void)captureCompleteSample:(CMSampleBufferRef)sample output:(QSStreamOutput)output;
@end

static QSProbeStream *activeStream;

@implementation QSProbeStream
- (instancetype)initWithFilter:(SCContentFilter *)filter configuration:(SCStreamConfiguration *)config index:(int)index liveMillis:(int)liveMillis {
    self = [super init];
    if (self) {
        _index = index;
        _liveMillis = liveMillis;
        _displayFilter = filter.style == SCShareableContentStyleDisplay;
        _frames = [[NSMutableArray alloc] init];
        _outputEvents = [[NSMutableArray alloc] init];
        _sampleQueue = dispatch_queue_create("qsolver.probe.capture", DISPATCH_QUEUE_SERIAL);
        config.pixelFormat = kCVPixelFormatType_32BGRA;
        config.colorSpaceName = kCGColorSpaceSRGB;
        config.minimumFrameInterval = CMTimeMake(1, 15);
        config.queueDepth = 3;
        config.capturesAudio = NO;
        _stream = [[SCStream alloc] initWithFilter:filter configuration:config delegate:self];
    }
    return self;
}
- (void)dealloc {
    [_stream release];
    [_frames release];
    [_outputEvents release];
    dispatch_release(_sampleQueue);
    [super dealloc];
}
- (void)start {
    NSError *error = nil;
    if (![_stream addStreamOutput:self type:SCStreamOutputTypeScreen sampleHandlerQueue:_sampleQueue error:&error]) {
        [self finish:error ? (int)error.code : -10007];
        return;
    }
    _starting = YES;
    [_stream startCaptureWithCompletionHandler:^(NSError *startError) {
        dispatch_async(dispatch_get_main_queue(), ^{
            self->_starting = NO;
            if (startError) { [self finish:(int)startError.code]; return; }
            self->_started = YES;
            self->_startedAt = monotonicNanos();
            if (self->_liveMillis) panelProbeStreamLifecycle(0, QSProbeHostTime());
            if (self->_cancelError) { [self finish:self->_cancelError]; return; }
            if (self->_liveMillis) {
                dispatch_after(dispatch_time(DISPATCH_TIME_NOW, self->_liveMillis * NSEC_PER_MSEC), dispatch_get_main_queue(), ^{
                    [self advanceLivePhase];
                });
                return;
            }
            dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 500 * NSEC_PER_MSEC), dispatch_get_main_queue(), ^{
                [self checkSamples];
            });
        });
    }];
}
- (void)advanceLivePhase {
    if (_finishing) return;
    if (_index == 2) { [self finish:0]; return; }
    if (!panelProbeLivePolicy(_index + 1)) { [self finish:-10014]; return; }
    _index++;
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, _liveMillis * NSEC_PER_MSEC), dispatch_get_main_queue(), ^{
        [self advanceLivePhase];
    });
}
- (void)checkSamples {
    if (_finishing) return;
    dispatch_async(_sampleQueue, ^{
        BOOL ready = NO;
        if (self->_frames.count >= 3) {
            QSStreamFrame first, last;
            [self->_frames.firstObject getValue:&first size:sizeof(first)];
            [self->_frames.lastObject getValue:&last size:sizeof(last)];
            ready = last.presentation - first.presentation >= 200 * NSEC_PER_MSEC;
        }
        BOOL expired = monotonicNanos() - self->_startedAt >= 3 * NSEC_PER_SEC;
        BOOL failed = self->_sampleError != 0;
        dispatch_async(dispatch_get_main_queue(), ^{
            if (ready || expired || failed) { [self finish:0]; return; }
            dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 50 * NSEC_PER_MSEC), dispatch_get_main_queue(), ^{ [self checkSamples]; });
        });
    });
}
- (void)finish:(int)errorCode {
    // All lifecycle calls are serialized on AppKit's main thread. Samples and
    // their metrics are owned by the separate serial output queue.
    if (_finishing) return;
    if (_starting) { _cancelError = errorCode; return; }
    _finishing = YES;
    void (^finishSamples)(NSError *) = ^(NSError *stopError) {
        int64_t stoppedAt = self->_started && !stopError ? QSProbeHostTime() : 0;
        dispatch_async(self->_sampleQueue, ^{
            int finalError = errorCode;
            if (!finalError) finalError = stopError ? (int)stopError.code : self->_sampleError;
            if (!finalError && self->_frames.count == 0) finalError = -10008;
            int duration = self->_startedAt ? (int)((monotonicNanos() - self->_startedAt) / NSEC_PER_MSEC) : 0;
            int64_t drainedAt = QSProbeHostTime();
            // The queue barrier drains every output before exporting the batch.
            dispatch_async(dispatch_get_main_queue(), ^{
                int reportedError = finalError;
                if (!reportedError) reportedError = self->_cancelError;
                NSError *removeError = nil;
                BOOL removed = [self->_stream removeStreamOutput:self type:SCStreamOutputTypeScreen error:&removeError];
                if (!removed && !reportedError) reportedError = removeError ? (int)removeError.code : -10012;
                activeStream = nil;
                if (QSProbeRunning()) {
                    if (self->_liveMillis && stoppedAt) panelProbeStreamLifecycle(1, stoppedAt);
                    if (self->_displayFilter) panelProbeStreamLifecycle(2, drainedAt);
                    for (int status = 0; status < 7; status++) panelProbeStreamStatus(status, self->_statuses[status]);
                    for (NSValue *value in self->_frames) {
                        QSStreamFrame frame;
                        [value getValue:&frame size:sizeof(frame)];
                        panelProbeStreamFrame(frame.pixels.width, frame.pixels.height, frame.pixels.red, frame.pixels.green,
                            frame.pixels.black, frame.pixels.transparent, frame.rawColor, frame.presentation, frame.display, frame.received, frame.file);
                    }
                    for (NSValue *value in self->_outputEvents) {
                        QSStreamOutput output;
                        [value getValue:&output size:sizeof(output)];
                        panelProbeStreamOutput(output.status, output.frame, output.presentation, output.display, output.received);
                    }
                    panelProbeStreamFinished(self->_index, duration, self->_outputs, reportedError);
                } else {
                    QSProbeEndRun();
                }
                [self release];
            });
        });
    };
    if (_started) [_stream stopCaptureWithCompletionHandler:finishSamples];
    else finishSamples(nil);
}
- (void)stream:(SCStream *)stream didStopWithError:(NSError *)error {
    dispatch_async(dispatch_get_main_queue(), ^{
        self->_cancelError = (int)error.code;
        [self finish:(int)error.code];
    });
}
- (void)stream:(SCStream *)stream didOutputSampleBuffer:(CMSampleBufferRef)sample ofType:(SCStreamOutputType)type {
    @autoreleasepool {
        if (type != SCStreamOutputTypeScreen || !CMSampleBufferIsValid(sample)) return;
        int64_t received = QSProbeHostTime();
        _outputs++;
        NSArray *attachments = (NSArray *)CMSampleBufferGetSampleAttachmentsArray(sample, false);
        NSNumber *status = attachments.count ? attachments[0][SCStreamFrameInfoStatus] : nil;
        NSInteger code = status ? status.integerValue : -1;
        int normalized = code >= 0 && code < 6 ? (int)code : 6;
        _statuses[normalized]++;
        NSNumber *display = attachments.count ? attachments[0][SCStreamFrameInfoDisplayTime] : nil;
        QSStreamOutput output = {normalized, -1, timeNanos(CMSampleBufferGetPresentationTimeStamp(sample)),
            display ? timeNanos(CMClockMakeHostTimeFromSystemUnits(display.unsignedLongLongValue)) : 0, received};
        if (code == SCFrameStatusComplete && !_sampleError) [self captureCompleteSample:sample output:output];
        output.frame = (int)_frames.count - 1;
        if (_displayFilter) [_outputEvents addObject:[NSValue valueWithBytes:&output objCType:@encode(QSStreamOutput)]];
    }
}
- (void)captureCompleteSample:(CMSampleBufferRef)sample output:(QSStreamOutput)output {
    CVPixelBufferRef buffer = CMSampleBufferGetImageBuffer(sample);
    if (!buffer || CVPixelBufferGetPixelFormatType(buffer) != kCVPixelFormatType_32BGRA ||
        CVPixelBufferLockBaseAddress(buffer, kCVPixelBufferLock_ReadOnly) != kCVReturnSuccess) {
        _sampleError = -10009;
        return;
    }
    size_t width = CVPixelBufferGetWidth(buffer), height = CVPixelBufferGetHeight(buffer);
    size_t stride = CVPixelBufferGetBytesPerRow(buffer);
    const void *base = CVPixelBufferGetBaseAddress(buffer);
    CGColorSpaceRef colors = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
    CGDataProviderRef provider = CGDataProviderCreateWithData(NULL, base, stride * height, NULL);
    CGImageRef image = CGImageCreate(width, height, 8, 32, stride, colors,
        kCGBitmapByteOrder32Little | kCGImageAlphaPremultipliedFirst, provider, NULL, false, kCGRenderingIntentDefault);
    QSStreamFrame frame = {0};
    if (_displayFilter) snprintf(frame.file, sizeof(frame.file), "capture-display-%lu.png", (unsigned long)_frames.count);
    else if (_liveMillis) snprintf(frame.file, sizeof(frame.file), "capture-live-%lu.png", (unsigned long)_frames.count);
    else snprintf(frame.file, sizeof(frame.file), "capture-stream-%d-%lu.png", _index, (unsigned long)_frames.count);
    if (image) frame.pixels = QSProbeReadImage(image, [NSString stringWithUTF8String:frame.file]);
    else frame.pixels.error = -10010;
    if (!frame.pixels.error) {
        size_t colored = 0;
        for (size_t row = 0; row < height; row++) {
            const unsigned char *pixels = (const unsigned char *)base + row * stride;
            for (size_t col = 0; col < width; col++) {
                const unsigned char *bgra = pixels + col * 4;
                if (bgra[0] || bgra[1] || bgra[2]) colored++;
            }
        }
        frame.rawColor = (double)colored / (width * height);
    }
    frame.presentation = output.presentation;
    frame.display = output.display;
    frame.received = output.received;
    if (image) CGImageRelease(image);
    CGDataProviderRelease(provider);
    CGColorSpaceRelease(colors);
    CVPixelBufferUnlockBaseAddress(buffer, kCVPixelBufferLock_ReadOnly);
    if (frame.pixels.error) { _sampleError = frame.pixels.error; return; }
    [_frames addObject:[NSValue valueWithBytes:&frame objCType:@encode(QSStreamFrame)]];
}
@end

void QSProbeStartStream(SCContentFilter *filter, SCStreamConfiguration *config, int index, int liveMillis) {
    activeStream = [[QSProbeStream alloc] initWithFilter:filter configuration:config index:index liveMillis:liveMillis];
    [activeStream start];
}

BOOL QSProbeCancelStream(void) {
    if (!activeStream) return NO;
    [activeStream finish:-10011];
    return YES;
}
