package application

import (
	capture "Q-Solver/internal/capture/domain"
	"Q-Solver/internal/solve/domain"
	"context"
	"crypto/rand"
	"errors"
	"slices"
	"sync"
)

type SettingsSource interface{ Snapshot() domain.Settings }
type Capturer interface {
	Capture(context.Context, capture.Options) (string, error)
}
type Generator interface {
	Generate(context.Context, domain.Request, func(domain.Chunk)) (domain.Result, error)
}

// Publish receives ordered events and must not block or reenter this service.
// UI/HTTP adapters only enqueue or serialize these immutable payloads.
type Observer interface{ Publish(domain.Event) }
type Service struct {
	commands               sync.Mutex
	mu                     sync.Mutex
	parent                 context.Context
	shutdown               context.CancelFunc
	settings               SettingsSource
	capture                Capturer
	generator              Generator
	observer               Observer
	queue                  []domain.Image
	history                []domain.Turn
	epoch                  uint64
	sequence               uint64
	sessionID              string
	nextRequest, nextImage int64
	request                domain.Request
	phase                  string
	result                 domain.Result
	failure                string
	cancel                 context.CancelFunc
	closed                 bool
	workers                sync.WaitGroup
}

func New(parent context.Context, settings SettingsSource, capture Capturer, generator Generator, observer Observer) *Service {
	ctx, cancel := context.WithCancel(parent)
	return &Service{parent: ctx, shutdown: cancel, settings: settings, capture: capture, generator: generator, observer: observer, phase: "idle", sessionID: rand.Text()}
}
func (s *Service) busy() bool { return s.phase == "capturing" || s.phase == "generating" }
func (s *Service) emit(event domain.Event) {
	s.sequence++
	event.Sequence = s.sequence
	event.SessionID = s.sessionID
	event.Images = slices.Clone(event.Images)
	if s.observer != nil {
		s.observer.Publish(event)
	}
}
func (s *Service) Capture() error {
	s.commands.Lock()
	defer s.commands.Unlock()
	snapshot := s.settings.Snapshot()
	if err := snapshot.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("工作流已停止")
	}
	if s.busy() {
		s.mu.Unlock()
		return domain.ErrBusy
	}
	if len(s.queue) >= domain.MaxScreenshots {
		s.mu.Unlock()
		return domain.ErrScreenshotLimit
	}
	s.mu.Unlock()
	data, err := s.capture.Capture(s.parent, snapshot.Capture)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("工作流已停止")
	}
	s.nextImage++
	s.queue = append(s.queue, domain.Image{ID: s.nextImage, DataURL: data})
	s.emit(domain.Event{Kind: "screenshots", Images: s.queue})
	return nil
}
func (s *Service) Remove(index int) {
	s.commands.Lock()
	defer s.commands.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.queue) {
		return
	}
	s.queue = append(s.queue[:index], s.queue[index+1:]...)
	s.emit(domain.Event{Kind: "screenshots", Images: s.queue})
}
func (s *Service) RemoveLast() {
	s.commands.Lock()
	defer s.commands.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queue) == 0 {
		return
	}
	s.queue = s.queue[:len(s.queue)-1]
	s.emit(domain.Event{Kind: "screenshots", Images: s.queue})
}
func (s *Service) ClearScreenshots() {
	s.commands.Lock()
	defer s.commands.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = nil
	s.emit(domain.Event{Kind: "screenshots"})
}

