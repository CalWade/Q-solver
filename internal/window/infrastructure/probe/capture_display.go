package probe

import "math"

const DisplayWindowStreamAPI = "ScreenCaptureKit.SCStream / display+includingOwnedWindow"

type CaptureRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func (r CaptureRect) valid() bool {
	for _, value := range []float64{r.X, r.Y, r.Width, r.Height} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return r.Width > 0 && r.Height > 0
}

type DisplayCaptureSelection struct {
	ExpectedWindowID uint32             `json:"expectedWindowID"`
	ActualWindowID   uint32             `json:"actualWindowID"`
	ExpectedOwnerPID int                `json:"expectedOwnerPID"`
	ActualOwnerPID   int                `json:"actualOwnerPID"`
	ExpectedDisplay  uint32             `json:"expectedDisplayID"`
	ActualDisplay    uint32             `json:"actualDisplayID"`
	WindowCount      int                `json:"includedWindows"`
	DisplayCount     int                `json:"includedDisplays"`
	Window           CaptureRect        `json:"windowRect"`
	Display          CaptureRect        `json:"displayRect"`
	Crop             CaptureRect        `json:"sourceRect"`
	Background       *CaptureBackground `json:"background,omitempty"`
}

type CaptureBackground struct {
	ExpectedWindowID uint32      `json:"expectedWindowID"`
	ActualWindowID   uint32      `json:"actualWindowID"`
	OwnerPID         int         `json:"ownerPID"`
	Window           CaptureRect `json:"windowRect"`
}

// The inclusion filter is restricted to the fixture's own window. Geometry is
// checked before starting capture, including displays with negative origins.
func (s DisplayCaptureSelection) Valid() bool {
	identity := s.ExpectedWindowID != 0 && s.ActualWindowID == s.ExpectedWindowID && s.ExpectedOwnerPID > 0 && s.ActualOwnerPID == s.ExpectedOwnerPID
	windows := 1
	if background := s.Background; background != nil {
		windows++
		identity = identity && background.ExpectedWindowID != 0 && background.ExpectedWindowID != s.ExpectedWindowID && background.ActualWindowID == background.ExpectedWindowID
		identity = identity && background.OwnerPID == s.ExpectedOwnerPID && background.Window == s.Window
	}
	identity = identity && s.ExpectedDisplay != 0 && s.ActualDisplay == s.ExpectedDisplay && s.WindowCount == windows && s.DisplayCount == 1
	if !identity || !s.Window.valid() || !s.Display.valid() || !s.Crop.valid() {
		return false
	}
	contained := s.Window.X >= s.Display.X && s.Window.Y >= s.Display.Y && s.Window.X+s.Window.Width <= s.Display.X+s.Display.Width && s.Window.Y+s.Window.Height <= s.Display.Y+s.Display.Height
	expected := CaptureRect{X: s.Window.X - s.Display.X, Y: s.Window.Y - s.Display.Y, Width: s.Window.Width, Height: s.Window.Height}
	return contained && s.Crop == expected
}
