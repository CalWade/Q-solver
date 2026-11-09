#import <Cocoa/Cocoa.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>

typedef struct {
    int width, height, error;
    double red, green, black, transparent;
} QSProbePixels;

QSProbePixels QSProbeReadImage(CGImageRef image, NSString *filename);
void QSProbeStartStream(SCContentFilter *filter, SCStreamConfiguration *config, int index);
BOOL QSProbeCancelStream(void);
void QSProbeEndRun(void);
