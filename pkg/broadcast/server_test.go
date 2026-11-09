package broadcast

import (
	"Q-Solver/internal/solve/domain"
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectionRejectsStaleRequestsAndLateChunks(t *testing.T) {
	s := NewServerAt("127.0.0.1:0")
	inputs := []domain.Event{{Kind: "started", RequestID: 1, Sequence: 1}, {Kind: "chunk", RequestID: 1, Sequence: 2, Text: "old"}, {Kind: "started", RequestID: 2, Sequence: 3}, {Kind: "chunk", RequestID: 1, Sequence: 4, Text: "STALE"}, {Kind: "chunk", RequestID: 2, Sequence: 5, Text: "draft"}, {Kind: "completed", RequestID: 2, Sequence: 6, Text: "final"}, {Kind: "chunk", RequestID: 2, Sequence: 7, Text: "LATE"}}
	for _, input := range inputs {
		s.Publish(input)
	}
	if s.state.RequestID != 2 || s.state.Content != "final" || s.state.Phase != "completed" {
		t.Fatal(s.state)
	}
	s.Clear()
	s.Publish(domain.Event{Kind: "chunk", RequestID: 2, Sequence: 8, Text: "again"})
	if s.state.Content != "" {
		t.Fatal("clear resurrected old answer")
	}
}
func TestSlowSubscriberGetsCompleteRecoverySnapshot(t *testing.T) {
	s := NewServerAt("127.0.0.1:0")
	ch := s.subscribe()
	defer s.unsubscribe(ch)
	s.Publish(domain.Event{Kind: "started", RequestID: 1, Sequence: 1})
	for i := uint64(2); i < 100; i++ {
		s.Publish(domain.Event{Kind: "chunk", RequestID: 1, Sequence: i, Text: "x"})
	}
	text := ""
	var last uint64
	for len(ch) > 0 {
		ev := <-ch
		if ev.Seq < last {
			t.Fatal("reordered", ev)
		}
		last = ev.Seq
		if ev.Kind == "snapshot" {
			text = ev.Content
		} else if ev.Kind == "chunk" {
			text += ev.Content
		}
	}
	if text != strings.Repeat("x", 98) {
		t.Fatal("dropped content", len(text))
	}
}
func TestHTTPReconnectAndStop(t *testing.T) {
	s := NewServerAt("127.0.0.1:0")
	server := httptest.NewServer(s.Handler())
	defer server.Close()
	s.Publish(domain.Event{Kind: "started", RequestID: 7, Sequence: 1})
	s.Publish(domain.Event{Kind: "chunk", RequestID: 7, Sequence: 2, Text: "partial"})
	response, err := http.Get(server.URL + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	var ev event
	if err = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Kind != "snapshot" || ev.RequestID != 7 || ev.Content != "partial" || ev.Phase != "capturing" {
		t.Fatal(ev)
	}
	s.Stop()
	for {
		if _, err = reader.ReadString('\n'); err != nil {
			break
		}
	}
}
func TestStartReportsBindFailureAndCanRestart(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := NewServerAt(listener.Addr().String())
	if err := s.Start(); err == nil {
		s.Stop()
		t.Fatal("occupied port reported success")
	}
	if len(s.LocalAddresses()) != 0 {
		t.Fatal("failed service advertised addresses")
	}
	listener.Close()
	if err = s.Start(); err != nil {
		t.Fatal(err)
	}
	s.Stop()
	if err = s.Start(); err != nil {
		t.Fatal(err)
	}
	s.Stop()
}
