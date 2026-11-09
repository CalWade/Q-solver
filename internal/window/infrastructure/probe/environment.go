package probe

type CaptureEnvironment struct {
	SessionKnown      bool `json:"sessionKnown"`
	SessionLocked     bool `json:"sessionLocked"`
	OnConsole         bool `json:"onConsole"`
	DisplaysKnown     bool `json:"displaysKnown"`
	ActiveDisplays    int  `json:"activeDisplays"`
	MainDisplayAsleep bool `json:"mainDisplayAsleep"`
}

func (e CaptureEnvironment) Available() bool {
	return e.SessionKnown && !e.SessionLocked && e.OnConsole && e.DisplaysKnown && e.ActiveDisplays > 0 && !e.MainDisplayAsleep
}
