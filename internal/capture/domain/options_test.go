package domain

import "testing"

func TestCaptureOptionsValidateModeAndQuality(t *testing.T) {
	for _, o := range []Options{{Quality: 80, Mode: "window"}, {Quality: 100, Mode: "fullscreen", Sharpen: 5}} {
		if err := o.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, o := range []Options{{Quality: 0, Mode: "window"}, {Quality: 80, Mode: "unknown"}, {Quality: 80, Mode: "window", Sharpen: -1}} {
		if o.Validate() == nil {
			t.Fatal(o)
		}
	}
}
