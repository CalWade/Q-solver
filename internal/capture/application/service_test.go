package application

import (
	"Q-Solver/internal/capture/domain"
	"context"
	"errors"
	"image"
	"testing"
)

type sourceFunc func(context.Context, string) (image.Image, error)

func (f sourceFunc) Capture(ctx context.Context, mode string) (image.Image, error) {
	return f(ctx, mode)
}

type processorFunc func(image.Image, domain.Options) (domain.Result, error)

func (f processorFunc) Process(img image.Image, o domain.Options) (domain.Result, error) {
	return f(img, o)
}

type guardFunc func() (func() error, error)

func (f guardFunc) BeginCapture() (func() error, error) { return f() }
func TestCaptureReleasesBeforeProcessingAndOnSourceFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		released := false
		processed := false
		guard := guardFunc(func() (func() error, error) { return func() error { released = true; return nil }, nil })
		source := sourceFunc(func(context.Context, string) (image.Image, error) {
			if released {
				t.Fatal("lease ended too soon")
			}
			if fail {
				return nil, errors.New("screen denied")
			}
			return image.NewRGBA(image.Rect(0, 0, 2, 2)), nil
		})
		processor := processorFunc(func(image.Image, domain.Options) (domain.Result, error) {
			if !released {
				t.Fatal("window stayed transparent during encoding")
			}
			processed = true
			return domain.Result{Bytes: []byte{1}}, nil
		})
		result, err := New(source, processor, guard).Capture(context.Background(), domain.Options{Quality: 80, Mode: "window"})
		if !released || processed == fail || (err != nil) != fail {
			t.Fatal(released, processed, err)
		}
		if !fail && len(result.Bytes) != 1 {
			t.Fatal(result)
		}
	}
}
func TestInvalidCancelledAndFailedPolicyNeverCapture(t *testing.T) {
	source := sourceFunc(func(context.Context, string) (image.Image, error) { t.Fatal("unexpected capture"); return nil, nil })
	denied := guardFunc(func() (func() error, error) { return nil, errors.New("suppression failed") })
	s := New(source, nil, denied)
	if _, err := s.Capture(context.Background(), domain.Options{Quality: 80, Mode: "fullscreen"}); err == nil {
		t.Fatal("policy failure ignored")
	}
	if _, err := s.Capture(context.Background(), domain.Options{Quality: 0, Mode: "bad"}); err == nil {
		t.Fatal("invalid options accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Capture(ctx, domain.Options{Quality: 80, Mode: "window"}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestFailedRestoreIsReturnedAndPreventsProcessing(t *testing.T) {
	source := sourceFunc(func(context.Context, string) (image.Image, error) { return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil })
	guard := guardFunc(func() (func() error, error) { return func() error { return errors.New("restore failed") }, nil })
	if _, err := New(source, nil, guard).Capture(context.Background(), domain.Options{Quality: 80, Mode: "window"}); err == nil {
		t.Fatal("restore failure lost")
	}
}
