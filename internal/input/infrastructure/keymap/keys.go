package keymap

import "Q-Solver/internal/input/domain"

var macKeys = map[uint32]domain.Key{
	0: "KeyA", 1: "KeyS", 2: "KeyD", 3: "KeyF", 4: "KeyH", 5: "KeyG", 6: "KeyZ", 7: "KeyX", 8: "KeyC", 9: "KeyV", 11: "KeyB",
	12: "KeyQ", 13: "KeyW", 14: "KeyE", 15: "KeyR", 16: "KeyY", 17: "KeyT", 18: "Digit1", 19: "Digit2", 20: "Digit3", 21: "Digit4", 22: "Digit6", 23: "Digit5", 24: "Equal", 25: "Digit9", 26: "Digit7", 27: "Minus", 28: "Digit8", 29: "Digit0", 30: "BracketRight", 31: "KeyO", 32: "KeyU", 33: "BracketLeft", 34: "KeyI", 35: "KeyP", 36: "Enter", 37: "KeyL", 38: "KeyJ", 39: "Quote", 40: "KeyK", 41: "Semicolon", 42: "Backslash", 43: "Comma", 44: "Slash", 45: "KeyN", 46: "KeyM", 47: "Period", 48: "Tab", 49: "Space", 50: "Backquote", 51: "Backspace", 53: "Escape",
	54: "MetaRight", 55: "MetaLeft", 56: "ShiftLeft", 58: "AltLeft", 59: "ControlLeft", 60: "ShiftRight", 61: "AltRight", 62: "ControlRight",
	76: "NumpadEnter", 96: "F5", 97: "F6", 98: "F7", 99: "F3", 100: "F8", 101: "F9", 103: "F11", 105: "F13", 106: "F16", 107: "F14", 109: "F10", 111: "F12", 113: "F15", 114: "Insert", 115: "Home", 116: "PageUp", 117: "Delete", 118: "F4", 119: "End", 120: "F2", 121: "PageDown", 122: "F1", 123: "ArrowLeft", 124: "ArrowRight", 125: "ArrowDown", 126: "ArrowUp", 64: "F17", 79: "F18", 80: "F19", 90: "F20",
	1000: "MouseBack", 1001: "MouseForward",
}

func Mac(code uint32) domain.Key { return macKeys[code] }
func MacCode(key domain.Key) (uint32, bool) {
	for code, k := range macKeys {
		if k == key {
			return code, true
		}
	}
	return 0, false
}

// Windows scan codes identify the physical key even when the character layout
// changes. Virtual keys remain necessary for modifiers and system hotkeys.
var scanKeys = map[uint32]domain.Key{
	0x02: "Digit1", 0x03: "Digit2", 0x04: "Digit3", 0x05: "Digit4", 0x06: "Digit5", 0x07: "Digit6", 0x08: "Digit7", 0x09: "Digit8", 0x0a: "Digit9", 0x0b: "Digit0", 0x0c: "Minus", 0x0d: "Equal",
	0x10: "KeyQ", 0x11: "KeyW", 0x12: "KeyE", 0x13: "KeyR", 0x14: "KeyT", 0x15: "KeyY", 0x16: "KeyU", 0x17: "KeyI", 0x18: "KeyO", 0x19: "KeyP", 0x1a: "BracketLeft", 0x1b: "BracketRight",
	0x1e: "KeyA", 0x1f: "KeyS", 0x20: "KeyD", 0x21: "KeyF", 0x22: "KeyG", 0x23: "KeyH", 0x24: "KeyJ", 0x25: "KeyK", 0x26: "KeyL", 0x27: "Semicolon", 0x28: "Quote", 0x29: "Backquote", 0x2b: "Backslash",
	0x2c: "KeyZ", 0x2d: "KeyX", 0x2e: "KeyC", 0x2f: "KeyV", 0x30: "KeyB", 0x31: "KeyN", 0x32: "KeyM", 0x33: "Comma", 0x34: "Period", 0x35: "Slash",
}
var virtualKeys = map[uint32]domain.Key{5: "MouseBack", 6: "MouseForward", 8: "Backspace", 9: "Tab", 13: "Enter", 27: "Escape", 32: "Space", 33: "PageUp", 34: "PageDown", 35: "End", 36: "Home", 37: "ArrowLeft", 38: "ArrowUp", 39: "ArrowRight", 40: "ArrowDown", 45: "Insert", 46: "Delete", 91: "MetaLeft", 92: "MetaRight", 160: "ShiftLeft", 161: "ShiftRight", 162: "ControlLeft", 163: "ControlRight", 164: "AltLeft", 165: "AltRight", 186: "Semicolon", 187: "Equal", 188: "Comma", 189: "Minus", 190: "Period", 191: "Slash", 192: "Backquote", 219: "BracketLeft", 220: "Backslash", 221: "BracketRight", 222: "Quote"}

func Windows(vk, scan uint32, extended bool) domain.Key {
	switch vk {
	case 16:
		if scan == 0x36 {
			return "ShiftRight"
		}
		return "ShiftLeft"
	case 17:
		if extended {
			return "ControlRight"
		}
		return "ControlLeft"
	case 18:
		if extended {
			return "AltRight"
		}
		return "AltLeft"
	case 13:
		if extended {
			return "NumpadEnter"
		}
	}
	if key := scanKeys[scan]; key != "" && !extended {
		return key
	}
	if vk >= 65 && vk <= 90 {
		return domain.Key("Key" + string(rune(vk)))
	}
	if vk >= 48 && vk <= 57 {
		return domain.Key("Digit" + string(rune(vk)))
	}
	if vk >= 112 && vk <= 135 {
		return functionKey(int(vk - 111))
	}
	return virtualKeys[vk]
}

func WindowsVK(key domain.Key) (uint32, bool) {
	s := string(key)
	if len(s) == 4 && s[:3] == "Key" {
		return uint32(s[3]), true
	}
	if len(s) == 6 && s[:5] == "Digit" {
		return uint32(s[5]), true
	}
	for vk, k := range virtualKeys {
		if k == key {
			return vk, true
		}
	}
	for vk := uint32(112); vk <= 135; vk++ {
		if functionKey(int(vk-111)) == key {
			return vk, true
		}
	}
	return 0, false
}

func WindowsScan(key domain.Key) (uint32, bool) {
	for scan, k := range scanKeys {
		if k == key {
			return scan, true
		}
	}
	return 0, false
}

func functionKey(i int) domain.Key {
	if i < 10 {
		return domain.Key("F" + string(rune('0'+i)))
	}
	return domain.Key("F" + string(rune('0'+i/10)) + string(rune('0'+i%10)))
}

func SystemChord(binding domain.Binding) ([]domain.Key, domain.Key, bool) {
	var mods []domain.Key
	var main domain.Key
	for _, key := range binding.Keys {
		if domain.IsModifier(key) {
			if domain.Family(key) != key {
				return nil, "", false
			}
			mods = append(mods, key)
		} else {
			if main != "" {
				return nil, "", false
			}
			main = key
		}
	}
	return mods, main, main != "" && main != "MouseBack" && main != "MouseForward"
}
