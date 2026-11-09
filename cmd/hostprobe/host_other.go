//go:build !darwin || !cgo

package main

import (
	"Q-Solver/internal/window/infrastructure/probe"
	"errors"
)

func runHost(func(string) error) (probe.HostReport, error) {
	return probe.HostReport{}, errors.New("hostprobe currently requires macOS and the C toolchain")
}
