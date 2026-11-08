package app

import (
	capture "Q-Solver/internal/capture/domain"
	"Q-Solver/internal/solve/domain"
	"Q-Solver/pkg/screen"
	"context"
	"errors"
)

type workflowCapture struct{ screen *screen.Service }

func (c workflowCapture) Capture(ctx context.Context, options capture.Options) (string, error) {
	result, err := c.screen.CapturePreviewContext(ctx, options)
	return result.Base64, err
}

type workflowObserver struct{ app *App }

func (o workflowObserver) Publish(event domain.Event) {
	o.app.EmitEvent("workflow-event", event)
	o.app.broadcaster.Publish(event)
}

func (a *App) workflowError(err error) {
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	if errors.Is(err, domain.ErrAPIKey) {
		a.EmitEvent("require-api-key")
		return
	}
	a.EmitEvent("toast", err.Error())
}
func (a *App) TriggerScreenshot() {
	if a.workflow != nil {
		a.workflowError(a.workflow.Capture())
	}
}
func (a *App) TriggerSend() {
	if a.workflow != nil {
		id, err := a.workflow.Send()
		if id == 0 {
			a.workflowError(err)
		}
	}
}
func (a *App) TriggerSolve() { a.TriggerSend() }
func (a *App) RemoveScreenshot(index int) {
	if a.workflow != nil {
		a.workflow.Remove(index)
	}
}
func (a *App) RemoveLastScreenshot() {
	if a.workflow != nil {
		a.workflow.RemoveLast()
	}
}
func (a *App) ClearScreenshots() {
	if a.workflow != nil {
		a.workflow.ClearScreenshots()
	}
}
func (a *App) TriggerDeleteScreenshot()    { a.RemoveLastScreenshot() }
func (a *App) CancelRequest(id int64) bool { return a.workflow != nil && a.workflow.Cancel(id) }
func (a *App) CancelRunningTask() bool     { return a.workflow != nil && a.workflow.CancelCurrent() }

// Distinct transport names prevent Wails from collapsing different bounded
// contexts' domain.Status types into one generated TypeScript class.
type WorkflowStatus domain.Status

func (a *App) GetWorkflowStatus() WorkflowStatus {
	if a.workflow == nil {
		return WorkflowStatus{Phase: "idle"}
	}
	return WorkflowStatus(a.workflow.Status())
}
func (a *App) IsInterruptThinkingEnabled() bool { return a.configManager.Get().InterruptThinking }
