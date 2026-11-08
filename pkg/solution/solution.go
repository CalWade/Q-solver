// Package solution adapts the request workflow to a model provider. It holds no
// mutable conversation, UI, or broadcasting state.
package solution

import (
	"Q-Solver/internal/solve/domain"
	"Q-Solver/pkg/config"
	prompts "Q-Solver/pkg/domain"
	"Q-Solver/pkg/llm"
	"context"
	"strings"
)

type StreamProvider interface {
	GenerateContentStream(context.Context, []llm.Message, llm.StreamCallback) (llm.Message, error)
}
type ProviderFactory func(domain.Settings) StreamProvider
type Generator struct{ factory ProviderFactory }

func NewGenerator(factory ProviderFactory) *Generator { return &Generator{factory: factory} }
func NewOpenAIGenerator() *Generator {
	return NewGenerator(func(s domain.Settings) StreamProvider {
		return llm.NewOpenAIAdapter(&config.Config{APIKey: s.APIKey, BaseURL: s.BaseURL, Model: s.Model})
	})
}

func (g *Generator) Generate(ctx context.Context, request domain.Request, emit func(domain.Chunk)) (domain.Result, error) {
	if err := ctx.Err(); err != nil {
		return domain.Result{}, err
	}
	if err := request.Settings.Validate(); err != nil {
		return domain.Result{}, err
	}
	response, err := g.factory(request.Settings).GenerateContentStream(ctx, messages(request), func(chunk llm.StreamChunk) {
		if emit != nil && ctx.Err() == nil && (chunk.Type == llm.ChunkThinking || chunk.Type == llm.ChunkContent) {
			emit(domain.Chunk{Thinking: chunk.Type == llm.ChunkThinking, Text: chunk.Content})
		}
	})
	if ctx.Err() != nil {
		return domain.Result{}, ctx.Err()
	}
	return domain.Result{Content: response.Content, Thinking: response.Thinking}, err
}

func messages(request domain.Request) []llm.Message {
	var system strings.Builder
	system.WriteString(prompts.GetSystemBehaviorPrompt())
	system.WriteString("\n\n" + prompts.GetHumanPacingPrompt())
	if prompt := prompts.GetPrompt(request.Settings.DomainID); prompt != "" {
		system.WriteString("\n\n<ScenePrompt>\n" + prompt + "\n</ScenePrompt>")
	}
	if resume := request.Settings.ResumeContent; resume != "" {
		system.WriteString("\n\n<CandidateProfile>\n<ResumeSummary>\n" + resume + "\n</ResumeSummary>\n</CandidateProfile>")
	}
	result := []llm.Message{llm.NewSystemMessage(system.String())}
	if request.Settings.KeepContext {
		for _, turn := range request.History {
			result = append(result, imageMessage(turn.Images), llm.NewAssistantMessage(turn.Answer))
		}
	}
	return append(result, imageMessage(request.Images))
}
func imageMessage(images []domain.Image) llm.Message {
	parts := make([]llm.ContentPart, 0, len(images))
	for _, img := range images {
		parts = append(parts, llm.ImagePart(img.DataURL))
	}
	return llm.NewMultiPartMessage(llm.RoleUser, parts)
}
