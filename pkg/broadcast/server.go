// Package broadcast projects workflow events to a reconnectable SSE view.
package broadcast

import (
	"Q-Solver/internal/solve/domain"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

const DefaultPort = 47821

type event struct {
	Kind      string `json:"kind"`
	Content   string `json:"content"`
	Seq       uint64 `json:"seq"`
	RequestID int64  `json:"requestId"`
	Phase     string `json:"phase"`
}
type Server struct {
	lifecycle        sync.Mutex
	mu               sync.Mutex
	address          string
	port             int
	srv              *http.Server
	listener         net.Listener
	running, closed  bool
	state            event
	workflowSequence uint64
	subs             map[chan event]struct{}
}

// NewServer preserves the existing opt-in LAN viewer address. Development uses
// Handler on its loopback server and never calls Start.
func NewServer(port int) *Server {
	if port <= 0 {
		port = DefaultPort
	}
	return NewServerAt(fmt.Sprintf(":%d", port))
}
func NewServerAt(address string) *Server {
	return &Server{address: address, state: event{Kind: "snapshot", Phase: "idle"}, subs: make(map[chan event]struct{})}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /stream", s.handleStream)
	mux.HandleFunc("GET /current", s.handleCurrent)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") })
	return mux
}
func (s *Server) Start() error {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return nil
	}
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return err
	}
	s.port = listener.Addr().(*net.TCPAddr).Port
	s.listener = listener
	s.srv = &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	s.running = true
	s.closed = false
	go s.srv.Serve(listener)
	return nil
}
func (s *Server) Stop() {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	s.mu.Lock()
	s.closed = true
	s.running = false
	for ch := range s.subs {
		close(ch)
		delete(s.subs, ch)
	}
	srv := s.srv
	listener := s.listener
	s.mu.Unlock()
	if listener != nil {
		_ = listener.Close()
	}
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			_ = srv.Close()
		}
	}
}
func (s *Server) Port() int { s.mu.Lock(); defer s.mu.Unlock(); return s.port }
func (s *Server) LocalAddresses() []string {
	s.mu.Lock()
	port, running := s.port, s.running
	s.mu.Unlock()
	if !running {
		return nil
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var result []string
	for _, i := range interfaces {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := i.Addrs()
		for _, address := range addresses {
			ip, ok := address.(*net.IPNet)
			if !ok || ip.IP.IsLoopback() || ip.IP.To4() == nil {
				continue
			}
			result = append(result, fmt.Sprintf("http://%s:%d/", ip.IP.String(), port))
		}
	}
	return result
}

// Publish is a bounded, nonblocking observer. Mutation and enqueue share a lock,
// so a slow client cannot reorder results or stall the producer.
func (s *Server) Publish(input domain.Event) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if input.Sequence <= s.workflowSequence {
		return
	}
	s.workflowSequence = input.Sequence
	if input.Kind == "started" {
		if input.RequestID <= s.state.RequestID {
			return
		}
		s.state.RequestID = input.RequestID
		s.state.Content = ""
		s.state.Phase = "capturing"
	} else {
		if input.RequestID != s.state.RequestID || domain.Terminal(s.state.Phase) {
			return
		}
		switch input.Kind {
		case "ready":
			s.state.Phase = "generating"
		case "chunk":
			s.state.Content += input.Text
		case "completed":
			s.state.Content = input.Text
			s.state.Phase = "completed"
		case "failed", "cancelled":
			s.state.Phase = input.Kind
		default:
			return
		}
	}
	s.state.Seq++
	ev := event{Kind: input.Kind, Content: input.Text, Seq: s.state.Seq, RequestID: s.state.RequestID, Phase: s.state.Phase}
	if input.Kind == "started" {
		ev.Content = ""
	}
	s.enqueue(ev)
}
func (s *Server) Clear() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Content = ""
	s.state.Phase = "cancelled"
	s.state.Seq++
	s.enqueue(s.snapshot())
}
func (s *Server) snapshot() event { state := s.state; state.Kind = "snapshot"; return state }
func (s *Server) enqueue(ev event) {
	for ch := range s.subs {
		select {
		case ch <- ev:
		default:
			// Replace an incomplete delta backlog with a complete current snapshot.
		drain:
			for {
				select {
				case <-ch:
				default:
					break drain
				}
			}
			ch <- s.snapshot()
		}
	}
}
func (s *Server) subscribe() chan event {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan event, 16)
	if s.closed {
		close(ch)
		return ch
	}
	s.subs[ch] = struct{}{}
	ch <- s.snapshot() // Atomic with registration: no snapshot/subscription gap.
	return ch
}
func (s *Server) unsubscribe(ch chan event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subs[ch]; ok {
		delete(s.subs, ch)
		close(ch)
	}
}
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, indexHTML)
}
func (s *Server) handleCurrent(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	snapshot := s.snapshot()
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(snapshot)
}
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	ch := s.subscribe()
	defer s.unsubscribe(ch)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(ev)
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
