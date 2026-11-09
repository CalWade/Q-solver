package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestStageGateRequiresTheObservedStage(t *testing.T) {
	var output bytes.Buffer
	g := NewStageGate(io.NopCloser(strings.NewReader("startup-passive\nexplicit-editor\n")), &output)
	defer g.Close()
	for _, stage := range []string{"startup-passive", "explicit-editor"} {
		if err := g.Wait(context.Background(), stage); err != nil {
			t.Fatal(err)
		}
		var message map[string]string
		if err := json.NewDecoder(&output).Decode(&message); err != nil || message["stage"] != stage || message["type"] != "native-stage" {
			t.Fatal(message, err)
		}
	}
}

func TestStageGateRejectsWrongOrMissingReceivers(t *testing.T) {
	for _, input := range []string{"", "wrong-stage\n"} {
		g := NewStageGate(io.NopCloser(strings.NewReader(input)), io.Discard)
		if err := g.Wait(context.Background(), "startup-passive"); err == nil {
			t.Fatal("unobserved stage accepted")
		}
		g.Close()
	}
}

func TestStageGateDeadlineAndCloseReleaseTheReader(t *testing.T) {
	r, w := io.Pipe()
	defer w.Close()
	g := NewStageGate(r, io.Discard)
	defer g.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if err := g.Wait(ctx, "startup-passive"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	g.Close()
	g.Close()
	if err := g.Wait(context.Background(), "explicit-editor"); err == nil {
		t.Fatal("closed receiver accepted")
	}
}
