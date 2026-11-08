package domain

import (
	capture "Q-Solver/internal/capture/domain"
	"errors"
	"slices"
)

const MaxScreenshots = 3
const MaxContextRounds = 10

var ErrBusy = errors.New("正在处理中，请先停止当前请求")
var ErrScreenshotLimit = errors.New("最多截图 3 张图片，请先发送或删除")
var ErrAPIKey = errors.New("请先配置 API Key")
var ErrModel = errors.New("请先选择模型")

type Settings struct {
	APIKey                                  string `json:"-"`
	BaseURL, Model, DomainID, ResumeContent string
	KeepContext, InterruptThinking          bool
	Capture                                 capture.Options
}

func (s Settings) Validate() error {
	if s.APIKey == "" {
		return ErrAPIKey
	}
	if s.Model == "" {
		return ErrModel
	}
	return s.Capture.Validate()
}

type Image struct {
	ID      int64  `json:"id"`
	DataURL string `json:"dataURL"`
}
type Turn struct {
	Images []Image
	Answer string
}
type Request struct {
	ID       int64
	Settings Settings
	Images   []Image
	History  []Turn
}

func (r Request) Clone() Request {
	r.Images = slices.Clone(r.Images)
	r.History = CloneHistory(r.History)
	return r
}
func CloneHistory(history []Turn) []Turn {
	copy := slices.Clone(history)
	for i := range copy {
		copy[i].Images = slices.Clone(copy[i].Images)
	}
	return copy
}
func AppendHistory(history []Turn, turn Turn, keep bool) []Turn {
	if !keep {
		history = nil
	}
	history = append(CloneHistory(history), Turn{Images: slices.Clone(turn.Images), Answer: turn.Answer})
	if len(history) > MaxContextRounds {
		history = history[len(history)-MaxContextRounds:]
	}
	return history
}

type Chunk struct {
	Thinking bool
	Text     string
}
type Result struct{ Content, Thinking string }
type Event struct {
	SessionID   string  `json:"sessionId"`
	Kind        string  `json:"kind"`
	RequestID   int64   `json:"requestId"`
	Sequence    uint64  `json:"sequence"`
	Images      []Image `json:"images,omitempty"`
	Text        string  `json:"text,omitempty"`
	Thinking    string  `json:"thinking,omitempty"`
	Code        string  `json:"code,omitempty"`
	KeepContext bool    `json:"keepContext"`
}
type Status struct {
	SessionID     string  `json:"sessionId"`
	RequestID     int64   `json:"requestId"`
	Sequence      uint64  `json:"sequence"`
	Phase         string  `json:"phase"`
	Images        []Image `json:"images"`
	RequestImages []Image `json:"requestImages"`
	Content       string  `json:"content"`
	Thinking      string  `json:"thinking"`
	Error         string  `json:"error"`
	KeepContext   bool    `json:"keepContext"`
}

func Terminal(phase string) bool {
	return phase == "completed" || phase == "failed" || phase == "cancelled"
}
