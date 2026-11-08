// Package probe evaluates observations from an isolated native host. It does
// not infer browser focus events or third-party capture behaviour from flags.
package probe

type Observation struct {
	Stage               string  `json:"stage"`
	Panel               bool    `json:"panel"`
	Visible             bool    `json:"visible"`
	NonActivating       bool    `json:"nonActivating"`
	CanBecomeKey        bool    `json:"canBecomeKey"`
	Key                 bool    `json:"key"`
	Main                bool    `json:"main"`
	CaptureProperty     bool    `json:"captureProperty"`
	Alpha               float64 `json:"alpha"`
	ForegroundUnchanged bool    `json:"foregroundUnchanged"`
}
type Check struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
}
type Report struct {
	System       string        `json:"system"`
	WebViewReady bool          `json:"webViewReady"`
	Observations []Observation `json:"observations"`
	Checks       []Check       `json:"checks"`
	Errors       []string      `json:"errors,omitempty"`
	Limitations  []string      `json:"limitations"`
}

func (r *Report) Evaluate() bool {
	r.Checks = []Check{{"WKWebView loaded isolated local fixture", r.WebViewReady}}
	expected := map[string]float64{"shown": 1, "capture-first": 0, "capture-nested": 0, "secondary-during-capture": 0, "first-release": 0, "last-release": 1, "secondary-restored": 1, "recreated": 1}
	seen := map[string]bool{}
	for _, o := range r.Observations {
		alpha, exists := expected[o.Stage]
		passed := exists && !seen[o.Stage] && o.Panel && o.Visible && o.NonActivating && !o.CanBecomeKey && !o.Key && !o.Main && o.CaptureProperty && o.Alpha == alpha && o.ForegroundUnchanged
		r.Checks = append(r.Checks, Check{o.Stage, passed})
		seen[o.Stage] = true
	}
	r.Checks = append(r.Checks, Check{"all lifecycle stages observed", len(seen) == len(expected)})
	r.Limitations = []string{
		"原型宿主是独立 AppKit NSPanel + WKWebView，尚未替换生产 Wails 2 宿主。",
		"验证系统属性、禁止成为 key window、显示/关闭/重建及截图租约；未模拟物理点击或输入法编辑。",
		"未采集任何屏幕或会议画面；sharingType 属性成功不等于 ScreenCaptureKit 或会议远端不可见。",
		"没有使用真实会议、公司群、笔试平台、模型凭据或全局键盘钩子。",
	}
	passed := len(r.Errors) == 0
	for _, check := range r.Checks {
		passed = passed && check.Passed
	}
	return passed
}
