package probe

type HostObservation struct {
	Stage              string `json:"stage"`
	Panel              bool   `json:"panel"`
	NonActivating      bool   `json:"nonActivating"`
	HasEditingGate     bool   `json:"hasEditingGate"`
	CanBecomeKey       bool   `json:"canBecomeKey"`
	Key                bool   `json:"key"`
	ForegroundIsApp    bool   `json:"foregroundIsApp"`
	ForegroundRestored bool   `json:"foregroundRestored"`
}
type HostReport struct {
	System        string            `json:"system"`
	Host          string            `json:"host"`
	PageReady     bool              `json:"pageReady"`
	TextRoundTrip bool              `json:"textRoundTrip"`
	Observations  []HostObservation `json:"observations"`
	Checks        []Check           `json:"checks"`
	Errors        []string          `json:"errors,omitempty"`
	Limitations   []string          `json:"limitations"`
}

func (r *HostReport) Evaluate() bool {
	r.Checks = []Check{{"real Wails page and bridge ready", r.PageReady}, {"explicit editor DOM focus and text round trip", r.TextRoundTrip}}
	seen := map[string]bool{}
	for _, o := range r.Observations {
		valid := o.Panel && o.NonActivating && o.HasEditingGate && !seen[o.Stage]
		switch o.Stage {
		case "startup-passive", "returned-passive":
			valid = valid && !o.CanBecomeKey && !o.Key && o.ForegroundRestored
		case "explicit-editor":
			valid = valid && o.CanBecomeKey && o.Key && o.ForegroundIsApp
		default:
			valid = false
		}
		r.Checks = append(r.Checks, Check{o.Stage, valid})
		seen[o.Stage] = true
	}
	r.Checks = append(r.Checks, Check{"all host transitions observed", seen["startup-passive"] && seen["explicit-editor"] && seen["returned-passive"] && len(seen) == 3})
	r.Limitations = []string{"运行固定补丁的真实 Wails 2 宿主与生产窗口管理器；没有调用模型或真实平台账号。", "文本由隔离页面程序写入，用于验证 WebView 焦点及桥接；不是物理按键或中文输入法验收。", "没有捕获屏幕或会议远端画面，也未验证外部浏览器的监测事件。"}
	passed := len(r.Errors) == 0
	for _, c := range r.Checks {
		passed = passed && c.Passed
	}
	return passed
}
