// Package harness assembles production rules and persistence with simulated
// external ports. This server is an explicit development entry point only.
package harness

import (
	"Q-Solver/internal/input/application"
	"Q-Solver/internal/input/domain"
	"Q-Solver/internal/input/infrastructure/simulated"
	windowapp "Q-Solver/internal/window/application"
	windowdomain "Q-Solver/internal/window/domain"
	windowfake "Q-Solver/internal/window/infrastructure/simulated"
	"Q-Solver/pkg/config"
	categories "Q-Solver/pkg/domain"
	"Q-Solver/pkg/shortcut"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Event struct {
	Name string `json:"name"`
	Args []any  `json:"args"`
}
type Server struct {
	requests     sync.Mutex
	mu           sync.Mutex
	config       *config.ConfigManager
	input        *shortcut.Service
	driver       *simulated.Driver
	windows      *windowapp.Service
	windowDriver *windowfake.Driver
	commands     []string
	events       []Event
	directory    string
	history      string
}

func New(directory string) (*Server, error) {
	if directory == "" {
		return nil, errors.New("an isolated config directory is required")
	}
	s := &Server{directory: directory}
	if err := s.reset(); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *Server) Close() error { return s.input.Stop() }
func (s *Server) reset() error {
	if s.input != nil {
		if err := s.input.Stop(); err != nil {
			return err
		}
	}
	s.config = config.NewConfigManagerAt(filepath.Join(s.directory, "config"))
	if err := s.config.Replace(config.NewDefaultConfig()); err != nil {
		return err
	}
	if err := s.config.Load(); err != nil {
		return err
	}
	s.mu.Lock()
	s.commands = nil
	s.events = nil
	s.history = ""
	s.mu.Unlock()
	s.driver = &simulated.Driver{}
	s.windowDriver = &windowfake.Driver{}
	s.windows = windowapp.New(s.windowDriver)
	if err := s.windows.Register("main"); err != nil {
		return err
	}
	s.input = shortcut.NewServiceWithDriver(s, s.driver)
	cfg := s.config.Get()
	return s.input.Start(cfg.Shortcuts, cfg.KeyboardProtection)
}
func (s *Server) EmitEvent(name string, args ...interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, Event{name, args})
}
func (s *Server) command(action string) {
	s.mu.Lock()
	s.commands = append(s.commands, action)
	s.mu.Unlock()
	s.EmitEvent("harness-command", action)
}
func (s *Server) TriggerSolve()                  { s.command("solve") }
func (s *Server) TriggerScreenshot()             { s.command("screenshot") }
func (s *Server) TriggerSend()                   { s.command("send") }
func (s *Server) TriggerDeleteScreenshot()       { s.command("delete") }
func (s *Server) ToggleVisibility()              { s.command("toggle") }
func (s *Server) ToggleClickThrough()            { s.command("clickthrough") }
func (s *Server) MoveWindow(x, y int)            { s.command(fmt.Sprintf("move:%d:%d", x, y)) }
func (s *Server) ScrollContent(direction string) { s.command("scroll:" + direction) }
func (s *Server) PanicExit()                     { s.command("panic") }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !localHost(r.Host) {
		http.Error(w, "loopback host required", http.StatusForbidden)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || !localHost(parsed.Host) {
			http.Error(w, "local origin required", http.StatusForbidden)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path == "/health" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready: simulated native ports"))
		return
	}
	if r.Method != "POST" || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "JSON POST required", http.StatusMethodNotAllowed)
		return
	}
	var args []json.RawMessage
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&args); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.requests.Lock()
	defer s.requests.Unlock()
	value, err := s.call(strings.TrimPrefix(r.URL.Path, "/rpc/"), args)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(value)
}
func localHost(host string) bool {
	if value, _, err := net.SplitHostPort(host); err == nil {
		host = value
	}
	ip := net.ParseIP(host)
	return host == "localhost" || ip != nil && ip.IsLoopback()
}
func argument[T any](args []json.RawMessage, index int) (T, error) {
	var result T
	if index >= len(args) {
		return result, errors.New("missing argument")
	}
	err := json.Unmarshal(args[index], &result)
	return result, err
}
func errorText(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
func (s *Server) call(method string, args []json.RawMessage) (any, error) {
	switch method {
	case "GetSettings":
		return s.config.Get(), nil
	case "GetDomainCategories":
		return categories.GetCategories(), nil
	case "GetInitStatus":
		return "ready", nil
	case "CheckScreenCapturePermission":
		return true, nil
	case "GetScreenshotPreview":
		return s.preview(args)
	case "GetWindowStatus":
		return s.windows.Status(), nil
	case "SetCaptureProtection":
		value, err := argument[bool](args, 0)
		if err != nil {
			return nil, err
		}
		err = s.windows.Update(func(p *windowdomain.Policy) { p.CaptureProtection = value })
		s.EmitEvent("window-state", s.windows.Status())
		return errorText(err), nil
	case "SetLocalVisible":
		value, err := argument[bool](args, 0)
		if err != nil {
			return nil, err
		}
		err = s.windows.Update(func(p *windowdomain.Policy) { p.Visible = value })
		s.EmitEvent("window-state", s.windows.Status())
		return errorText(err), nil
	case "RetryWindowPolicy":
		err := s.windows.Retry()
		s.EmitEvent("window-state", s.windows.Status())
		return errorText(err), nil
	case "HarnessFailWindow":
		operation, err := argument[string](args, 0)
		if err != nil {
			return nil, err
		}
		s.windowDriver.Fail(operation)
		return nil, nil
	case "GetKeyboardProtectionStatus":
		return s.input.Status(), nil
	case "GetProtectionShortcutPreset":
		return config.GetProtectionShortcuts(), nil
	case "UpdateSettings":
		raw, err := argument[string](args, 0)
		if err != nil {
			return nil, err
		}
		cfg, err := s.config.Decode(raw)
		if err != nil {
			return err.Error(), nil
		}
		err = s.input.Configure(cfg.Shortcuts, cfg.KeyboardProtection, func() error { return s.config.Replace(cfg) })
		return errorText(err), nil
	case "SetInputEditing":
		editing, err := argument[bool](args, 0)
		if err != nil {
			return nil, err
		}
		return errorText(s.input.SetEditing(editing)), nil
	case "RetryKeyboardService":
		cfg := s.config.Get()
		return errorText(s.input.Restart(cfg.Shortcuts, cfg.KeyboardProtection)), nil
	case "RestoreFocus", "RemoveFocus":
		mode := windowdomain.Passive
		if method == "RestoreFocus" {
			mode = windowdomain.Editing
		}
		err := s.windows.Update(func(p *windowdomain.Policy) { p.InputMode = mode })
		s.EmitEvent("window-state", s.windows.Status())
		return errorText(err), nil
	case "SetWindowAlwaysOnTop":
		value, err := argument[bool](args, 0)
		if err != nil {
			return nil, err
		}
		err = s.windows.Update(func(p *windowdomain.Policy) { p.AlwaysOnTop = value })
		return errorText(err), nil
	case "LoadHistorySnapshot":
		return s.history, nil
	case "SaveHistorySnapshot":
		value, err := argument[string](args, 0)
		if err != nil {
			return nil, err
		}
		s.history = value
		return "", nil
	case "GetHistoryFilePath":
		return filepath.Join(s.directory, "history-test-only"), nil
	case "HarnessInput":
		events, err := argument[[]domain.Event](args, 0)
		if err != nil {
			return nil, err
		}
		result := make([]domain.Decision, 0, len(events))
		for _, e := range events {
			result = append(result, s.driver.Input(e))
		}
		return result, nil
	case "HarnessSnapshot":
		s.mu.Lock()
		commands := append([]string{}, s.commands...)
		s.mu.Unlock()
		return struct {
			Commands []string           `json:"commands"`
			Status   application.Status `json:"status"`
		}{commands, s.input.Status()}, nil
	case "HarnessEvents":
		s.mu.Lock()
		events := append([]Event{}, s.events...)
		s.events = nil
		s.mu.Unlock()
		return events, nil
	case "HarnessReset":
		return "", s.reset()
	case "HarnessReload":
		return "", s.config.Load()
	case "HarnessFailDriver":
		s.driver.Fail(errors.New("simulated listener interrupted"))
		return nil, nil
	case "HarnessFailSave":
		// A separate directory path makes atomic file replacement fail on all OSes.
		s.config = config.NewConfigManagerAt(s.directory)
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported harness method %s", method)
	}
}

// Temporary creates a disposable session. No path under the user's real app
// configuration is consulted, and cleanup only removes this owned directory.
func Temporary() (*Server, func(), error) {
	dir, err := os.MkdirTemp("", "q-solver-harness-")
	if err != nil {
		return nil, nil, err
	}
	s, err := New(dir)
	if err != nil {
		os.RemoveAll(dir)
		return nil, nil, err
	}
	return s, func() { s.Close(); os.RemoveAll(dir) }, nil
}
