package application

import (
	"Q-Solver/internal/capture/domain"
	"context"
	"errors"
	"image"
)

type Source interface {
	Capture(context.Context, string) (image.Image, error)
}
type Processor interface {
	Process(image.Image, domain.Options) (domain.Result, error)
}
type Guard interface{ BeginCapture() (func() error, error) }
type Service struct {
	source    Source
	processor Processor
	guard     Guard
}

func New(source Source, processor Processor, guard Guard) *Service {
	return &Service{source: source, processor: processor, guard: guard}
}
func (s *Service) Capture(ctx context.Context, options domain.Options) (domain.Result, error) {
	if err := options.Validate(); err != nil {
		return domain.Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return domain.Result{}, err
	}
	if s.guard == nil {
		return domain.Result{}, errors.New("capture window policy is not initialized")
	}
	release, err := s.guard.BeginCapture()
	if err != nil {
		return domain.Result{}, err
	}
	frame, err := func() (frame image.Image, err error) {
		defer func() { err = errors.Join(err, release()) }()
		return s.source.Capture(ctx, options.Mode)
	}()
	if err != nil {
		return domain.Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return domain.Result{}, err
	}
	if frame == nil {
		return domain.Result{}, errors.New("capture source returned an empty image")
	}
	result, err := s.processor.Process(frame, options)
	if err != nil {
		return domain.Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return domain.Result{}, err
	}
	return result, nil
}
