package main

import "testing"

func TestCaptureModesAreExplicitAndMutuallyExclusive(t *testing.T) {
	for _, test := range []struct {
		screenshot, stream, live, protected bool
		want                                captureMode
		invalid                             bool
	}{
		{want: captureNone},
		{screenshot: true, want: captureScreenshot},
		{stream: true, want: captureFreshStream},
		{live: true, want: captureLiveStream},
		{protected: true, want: captureProtectedSession},
		{screenshot: true, protected: true, invalid: true},
		{stream: true, protected: true, invalid: true},
		{live: true, protected: true, invalid: true},
		{screenshot: true, stream: true, invalid: true},
		{screenshot: true, live: true, invalid: true},
		{stream: true, live: true, invalid: true},
		{screenshot: true, stream: true, live: true, invalid: true},
	} {
		mode, err := selectCaptureMode(test.screenshot, test.stream, test.live, test.protected)
		if (err != nil) != test.invalid || mode != test.want {
			t.Fatalf("mode=%v error=%v for %+v", mode, err, test)
		}
	}
}

func TestSteadyProtectionRequiresItsOwnExplicitDisplayMode(t *testing.T) {
	if err := validateDisplayFilter(captureProtectedSession, true, false); err != nil {
		t.Fatal(err)
	}
	if err := validateDisplayFilter(captureProtectedSession, false, false); err == nil {
		t.Fatal("steady protection accepted without an owned display filter")
	}
	if err := validateDisplayFilter(captureProtectedSession, true, true); err == nil {
		t.Fatal("steady protection accepted toggle-mode options")
	}
}

func TestDisplayInclusionIsLimitedToLiveDiagnostics(t *testing.T) {
	for _, mode := range []captureMode{captureNone, captureScreenshot, captureFreshStream, captureLiveStream} {
		if err := validateDisplayFilter(mode, false, false); err != nil {
			t.Fatal(err)
		}
		if err := validateDisplayFilter(mode, true, false); (err == nil) != (mode == captureLiveStream) {
			t.Fatalf("unexpected display filter result for mode %v: %v", mode, err)
		}
		if err := validateDisplayFilter(mode, true, true); (err == nil) != (mode == captureLiveStream) {
			t.Fatalf("unexpected protected startup result for mode %v: %v", mode, err)
		}
		if err := validateDisplayFilter(mode, false, true); err == nil {
			t.Fatal("protected startup accepted without an owned-window display filter")
		}
	}
}
