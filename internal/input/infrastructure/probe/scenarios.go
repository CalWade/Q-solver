package probe

import "Q-Solver/internal/input/domain"

const FixtureAction = "fixture-capture"

type Packet struct {
	Key    domain.Key
	Down   bool
	Repeat bool
	Tagged bool
}

type Scenario struct {
	ID            string
	Protection    bool
	OwnEditor     bool
	EditingIntent bool
	Packets       []Packet
	Suppressed    []bool
	Actions       int
}

func down(key domain.Key) Packet { return Packet{Key: key, Down: true, Tagged: true} }
func up(key domain.Key) Packet   { return Packet{Key: key, Tagged: true} }

func chord(side domain.Key) []Packet {
	return []Packet{down(side), down("KeyA"), up("KeyA"), up(side), down("KeyB"), up("KeyB")}
}

func (s Scenario) Policy() domain.Policy {
	policy := domain.Policy{Modifier: domain.Meta, Protection: domain.Protection{Enabled: s.Protection, BlockLeft: true}}
	if s.Protection {
		policy.Bindings = []domain.Binding{{Action: FixtureAction, Keys: []domain.Key{domain.Meta, "KeyA"}}}
	}
	return policy
}

func PassiveScenarios() []Scenario {
	repeated := down("KeyA")
	repeated.Repeat = true
	untagged := chord("MetaLeft")
	for i := 0; i < 4; i++ {
		untagged[i].Tagged = false
	}
	return []Scenario{
		{ID: "baseline-left", Packets: chord("MetaLeft"), Suppressed: []bool{false, false, false, false, false, false}},
		{ID: "protected-left", Protection: true, Packets: chord("MetaLeft"), Suppressed: []bool{true, true, true, true, false, false}, Actions: 1},
		{ID: "release-order-repeat", Protection: true,
			Packets:    []Packet{down("MetaLeft"), down("KeyA"), repeated, repeated, up("MetaLeft"), up("KeyA"), down("KeyB"), up("KeyB")},
			Suppressed: []bool{true, true, true, true, true, true, false, false}, Actions: 1},
		{ID: "baseline-right", Packets: chord("MetaRight"), Suppressed: []bool{false, false, false, false, false, false}},
		{ID: "unselected-right", Protection: true, Packets: chord("MetaRight"), Suppressed: []bool{false, false, false, false, false, false}},
		{ID: "both-sides", Protection: true,
			Packets:    []Packet{down("MetaLeft"), down("MetaRight"), down("KeyA"), up("KeyA"), up("MetaLeft"), up("MetaRight"), down("KeyB"), up("KeyB")},
			Suppressed: []bool{true, true, true, true, true, true, false, false}, Actions: 1},
		{ID: "untagged-control", Protection: true, Packets: untagged, Suppressed: []bool{false, false}},
		{ID: "protection-off", Packets: chord("MetaLeft"), Suppressed: []bool{false, false, false, false, false, false}},
	}
}

func EditorScenario() Scenario {
	return Scenario{ID: "own-editor", Protection: true, OwnEditor: true, EditingIntent: true, Packets: chord("MetaLeft"), Suppressed: []bool{false, false, false, false, false, false}}
}

func StaleEditorScenario() Scenario {
	return Scenario{ID: "stale-editor-intent", Protection: true, EditingIntent: true, Packets: chord("MetaLeft"), Suppressed: []bool{true, true, true, true, false, false}, Actions: 1}
}

func RestoredScenario() Scenario {
	return Scenario{ID: "restored-protection", Protection: true, Packets: chord("MetaLeft"), Suppressed: []bool{true, true, true, true, false, false}, Actions: 1}
}

func AllScenarios() []Scenario {
	return append(PassiveScenarios(), EditorScenario(), StaleEditorScenario(), RestoredScenario())
}
