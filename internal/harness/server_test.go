package harness

import (
	"Q-Solver/internal/input/domain"
	"Q-Solver/pkg/config"
	"Q-Solver/pkg/shortcut"
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"
)

func request(t *testing.T, s *Server, method string, args ...any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	if args == nil {
		raw = []byte("[]")
	}
	r := httptest.NewRequest("POST", "http://127.0.0.1/rpc/"+method, bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func rpc[T any](t *testing.T, s *Server, method string, args ...any) T {
	t.Helper()
	w := request(t, s, method, args...)
	if w.Code != 200 {
		t.Fatalf("%s: %d %s", method, w.Code, w.Body.String())
	}
	var value T
	if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func TestSettingsInputAndReloadIntegration(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cfg := rpc[config.Config](t, s, "GetSettings")
	modifier := domain.Control
	if runtime.GOOS == "darwin" {
		modifier = domain.Meta
	}
	cfg.Shortcuts["screenshot"] = shortcut.KeyBinding{Keys: []domain.Key{modifier, "KeyQ"}}
	cfg.KeyboardProtection = domain.Protection{Enabled: true, BlockLeft: true}
	if result := rpc[string](t, s, "UpdateSettings", cfg.ToJSON()); result != "" {
		t.Fatal(result)
	}
	rpc[string](t, s, "HarnessReload")
	saved := rpc[config.Config](t, s, "GetSettings")
	if !saved.KeyboardProtection.Enabled || saved.Shortcuts["screenshot"].ComboID == "" {
		t.Fatal(saved.Shortcuts)
	}
	sequence := []domain.Event{{Key: domain.Key(string(modifier) + "Left"), Down: true}, {Key: "KeyQ", Down: true}, {Key: domain.Key(string(modifier) + "Left")}, {Key: "KeyQ"}}
	decisions := rpc[[]domain.Decision](t, s, "HarnessInput", sequence)
	for _, d := range decisions {
		if !d.Suppress {
			t.Fatal(decisions)
		}
	}
	deadline := time.Now().Add(time.Second)
	for {
		s.mu.Lock()
		count := len(s.commands)
		s.mu.Unlock()
		if count == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no action dispatched")
		}
		time.Sleep(time.Millisecond)
	}
	cfg.Shortcuts["send"] = cfg.Shortcuts["screenshot"]
	if result := rpc[string](t, s, "UpdateSettings", cfg.ToJSON()); result == "" {
		t.Fatal("conflict saved")
	}
	if got := rpc[config.Config](t, s, "GetSettings"); got.Shortcuts["send"].ComboID == got.Shortcuts["screenshot"].ComboID {
		t.Fatal("invalid update changed memory")
	}
}
func TestHarnessRejectsExternalAndMalformedCalls(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, tc := range []struct {
		host, origin, content, body, method string
		status                              int
	}{
		{"attacker.example", "", "application/json", "[]", "POST", 403},
		{"127.0.0.1", "https://attacker.example", "application/json", "[]", "POST", 403},
		{"127.0.0.1", "", "text/plain", "[]", "POST", 405},
		{"127.0.0.1", "", "application/json", "bad", "POST", 400},
		{"127.0.0.1", "", "application/json", "[]", "GET", 405},
	} {
		r := httptest.NewRequest(tc.method, "http://"+tc.host+"/rpc/GetSettings", bytes.NewBufferString(tc.body))
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Content-Type", tc.content)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%+v got %d", tc, w.Code)
		}
	}
	if w := request(t, s, "SetInputEditing"); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w := request(t, s, "Unknown"); w.Code != 400 {
		t.Fatal(w.Code)
	}
}
