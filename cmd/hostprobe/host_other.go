//go:build !darwin || !cgo

package main

import (
	"Q-Solver/internal/window/infrastructure/probe"
	"errors"
)

func runHost() (probe.HostReport, error) {
	return probe.HostReport{}, errors.New("hostprobe currently requires macOS and the C toolchain")
}
