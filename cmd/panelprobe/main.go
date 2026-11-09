// panelprobe is an isolated native-host experiment, not the production app.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type captureMode int

const (
	captureNone captureMode = iota
	captureScreenshot
	captureFreshStream
	captureLiveStream
)

func selectCaptureMode(screenshot, stream, live bool) (captureMode, error) {
	selected := captureNone
	for _, option := range []struct {
		enabled bool
		mode    captureMode
	}{{screenshot, captureScreenshot}, {stream, captureFreshStream}, {live, captureLiveStream}} {
		if option.enabled {
			if selected != captureNone {
				return captureNone, fmt.Errorf("select one capture method per diagnostic")
			}
			selected = option.mode
		}
	}
	return selected, nil
}

func validateDisplayFilter(mode captureMode, display bool) error {
	if display && mode != captureLiveStream {
		return fmt.Errorf("the display inclusion filter requires live stream diagnostics")
	}
	return nil
}

func main() {
	output := flag.String("output", ".local/reports/panel-probe.json", "native observation report")
	capture := flag.Bool("capture", false, "compare capture of the owned synthetic window with protection off/on/off")
	stream := flag.Bool("capture-stream", false, "compare short SCStream sessions of the owned window with protection off/on/off")
	live := flag.Bool("capture-live", false, "toggle protection while the same owned-window SCStream keeps running")
	display := flag.Bool("display-window", false, "use a display inclusion filter restricted to the owned fixture window")
	flag.Parse()
	mode, err := selectCaptureMode(*capture, *stream, *live)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := validateDisplayFilter(mode, *display); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	report, err := runNative(mode, *display)
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
	if report.LiveCapture != nil {
		fmt.Printf("Live stream diagnostic completed=%t; protected-window outcome=%s\n", report.LiveCapture.Completed, report.LiveCapture.Outcome)
	}
	if !passed {
		os.Exit(1)
	}
}
