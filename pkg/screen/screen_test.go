package screen

import (
	"Q-Solver/internal/capture/domain"
	"context"
	"errors"
	"testing"
)

func TestUninitializedScreenDoesNotCapture(t *testing.T) {
	s := NewService()
	if _, err := s.CapturePreview(80, 0, false, false, "window"); err == nil {
		t.Fatal("uninitialized capture accepted")
	}
	if _, err := s.CapturePreviewContext(context.Background(), domain.Options{Quality: 80, Mode: "window"}); err == nil {
		t.Fatal("uninitialized capture accepted")
	}
}
func TestRequestCancellationIsUsedInsteadOfWindowLifetime(t *testing.T) {
	s := NewService()
	s.Startup(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.CapturePreviewContext(ctx, domain.Options{Quality: 80, Mode: "window"})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("request cancellation lost", err)
	}
}
