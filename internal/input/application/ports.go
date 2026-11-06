package application

import "Q-Solver/internal/input/domain"

type Capabilities struct {
	Filtering bool   `json:"filtering"`
	Backend   string `json:"backend"`
	Reason    string `json:"reason,omitempty"`
}

type Callbacks struct {
	Event    func(domain.Event) domain.Decision
	Shortcut func(revision uint64, action string)
	Seed     func([]domain.Key)
	Failed   func(error)
}

type NativePlan struct {
	Revision uint64
	Bindings []domain.Binding
}

// Driver is owned by the application layer. OS adapters implement it and never
// call screenshot, settings, network or Wails APIs from input callbacks.
type Driver interface {
	Start(Callbacks) (Capabilities, error)
	CanRegister(domain.Binding) bool
	Apply(NativePlan) error
	Stop() error
}

type Commands interface{ Execute(action string) }
type Observer interface{ InputStatusChanged(Status) }

type Status struct {
	State        string            `json:"state"`
	Protection   domain.Protection `json:"protection"`
	Capabilities Capabilities      `json:"capabilities"`
	Revision     uint64            `json:"revision"`
	Error        string            `json:"error,omitempty"`
}
