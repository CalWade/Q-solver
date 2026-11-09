package probe

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func protectedCaptureFixture() ProtectedCaptureReport {
	r := ProtectedCaptureReport{API: DisplayWindowStreamAPI, WindowCount: 1}
	for i := 0; i < 2; i++ {
		selection := displaySelectionFixture()
		selection.WindowCount = 2
		selection.Background = &CaptureBackground{ExpectedWindowID: 11, ActualWindowID: 11, OwnerPID: selection.ExpectedOwnerPID, Window: selection.Window}
		start := int64(1_000_000_000 + i*31_000_000_000)
		before := CaptureTransition{Stage: "protected-session-start", AppliedNanos: start - 100_000_000, PropertySet: true, LocalVisible: true, Alpha: 1}
		after := before
		after.Stage, after.AppliedNanos = "protected-session-end", start+30_120_000_000
		raw := 1.0
		frame := CaptureFrame{CapturePixels: CapturePixels{Width: 390, Height: 170, BlueRatio: 1},
			PresentationNanos: start + 54_000_000, DisplayNanos: start + 54_000_000, ReceivedNanos: start + 50_000_000,
			RawColorRatio: &raw, File: fmt.Sprintf("protected-%d.png", i)}
		session := ProtectedCaptureSession{Selection: &selection, Before: before, After: after,
			Starts: 1, Stops: 1, StartedNanos: start, StoppedNanos: start + 30_100_000_000, DrainedNanos: start + 30_110_000_000,
			StreamObservation: StreamObservation{DurationMillis: 30_100, OutputCount: 301,
				FrameStatuses: map[string]int{"complete": 1, "idle": 300, "blank": 0, "suspended": 0, "started": 0, "stopped": 0, "unknown": 0}, Frames: []CaptureFrame{frame}}}
		for j := 0; j < 301; j++ {
			output := CaptureOutput{Status: "idle", FrameIndex: 0, ReceivedNanos: frame.ReceivedNanos + int64(j)*100_000_000}
			if j == 0 {
				output.Status, output.PresentationNanos, output.DisplayNanos = "complete", frame.PresentationNanos, frame.DisplayNanos
			}
			session.Outputs = append(session.Outputs, output)
		}
		r.Sessions = append(r.Sessions, session)
	}
	return r
}

