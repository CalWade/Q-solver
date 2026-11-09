package probe

import "Q-Solver/internal/input/domain"

type EventRecord struct {
	Event    domain.Event    `json:"event"`
	Decision domain.Decision `json:"decision"`
}

type BrowserEvent struct {
	Code    string `json:"code"`
	Type    string `json:"type"`
	Repeat  bool   `json:"repeat"`
	Meta    bool   `json:"meta"`
	Trusted bool   `json:"trusted"`
}

type EditorObservation struct {
	Events          []BrowserEvent `json:"events"`
	Focused         bool           `json:"focused"`
	FixtureReplaced bool           `json:"fixtureReplaced"`
}

type Case struct {
	ID                 string             `json:"id"`
	Events             []EventRecord      `json:"events"`
	Actions            []string           `json:"actions"`
	ForegroundRetained bool               `json:"foregroundRetained"`
	Editor             *EditorObservation `json:"editor,omitempty"`
	Focus              FocusFacts         `json:"focus"`
}

type FocusFacts struct {
	AppActive    bool `json:"appActive"`
	AppKeyWindow bool `json:"appKeyWindow"`
	KeyWindows   int  `json:"keyWindows"`
	Foreground   bool `json:"foreground"`
}

type Check struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
}

type Report struct {
	Filtering        bool     `json:"filtering"`
	Stopped          bool     `json:"stopped"`
	ListenerReleased bool     `json:"listenerReleased"`
	NativeTapsBefore int      `json:"nativeTapsBefore"`
	NativeTapsAfter  int      `json:"nativeTapsAfter"`
	Scope            string   `json:"scope"`
	Cases            []Case   `json:"cases"`
	Checks           []Check  `json:"checks"`
	Errors           []string `json:"errors,omitempty"`
}

func (r *Report) Evaluate() bool {
	wanted := AllScenarios()
	r.Checks = []Check{
		{"native filtering available", r.Filtering},
		{"native listener stopped", r.Stopped},
		{"native event tap released", r.ListenerReleased && r.NativeTapsBefore >= 0 && r.NativeTapsAfter == r.NativeTapsBefore},
		{"all input scenarios observed", len(r.Cases) == len(wanted)},
	}
	for i, actual := range r.Cases {
		valid := i < len(wanted)
		if valid {
			s := wanted[i]
			var packets []Packet
			for _, packet := range s.Packets {
				if packet.Tagged {
					packets = append(packets, packet)
				}
			}
			valid = actual.ID == s.ID && actual.ForegroundRetained && len(actual.Events) == len(packets) && len(actual.Actions) == s.Actions
			dispatched := 0
			for j, record := range actual.Events {
				if j >= len(packets) {
					valid = false
					break
				}
				packet := packets[j]
				eventMatches := record.Event.Key == packet.Key && record.Event.Down == packet.Down && record.Event.Repeat == packet.Repeat
				contextMatches := record.Event.Injected && record.Event.OwnWindow == s.OwnEditor
				valid = valid && eventMatches && contextMatches && record.Decision.Suppress == s.Suppressed[j]
				if record.Decision.Action != "" {
					dispatched++
					valid = valid && record.Decision.Action == FixtureAction
				}
			}
			valid = valid && dispatched == s.Actions
			for _, action := range actual.Actions {
				valid = valid && action == FixtureAction
			}
			if s.OwnEditor {
				valid = valid && validEditor(actual.Editor) && actual.Focus.AppActive && actual.Focus.Foreground && actual.Focus.KeyWindows > 0
			}
		}
		r.Checks = append(r.Checks, Check{actual.ID, valid})
	}
	passed := len(r.Errors) == 0
	for _, check := range r.Checks {
		passed = passed && check.Passed
	}
	return passed
}

func validEditor(editor *EditorObservation) bool {
	if editor == nil || !editor.Focused || !editor.FixtureReplaced {
		return false
	}
	var modifier, selectAll, barrier bool
	for _, event := range editor.Events {
		if !event.Trusted {
			return false
		}
		modifier = modifier || event.Code == "MetaLeft" && event.Type == "keydown" && event.Meta
		selectAll = selectAll || event.Code == "KeyA" && event.Type == "keydown" && event.Meta
		barrier = barrier || event.Code == "KeyB" && event.Type == "keyup" && !event.Meta
	}
	return modifier && selectAll && barrier
}
