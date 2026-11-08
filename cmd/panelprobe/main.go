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
	flag.Parse()
	report, err := runNative()
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
	if !passed {
		os.Exit(1)
	}
}