func TestSteadyProtectionRequiresAnObservedPositiveControlAndReconnection(t *testing.T) {
	r := protectedCaptureFixture()
	if !r.Evaluate() || r.Outcome != "background-only" || r.Sessions[0].Phase.Outcomes["background-only"] != 301 {
		t.Fatal(r)
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var restored ProtectedCaptureReport
	if err := json.Unmarshal(data, &restored); err != nil || !restored.Evaluate() || restored.Outcome != "background-only" {
		t.Fatal("source identity depended on pointer identity", err, restored)
	}
	parent := completeReport()
	parent.ProtectedCapture = &restored
	if !parent.Evaluate() {
		t.Fatal(parent)
	}
	restored.Sessions[0].Stops = 0
	if parent.Evaluate() {
		t.Fatal("lifecycle observations masked missing capture cleanup", parent)
	}
}

func TestVisibleContentIsReportedEvenWhenDefaultProtectionWasAcknowledged(t *testing.T) {
	r := protectedCaptureFixture()
	r.Sessions[1].Frames[0].RedRatio = .00001
	r.Sessions[1].Frames[0].BlueRatio = .99999
	if !r.Evaluate() || r.Outcome != "transition-leak" || r.Sessions[1].Outcome != "visible" {
		t.Fatal("small target marker disappeared into the positive control", r)
	}
	r.Sessions[0].Frames[0].GreenRatio = .00001
	r.Sessions[0].Frames[0].BlueRatio = .99999
	if !r.Evaluate() || r.Outcome != "visible" {
		t.Fatal(r)
	}
}

func TestSteadyProtectionRejectsMissingControlsChangesAndIncompleteEvidence(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*ProtectedCaptureReport)
	}{
		{"wrong API", func(r *ProtectedCaptureReport) { r.API = StreamAPI }},
		{"extra managed target", func(r *ProtectedCaptureReport) { r.WindowCount = 2 }},
		{"reconnection missing", func(r *ProtectedCaptureReport) { r.Sessions = r.Sessions[:1] }},
		{"image filename reused after reconnect", func(r *ProtectedCaptureReport) { r.Sessions[1].Frames[0].File = r.Sessions[0].Frames[0].File }},
		{"protection initially off", func(r *ProtectedCaptureReport) { r.Sessions[0].Before.PropertySet = false }},
		{"protection ended off", func(r *ProtectedCaptureReport) { r.Sessions[0].After.PropertySet = false }},
		{"target locally hidden", func(r *ProtectedCaptureReport) { r.Sessions[0].Before.LocalVisible = false }},
		{"target locally transparent", func(r *ProtectedCaptureReport) { r.Sessions[0].After.Alpha = 0 }},
		{"late initial acknowledgement", func(r *ProtectedCaptureReport) { r.Sessions[0].Before.AppliedNanos = r.Sessions[0].StartedNanos + 1 }},
		{"early final acknowledgement", func(r *ProtectedCaptureReport) { r.Sessions[0].After.AppliedNanos = r.Sessions[0].StoppedNanos - 1 }},
		{"stream restarted", func(r *ProtectedCaptureReport) { r.Sessions[0].Starts = 2 }},
		{"stream not stopped", func(r *ProtectedCaptureReport) { r.Sessions[0].Stops = 0 }},
		{"stream error", func(r *ProtectedCaptureReport) { r.Sessions[0].ErrorCode = -1 }},
		{"short session", func(r *ProtectedCaptureReport) {
			r.Sessions[0].StoppedNanos = r.Sessions[0].StartedNanos + 1_000_000_000
		}},
		{"missing frame", func(r *ProtectedCaptureReport) { r.Sessions[0].Frames = nil }},
		{"missing output", func(r *ProtectedCaptureReport) { r.Sessions[0].Outputs = r.Sessions[0].Outputs[1:] }},
		{"wrong idle image", func(r *ProtectedCaptureReport) { r.Sessions[0].Outputs[1].FrameIndex = 1 }},
		{"missing raw pixels", func(r *ProtectedCaptureReport) { r.Sessions[0].Frames[0].RawColorRatio = nil }},
		{"control absent from raw buffer", func(r *ProtectedCaptureReport) {
			raw := 0.0
			r.Sessions[0].Frames[0].RawColorRatio = &raw
		}},
		{"foreign owner", func(r *ProtectedCaptureReport) { r.Sessions[0].Selection.ActualOwnerPID++ }},
		{"background missing", func(r *ProtectedCaptureReport) { r.Sessions[0].Selection.Background = nil }},
		{"different reconnect source", func(r *ProtectedCaptureReport) {
			r.Sessions[1].Selection.ExpectedWindowID += 100
			r.Sessions[1].Selection.ActualWindowID += 100
		}},
		{"overlapping sessions", func(r *ProtectedCaptureReport) {
			r.Sessions[1].Before.AppliedNanos = r.Sessions[0].After.AppliedNanos - 1
		}},
		{"positive control missing", func(r *ProtectedCaptureReport) { r.Sessions[0].Frames[0].BlueRatio = 0 }},
		{"blank output", func(r *ProtectedCaptureReport) {
			r.Sessions[0].Frames[0].CapturePixels = CapturePixels{Width: 390, Height: 170, BlackRatio: 1}
		}},
		{"output gap", func(r *ProtectedCaptureReport) {
			for i := 100; i < 104; i++ {
				r.Sessions[0].Outputs[i].ReceivedNanos = r.Sessions[0].Outputs[99].ReceivedNanos
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := protectedCaptureFixture()
			test.change(&r)
			if r.Evaluate() || r.Outcome != "inconclusive" {
				t.Fatal("unreliable steady-protection evidence accepted", r)
			}
		})
	}
}

func TestStoredSteadyProtectionEvidenceAndRawArchives(t *testing.T) {
	for i := 1; i <= 3; i++ {
		directory := filepath.Join("../../../../docs/compatibility/evidence/macos-27-steady-protection-2026-11-09", fmt.Sprintf("run-%d", i))
		data, err := os.ReadFile(filepath.Join(directory, "report.json"))
		if err != nil {
			t.Fatal(err)
		}
		var report Report
		if err := json.Unmarshal(data, &report); err != nil || !report.Evaluate() || report.ProtectedCapture == nil || report.ProtectedCapture.Outcome != "background-only" {
			t.Fatal("stored native evidence failed current evaluation", directory, err, report)
		}
		archive, err := zip.OpenReader(filepath.Join(directory, "frames.zip"))
		if err != nil {
			t.Fatal(err)
		}
		files := map[string]bool{}
		for _, file := range archive.File {
			files[file.Name] = true
		}
		total := 0
		for _, session := range report.ProtectedCapture.Sessions {
			for _, frame := range session.Frames {
				if !files[frame.File] || filepath.Base(frame.File) != frame.File {
					t.Errorf("raw image missing from archive: %s/%s", directory, frame.File)
				}
				total++
			}
		}
		if len(archive.File) != total || len(files) != total {
			t.Error("raw archive has missing or duplicate entries", directory)
		}
		if err := archive.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
