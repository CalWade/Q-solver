#include <stdint.h>
#include <stdbool.h>

void *qs_input_create(uintptr_t handle, uint64_t diagnosticTag);
void qs_input_run(void *context);
void qs_input_stop(void *context);
void qs_input_destroy(void *context);
bool qs_input_permission(bool prompt);
