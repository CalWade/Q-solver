package probe

import (
	"Q-Solver/internal/input/domain"
	"testing"
)

func reportFixture() Report {
	r := Report{Filtering: true, Stopped: true, ListenerReleased: true}
	for _, scenario := range AllScenarios() {
		c := Case{ID: scenario.ID, ForegroundRetained: true}
		dispatched := false
		for _, packet := range scenario.Packets {
			if packet.Tagged {
				i := len(c.Events)
				decision := domain.Decision{Suppress: scenario.Suppressed[i]}
				if scenario.Actions > 0 && !dispatched && packet.Key == "KeyA" && packet.Down {
					decision.Action = FixtureAction
					dispatched = true
				}
				c.Events = append(c.Events, EventRecord{Event: domain.Event{Key: packet.Key, Down: packet.Down, Repeat: packet.Repeat, Injected: true, OwnWindow: scenario.OwnEditor}, Decision: decision})
			}
		}
		for i := 0; i < scenario.Actions; i++ {
			c.Actions = append(c.Actions, FixtureAction)
		}
		if scenario.OwnEditor {
			c.Focus = FocusFacts{AppActive: true, Foreground: true, AppKeyWindow: true, KeyWindows: 1}
			c.Editor = &EditorObservation{Focused: true, FixtureReplaced: true, Events: []BrowserEvent{{Code: "MetaLeft", Type: "keydown", Meta: true, Trusted: true}, {Code: "KeyA", Type: "keydown", Meta: true, Trusted: true}, {Code: "KeyB", Type: "keyup", Trusted: true}}}
		}
		r.Cases = append(r.Cases, c)
	}
	return r
}

func TestInputEvidenceRequiresCompleteEventsCommandsAndCleanup(t *testing.T) {
	r := reportFixture()
	if !r.Evaluate() {
		t.Fatal(r)
	}
	for _, change := range []func(*Report){
		func(r *Report) { r.Filtering = false },
		func(r *Report) { r.Stopped = false },
		func(r *Report) { r.NativeTapsAfter = 1 },
		func(r *Report) { r.Cases = r.Cases[:len(r.Cases)-1] },
		func(r *Report) { r.Cases[1].Events[3].Decision.Suppress = false },
		func(r *Report) { r.Cases[1].Actions = nil },
		func(r *Report) { r.Cases[2].Actions = []string{FixtureAction, FixtureAction} },
		func(r *Report) { r.Cases[4].Events[0].Decision.Suppress = true },
		func(r *Report) { r.Cases[5].ForegroundRetained = false },
		func(r *Report) { r.Cases[6].Events = r.Cases[0].Events },
		func(r *Report) { r.Cases[8].Editor = nil },
		func(r *Report) { r.Cases[8].Editor.FixtureReplaced = false },
		func(r *Report) { r.Cases[8].Events[0].Event.OwnWindow = false },
		func(r *Report) { r.Cases[8].Editor.Events = r.Cases[8].Editor.Events[2:] },
		func(r *Report) { r.Cases[8].Events[1].Decision.Action = FixtureAction },
	} {
		r = reportFixture()
		change(&r)
		if r.Evaluate() {
			t.Fatal("invalid native evidence accepted")
		}
	}
}

func TestDiagnosticVectorsStayInsideTheFixtureKeyboard(t *testing.T) {
	seen := map[string]bool{}
	for _, scenario := range AllScenarios() {
		if seen[scenario.ID] {
			t.Fatal("duplicate scenario")
		}
		seen[scenario.ID] = true
		if err := scenario.Policy().Validate(); err != nil {
			t.Fatal(err)
		}
		matched := 0
		for _, packet := range scenario.Packets {
			if packet.Tagged {
				matched++
			}
			if packet.Key != "MetaLeft" && packet.Key != "MetaRight" && packet.Key != "KeyA" && packet.Key != "KeyB" {
				t.Fatal("unexpected fixture key", packet)
			}
		}
		if matched != len(scenario.Suppressed) {
			t.Fatal("incomplete expectation")
		}
	}
}
