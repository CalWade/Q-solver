// panelprobe is an isolated native-host experiment, not the production app.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	output := flag.String("output", ".local/reports/panel-probe.json", "native observation report")
	capture := flag.Bool("capture", false, "compare capture of the owned synthetic window with protection off/on/off")
	stream := flag.Bool("capture-stream", false, "compare short SCStream sessions of the owned window with protection off/on/off")
	flag.Parse()
	if *capture && *stream {
		fmt.Fprintln(os.Stderr, "select one capture method per diagnostic")
		os.Exit(1)
	}
	report, err := runNative(*capture, *stream)
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
	fmt.Printf("Native panel probe: passed=%t; report=%s\n", passed, *output)
	if report.Capture != nil {
		fmt.Printf("ScreenCaptureKit diagnostic completed=%t; protected-window outcome=%s (separate from diagnostic completion)\n", report.Capture.Completed, report.Capture.Outcome)
	}
	if !passed {
		os.Exit(1)
	}
}
