//go:build darwin && cgo

#import "capture_darwin.h"
#import "native_darwin.h"
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>
#include <time.h>

extern void panelProbeStreamFrame(int width, int height, double red, double green, double black, double transparent, int64_t presentation, const char *file);
extern void panelProbeStreamFinished(int index, int duration, int outputs, int errorCode);
extern void panelProbeStreamStatus(int status, int count);

typedef struct {
    QSProbePixels pixels;
    int64_t presentation;
    char file[64];
} QSStreamFrame;

static int64_t monotonicNanos(void) {
    struct timespec now;
    clock_gettime(CLOCK_MONOTONIC, &now);
    return (int64_t)now.tv_sec * NSEC_PER_SEC + now.tv_nsec;
}

@interface QSProbeStream : NSObject <SCStreamOutput, SCStreamDelegate> {
    SCStream *_stream;
    dispatch_queue_t _sampleQueue;
    NSMutableArray *_frames;
    int _index, _outputs, _sampleError, _cancelError;
    int _statuses[7];
    int64_t _startedAt;
    BOOL _starting, _started, _finishing;
}
- (instancetype)initWithFilter:(SCContentFilter *)filter configuration:(SCStreamConfiguration *)config index:(int)index;
- (void)start;
- (void)finish:(int)errorCode;
- (void)checkSamples;
@end

static QSProbeStream *activeStream;

@implementation QSProbeStream
- (instancetype)initWithFilter:(SCContentFilter *)filter configuration:(SCStreamConfiguration *)config index:(int)index {
    self = [super init];
    if (self) {
        _index = index;
        _frames = [[NSMutableArray alloc] init];
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
            if (self->_cancelError) { [self finish:self->_cancelError]; return; }
            dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 500 * NSEC_PER_MSEC), dispatch_get_main_queue(), ^{
                [self checkSamples];
            });
        });
    }];
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
        dispatch_async(self->_sampleQueue, ^{
            int finalError = errorCode;
            if (!finalError) finalError = stopError ? (int)stopError.code : self->_sampleError;
            if (!finalError && self->_frames.count == 0) finalError = -10008;
            int duration = self->_startedAt ? (int)((monotonicNanos() - self->_startedAt) / NSEC_PER_MSEC) : 0;
            // The queue barrier drains every output before exporting the batch.
            dispatch_async(dispatch_get_main_queue(), ^{
                int reportedError = finalError;
                if (!reportedError) reportedError = self->_cancelError;
                NSError *removeError = nil;
                BOOL removed = [self->_stream removeStreamOutput:self type:SCStreamOutputTypeScreen error:&removeError];
                if (!removed && !reportedError) reportedError = removeError ? (int)removeError.code : -10012;
                activeStream = nil;
                if (QSProbeRunning()) {
                    for (int status = 0; status < 7; status++) panelProbeStreamStatus(status, self->_statuses[status]);
                    for (NSValue *value in self->_frames) {
                        QSStreamFrame frame;
                        [value getValue:&frame size:sizeof(frame)];
                        panelProbeStreamFrame(frame.pixels.width, frame.pixels.height, frame.pixels.red, frame.pixels.green,
                            frame.pixels.black, frame.pixels.transparent, frame.presentation, frame.file);
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
        _outputs++;
        NSArray *attachments = (NSArray *)CMSampleBufferGetSampleAttachmentsArray(sample, false);
        NSNumber *status = attachments.count ? attachments[0][SCStreamFrameInfoStatus] : nil;
        NSInteger code = status ? status.integerValue : -1;
        _statuses[code >= 0 && code < 6 ? code : 6]++;
        if (!status || status.integerValue != SCFrameStatusComplete) return;
        if (_sampleError) return;
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
        snprintf(frame.file, sizeof(frame.file), "capture-stream-%d-%lu.png", _index, (unsigned long)_frames.count);
        if (image) frame.pixels = QSProbeReadImage(image, [NSString stringWithUTF8String:frame.file]);
        else frame.pixels.error = -10010;
        CMTime presentation = CMTimeConvertScale(CMSampleBufferGetPresentationTimeStamp(sample), NSEC_PER_SEC, kCMTimeRoundingMethod_Default);
        frame.presentation = presentation.value;
        if (image) CGImageRelease(image);
        CGDataProviderRelease(provider);
        CGColorSpaceRelease(colors);
        CVPixelBufferUnlockBaseAddress(buffer, kCVPixelBufferLock_ReadOnly);
        if (frame.pixels.error) { _sampleError = frame.pixels.error; return; }
        [_frames addObject:[NSValue valueWithBytes:&frame objCType:@encode(QSStreamFrame)]];
    }
}
@end

void QSProbeStartStream(SCContentFilter *filter, SCStreamConfiguration *config, int index) {
    activeStream = [[QSProbeStream alloc] initWithFilter:filter configuration:config index:index];
    [activeStream start];
}

BOOL QSProbeCancelStream(void) {
    if (!activeStream) return NO;
    [activeStream finish:-10011];
    return YES;
}
