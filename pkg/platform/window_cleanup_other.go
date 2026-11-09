//go:build !windows

package platform

func ReleaseWindowState(uintptr) error { return nil }
