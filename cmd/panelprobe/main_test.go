package main

import "testing"

func TestCaptureModesAreExplicitAndMutuallyExclusive(t *testing.T) {
	for _, test := range []struct {
		screenshot, stream, live bool
		want                     captureMode
		invalid                  bool
	}{
		{want: captureNone},
		{screenshot: true, want: captureScreenshot},
		{stream: true, want: captureFreshStream},
		{live: true, want: captureLiveStream},
		{screenshot: true, stream: true, invalid: true},
		{screenshot: true, live: true, invalid: true},
		{stream: true, live: true, invalid: true},
		{screenshot: true, stream: true, live: true, invalid: true},
	} {
		mode, err := selectCaptureMode(test.screenshot, test.stream, test.live)
		if (err != nil) != test.invalid || mode != test.want {
			t.Fatalf("mode=%v error=%v for %+v", mode, err, test)
		}
	}
}

func TestDisplayInclusionIsLimitedToLiveDiagnostics(t *testing.T) {
	for _, mode := range []captureMode{captureNone, captureScreenshot, captureFreshStream, captureLiveStream} {
		if err := validateDisplayFilter(mode, false); err != nil {
			t.Fatal(err)
		}
		if err := validateDisplayFilter(mode, true); (err == nil) != (mode == captureLiveStream) {
			t.Fatalf("unexpected display filter result for mode %v: %v", mode, err)
		}
	}
}
