package domain

import (
	"fmt"
	"slices"
)

type Protection struct {
	Enabled    bool `json:"enabled"`
	BlockLeft  bool `json:"blockLeft"`
	BlockRight bool `json:"blockRight"`
}

func (p Protection) Validate() error {
	if p.Enabled && !p.BlockLeft && !p.BlockRight {
		return fmt.Errorf("启用键盘防护时请至少选择一侧修饰键")
	}
	return nil
}

type Binding struct {
	Action string `json:"action"`
	Keys   []Key  `json:"keys"`
	Repeat bool   `json:"repeat"`
}

type Policy struct {
	Bindings   []Binding
	Protection Protection
	Modifier   Key
	Suspended  bool
}

func (p Policy) Validate() error {
	if err := p.Protection.Validate(); err != nil {
		return err
	}
	if p.Modifier != Control && p.Modifier != Meta {
		return fmt.Errorf("防护修饰键必须是 Ctrl 或 Command")
	}
	actions := map[string]bool{}
	for i, binding := range p.Bindings {
		if binding.Action == "" || actions[binding.Action] {
			return fmt.Errorf("快捷键动作为空或重复")
		}
		actions[binding.Action] = true
		if _, err := CanonicalKeys(binding.Keys); err != nil {
			return fmt.Errorf("%s: %w", binding.Action, err)
		}
		for _, previous := range p.Bindings[:i] {
			if overlaps(binding.Keys, previous.Keys) {
				return fmt.Errorf("快捷键冲突：%s 与 %s", previous.Action, binding.Action)
			}
		}
	}
	return nil
}

func (p Policy) Clone() Policy {
	p.Bindings = slices.Clone(p.Bindings)
	for i := range p.Bindings {
		p.Bindings[i].Keys = slices.Clone(p.Bindings[i].Keys)
	}
	return p
}

func overlaps(a, b []Key) bool {
	// Expand generic modifiers so a left-specific binding cannot silently shadow
	// a binding that promises to work on either side.
	for _, aa := range expand(a) {
		for _, bb := range expand(b) {
			if slices.Equal(aa, bb) {
				return true
			}
		}
	}
	return false
}

func expand(keys []Key) [][]Key {
	result := [][]Key{{}}
	for _, key := range keys {
		options := []Key{key}
		if IsModifier(key) && Family(key) == key {
			options = []Key{Key(string(key) + "Left"), Key(string(key) + "Right")}
		}
		var next [][]Key
		for _, prefix := range result {
			for _, option := range options {
				next = append(next, append(slices.Clone(prefix), option))
			}
		}
		result = next
	}
	for _, keys := range result {
		slices.Sort(keys)
	}
	return result
}
