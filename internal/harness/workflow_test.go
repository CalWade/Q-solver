package harness

import (
	"Q-Solver/internal/solve/domain"
	"Q-Solver/pkg/config"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func waitWorkflow(t *testing.T, s *Server, condition func(domain.Status) bool) domain.Status {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		state := rpc[domain.Status](t, s, "GetWorkflowStatus")
		if condition(state) {
			return state
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("workflow did not reach expected state")
	return domain.Status{}
}
func TestRPCWorkflowUsesCaptureAndPublishesConsistentResult(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cfg := rpc[config.Config](t, s, "GetSettings")
	cfg.APIKey = "fixture-only"
	cfg.Model = "fixture-a"
	cfg.KeepContext = true
	if result := rpc[string](t, s, "UpdateSettings", cfg.ToJSON()); result != "" {
		t.Fatal(result)
	}
	rpc[any](t, s, "TriggerScreenshot")
	queued := rpc[domain.Status](t, s, "GetWorkflowStatus")
	if len(queued.Images) != 1 || !strings.HasPrefix(queued.Images[0].DataURL, "data:image/") {
		t.Fatal("missing processed fixture screenshot")
	}
	rpc[any](t, s, "TriggerSend")
	state := waitWorkflow(t, s, func(state domain.Status) bool { _, err := s.model.get(state.RequestID); return err == nil })
	if len(state.Images) != 0 || state.RequestImages[0].ID != queued.Images[0].ID {
		t.Fatal("queue/request identity mismatch")
	}
	if s.windows.Status().Captures != 0 || s.windows.Status().Windows[0].CaptureSuppressed {
		t.Fatal("capture lease leaked")
	}
	cfg.Model = "fixture-b"
	rpc[string](t, s, "UpdateSettings", cfg.ToJSON())
	call, _ := s.model.get(state.RequestID)
	if call.request.Settings.Model != "fixture-a" {
		t.Fatal("settings changed in flight")
	}
	rpc[any](t, s, "HarnessModelChunk", state.RequestID, "draft")
	rpc[any](t, s, "HarnessModelFinish", state.RequestID, "authoritative final")
	state = waitWorkflow(t, s, func(state domain.Status) bool { return state.Phase == "completed" })
	r := httptest.NewRequest("GET", "http://127.0.0.1/broadcast/current", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	var remote struct {
		Content   string
		RequestID int64 `json:"requestId"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &remote); err != nil {
		t.Fatal(err)
	}
	if remote.Content != state.Content || remote.RequestID != state.RequestID {
		t.Fatal(remote, state)
	}
	rpc[any](t, s, "TriggerSend")
	next := waitWorkflow(t, s, func(next domain.Status) bool {
		_, err := s.model.get(next.RequestID)
		return next.RequestID > state.RequestID && err == nil
	})
	call, _ = s.model.get(next.RequestID)
	if call.request.Settings.Model != "fixture-b" || len(call.request.History) != 1 {
		t.Fatal("new request lost snapshot/history")
	}
	if rpc[bool](t, s, "CancelRequest", state.RequestID) {
		t.Fatal("old stop request cancelled new task")
	}
	if !rpc[bool](t, s, "CancelRequest", next.RequestID) {
		t.Fatal("cancel failed")
	}
	rpc[any](t, s, "HarnessModelChunk", next.RequestID, "late")
	if rpc[domain.Status](t, s, "GetWorkflowStatus").Content != "" {
		t.Fatal("cancelled output delivered")
	}
}

func TestResetClosesOldWorkflowAndChangesSessionIdentity(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	before := rpc[domain.Status](t, s, "GetWorkflowStatus")
	rpc[string](t, s, "HarnessReset")
	after := rpc[domain.Status](t, s, "GetWorkflowStatus")
	if before.SessionID == "" || before.SessionID == after.SessionID || after.Phase != "idle" {
		t.Fatal(before, after)
	}
}
