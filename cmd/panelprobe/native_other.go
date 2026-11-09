//go:build !darwin || !cgo

package main

import (
	"Q-Solver/internal/window/infrastructure/probe"
	"errors"
)

func runNative(bool, bool) (probe.Report, error) {
	return probe.Report{}, errors.New("panelprobe requires macOS and the existing C toolchain")
}
