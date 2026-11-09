#include <stdint.h>

typedef struct {
    int panel, visible, nonactivating, canKey, key, main, protectedContent;
    double alpha;
    int foreground;
} QSProbeObservation;
typedef struct {
    uint32_t expectedWindow, actualWindow, expectedDisplay, actualDisplay;
    int ownerPID, windows, displays;
    double windowX, windowY, windowWidth, windowHeight;
    double displayX, displayY, displayWidth, displayHeight;
    double cropX, cropY, cropWidth, cropHeight;
} QSProbeDisplaySelection;
typedef struct {
    int sessionKnown, locked, onConsole, displaysKnown, activeDisplays, asleep;
} QSProbeEnvironment;
QSProbeEnvironment QSProbeCaptureEnvironment(void);
void QSProbeRun(void);
uintptr_t QSProbeCreate(void);
void QSProbeClose(uintptr_t handle);
QSProbeObservation QSProbeObserve(uintptr_t handle);
int QSProbeForeground(void);
const char *QSProbeSystem(void);
void QSProbeTryKey(uintptr_t handle);
void QSProbeStop(void);
void QSProbeCapture(uintptr_t handle, int index, int stream, int liveMillis, int displayWindow);
int64_t QSProbeHostTime(void);
int QSProbeRunning(void);
