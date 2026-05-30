package config

import (
	"Q-Solver/pkg/shortcut"
	"encoding/json"
	"runtime"
)

type Config struct {
	APIKey             string                         `json:"apiKey,omitempty"`
	BaseURL            string                         `json:"baseURL,omitempty"`
	Model              string                         `json:"model,omitempty"`
	Prompt             string                         `json:"prompt,omitempty"`
	DomainId           string                         `json:"domainId,omitempty"` // 行业/岗位选择ID
	Opacity            float64                        `json:"opacity,omitempty"`
	NoCompression      bool                           `json:"noCompression,omitempty"`
	CompressionQuality int                            `json:"compressionQuality,omitempty"`
	Sharpening         float64                        `json:"sharpening,omitempty"`
	Grayscale          bool                           `json:"grayscale,omitempty"`
	KeepContext        bool                           `json:"keepContext,omitempty"`
	InterruptThinking  bool                           `json:"interruptThinking,omitempty"`
	ScreenshotMode     string                         `json:"screenshotMode,omitempty"`
	ResumePath         string                         `json:"resumePath,omitempty"`
	ResumeContent      string                         `json:"resumeContent,omitempty"`
	Shortcuts          map[string]shortcut.KeyBinding `json:"shortcuts,omitempty"`

	// 辅助模型（用于总结对话生成问题导图）
	AssistantModel string `json:"assistantModel,omitempty"`

	// 窗口尺寸
	WindowWidth  int `json:"windowWidth,omitempty"`
	WindowHeight int `json:"windowHeight,omitempty"`

	// 主题（light / dark）
	Theme string `json:"theme,omitempty"`
}

const DefaultModel = ""

func NewDefaultConfig() Config {
	return Config{
		APIKey:             "",
		BaseURL:            "https://api.openai.com/v1",
		Model:              DefaultModel,
		ResumePath:         "",
		Prompt:             "",
		DomainId:           "general-assistant", // 默认选择通用的
		Opacity:            1.0,
		KeepContext:        false,
		InterruptThinking:  false,
		ScreenshotMode:     "fullscreen", // 默认全屏截图，确保捕获完整内容
		NoCompression:      false,        // 保持压缩以减小文件大小
		CompressionQuality: 92,           // 高质量压缩，确保 AI 清晰识别文字
		Sharpening:         0.3,          // 适度锐化，增强文字边缘清晰度
		Grayscale:          false,        // 保持彩色，某些场景颜色有意义
		ResumeContent:      "",

		Shortcuts: GetDefaultShortcuts(),

		// 辅助模型
		AssistantModel: "",

		// 窗口尺寸默认值
		WindowWidth:  0,
		WindowHeight: 0,

		// 主题默认值
		Theme: "light",
	}
}

// GetDefaultShortcuts 根据平台返回默认快捷键配置
//
// macOS 侧说明：实际快捷键由 shortcut_darwin.go 的 macDefaultShortcuts 硬编码决定，
// 这里的 map 仅用于前端设置页展示 KeyName。修改 macOS 真实快捷键必须同步改 shortcut_darwin.go。
//
// 设计原则：
//   - 主操作单手可按（Cmd/Alt + 数字 / Enter）
//   - 不冲突 IDE / 浏览器主流快捷键
//   - 所有 ComboID 升级时必须同步更新 manager.go 的 migrateShortcuts 老 ID 黑名单
func GetDefaultShortcuts() map[string]shortcut.KeyBinding {
	if runtime.GOOS == "darwin" {
		return map[string]shortcut.KeyBinding{
			"screenshot":   {ComboID: "Cmd+1", KeyName: "⌘1"},
			"send":         {ComboID: "Cmd+Return", KeyName: "⌘↩"},
			"toggle":       {ComboID: "Cmd+2", KeyName: "⌘2"},
			"clickthrough": {ComboID: "Cmd+3", KeyName: "⌘3"},
			"delete":       {ComboID: "Cmd+D", KeyName: "⌘D"},
			"move_up":      {ComboID: "Cmd+Option+Up", KeyName: "⌘⌥↑"},
			"move_down":    {ComboID: "Cmd+Option+Down", KeyName: "⌘⌥↓"},
			"move_left":    {ComboID: "Cmd+Option+Left", KeyName: "⌘⌥←"},
			"move_right":   {ComboID: "Cmd+Option+Right", KeyName: "⌘⌥→"},
			"scroll_up":    {ComboID: "Cmd+Option+Shift+Up", KeyName: "⌘⌥⇧↑"},
			"scroll_down":  {ComboID: "Cmd+Option+Shift+Down", KeyName: "⌘⌥⇧↓"},
			"panic":        {ComboID: "Cmd+Shift+Escape", KeyName: "⌘⇧⎋"},
		}
	}
	// Windows 默认快捷键
	// VK 码参考：Alt=0x12(18), '1'..'5'=0x31..0x35(49..53), Enter=0x0D(13),
	// 'D'=0x44(68), Esc=0x1B(27), Shift=0x10(16), Ctrl=0x11(17),
	// 方向键 ↑↓←→ = 0x26..0x28 (38, 40, 37, 39), 164=Right Alt
	// ComboID = VK 码升序 + 号连接（见 pkg/shortcut/shortcut.go GetComboID）
	return map[string]shortcut.KeyBinding{
		"screenshot":   {ComboID: "18+49", KeyName: "Alt+1"},
		"send":         {ComboID: "13+18", KeyName: "Alt+Enter"},
		"toggle":       {ComboID: "18+50", KeyName: "Alt+2"},
		"clickthrough": {ComboID: "18+51", KeyName: "Alt+3"},
		"delete":       {ComboID: "18+68", KeyName: "Alt+D"},
		"move_up":      {ComboID: "38+164", KeyName: "Alt+↑"},
		"move_down":    {ComboID: "40+164", KeyName: "Alt+↓"},
		"move_left":    {ComboID: "37+164", KeyName: "Alt+←"},
		"move_right":   {ComboID: "39+164", KeyName: "Alt+→"},
		"scroll_up":    {ComboID: "33+164", KeyName: "Alt+PgUp"},
		"scroll_down":  {ComboID: "34+164", KeyName: "Alt+PgDn"},
		"panic":        {ComboID: "16+17+18+27", KeyName: "Ctrl+Alt+Shift+Esc"},
	}
}

func (c *Config) ToJSON() string {
	data, _ := json.MarshalIndent(c, "", "  ")
	return string(data)
}

func (c *Config) Validate() error {
	if c.ScreenshotMode != "" && c.ScreenshotMode != "fullscreen" && c.ScreenshotMode != "window" {
		return &ValidationError{Field: "screenshotMode", Message: "截图模式必须是 'fullscreen' 或 'window'"}
	}
	if c.Opacity < 0 || c.Opacity > 1 {
		return &ValidationError{Field: "opacity", Message: "透明度必须在 0-1 之间"}
	}
	if c.CompressionQuality < 1 || c.CompressionQuality > 100 {
		return &ValidationError{Field: "compressionQuality", Message: "压缩质量必须在 1-100 之间"}
	}
	return nil
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
