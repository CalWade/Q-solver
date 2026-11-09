package main

import (
	inputprobe "Q-Solver/internal/input/infrastructure/probe"
	"Q-Solver/internal/window/infrastructure/probe"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type hostReport struct {
	probe.HostReport
	Keyboard *inputprobe.Report `json:"keyboard,omitempty"`
}

func (r *hostReport) Evaluate() bool {
	passed := r.HostReport.Evaluate()
	if r.Keyboard != nil {
		passed = r.Keyboard.Evaluate() && passed
	}
	return passed
}

func main() {
	output := flag.String("output", ".local/reports/host-probe.json", "native host report")
	controlled := flag.Bool("controlled", false, "wait for an independent observer to acknowledge each stage on stdin")
	keyboard := flag.Bool("keyboard", false, "exercise tagged native keyboard events with an independent receiver")
	receiverPID := flag.Int("receiver-pid", 0, "owned browser receiver process for keyboard diagnostics")
	flag.Parse()
	if *keyboard && (!*controlled || *receiverPID <= 0) {
		fmt.Fprintln(os.Stderr, "keyboard diagnostics require the controlled browser receiver and its process ID")
		os.Exit(1)
	}
	var observe func(string) error
	if *controlled {
		gate := probe.NewStageGate(os.Stdin, os.Stdout)
		defer gate.Close()
		observe = func(stage string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return gate.Wait(ctx, stage)
		}
	}
	report, err := runHost(observe, *keyboard, *receiverPID)
	if err != nil {
		report.Errors = append(report.Errors, err.Error())
	}
	passed := report.Evaluate()
	data, _ := json.MarshalIndent(report, "", "  ")
	if err := os.MkdirAll(filepath.Dir(*output), 0755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*output, append(data, '\n'), 0600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Native Wails host probe: passed=%t; report=%s\n", passed, *output)
	if !passed {
		os.Exit(1)
	}
}
