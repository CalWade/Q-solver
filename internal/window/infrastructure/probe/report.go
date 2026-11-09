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
	System       string         `json:"system"`
	WebViewReady bool           `json:"webViewReady"`
	Observations []Observation  `json:"observations"`
	Checks       []Check        `json:"checks"`
	Errors       []string       `json:"errors,omitempty"`
	Limitations  []string       `json:"limitations"`
	Capture      *CaptureReport `json:"capture,omitempty"`
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
		"该报告来自独立 AppKit NSPanel + WKWebView；生产 Wails 宿主须单独运行 host-probe。",
		"验证系统属性、禁止成为 key window、显示/关闭/重建及截图租约；未模拟物理点击或输入法编辑。",
		"未采集任何屏幕或会议画面；sharingType 属性成功不等于 ScreenCaptureKit 或会议远端不可见。",
		"没有使用真实会议、公司群、笔试平台、模型凭据或全局键盘钩子。",
	}
	passed := len(r.Errors) == 0
	if r.Capture != nil {
		passed = r.Capture.Evaluate() && passed
		r.Limitations = []string{
			"仅通过 ScreenCaptureKit 单窗口接口捕获自建合成面板，不采集其他窗口或显示器画面。",
			"completed 表示对照测试完成；protectedWindowOutcome=visible 表示 sharingType none 未隐藏测试标记。",
			"透明/留空/标记缺失不等同于窗口从共享源列表消失；成功抓图前仍按 windowID 找到了该窗口。不外推为第三方会议或监考平台结果。",
		}
	}
	for _, check := range r.Checks {
		passed = passed && check.Passed
	}
	return passed
}
