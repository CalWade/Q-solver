package application

import (
	capture "Q-Solver/internal/capture/domain"
	"Q-Solver/internal/solve/domain"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type settingsStub struct {
	mu    sync.Mutex
	value domain.Settings
}

func (s *settingsStub) Snapshot() domain.Settings { s.mu.Lock(); defer s.mu.Unlock(); return s.value }
func (s *settingsStub) change(f func(*domain.Settings)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(&s.value)
}

type captureFunc func(context.Context, capture.Options) (string, error)

func (f captureFunc) Capture(ctx context.Context, options capture.Options) (string, error) {
	return f(ctx, options)
}

type modelReply struct {
	result domain.Result
	err    error
}
type modelCall struct {
	ctx     context.Context
	request domain.Request
	emit    func(domain.Chunk)
	done    chan modelReply
}
type modelStub struct{ calls chan modelCall }

func (m *modelStub) Generate(ctx context.Context, request domain.Request, emit func(domain.Chunk)) (domain.Result, error) {
	call := modelCall{ctx, request, emit, make(chan modelReply, 1)}
	m.calls <- call
	// Intentionally allow a late result after cancellation, as an external adapter might.
	r := <-call.done
	return r.result, r.err
}

type eventsStub struct {
	mu     sync.Mutex
	events []domain.Event
}

func (e *eventsStub) Publish(event domain.Event) {
	e.mu.Lock()
	e.events = append(e.events, event)
	e.mu.Unlock()
}
func (e *eventsStub) all() []domain.Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]domain.Event{}, e.events...)
}
func fixture(t *testing.T, source Capturer) (*Service, *settingsStub, *modelStub, *eventsStub) {
	t.Helper()
	settings := &settingsStub{value: domain.Settings{APIKey: "test", Model: "model-a", Capture: capture.Options{Quality: 80, Mode: "window"}}}
	if source == nil {
		source = captureFunc(func(context.Context, capture.Options) (string, error) { return "data:fixture", nil })
	}
	model := &modelStub{calls: make(chan modelCall, 10)}
	events := &eventsStub{}
	s := New(context.Background(), settings, source, model, events)
	t.Cleanup(s.Close)
	return s, settings, model, events
}
func awaitCall(t *testing.T, m *modelStub) modelCall {
	t.Helper()
	select {
	case c := <-m.calls:
		return c
	case <-time.After(time.Second):
		t.Fatal("generator not called")
		return modelCall{}
	}
}
func awaitPhase(t *testing.T, s *Service, phase string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if s.Status().Phase == phase {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("want %s, got %+v", phase, s.Status())
}

func TestRequestSnapshotsAndStaleCallbacks(t *testing.T) {
	s, settings, model, events := fixture(t, nil)
	first, err := s.Send()
	if err != nil {
		t.Fatal(err)
	}
	a := awaitCall(t, model)
	a.emit(domain.Chunk{Text: "partial"})
	settings.change(func(v *domain.Settings) { v.Model = "model-b"; v.InterruptThinking = true })
	second, err := s.Send()
	if err != nil {
		t.Fatal(err)
	}
	b := awaitCall(t, model)
	if a.request.Settings.Model != "model-a" || b.request.Settings.Model != "model-b" || second <= first {
		t.Fatal("snapshot changed")
	}
	if s.Cancel(first) {
		t.Fatal("stale cancel stopped new request")
	}
	a.emit(domain.Chunk{Text: "STALE"})
	a.done <- modelReply{result: domain.Result{Content: "STALE FINAL"}}
	b.emit(domain.Chunk{Thinking: true, Text: "think"})
	b.emit(domain.Chunk{Text: "draft"})
	b.done <- modelReply{result: domain.Result{Content: "final", Thinking: "think"}}
	awaitPhase(t, s, "completed")
	state := s.Status()
	if state.Content != "final" || state.RequestID != second || state.Thinking != "think" {
		t.Fatal(state)
	}
	var previous uint64
	for _, e := range events.all() {
		if e.Sequence <= previous || e.Text == "STALE" || e.Text == "STALE FINAL" {
			t.Fatal(e)
		}
		previous = e.Sequence
	}
}

func TestBusySendDoesNotCaptureAndCancelIsScoped(t *testing.T) {
	var captures atomic.Int64
	s, _, m, _ := fixture(t, captureFunc(func(context.Context, capture.Options) (string, error) { captures.Add(1); return "image", nil }))
	id, _ := s.Send()
	call := awaitCall(t, m)
	if _, err := s.Send(); !errors.Is(err, domain.ErrBusy) {
		t.Fatal(err)
	}
	if err := s.Capture(); !errors.Is(err, domain.ErrBusy) {
		t.Fatal(err)
	}
	if captures.Load() != 1 {
		t.Fatal("busy command captured screen")
	}
	if !s.Cancel(id) || s.Cancel(id) {
		t.Fatal("cancel lifecycle")
	}
	call.done <- modelReply{}
}

func TestConcurrentScreenshotQueueIsBoundedAndImmutable(t *testing.T) {
	var captures atomic.Int64
	s, _, _, _ := fixture(t, captureFunc(func(context.Context, capture.Options) (string, error) { return fmt.Sprint(captures.Add(1)), nil }))
	var wg sync.WaitGroup
	var success atomic.Int64
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.Capture() == nil {
				success.Add(1)
			}
		}()
	}
	wg.Wait()
	if success.Load() != 3 || captures.Load() != 3 {
		t.Fatal(success.Load(), captures.Load())
	}
	state := s.Status()
	state.Images[0].DataURL = "changed"
	if s.Status().Images[0].DataURL != "1" {
		t.Fatal("queue aliases snapshot")
	}
	s.Remove(1)
	s.Remove(99)
	s.RemoveLast()
	if len(s.Status().Images) != 1 || s.Status().Images[0].ID != 1 {
		t.Fatal(s.Status())
	}
	s.ClearScreenshots()
	if len(s.Status().Images) != 0 {
		t.Fatal("clear failed")
	}
}

