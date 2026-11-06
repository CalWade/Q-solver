package config

import (
	"Q-Solver/internal/input/domain"
	"Q-Solver/pkg/shortcut"
	"runtime"
	"testing"
)

func TestProtectionPresetCoversAllActionsWithoutConflicts(t *testing.T) {
	bindings := GetProtectionShortcuts()
	p, err := shortcut.Profile(bindings, domain.Protection{Enabled: true, BlockLeft: true}, runtime.GOOS)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Bindings) != len(GetDefaultShortcuts()) {
		t.Fatal("preset lost actions")
	}
	for _, b := range p.Bindings {
		covered := false
		for _, k := range b.Keys {
			covered = covered || domain.Family(k) == p.Modifier
		}
		if !covered {
			t.Fatal("unprotected preset", b)
		}
	}
}
