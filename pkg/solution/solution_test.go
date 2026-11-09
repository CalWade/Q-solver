package solution

import (
	capture "Q-Solver/internal/capture/domain"
	"Q-Solver/internal/solve/domain"
	"Q-Solver/pkg/llm"
	"context"
	"errors"
	"strings"
	"testing"
)

type providerFunc func(context.Context, []llm.Message, llm.StreamCallback) (llm.Message, error)

func (f providerFunc) GenerateContentStream(ctx context.Context, m []llm.Message, c llm.StreamCallback) (llm.Message, error) {
	return f(ctx, m, c)
}
func validRequest() domain.Request {
	return domain.Request{Settings: domain.Settings{APIKey: "test", Model: "model-a", KeepContext: true, ResumeContent: "synthetic profile", Capture: capture.Options{Quality: 80, Mode: "window"}}, Images: []domain.Image{{DataURL: "current"}}, History: []domain.Turn{{Images: []domain.Image{{DataURL: "previous"}}, Answer: "previous answer"}}}
}

func TestProviderReceivesSnapshotAndCompleteConversation(t *testing.T) {
	request := validRequest()
	g := NewGenerator(func(s domain.Settings) StreamProvider {
		if s.Model != "model-a" {
			t.Fatal(s.Model)
		}
		return providerFunc(func(_ context.Context, m []llm.Message, emit llm.StreamCallback) (llm.Message, error) {
			if len(m) != 4 || m[0].Role != llm.RoleSystem || !strings.Contains(m[0].Content, "synthetic profile") || m[1].Parts[0].Base64 != "previous" || m[2].Content != "previous answer" || m[3].Parts[0].Base64 != "current" {
				t.Fatal(m)
			}
			emit(llm.StreamChunk{Type: llm.ChunkThinking, Content: "reason"})
			emit(llm.StreamChunk{Type: llm.ChunkContent, Content: "draft"})
			return llm.Message{Content: "final", Thinking: "reason"}, nil
		})
	})
	var chunks []domain.Chunk
	result, err := g.Generate(context.Background(), request, func(c domain.Chunk) { chunks = append(chunks, c) })
	if err != nil || result.Content != "final" || len(chunks) != 2 || !chunks[0].Thinking || chunks[1].Thinking {
		t.Fatal(result, err, chunks)
	}
	request.Settings.KeepContext = false
	if len(messages(request)) != 2 {
		t.Fatal("disabled context leaked history")
	}
}

func TestCancellationAndProviderFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	g := NewGenerator(func(domain.Settings) StreamProvider {
		return providerFunc(func(_ context.Context, _ []llm.Message, emit llm.StreamCallback) (llm.Message, error) {
			cancel()
			emit(llm.StreamChunk{Type: llm.ChunkContent, Content: "late"})
			return llm.Message{}, errors.New("provider error")
		})
	})
	_, err := g.Generate(ctx, validRequest(), func(domain.Chunk) { t.Error("late callback") })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	g = NewGenerator(func(domain.Settings) StreamProvider {
		return providerFunc(func(context.Context, []llm.Message, llm.StreamCallback) (llm.Message, error) {
			return llm.Message{}, errors.New("fixture failure")
		})
	})
	_, err = g.Generate(context.Background(), validRequest(), nil)
	if err == nil || err.Error() != "fixture failure" {
		t.Fatal(err)
	}
}
