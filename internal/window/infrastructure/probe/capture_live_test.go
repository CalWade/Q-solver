package probe

import (
	"fmt"
	"testing"
)

func liveCaptureFixture() LiveCaptureReport {
	r := LiveCaptureReport{API: StreamAPI, WindowCount: 1, Starts: 1, Stops: 1, StartedNanos: 1_100_000_000, StoppedNanos: 7_400_000_000,
		FrameStatuses: map[string]int{"complete": 15, "idle": 0, "blank": 0, "suspended": 0, "started": 0, "stopped": 0, "unknown": 0}, OutputCount: 15}
	for i, stage := range []string{"baseline-before", "sharing-none", "baseline-after"} {
		applied := int64(1_000_000_000 + i*2_100_000_000)
		r.Transitions = append(r.Transitions, CaptureTransition{Stage: stage, AppliedNanos: applied, PropertySet: i == 1, LocalVisible: true, Alpha: 1})
		for j := 0; j < 5; j++ {
			pixels := CapturePixels{Width: 390, Height: 170, RedRatio: .4, GreenRatio: .4}
			rawColor := 1.0
			if i == 1 {
				pixels.RedRatio, pixels.GreenRatio, pixels.TransparentRatio = 0, 0, 1
				rawColor = 0
			}
			display := applied + 100_000_000 + int64(j)*400_000_000
			r.Frames = append(r.Frames, CaptureFrame{CapturePixels: pixels, DisplayNanos: display, PresentationNanos: display, ReceivedNanos: display + 10_000_000, RawColorRatio: &rawColor, File: fmt.Sprintf("frame-%d-%d.png", i, j)})
		}
	}
	return r
}

func TestLiveCaptureReportsEveryProtectedVisibleFrame(t *testing.T) {
	r := liveCaptureFixture()
	if !r.Evaluate() || r.Outcome != "transparent" || r.Phases[1].Frames != 5 {
		t.Fatal(r)
	}
	r.Frames[5].CapturePixels = r.Frames[0].CapturePixels
	if !r.Evaluate() || r.Outcome != "transition-leak" || r.Phases[1].Outcomes["visible"] != 1 {
		t.Fatal("visible transition frame was ignored", r)
	}
	if *r.Phases[1].LastVisibleMillis != 100 || *r.Phases[1].FirstTransparentMillis != 500 {
		t.Fatal("wrong transition latency", r.Phases[1])
	}
	for i := 5; i < 10; i++ {
		r.Frames[i].CapturePixels = r.Frames[0].CapturePixels
	}
	if !r.Evaluate() || r.Outcome != "visible" {
		t.Fatal("diagnostic completion hid an ineffective policy", r)
	}
}

func TestLiveCaptureClassifiesQueuedFramesByDisplayTime(t *testing.T) {
	r := liveCaptureFixture()
	queued := r.Frames[4]
	queued.DisplayNanos, queued.PresentationNanos, queued.ReceivedNanos = 3_000_000_000, 3_000_000_000, 3_200_000_000
	queued.File = "queued.png"
	r.Frames = append(r.Frames[:5], append([]CaptureFrame{queued}, r.Frames[5:]...)...)
	r.OutputCount++
	r.FrameStatuses["complete"]++
	if !r.Evaluate() || r.Phases[0].Frames != 6 || r.Outcome != "transparent" {
		t.Fatal("queued pre-change image became a post-change observation", r)
	}
	// Restoration latency stays visible in the trace while the final control
	// must still end with three normal visible frames.
	r.Frames[11].CapturePixels = r.Frames[6].CapturePixels
	if !r.Evaluate() || *r.Phases[2].FirstVisibleMillis != 500 {
		t.Fatal("restoration transition was lost", r)
	}
}

func TestLiveCaptureKeepsStartupFramesAndSeparatesPixelOutcomes(t *testing.T) {
	r := liveCaptureFixture()
	startup := r.Frames[0]
	startup.DisplayNanos, startup.PresentationNanos, startup.ReceivedNanos = 500_000_000, 500_000_000, 510_000_000
	startup.File = "startup.png"
	r.Frames = append([]CaptureFrame{startup}, r.Frames...)
	r.OutputCount++
	r.FrameStatuses["complete"]++
	if !r.Evaluate() || r.StartupFrames != 1 || len(r.Frames) != 16 {
		t.Fatal("startup frame disappeared from raw evidence", r)
	}
	for _, outcome := range []struct {
		pixels CapturePixels
		want   string
	}{
		{CapturePixels{Width: 390, Height: 170, BlackRatio: 1}, "blank"},
		{CapturePixels{Width: 390, Height: 170}, "marker_absent"},
	} {
		r = liveCaptureFixture()
		for i := 5; i < 10; i++ {
			r.Frames[i].CapturePixels = outcome.pixels
		}
		if !r.Evaluate() || r.Outcome != outcome.want {
			t.Fatal("wrong uniform pixel outcome", r)
		}
	}
	r = liveCaptureFixture()
	r.Frames[5].CapturePixels = CapturePixels{Width: 390, Height: 170, BlackRatio: 1}
	if r.Evaluate() || r.Outcome != "inconclusive" {
		t.Fatal("mixed nonvisible outcomes were certified", r)
	}
}

