package native

import "testing"

func TestWindowBindingNeverReplacesALiveHandleImplicitly(t *testing.T) {
	d := New()
	if err := d.Bind("main", 42); err != nil {
		t.Fatal(err)
	}
	if err := d.Bind("main", 43); err == nil {
		t.Fatal("overwrote live window")
	}
	if handle, err := d.handle("main"); err != nil || handle != 42 {
		t.Fatal(handle, err)
	}
	d.Unbind("main")
	if _, err := d.handle("main"); err == nil {
		t.Fatal("stale handle retained")
	}
	if err := d.Bind("main", 43); err != nil {
		t.Fatal(err)
	}
	if err := d.Bind("empty", 0); err == nil {
		t.Fatal("zero handle accepted")
	}
}
