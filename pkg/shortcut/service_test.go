package shortcut

import (
	"Q-Solver/internal/input/application"
	"fmt"
	"testing"
)

type recordingDelegate struct {
	commands []string
	event    string
}

func (d *recordingDelegate) TriggerSolve()            { d.commands = append(d.commands, "solve") }
func (d *recordingDelegate) TriggerScreenshot()       { d.commands = append(d.commands, "screenshot") }
func (d *recordingDelegate) TriggerSend()             { d.commands = append(d.commands, "send") }
func (d *recordingDelegate) TriggerDeleteScreenshot() { d.commands = append(d.commands, "delete") }
func (d *recordingDelegate) ToggleVisibility()        { d.commands = append(d.commands, "toggle") }
func (d *recordingDelegate) ToggleClickThrough()      { d.commands = append(d.commands, "clickthrough") }
func (d *recordingDelegate) MoveWindow(x, y int) {
	d.commands = append(d.commands, fmt.Sprintf("move:%d:%d", x, y))
}
func (d *recordingDelegate) ScrollContent(direction string) {
	d.commands = append(d.commands, "scroll:"+direction)
}
func (d *recordingDelegate) PanicExit()                              { d.commands = append(d.commands, "panic") }
func (d *recordingDelegate) EmitEvent(name string, _ ...interface{}) { d.event = name }
func TestCommandAdapterDispatchesEverySupportedAction(t *testing.T) {
	cases := map[string]string{"screenshot": "screenshot", "send": "send", "delete": "delete", "toggle": "toggle", "clickthrough": "clickthrough", "move_up": "move:0:-10", "move_down": "move:0:10", "move_left": "move:-10:0", "move_right": "move:10:0", "scroll_up": "scroll:up", "scroll_down": "scroll:down", "panic": "panic"}
	d := &recordingDelegate{}
	s := &Service{delegate: d}
	for action, want := range cases {
		s.Execute(action)
		if got := d.commands[len(d.commands)-1]; got != want {
			t.Fatalf("%s: %s", action, got)
		}
	}
	s.Execute("unknown")
	if len(d.commands) != len(cases) {
		t.Fatal("unknown command dispatched")
	}
	s.InputStatusChanged(application.Status{State: "active"})
	if d.event != "keyboard-protection-status" {
		t.Fatal(d.event)
	}
}
