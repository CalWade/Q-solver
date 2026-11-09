package native

import (
	"Q-Solver/internal/input/application"
	"Q-Solver/internal/input/domain"
	"testing"
)

func TestNativePlansRejectUnsupportedAndConflictingChordsBeforeEffects(t *testing.T) {
	one := domain.Binding{Action: "send", Keys: []domain.Key{domain.Control, "KeyQ"}}
	two := domain.Binding{Action: "capture", Keys: []domain.Key{domain.Control, "KeyQ"}}
	if validatePlan(application.NativePlan{Bindings: []domain.Binding{one}}, func(domain.Binding) bool { return false }) == nil {
		t.Fatal("unsupported plan accepted")
	}
	if validatePlan(application.NativePlan{Bindings: []domain.Binding{one, two}}, func(domain.Binding) bool { return true }) == nil {
		t.Fatal("conflicting plan accepted")
	}
	if err := validatePlan(application.NativePlan{Bindings: []domain.Binding{one}}, func(domain.Binding) bool { return true }); err != nil {
		t.Fatal(err)
	}
}
