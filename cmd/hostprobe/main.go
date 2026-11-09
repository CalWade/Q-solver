package main

import (
	"Q-Solver/internal/window/infrastructure/probe"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	output := flag.String("output", ".local/reports/host-probe.json", "native host report")
	controlled := flag.Bool("controlled", false, "wait for an independent observer to acknowledge each stage on stdin")
	flag.Parse()
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
	report, err := runHost(observe)
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
