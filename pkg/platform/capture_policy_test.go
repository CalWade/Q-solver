package platform

import "testing"

func TestCaptureAffinityDoesNotMislabelOldWindowsAsExclusion(t *testing.T) {
	for _, tc := range []struct {
		build    uint32
		enabled  bool
		affinity uint32
		mode     string
	}{{0, true, 1, "blank-content"}, {18363, true, 1, "blank-content"}, {19041, true, 0x11, "exclude-from-capture"}, {26100, true, 0x11, "exclude-from-capture"}, {26100, false, 0, "off"}} {
		affinity, mode := captureAffinity(tc.build, tc.enabled)
		if affinity != tc.affinity || mode != tc.mode {
			t.Fatal(tc, affinity, mode)
		}
	}
}
