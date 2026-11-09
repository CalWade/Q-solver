package screen

import (
	"Q-Solver/internal/capture/application"
	"Q-Solver/internal/capture/domain"
	"Q-Solver/internal/capture/infrastructure"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/kbinani/screenshot"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"image"
	"sync"
)

type PreviewResult struct {
	ImgBytes []byte `json:"imgBytes"`
	Base64   string `json:"base64"`
	Size     string `json:"size"`
}
type Service struct {
	mu    sync.RWMutex
	ctx   context.Context
	guard application.Guard
}

func NewService() *Service                     { return &Service{} }
func (s *Service) Startup(ctx context.Context) { s.mu.Lock(); s.ctx = ctx; s.mu.Unlock() }
func (s *Service) SetCaptureGuard(guard application.Guard) {
	s.mu.Lock()
	s.guard = guard
	s.mu.Unlock()
}
func (s *Service) CapturePreview(quality int, sharpen float64, grayscale, noCompression bool, mode string) (PreviewResult, error) {
	s.mu.RLock()
	ctx := s.ctx
	s.mu.RUnlock()
	if ctx == nil {
		return PreviewResult{}, fmt.Errorf("capture context is not initialized")
	}
	return s.CapturePreviewContext(ctx, domain.Options{Quality: quality, Sharpen: sharpen, Grayscale: grayscale, NoCompression: noCompression, Mode: mode})
}

func (s *Service) CapturePreviewContext(ctx context.Context, options domain.Options) (PreviewResult, error) {
	s.mu.RLock()
	windowContext, guard := s.ctx, s.guard
	s.mu.RUnlock()
	if windowContext == nil {
		return PreviewResult{}, fmt.Errorf("capture context is not initialized")
	}
	pipeline := application.New(nativeSource{windowContext}, infrastructure.Processor{}, guard)
	result, err := pipeline.Capture(ctx, options)
	if err != nil {
		return PreviewResult{}, err
	}
	encoded := "data:" + result.MIME + ";base64," + base64.StdEncoding.EncodeToString(result.Bytes)
	return PreviewResult{ImgBytes: result.Bytes, Base64: encoded, Size: fmt.Sprintf("%.2f KB", float64(len(result.Bytes))/1024)}, nil
}

type nativeSource struct{ ctx context.Context }

func (s nativeSource) Capture(ctx context.Context, mode string) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var x, y, w, h int
	if mode == "fullscreen" {
		bounds := screenshot.GetDisplayBounds(0)
		x, y, w, h = bounds.Min.X, bounds.Min.Y, bounds.Dx(), bounds.Dy()
	} else {
		x, y = runtime.WindowGetPosition(s.ctx)
		w, h = runtime.WindowGetSize(s.ctx)
	}
	return screenshot.Capture(x, y, w, h)
}
