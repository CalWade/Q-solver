#include <stdint.h>

typedef struct {
    int panel, visible, nonactivating, canKey, key, main, protectedContent;
    double alpha;
    int foreground;
} QSProbeObservation;
void QSProbeRun(void);
uintptr_t QSProbeCreate(void);
void QSProbeClose(uintptr_t handle);
QSProbeObservation QSProbeObserve(uintptr_t handle);
int QSProbeForeground(void);
const char *QSProbeSystem(void);
void QSProbeTryKey(uintptr_t handle);
void QSProbeStop(void);
void QSProbeCapture(uintptr_t handle, int index, int stream, int liveMillis);
int64_t QSProbeHostTime(void);
int QSProbeRunning(void);
