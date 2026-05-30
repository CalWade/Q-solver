package solution

import (
	"Q-Solver/pkg/broadcast"
	"Q-Solver/pkg/config"
	"Q-Solver/pkg/domain"
	"Q-Solver/pkg/llm"
	"Q-Solver/pkg/logger"
	"bytes"
	"context"
	"errors"
)

// MaxConversationRounds is the maximum number of conversation rounds to keep.
const MaxConversationRounds = 10

type Callbacks struct {
	EmitEvent func(event string, data ...interface{})
	// Broadcaster 可选：若非 nil，会把流式答案同步推送到局域网 HTTP server（用于 iPad 查看）。
	// 不影响现有 EmitEvent 行为。
	Broadcaster *broadcast.Server
}

type Request struct {
	Config      config.Config
	Screenshots []string
}

type Solver struct {
	llmProvider llm.Provider
	chatHistory []llm.Message
}

func NewSolver(provider llm.Provider) *Solver {
	return &Solver{
		llmProvider: provider,
		chatHistory: make([]llm.Message, 0),
	}
}

func (s *Solver) SetProvider(provider llm.Provider) {
	s.llmProvider = provider
}

func (s *Solver) ClearHistory() {
	s.chatHistory = make([]llm.Message, 0)
}

func (s *Solver) Solve(ctx context.Context, req Request, cb Callbacks) bool {
	if req.Config.APIKey == "" {
		if cb.EmitEvent != nil {
			cb.EmitEvent("require-api-key")
		}
		return false
	}

	logger.Println("开始解题流程")

	var systemPrompt bytes.Buffer
	systemPrompt.WriteString(domain.GetSystemBehaviorPrompt())

	// 追加反 AI 节奏提示词（模仿真人边想边写的语气/代码风格）
	// 与主行为策略解耦，方便以后按配置独立开关
	systemPrompt.WriteString("\n\n")
	systemPrompt.WriteString(domain.GetHumanPacingPrompt())

	if req.Config.DomainId != "" {
		if prompt := domain.GetPrompt(req.Config.DomainId); prompt != "" {
			systemPrompt.WriteString("\n\n<ScenePrompt>\n")
			systemPrompt.WriteString(prompt)
			systemPrompt.WriteString("\n</ScenePrompt>")
			logger.Printf("已注入场景提示词 (DomainID: %s)", req.Config.DomainId)
		}
	}

	if req.Config.ResumeContent != "" {
		logger.Println("使用 Markdown 简历内容")
		systemPrompt.WriteString("\n\n<CandidateProfile>\n")
		systemPrompt.WriteString("  <ResumeSummary>\n")
		systemPrompt.WriteString(req.Config.ResumeContent)
		systemPrompt.WriteString("\n  </ResumeSummary>\n")
		systemPrompt.WriteString("</CandidateProfile>\n")
	}

	logger.Println("system 提示词:", systemPrompt.String())

	userParts := make([]llm.ContentPart, 0, len(req.Screenshots))
	for _, screenshot := range req.Screenshots {
		userParts = append(userParts, llm.ImagePart(screenshot))
	}
	currentUserMsg := llm.NewMultiPartMessage(llm.RoleUser, userParts)

	var messagesToSend []llm.Message
	messagesToSend = append(messagesToSend, llm.NewSystemMessage(systemPrompt.String()))
	messagesToSend = append(messagesToSend, currentUserMsg)

	if cb.EmitEvent != nil {
		cb.EmitEvent("solution-stream-start")
	}
	// 同步通知 broadcaster 开始新答案（iPad 端 SSE 会收到 start 并清空）
	cb.Broadcaster.StartNewAnswer()

	response, err := s.llmProvider.GenerateContentStream(ctx, messagesToSend, func(chunk llm.StreamChunk) {
		// chunk 分发：优先保证原有 EmitEvent 行为不变；broadcaster 是附加副作用
		if chunk.Type == llm.ChunkContent {
			cb.Broadcaster.AppendChunk(chunk.Content)
		}

		if cb.EmitEvent == nil {
			return
		}

		switch chunk.Type {
		case llm.ChunkThinking:
			cb.EmitEvent("solution-stream-thinking", chunk.Content)
		case llm.ChunkContent:
			cb.EmitEvent("solution-stream-chunk", chunk.Content)
		}
	})

	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			logger.Println("当前任务已中断（用户产生新输入）")
			if cb.EmitEvent != nil {
				cb.EmitEvent("solution-error", "context canceled")
			}
			return false
		}

		logger.Printf("LLM 请求失败: %v\n", err)
		if cb.EmitEvent != nil {
			cb.EmitEvent("solution-error", err.Error())
		}
		return false
	}

	logger.Printf("[解题] 模型返回内容长度: %d", len(response.Content))
	logger.Printf("[解题] 模型返回内容: %s", response.Content)
	logger.Printf("[解题] 模型返回思考链长度: %d", len(response.Thinking))

	if response.Content == "" && response.Thinking == "" {
		logger.Println("[解题] 警告: 模型返回内容为空")
		if cb.EmitEvent != nil {
			cb.EmitEvent("solution-error", "模型返回内容为空，请检查模型配置或稍后重试")
		}
		return false
	}

	if cb.EmitEvent != nil {
		cb.EmitEvent("solution", response.Content)
	}
	// 用最终整段覆盖，保证和 LLM 真实返回一致（某些 provider 流式和整体不完全相等）
	cb.Broadcaster.FinalizeAnswer(response.Content)

	s.chatHistory = []llm.Message{}
	return true
}

// ensureSystemPrompt keeps the first history message aligned with the active system prompt.
func (s *Solver) ensureSystemPrompt(prompt string) {
	if len(s.chatHistory) == 0 {
		s.chatHistory = append(s.chatHistory, llm.NewSystemMessage(prompt))
		logger.Println("插入 SystemPrompt")
		return
	}

	if s.chatHistory[0].Role == llm.RoleSystem {
		if s.chatHistory[0].Content != prompt {
			s.chatHistory[0] = llm.NewSystemMessage(prompt)
			logger.Println("替换 SystemPrompt")
		}
		return
	}

	s.chatHistory = append([]llm.Message{llm.NewSystemMessage(prompt)}, s.chatHistory...)
	logger.Println("插入 SystemPrompt 到消息历史头部")
}

// trimChatHistory keeps only the most recent conversation rounds.
func (s *Solver) trimChatHistory() {
	if len(s.chatHistory) <= 1 {
		return
	}

	nonSystemMsgs := len(s.chatHistory) - 1
	maxNonSystemMsgs := MaxConversationRounds * 2
	if nonSystemMsgs <= maxNonSystemMsgs {
		return
	}

	startIndex := len(s.chatHistory) - maxNonSystemMsgs
	newHistory := make([]llm.Message, 0, maxNonSystemMsgs+1)
	newHistory = append(newHistory, s.chatHistory[0])
	newHistory = append(newHistory, s.chatHistory[startIndex:]...)

	oldLen := len(s.chatHistory)
	s.chatHistory = newHistory
	logger.Printf("裁剪对话历史: %d -> %d 条消息 (保留最近 %d 轮对话)", oldLen, len(s.chatHistory), MaxConversationRounds)
}
