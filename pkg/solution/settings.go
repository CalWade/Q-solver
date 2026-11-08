package solution

import (
	capture "Q-Solver/internal/capture/domain"
	"Q-Solver/internal/solve/domain"
	"Q-Solver/pkg/config"
)

type SettingsSource struct{ Manager *config.ConfigManager }

func (s SettingsSource) Snapshot() domain.Settings {
	cfg := s.Manager.Get()
	return domain.Settings{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, DomainID: cfg.DomainId, ResumeContent: cfg.ResumeContent,
		KeepContext: cfg.KeepContext, InterruptThinking: cfg.InterruptThinking,
		Capture: capture.Options{Quality: cfg.CompressionQuality, Sharpen: cfg.Sharpening, Grayscale: cfg.Grayscale, NoCompression: cfg.NoCompression, Mode: cfg.ScreenshotMode}}
}
