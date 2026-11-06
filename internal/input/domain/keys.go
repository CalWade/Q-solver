package domain

import (
	"fmt"
	"sort"
	"strings"
)

// Key uses physical DOM code names. Generic modifiers are allowed in bindings,
// while incoming events always preserve their left/right identity.
type Key string

const (
	Control Key = "Control"
	Meta    Key = "Meta"
	Alt     Key = "Alt"
	Shift   Key = "Shift"
)

func Family(key Key) Key {
	switch key {
	case "ControlLeft", "ControlRight":
		return Control
	case "MetaLeft", "MetaRight":
		return Meta
	case "AltLeft", "AltRight":
		return Alt
	case "ShiftLeft", "ShiftRight":
		return Shift
	default:
		return key
	}
}

func IsModifier(key Key) bool {
	switch Family(key) {
	case Control, Meta, Alt, Shift:
		return true
	}
	return false
}

func ValidKey(key Key) bool {
	s := string(key)
	if IsModifier(key) {
		return true
	}
	if len(s) == 4 && strings.HasPrefix(s, "Key") && s[3] >= 'A' && s[3] <= 'Z' {
		return true
	}
	if len(s) == 6 && strings.HasPrefix(s, "Digit") && s[5] >= '0' && s[5] <= '9' {
		return true
	}
	for i := 1; i <= 24; i++ {
		if s == fmt.Sprintf("F%d", i) {
			return true
		}
	}
	switch key {
	case "Enter", "NumpadEnter", "Space", "Tab", "Escape", "Backspace", "Delete", "Insert",
		"ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "Home", "End", "PageUp", "PageDown",
		"Backquote", "Minus", "Equal", "BracketLeft", "BracketRight", "Backslash", "Semicolon",
		"Quote", "Comma", "Period", "Slash", "MouseBack", "MouseForward":
		return true
	}
	return false
}

func CanonicalKeys(keys []Key) ([]Key, error) {
	if len(keys) == 0 || len(keys) > 6 {
		return nil, fmt.Errorf("快捷键需要 1 至 6 个按键")
	}
	seen := make(map[Key]bool, len(keys))
	main := false
	for _, key := range keys {
		if !ValidKey(key) || seen[key] {
			return nil, fmt.Errorf("不支持或重复的按键 %q", key)
		}
		seen[key] = true
		main = main || !IsModifier(key)
	}
	if !main {
		return nil, fmt.Errorf("快捷键需要一个非修饰键")
	}
	for key := range seen {
		if f := Family(key); f != key && seen[f] {
			return nil, fmt.Errorf("不能同时指定 %s 与 %s", f, key)
		}
	}
	result := append([]Key(nil), keys...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func DisplayKeys(keys []Key) string {
	keys = append([]Key(nil), keys...)
	sort.SliceStable(keys, func(i, j int) bool {
		if IsModifier(keys[i]) != IsModifier(keys[j]) {
			return IsModifier(keys[i])
		}
		return keys[i] < keys[j]
	})
	labels := make([]string, 0, len(keys))
	for _, key := range keys {
		s := string(key)
		s = strings.TrimPrefix(strings.TrimPrefix(s, "Key"), "Digit")
		s = strings.ReplaceAll(s, "Control", "Ctrl")
		s = strings.ReplaceAll(s, "Meta", "Cmd")
		labels = append(labels, s)
	}
	return strings.Join(labels, "+")
}
