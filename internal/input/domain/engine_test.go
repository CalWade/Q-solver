package domain

import (
	"fmt"
	"testing"
)

func testPolicy(modifier Key) Policy {
	return Policy{Modifier: modifier, Protection: Protection{Enabled: true, BlockLeft: true}, Bindings: []Binding{
		{Action: "screenshot", Keys: []Key{modifier, "KeyA"}},
		{Action: "scroll", Keys: []Key{modifier, "ArrowDown"}, Repeat: true},
	}}
}

func mustEngine(t *testing.T, p Policy) *Engine {
	t.Helper()
	e, err := NewEngine(p)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestAddingModifierToHeldMainKeyDoesNotTriggerAnAction(t *testing.T) {
	for _, key := range []Key{"KeyA", "ArrowDown"} {
		e := mustEngine(t, testPolicy(Control))
		if e.Process(Event{Key: key, Down: true}).Suppress {
			t.Fatal("ordinary key swallowed")
		}
		e.Process(Event{Key: "ControlLeft", Down: true})
		if d := e.Process(Event{Key: key, Down: true, Repeat: true}); d.Action != "" {
			t.Fatal("late modifier triggered", d)
		}
		e.Process(Event{Key: "ControlLeft"})
		if e.Process(Event{Key: key}).Suppress {
			t.Fatal("originally forwarded release lost")
		}
	}
}

func TestDisplayKeysKeepsModifiersFirstWithoutMutatingBinding(t *testing.T) {
	keys := []Key{"Digit1", Meta}
	if label := DisplayKeys(keys); label != "Cmd+1" {
		t.Fatal(label)
	}
	if keys[0] != "Digit1" {
		t.Fatal("mutated input")
	}
}

func TestProtectedSequence(t *testing.T) {
	for _, mod := range []Key{Control, Meta} {
		for _, side := range []string{"Left", "Right"} {
			for _, modifierFirst := range []bool{true, false} {
				t.Run(fmt.Sprintf("%s/%s/releaseModifierFirst=%v", mod, side, modifierFirst), func(t *testing.T) {
					p := testPolicy(mod)
					p.Protection.BlockLeft, p.Protection.BlockRight = side == "Left", side == "Right"
					e := mustEngine(t, p)
					modifier := Key(string(mod) + side)
					sequence := []Event{{Key: modifier, Down: true}, {Key: "KeyA", Down: true}, {Key: "KeyA", Down: true, Repeat: true}}
					if modifierFirst {
						sequence = append(sequence, Event{Key: modifier}, Event{Key: "KeyA"})
					} else {
						sequence = append(sequence, Event{Key: "KeyA"}, Event{Key: modifier})
					}
					actions := 0
					for _, event := range sequence {
						d := e.Process(event)
						if !d.Suppress {
							t.Errorf("leaked event: %+v", event)
						}
						if d.VisibleModifiers != 0 {
							t.Errorf("leaked modifier flags: %+v", d)
						}
						if d.Action == "screenshot" {
							actions++
						}
					}
					if actions != 1 || e.HeldCount() != 0 {
						t.Fatalf("actions=%d held=%d", actions, e.HeldCount())
					}
					for _, event := range []Event{{Key: "KeyZ", Down: true}, {Key: "KeyZ"}} {
						if e.Process(event).Suppress {
							t.Fatal("normal typing did not recover")
						}
					}
				})
			}
		}
	}
}

func TestUnselectedSideAndUnknownCombination(t *testing.T) {
	e := mustEngine(t, testPolicy(Control))
	for _, event := range []Event{{Key: "ControlRight", Down: true}, {Key: "KeyA", Down: true}, {Key: "KeyA"}, {Key: "ControlRight"}} {
		d := e.Process(event)
		if d.Suppress || d.Action != "" {
			t.Fatalf("unselected side changed: %+v", d)
		}
	}
	for _, event := range []Event{{Key: "ControlLeft", Down: true}, {Key: "KeyZ", Down: true}, {Key: "ControlLeft"}, {Key: "KeyZ"}} {
		d := e.Process(event)
		if !d.Suppress || d.Action != "" {
			t.Fatalf("unknown combination leaked or triggered: %+v", d)
		}
	}
}

func TestStandaloneModifiersAndBothSides(t *testing.T) {
	e := mustEngine(t, testPolicy(Meta))
	if !e.Process(Event{Key: "MetaLeft", Down: true}).Suppress || !e.Process(Event{Key: "MetaLeft"}).Suppress {
		t.Fatal("standalone modifier leaked")
	}
	e.Process(Event{Key: "MetaLeft", Down: true})
	e.Process(Event{Key: "MetaRight", Down: true})
	if d := e.Process(Event{Key: "KeyA", Down: true}); d.Action != "screenshot" || !d.Suppress {
		t.Fatalf("both sides: %+v", d)
	}
	e.Process(Event{Key: "MetaLeft"})
	e.Process(Event{Key: "KeyA"})
	e.Process(Event{Key: "MetaRight"})
	if e.HeldCount() != 0 {
		t.Fatal("keys stuck")
	}
}

func TestPendingPolicyDrainsOldPress(t *testing.T) {
	for _, change := range []string{"disable", "pause", "rebind", "switch-side"} {
		t.Run(change, func(t *testing.T) {
			p := testPolicy(Control)
			e := mustEngine(t, p)
			e.Process(Event{Key: "ControlLeft", Down: true})
			e.Process(Event{Key: "KeyA", Down: true})
			switch change {
			case "disable":
				p.Protection.Enabled = false
			case "pause":
				p.Suspended = true
			case "rebind":
				p.Bindings[0].Keys = []Key{Control, "KeyB"}
			case "switch-side":
				p.Protection.BlockLeft, p.Protection.BlockRight = false, true
			}
			if err := e.Configure(p); err != nil {
				t.Fatal(err)
			}
			if !e.Pending() {
				t.Fatal("did not defer policy")
			}
			if d := e.Process(Event{Key: "KeyA", Down: true}); !d.Suppress || d.Action != "" {
				t.Fatalf("repeat during transition: %+v", d)
			}
			if !e.Process(Event{Key: "ControlLeft"}).Suppress || !e.Process(Event{Key: "KeyA"}).Suppress {
				t.Fatal("release ownership lost")
			}
			if e.Pending() {
				t.Fatal("policy did not commit after release")
			}
			if change == "rebind" {
				e.Process(Event{Key: "ControlLeft", Down: true})
				if d := e.Process(Event{Key: "KeyB", Down: true}); d.Action != "screenshot" {
					t.Fatalf("new binding not used: %+v", d)
				}
			}
		})
	}
}

func TestPauseAndResume(t *testing.T) {
	p := testPolicy(Meta)
	e := mustEngine(t, p)
	p.Suspended = true
	e.Configure(p)
	for _, event := range []Event{{Key: "MetaLeft", Down: true}, {Key: "KeyA", Down: true}, {Key: "KeyA"}, {Key: "MetaLeft"}} {
		d := e.Process(event)
		if d.Suppress || d.Action != "" {
			t.Fatalf("editing intercepted: %+v", d)
		}
	}
	p.Suspended = false
	e.Configure(p)
	if !e.Process(Event{Key: "MetaLeft", Down: true}).Suppress {
		t.Fatal("did not resume")
	}
}

func TestPreviouslyDeliveredKeyKeepsRelease(t *testing.T) {
	e := mustEngine(t, testPolicy(Control))
	if e.Process(Event{Key: "KeyZ", Down: true}).Suppress {
		t.Fatal("normal key suppressed")
	}
	e.Process(Event{Key: "ControlLeft", Down: true})
	if !e.Process(Event{Key: "KeyZ", Down: true, Repeat: true}).Suppress {
		t.Fatal("repeat should be protected")
	}
	if e.Process(Event{Key: "KeyZ"}).Suppress {
		t.Fatal("lost release of already-delivered key")
	}
	e.Process(Event{Key: "ControlLeft"})
}

func TestSeedDoesNotSwallowExistingModifierRelease(t *testing.T) {
	e := mustEngine(t, testPolicy(Control))
	e.Seed([]Key{"ControlLeft"})
	if !e.Pending() {
		t.Fatal("must wait for existing press")
	}
	if e.Process(Event{Key: "ControlLeft"}).Suppress {
		t.Fatal("existing release swallowed")
	}
	if !e.Process(Event{Key: "ControlLeft", Down: true}).Suppress {
		t.Fatal("not armed on next press")
	}
}

func TestRepeatIsOptIn(t *testing.T) {
	e := mustEngine(t, testPolicy(Control))
	e.Process(Event{Key: "ControlLeft", Down: true})
	for i := 0; i < 3; i++ {
		if d := e.Process(Event{Key: "ArrowDown", Down: true, Repeat: i > 0}); d.Action != "scroll" {
			t.Fatalf("repeat %d: %+v", i, d)
		}
	}
}

func TestConfigurationIsCopiedAndValidated(t *testing.T) {
	p := testPolicy(Control)
	e := mustEngine(t, p)
	p.Bindings[0].Keys[1] = "KeyZ"
	e.Process(Event{Key: "ControlLeft", Down: true})
	if e.Process(Event{Key: "KeyA", Down: true}).Action != "screenshot" {
		t.Fatal("caller mutated active policy")
	}
	p.Protection = Protection{Enabled: true}
	if e.Configure(p) == nil {
		t.Fatal("invalid protection accepted")
	}
}

func TestBindingValidation(t *testing.T) {
	for _, keys := range [][]Key{{}, {Control}, {"KeyA", "KeyA"}, {"Bogus"}, {Control, "ControlLeft", "KeyA"}} {
		if _, err := CanonicalKeys(keys); err == nil {
			t.Fatalf("accepted invalid keys %v", keys)
		}
	}
	p := testPolicy(Control)
	p.Bindings = append(p.Bindings, Binding{Action: "conflict", Keys: []Key{"ControlLeft", "KeyA"}})
	if p.Validate() == nil {
		t.Fatal("generic/side-specific conflict accepted")
	}
	p.Bindings = []Binding{{Action: "left", Keys: []Key{"ControlLeft", "KeyA"}}, {Action: "right", Keys: []Key{"ControlRight", "KeyA"}}}
	if err := p.Validate(); err != nil {
		t.Fatalf("independent sides conflict: %v", err)
	}
}

func TestNormalShortcutsWithoutProtection(t *testing.T) {
	p := testPolicy(Control)
	p.Protection.Enabled = false
	e := mustEngine(t, p)
	if e.Process(Event{Key: "ControlLeft", Down: true}).Suppress {
		t.Fatal("modifier blocked with protection off")
	}
	if d := e.Process(Event{Key: "KeyA", Down: true}); d.Action != "screenshot" || !d.Suppress {
		t.Fatalf("shortcut lost: %+v", d)
	}
	if !e.Process(Event{Key: "KeyA"}).Suppress {
		t.Fatal("shortcut release leaked")
	}
	if e.Process(Event{Key: "ControlLeft"}).Suppress {
		t.Fatal("modifier release lost")
	}
}