func TestCancelDuringFallbackNeverCallsModel(t *testing.T) {
	started := make(chan struct{})
	s, _, m, _ := fixture(t, captureFunc(func(ctx context.Context, _ capture.Options) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	}))
	done := make(chan error, 1)
	go func() { _, err := s.Send(); done <- err }()
	<-started
	if !s.Cancel(s.Status().RequestID) {
		t.Fatal("could not cancel capture")
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(m.calls) != 0 || s.Status().Phase != "cancelled" {
		t.Fatal(s.Status())
	}
}

func TestClearContextCannotBeUndoneByInflightCompletion(t *testing.T) {
	s, settings, m, _ := fixture(t, nil)
	settings.change(func(v *domain.Settings) { v.KeepContext = true })
	s.Send()
	first := awaitCall(t, m)
	s.ClearContext()
	first.done <- modelReply{result: domain.Result{Content: "old"}}
	awaitPhase(t, s, "completed")
	s.Send()
	second := awaitCall(t, m)
	if len(second.request.History) != 0 {
		t.Fatal("cleared context restored")
	}
	second.done <- modelReply{result: domain.Result{Content: "new"}}
	awaitPhase(t, s, "completed")
	s.Send()
	third := awaitCall(t, m)
	if len(third.request.History) != 1 || third.request.History[0].Answer != "new" {
		t.Fatal(third.request.History)
	}
	third.done <- modelReply{err: errors.New("provider failed")}
	awaitPhase(t, s, "failed")
}

func TestCloseCancelsOutstandingStandaloneCapture(t *testing.T) {
	started := make(chan struct{})
	s, _, _, _ := fixture(t, captureFunc(func(ctx context.Context, _ capture.Options) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	}))
	done := make(chan error, 1)
	go func() { done <- s.Capture() }()
	<-started
	s.Close()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := s.Send(); err == nil {
		t.Fatal("closed service accepted request")
	}
}

func TestValidationFailureAndEmptyResults(t *testing.T) {
	s, settings, m, _ := fixture(t, nil)
	settings.change(func(v *domain.Settings) { v.APIKey = "" })
	if err := s.Capture(); !errors.Is(err, domain.ErrAPIKey) {
		t.Fatal(err)
	}
	if _, err := s.Send(); !errors.Is(err, domain.ErrAPIKey) {
		t.Fatal(err)
	}
	settings.change(func(v *domain.Settings) { v.APIKey = "fixture" })
	s.Send()
	call := awaitCall(t, m)
	call.done <- modelReply{}
	awaitPhase(t, s, "failed")
	if s.Status().Error == "" {
		t.Fatal("empty output treated as success")
	}
}
