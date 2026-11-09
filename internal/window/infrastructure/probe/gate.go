package probe

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

type acknowledgement struct {
	stage string
	err   error
}

// StageGate keeps the native state fixed while an independent receiver observes
// it. EOF, a wrong acknowledgement or a deadline ends the probe, not the wait.
type StageGate struct {
	reader io.ReadCloser
	writer io.Writer
	lines  chan acknowledgement
	done   chan struct{}
	once   sync.Once
}

func NewStageGate(reader io.ReadCloser, writer io.Writer) *StageGate {
	g := &StageGate{reader: reader, writer: writer, lines: make(chan acknowledgement, 1), done: make(chan struct{})}
	go func() {
		defer close(g.lines)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 256), 1024)
		for scanner.Scan() {
			select {
			case g.lines <- acknowledgement{stage: scanner.Text()}:
			case <-g.done:
				return
			}
		}
		err := scanner.Err()
		if err == nil {
			err = io.EOF
		}
		select {
		case g.lines <- acknowledgement{err: err}:
		case <-g.done:
		}
	}()
	return g
}

func (g *StageGate) Wait(ctx context.Context, stage string) error {
	select {
	case <-g.done:
		return fmt.Errorf("native observer closed")
	default:
	}
	if err := json.NewEncoder(g.writer).Encode(map[string]string{"type": "native-stage", "stage": stage}); err != nil {
		return fmt.Errorf("publish native stage: %w", err)
	}
	select {
	case line, open := <-g.lines:
		if !open || line.err != nil {
			return fmt.Errorf("native observer disconnected at %s", stage)
		}
		if line.stage != stage {
			return fmt.Errorf("native observer acknowledged the wrong stage, expected %s", stage)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("native observer at %s: %w", stage, ctx.Err())
	case <-g.done:
		return fmt.Errorf("native observer closed")
	}
}

func (g *StageGate) Close() {
	g.once.Do(func() { close(g.done); g.reader.Close() })
}