func (s *Service) Send() (int64, error) {
	s.commands.Lock()
	defer s.commands.Unlock()
	snapshot := s.settings.Snapshot()
	if err := snapshot.Validate(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return 0, errors.New("工作流已停止")
	}
	if s.busy() {
		if !snapshot.InterruptThinking {
			s.mu.Unlock()
			return 0, domain.ErrBusy
		}
		s.cancelLocked()
	}
	s.nextRequest++
	id := s.nextRequest
	ctx, cancel := context.WithCancel(s.parent)
	s.cancel = cancel
	request := domain.Request{ID: id, Settings: snapshot, Images: slices.Clone(s.queue)}
	if snapshot.KeepContext {
		request.History = domain.CloneHistory(s.history)
	}
	s.queue = nil
	s.request = request
	s.phase = "capturing"
	s.result = domain.Result{}
	s.failure = ""
	epoch := s.epoch
	s.emit(domain.Event{Kind: "started", RequestID: id, Images: request.Images, KeepContext: snapshot.KeepContext})
	s.emit(domain.Event{Kind: "screenshots"})
	s.mu.Unlock()
	if len(request.Images) == 0 {
		data, err := s.capture.Capture(ctx, snapshot.Capture)
		if err != nil {
			s.finish(id, epoch, domain.Result{}, err)
			return id, err
		}
		s.mu.Lock()
		s.nextImage++
		request.Images = []domain.Image{{ID: s.nextImage, DataURL: data}}
		s.mu.Unlock()
	}
	s.mu.Lock()
	if s.request.ID != id || !s.busy() || ctx.Err() != nil {
		s.mu.Unlock()
		return id, context.Canceled
	}
	s.request = request.Clone()
	s.phase = "generating"
	s.emit(domain.Event{Kind: "ready", RequestID: id, Images: request.Images, KeepContext: snapshot.KeepContext})
	s.workers.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.workers.Done()
		result, err := s.generator.Generate(ctx, request.Clone(), func(chunk domain.Chunk) {
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.request.ID != id || s.phase != "generating" || ctx.Err() != nil {
				return
			}
			kind := "chunk"
			if chunk.Thinking {
				kind = "thinking"
				s.result.Thinking += chunk.Text
			} else {
				s.result.Content += chunk.Text
			}
			s.emit(domain.Event{Kind: kind, RequestID: id, Text: chunk.Text})
		})
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		s.finish(id, epoch, result, err)
	}()
	return id, nil
}
func (s *Service) finish(id int64, epoch uint64, result domain.Result, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.request.ID != id || !s.busy() {
		return
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	kind, text := "completed", result.Content
	if err != nil {
		kind = "failed"
		text = err.Error()
		if errors.Is(err, context.Canceled) {
			kind = "cancelled"
		}
	}
	if kind == "completed" && result.Content == "" && result.Thinking == "" {
		kind = "failed"
		text = "模型返回内容为空"
	}
	s.phase = kind
	if kind == "completed" {
		s.result = result
	} else if kind == "failed" {
		s.failure = text
	}
	if kind == "completed" && s.epoch == epoch {
		s.history = domain.AppendHistory(s.history, domain.Turn{Images: s.request.Images, Answer: result.Content}, s.request.Settings.KeepContext)
	}
	s.emit(domain.Event{Kind: kind, RequestID: id, Text: text, Thinking: result.Thinking})
}
func (s *Service) cancelLocked() {
	if !s.busy() {
		return
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.phase = "cancelled"
	s.emit(domain.Event{Kind: "cancelled", RequestID: s.request.ID, Text: "请求已停止"})
}

// Cancel rejects stale stop buttons: they cannot cancel a newer request.
func (s *Service) Cancel(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id != s.request.ID || !s.busy() {
		return false
	}
	s.cancelLocked()
	return true
}
func (s *Service) CancelCurrent() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.busy() {
		return false
	}
	s.cancelLocked()
	return true
}
func (s *Service) ClearContext() { s.mu.Lock(); s.history = nil; s.epoch++; s.mu.Unlock() }
func (s *Service) Status() domain.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return domain.Status{SessionID: s.sessionID, RequestID: s.request.ID, Sequence: s.sequence, Phase: s.phase,
		Images: slices.Clone(s.queue), RequestImages: slices.Clone(s.request.Images),
		Content: s.result.Content, Thinking: s.result.Thinking, Error: s.failure,
		KeepContext: s.request.Settings.KeepContext}
}
func (s *Service) Close() {
	s.shutdown()
	s.mu.Lock()
	s.closed = true
	s.cancelLocked()
	s.mu.Unlock()
	// Wait for a concurrent capture/send to finish reserving its worker before Wait.
	s.commands.Lock()
	s.commands.Unlock()
	s.workers.Wait()
}
