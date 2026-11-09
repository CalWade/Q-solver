package shortcut

import (
	"Q-Solver/internal/input/domain"
	"Q-Solver/internal/input/infrastructure/keymap"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

var actions = map[string]bool{"screenshot": true, "send": true, "toggle": true, "clickthrough": true, "delete": true, "move_up": true, "move_down": true, "move_left": true, "move_right": true, "scroll_up": true, "scroll_down": true, "panic": true}

func ParseBinding(binding KeyBinding) ([]domain.Key, error) {
	if len(binding.Keys) > 0 {
		return domain.CanonicalKeys(binding.Keys)
	}
	var keys []domain.Key
	symbolic := strings.IndexFunc(binding.ComboID, func(r rune) bool { return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' }) >= 0
	for _, part := range strings.Split(binding.ComboID, "+") {
		var key domain.Key
		if symbolic && len(part) == 1 && part[0] >= '0' && part[0] <= '9' {
			key = domain.Key("Digit" + part)
		} else if vk, err := strconv.ParseUint(part, 10, 32); err == nil {
			switch vk {
			case 16:
				key = domain.Shift
			case 17:
				key = domain.Control
			case 18:
				key = domain.Alt
			default:
				key = keymap.Windows(uint32(vk), 0, false)
			}
		} else {
			switch part {
			case "Cmd", "Command":
				key = domain.Meta
			case "Ctrl":
				key = domain.Control
			case "Option":
				key = domain.Alt
			case "Return":
				key = "Enter"
			case "Esc":
				key = "Escape"
			case "Up", "Down", "Left", "Right":
				key = domain.Key("Arrow" + part)
			case "PgUp":
				key = "PageUp"
			case "PgDn":
				key = "PageDown"
			default:
				key = domain.Key(part)
				if len(part) == 1 && part[0] >= 'A' && part[0] <= 'Z' {
					key = domain.Key("Key" + part)
				}
			}
		}
		if key == "" {
			return nil, fmt.Errorf("无法识别快捷键 %q", part)
		}
		keys = append(keys, key)
	}
	return domain.CanonicalKeys(keys)
}

func Profile(bindings map[string]KeyBinding, protection domain.Protection, platform string) (domain.Policy, error) {
	p := domain.Policy{Protection: protection, Modifier: domain.Control}
	if platform == "darwin" {
		p.Modifier = domain.Meta
	}
	names := make([]string, 0, len(bindings))
	for action := range bindings {
		names = append(names, action)
	}
	sort.Strings(names)
	for _, action := range names {
		if !actions[action] {
			return p, fmt.Errorf("未知快捷键动作 %q", action)
		}
		keys, err := ParseBinding(bindings[action])
		if err != nil {
			return p, fmt.Errorf("%s: %w", action, err)
		}
		p.Bindings = append(p.Bindings, domain.Binding{Action: action, Keys: keys, Repeat: strings.HasPrefix(action, "move_") || strings.HasPrefix(action, "scroll_")})
	}
	return p, p.Validate()
}

func NormalizeBindings(bindings map[string]KeyBinding) (map[string]KeyBinding, error) {
	result := make(map[string]KeyBinding, len(bindings))
	for action, binding := range bindings {
		keys, err := ParseBinding(binding)
		if err != nil {
			return nil, err
		}
		parts := make([]string, len(keys))
		for i, key := range keys {
			parts[i] = string(key)
		}
		result[action] = KeyBinding{Keys: keys, ComboID: strings.Join(parts, "+"), KeyName: domain.DisplayKeys(keys)}
	}
	return result, nil
}

func CloneBindings(bindings map[string]KeyBinding) map[string]KeyBinding {
	result := make(map[string]KeyBinding, len(bindings))
	for action, binding := range bindings {
		binding.Keys = append([]domain.Key(nil), binding.Keys...)
		result[action] = binding
	}
	return result
}
