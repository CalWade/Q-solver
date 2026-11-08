package harness

import (
	captureapp "Q-Solver/internal/capture/application"
	capturedomain "Q-Solver/internal/capture/domain"
	captureinfra "Q-Solver/internal/capture/infrastructure"
	solveapp "Q-Solver/internal/solve/application"
	"Q-Solver/internal/solve/domain"
	windowapp "Q-Solver/internal/window/application"
	"Q-Solver/pkg/broadcast"
	"Q-Solver/pkg/solution"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sync"
)

type fixtureCapture struct{ windows *windowapp.Service }

func (c fixtureCapture) Capture(ctx context.Context, options capturedomain.Options) (string, error) {
	result, err := captureapp.New(fixtureScreen{}, captureinfra.Processor{}, c.windows).Capture(ctx, options)
	if err != nil {
		return "", err
	}
	return "data:" + result.MIME + ";base64," + base64.StdEncoding.EncodeToString(result.Bytes), nil
}

type fixtureReply struct {
	result domain.Result
	err    error
}
type fixtureCall struct {
	request domain.Request
	emit    func(domain.Chunk)
	done    chan fixtureReply
}
type fixtureModel struct {
	mu    sync.Mutex
	calls map[int64]fixtureCall
}

func (m *fixtureModel) Generate(ctx context.Context, request domain.Request, emit func(domain.Chunk)) (domain.Result, error) {
	call := fixtureCall{request: request, emit: emit, done: make(chan fixtureReply, 1)}
	m.mu.Lock()
	m.calls[request.ID] = call
	// Retain a few callbacks so tests can deliberately deliver cancelled output.
	for id := range m.calls {
		if id < request.ID-8 {
			delete(m.calls, id)
		}
	}
	m.mu.Unlock()
	select {
	case <-ctx.Done():
		return domain.Result{}, ctx.Err()
	case reply := <-call.done:
		return reply.result, reply.err
	}
}
func (m *fixtureModel) get(id int64) (fixtureCall, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	call, ok := m.calls[id]
	if !ok {
		return fixtureCall{}, errors.New("fixture model has not received this request")
	}
	return call, nil
}

type fixtureObserver struct {
	server    *Server
	broadcast *broadcast.Server
}

func (o fixtureObserver) Publish(event domain.Event) {
	o.server.EmitEvent("workflow-event", event)
	o.broadcast.Publish(event)
}

func (s *Server) initWorkflow() {
	s.broadcast = broadcast.NewServerAt("127.0.0.1:0") // Handler only; no extra listener.
	s.model = &fixtureModel{calls: make(map[int64]fixtureCall)}
	s.workflow = solveapp.New(context.Background(), solution.SettingsSource{Manager: s.config}, fixtureCapture{s.windows}, s.model, fixtureObserver{s, s.broadcast})
}
func (s *Server) workflowCall(method string, args []json.RawMessage) (any, error) {
	switch method {
	case "GetWorkflowStatus":
		return s.workflow.Status(), nil
	case "TriggerScreenshot":
		s.TriggerScreenshot()
		return nil, nil
	case "TriggerSend", "TriggerSolve":
		s.TriggerSend()
		return nil, nil
	case "RemoveScreenshot":
		index, err := argument[int](args, 0)
		if err != nil {
			return nil, err
		}
		s.workflow.Remove(index)
		return nil, nil
	case "ClearScreenshots":
		s.workflow.ClearScreenshots()
		return nil, nil
	case "CancelRunningTask":
		return s.workflow.CancelCurrent(), nil
	case "CancelRequest":
		id, err := argument[int64](args, 0)
		if err != nil {
			return nil, err
		}
		return s.workflow.Cancel(id), nil
	case "HarnessModelSnapshot":
		type summary struct {
			ID      int64  `json:"id"`
			Model   string `json:"model"`
			Images  int    `json:"images"`
			History int    `json:"history"`
		}
		s.model.mu.Lock()
		defer s.model.mu.Unlock()
		result := []summary{}
		for _, call := range s.model.calls {
			r := call.request
			result = append(result, summary{r.ID, r.Settings.Model, len(r.Images), len(r.History)})
		}
		return result, nil
	case "HarnessModelChunk", "HarnessModelFinish", "HarnessModelFail":
		id, err := argument[int64](args, 0)
		if err != nil {
			return nil, err
		}
		text, err := argument[string](args, 1)
		if err != nil {
			return nil, err
		}
		call, err := s.model.get(id)
		if err != nil {
			return nil, err
		}
		if method == "HarnessModelChunk" {
			thinking := false
			if len(args) > 2 {
				thinking, err = argument[bool](args, 2)
				if err != nil {
					return nil, err
				}
			}
			call.emit(domain.Chunk{Text: text, Thinking: thinking})
			return nil, nil
		}
		reply := fixtureReply{result: domain.Result{Content: text}}
		if method == "HarnessModelFail" {
			reply.err = errors.New(text)
		}
		select {
		case call.done <- reply:
			return nil, nil
		default:
			return nil, errors.New("fixture already finished")
		}
	}
	return nil, errors.New("unsupported workflow method")
}