func TestTransparentAlphaDoesNotHideRawColorDataFromTheReport(t *testing.T) {
	r := liveCaptureFixture()
	color := .8
	for i := 5; i < 10; i++ {
		r.Frames[i].RawColorRatio = &color
	}
	if !r.Evaluate() || r.Outcome != "transparent-with-data" || r.Phases[1].Outcomes["transparent"] != 0 {
		t.Fatal("raw color data was certified as transparent content", r)
	}
}

func TestIncompleteOrUnreliableLiveTraceCannotProveProtection(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*LiveCaptureReport)
	}{
		{"wrong API", func(r *LiveCaptureReport) { r.API = ScreenshotAPI }},
		{"extra managed window", func(r *LiveCaptureReport) { r.WindowCount = 2 }},
		{"restarted stream", func(r *LiveCaptureReport) { r.Starts = 2 }},
		{"stop missing", func(r *LiveCaptureReport) { r.Stops = 0 }},
		{"stop failed", func(r *LiveCaptureReport) { r.ErrorCode = -1 }},
		{"transition missing", func(r *LiveCaptureReport) { r.Transitions = r.Transitions[:2] }},
		{"wrong property", func(r *LiveCaptureReport) { r.Transitions[1].PropertySet = false }},
		{"local window hidden", func(r *LiveCaptureReport) { r.Transitions[1].LocalVisible = false }},
		{"local window transparent", func(r *LiveCaptureReport) { r.Transitions[1].Alpha = 0 }},
		{"transition order", func(r *LiveCaptureReport) { r.Transitions[1].AppliedNanos = r.Transitions[0].AppliedNanos }},
		{"short phase", func(r *LiveCaptureReport) { r.Transitions[2].AppliedNanos -= 500_000_000 }},
		{"missing display clock", func(r *LiveCaptureReport) { r.Frames[5].DisplayNanos = 0 }},
		{"duplicate image time", func(r *LiveCaptureReport) { r.Frames[5].DisplayNanos = r.Frames[4].DisplayNanos }},
		{"receive clock invalid", func(r *LiveCaptureReport) { r.Frames[5].ReceivedNanos = 1 }},
		{"missing raw image", func(r *LiveCaptureReport) { r.Frames[5].File = "" }},
		{"bad pixels", func(r *LiveCaptureReport) { r.Frames[5].TransparentRatio = -1 }},
		{"raw pixel measurement missing", func(r *LiveCaptureReport) { r.Frames[5].RawColorRatio = nil }},
		{"raw pixel measurement invalid", func(r *LiveCaptureReport) { value := 2.0; r.Frames[5].RawColorRatio = &value }},
		{"missing metadata", func(r *LiveCaptureReport) { r.FrameStatuses = nil }},
		{"suspended output", func(r *LiveCaptureReport) { r.FrameStatuses["suspended"] = 1 }},
		{"missing output", func(r *LiveCaptureReport) { r.OutputCount++ }},
		{"short frame span", func(r *LiveCaptureReport) {
			for i := 5; i < 10; i++ {
				display := r.Transitions[1].AppliedNanos + int64(i-4)*100_000_000
				r.Frames[i].DisplayNanos, r.Frames[i].PresentationNanos, r.Frames[i].ReceivedNanos = display, display, display+1_000_000
			}
		}},
		{"baseline failed", func(r *LiveCaptureReport) { r.Frames[4].GreenRatio = 0 }},
		{"restored baseline failed", func(r *LiveCaptureReport) { r.Frames[14].GreenRatio = 0 }},
		{"post-stop frame", func(r *LiveCaptureReport) { r.StoppedNanos = r.Frames[14].DisplayNanos - 1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := liveCaptureFixture()
			test.change(&r)
			if r.Evaluate() || r.Outcome != "inconclusive" {
				t.Fatal("invalid live evidence accepted", r)
			}
		})
	}
}
