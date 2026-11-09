//go:build !darwin || !cgo

package main

import (
	"errors"
)

func runHost(func(string) error, bool, int) (hostReport, error) {
	return hostReport{}, errors.New("hostprobe currently requires macOS and the C toolchain")
}
