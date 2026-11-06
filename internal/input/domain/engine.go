package domain

// Event describes input before it reaches the foreground application.
type Event struct {
	Key       Key  `json:"key"`
	Down      bool `json:"down"`
	Repeat    bool `json:"repeat"`
	Injected  bool `json:"injected"`
	OwnWindow bool `json:"ownWindow"`
}

type Decision struct {
	Suppress bool   `json:"suppress"`
	Action   string `json:"action,omitempty"`
	// VisibleModifiers describes modifiers whose down events were delivered.
	// The macOS adapter uses it when forwarding flagsChanged and key events.
	VisibleModifiers uint8 `json:"visibleModifiers"`
	VisibleSides     uint8 `json:"visibleSides"`
	RewriteModifiers bool  `json:"rewriteModifiers"`
}

const (
	VisibleControl uint8 = 1 << iota
	VisibleMeta
	VisibleAlt
	VisibleShift
)

type press struct{ suppressed bool }

// Engine is deliberately synchronous and has no locks or external effects.
// Its application service serializes calls. A press owns its release even when
// the modifier is released first or the desired policy changes halfway through.
type Engine struct {
	policy  Policy
	pending *Policy
	pressed map[Key]press
	fired   map[string]bool
}

func NewEngine(policy Policy) (*Engine, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &Engine{policy: policy.Clone(), pressed: map[Key]press{}, fired: map[string]bool{}}, nil
}

func (e *Engine) Configure(policy Policy) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	copy := policy.Clone()
	if len(e.pressed) > 0 {
		e.pending = &copy
	} else {
		e.policy = copy
		e.pending = nil
		e.fired = map[string]bool{}
	}
	return nil
}

func (e *Engine) Pending() bool  { return e.pending != nil }
func (e *Engine) HeldCount() int { return len(e.pressed) }

// Seed handles keys that were already held when a native listener started.
// They belong to the existing foreground input sequence and must be released
// before protection is armed.
func (e *Engine) Seed(keys []Key) {
	e.pressed = map[Key]press{}
	e.fired = map[string]bool{}
	for _, key := range keys {
		e.pressed[key] = press{}
	}
	if len(keys) > 0 {
		policy := e.policy.Clone()
		e.pending = &policy
		e.policy.Suspended = true
	}
}

func (e *Engine) Process(event Event) Decision {
	var result Decision
	if !event.Down {
		if p, exists := e.pressed[event.Key]; exists {
			result.Suppress = p.suppressed
		}
		delete(e.pressed, event.Key)
		for _, binding := range e.policy.Bindings {
			if !e.matches(binding.Keys) {
				delete(e.fired, binding.Action)
			}
		}
		if len(e.pressed) == 0 && e.pending != nil {
			e.policy = *e.pending
			e.pending = nil
			e.fired = map[string]bool{}
		}
		result.VisibleModifiers = e.visibleModifiers()
		result.VisibleSides, result.RewriteModifiers = e.visibleSides()
		return result
	}

	previous, held := e.pressed[event.Key]
	if !held {
		e.pressed[event.Key] = press{}
	}
	protect := !e.policy.Suspended && e.policy.Protection.Enabled && e.protectedHeld()
	result.Suppress = previous.suppressed || protect
	if !e.policy.Suspended && e.pending == nil && !IsModifier(event.Key) {
		for _, binding := range e.policy.Bindings {
			// Keep the unselected side available while protection is enabled.
			if e.policy.Protection.Enabled && containsFamily(binding.Keys, e.policy.Modifier) && !protect {
				continue
			}
			if e.matches(binding.Keys) {
				result.Suppress = true
				if (!held && !event.Repeat && !e.fired[binding.Action]) || (e.fired[binding.Action] && binding.Repeat) {
					result.Action = binding.Action
					e.fired[binding.Action] = true
				}
				break
			}
		}
	}
	if !held {
		e.pressed[event.Key] = press{suppressed: result.Suppress}
	}
	result.VisibleModifiers = e.visibleModifiers()
	result.VisibleSides, result.RewriteModifiers = e.visibleSides()
	return result
}

func (e *Engine) visibleSides() (uint8, bool) {
	var visible uint8
	var rewrite bool
	keys := [...]Key{"ControlLeft", "ControlRight", "MetaLeft", "MetaRight", "AltLeft", "AltRight", "ShiftLeft", "ShiftRight"}
	for i, key := range keys {
		if state, held := e.pressed[key]; held {
			if state.suppressed {
				rewrite = true
			} else {
				visible |= 1 << i
			}
		}
	}
	return visible, rewrite
}

func (e *Engine) protectedHeld() bool {
	p := e.policy.Protection
	_, left := e.pressed[Key(string(e.policy.Modifier)+"Left")]
	_, right := e.pressed[Key(string(e.policy.Modifier)+"Right")]
	return p.BlockLeft && left || p.BlockRight && right
}

func (e *Engine) matches(keys []Key) bool {
	wanted := make(map[Key]bool, len(keys))
	for _, key := range keys {
		wanted[key] = true
	}
	actual := make(map[Key]bool, len(e.pressed))
	for key := range e.pressed {
		if wanted[Family(key)] {
			key = Family(key)
		}
		actual[key] = true
	}
	if len(wanted) != len(actual) {
		return false
	}
	for key := range wanted {
		if !actual[key] {
			return false
		}
	}
	return true
}

func containsFamily(keys []Key, family Key) bool {
	for _, key := range keys {
		if Family(key) == family {
			return true
		}
	}
	return false
}

func (e *Engine) visibleModifiers() uint8 {
	var flags uint8
	for key, state := range e.pressed {
		if state.suppressed {
			continue
		}
		switch Family(key) {
		case Control:
			flags |= VisibleControl
		case Meta:
			flags |= VisibleMeta
		case Alt:
			flags |= VisibleAlt
		case Shift:
			flags |= VisibleShift
		}
	}
	return flags
}
