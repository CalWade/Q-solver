package native

import (
	"Q-Solver/internal/input/application"
	"Q-Solver/internal/input/domain"
	"fmt"
)

func validatePlan(plan application.NativePlan, supported func(domain.Binding) bool) error {
	policy := domain.Policy{Modifier: domain.Control, Bindings: plan.Bindings}
	if err := policy.Validate(); err != nil {
		return err
	}
	for _, binding := range plan.Bindings {
		if !supported(binding) {
			return fmt.Errorf("无效的系统快捷键 %s", binding.Action)
		}
	}
	return nil
}
