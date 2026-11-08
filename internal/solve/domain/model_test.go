package domain

import (
	capture "Q-Solver/internal/capture/domain"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSnapshotsDoNotShareImagesOrCredentials(t *testing.T) {
	r := Request{Settings: Settings{APIKey: "private-key"}, Images: []Image{{ID: 1}}, History: []Turn{{Images: []Image{{ID: 2}}, Answer: "old"}}}
	copy := r.Clone()
	copy.Images[0].ID = 8
	copy.History[0].Images[0].ID = 9
	if r.Images[0].ID != 1 || r.History[0].Images[0].ID != 2 {
		t.Fatal("snapshot aliases input")
	}
	encoded, _ := json.Marshal(r)
	if strings.Contains(string(encoded), "private-key") {
		t.Fatal("serialized credentials")
	}
}

func TestHistoryKeepsCompleteBoundedRounds(t *testing.T) {
	var history []Turn
	for i := 0; i < 15; i++ {
		history = AppendHistory(history, Turn{Images: []Image{{ID: int64(i)}}}, true)
	}
	if len(history) != MaxContextRounds || history[0].Images[0].ID != 5 {
		t.Fatal(history)
	}
	reset := AppendHistory(history, Turn{Answer: "new"}, false)
	if len(reset) != 1 || reset[0].Answer != "new" {
		t.Fatal(reset)
	}
}

func TestValidateAndTerminal(t *testing.T) {
	s := Settings{Capture: capture.Options{Quality: 80, Mode: "window"}}
	if !errors.Is(s.Validate(), ErrAPIKey) {
		t.Fatal("missing key accepted")
	}
	s.APIKey = "test"
	if !errors.Is(s.Validate(), ErrModel) {
		t.Fatal("missing model accepted")
	}
	s.Model = "test"
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"completed", "failed", "cancelled"} {
		if !Terminal(phase) {
			t.Fatal(phase)
		}
	}
	if Terminal("generating") || Terminal("idle") {
		t.Fatal("active phase is terminal")
	}
}
