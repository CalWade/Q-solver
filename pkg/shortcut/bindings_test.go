package shortcut

import (
	"Q-Solver/internal/input/domain"
	"reflect"
	"testing"
)

func TestLegacyBindingMigration(t *testing.T) {
	for _, tc := range []struct {
		old  string
		want []domain.Key
	}{{"Cmd+1", []domain.Key{"Digit1", domain.Meta}}, {"Cmd+Return", []domain.Key{"Enter", domain.Meta}}, {"18+49", []domain.Key{domain.Alt, "Digit1"}}, {"74+162", []domain.Key{"ControlLeft", "KeyJ"}}, {"Cmd+Option+Shift+Down", []domain.Key{domain.Alt, "ArrowDown", domain.Meta, domain.Shift}}, {"Control+KeyB", []domain.Key{domain.Control, "KeyB"}}} {
		t.Run(tc.old, func(t *testing.T) {
			got, err := ParseBinding(KeyBinding{ComboID: tc.old})
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got=%v err=%v want=%v", got, err, tc.want)
			}
		})
	}
}

func TestChangingLetterChangesCompiledPolicy(t *testing.T) {
	bindings := map[string]KeyBinding{"screenshot": {Keys: []domain.Key{domain.Meta, "KeyA"}}}
	p, err := Profile(bindings, domain.Protection{Enabled: true, BlockLeft: true}, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	bindings["screenshot"] = KeyBinding{Keys: []domain.Key{domain.Meta, "KeyB"}}
	updated, err := Profile(bindings, p.Protection, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(p.Bindings, updated.Bindings) {
		t.Fatal("letter update not compiled")
	}
}

func TestProfilesRejectConflictsAndUnknownActions(t *testing.T) {
	for _, bindings := range []map[string]KeyBinding{{"unknown": {ComboID: "Cmd+1"}}, {"screenshot": {ComboID: "Cmd+1"}, "send": {ComboID: "Cmd+1"}}} {
		if _, err := Profile(bindings, domain.Protection{}, "darwin"); err == nil {
			t.Fatal("invalid profile accepted")
		}
	}
}
