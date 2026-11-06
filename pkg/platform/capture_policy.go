package platform

// The exclusion value was introduced in Windows 10 2004 (build 19041).
// An unknown or older build must be reported as blank-content protection.
func captureAffinity(build uint32, enabled bool) (uint32, string) {
	if !enabled {
		return 0, "off"
	}
	if build >= 19041 {
		return 0x11, "exclude-from-capture"
	}
	return 1, "blank-content"
}
