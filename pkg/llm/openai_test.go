package llm

import (
	"Q-Solver/pkg/config"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestModelSnapshotUsedByActualStreamingRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Model != "first-model" || r.Header.Get("Authorization") != "Bearer fixture-only" {
			t.Error("request settings changed", body.Model)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"fixture\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	cfg := config.Config{APIKey: "fixture-only", BaseURL: server.URL, Model: "first-model"}
	adapter := NewOpenAIAdapter(&cfg)
	cfg.Model = "second-model"
	cfg.APIKey = "changed"
	var text string
	result, err := adapter.GenerateContentStream(context.Background(), []Message{NewUserMessage("fixture")}, func(c StreamChunk) { text += c.Content })
	if err != nil || result.Content != "hello" || text != "hello" {
		t.Fatal(result, err, text)
	}
}
func TestDefaultModelIsStoredInSnapshot(t *testing.T) {
	adapter := NewOpenAIAdapter(&config.Config{})
	if adapter.config.Model == "" {
		t.Fatal("empty default model")
	}
}
