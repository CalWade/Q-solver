//go:build !darwin && !windows

package native

import (
	"Q-Solver/internal/input/application"
	"Q-Solver/internal/input/domain"
	"fmt"
)

type unsupported struct{}

func New() application.Driver { return unsupported{} }

func RequestPermission() bool { return false }
func (unsupported) Start(application.Callbacks) (application.Capabilities, error) {
	return application.Capabilities{Backend: "unsupported", Reason: "仅支持 macOS 和 Windows"}, fmt.Errorf("仅支持 macOS 和 Windows")
}
func (unsupported) CanRegister(domain.Binding) bool    { return false }
func (unsupported) Apply(application.NativePlan) error { return fmt.Errorf("平台不支持") }
func (unsupported) Stop() error                        { return nil }
