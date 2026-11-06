package keymap

import (
	"Q-Solver/internal/input/domain"
	"testing"
)

func TestModifierSides(t *testing.T) {
	for _, tc := range []struct {
		vk, scan uint32
		ext      bool
		want     domain.Key
	}{{17, 0x1d, false, "ControlLeft"}, {17, 0x1d, true, "ControlRight"}, {16, 0x36, false, "ShiftRight"}, {18, 0x38, true, "AltRight"}, {162, 0x1d, false, "ControlLeft"}, {163, 0x1d, true, "ControlRight"}} {
		if got := Windows(tc.vk, tc.scan, tc.ext); got != tc.want {
			t.Errorf("%+v -> %s", tc, got)
		}
	}
	if Mac(55) != "MetaLeft" || Mac(54) != "MetaRight" {
		t.Fatal("mac command side mapping")
	}
}
func TestPhysicalKeysAndRoundtrip(t *testing.T) {
	if Windows(81, 0x1e, false) != "KeyA" {
		t.Fatal("physical scan code lost to layout-dependent VK")
	}
	for code, key := range macKeys {
		if !domain.ValidKey(key) {
			t.Errorf("invalid Mac key %d=%s", code, key)
		}
		if got, ok := MacCode(key); !ok || Mac(got) != key {
			t.Errorf("roundtrip %s", key)
		}
	}
	for vk, key := range virtualKeys {
		if got, ok := WindowsVK(key); !ok || virtualKeys[got] != key {
			t.Errorf("roundtrip %d=%s", vk, key)
		}
	}
	if Windows(13, 0, true) != "NumpadEnter" {
		t.Fatal("numpad Enter identity lost")
	}
}
func TestSystemChordClassification(t *testing.T) {
	for _, keys := range [][]domain.Key{{"ControlLeft", "KeyA"}, {"Control", "KeyA", "KeyB"}, {"MouseBack"}} {
		if _, _, ok := SystemChord(domain.Binding{Keys: keys}); ok {
			t.Fatalf("cannot use generic system registration: %v", keys)
		}
	}
	if _, main, ok := SystemChord(domain.Binding{Keys: []domain.Key{domain.Control, "KeyA"}}); !ok || main != "KeyA" {
		t.Fatal("ordinary system chord rejected")
	}
}
