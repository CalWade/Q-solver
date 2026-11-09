package native

import (
	"Q-Solver/internal/input/application"
	"Q-Solver/internal/input/domain"
	"testing"
)

func TestDiagnosticScopeRequiresATagAndRejectsGlobalRegistrations(t *testing.T) {
	if _, err := NewTaggedDiagnostic(0); err == nil {
		t.Fatal("unscoped diagnostic accepted")
	}
	driver, err := NewTaggedDiagnostic(42)
	if err != nil {
		t.Fatal(err)
	}
	plan := application.NativePlan{Bindings: []domain.Binding{{Action: "probe", Keys: []domain.Key{domain.Meta, "KeyA"}}}}
	if err := driver.Apply(plan); err == nil {
		t.Fatal("diagnostic installed an unscoped system shortcut")
	}
}
