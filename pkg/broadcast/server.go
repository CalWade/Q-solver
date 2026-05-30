// Package broadcast 提供一个本地 HTTP server，用来把 LLM 答案实时推给局域网内的其他设备（如 iPad）。
//
// 设计目标：
//   - 零侵入核心隐身逻辑：只监听本地端口，不触碰任何 platform/window/shortcut 模块。
//   - SSE（Server-Sent Events）流式推送，iPad Safari 原生支持。
//   - 当前答案在内存中始终保留，新客户端连上后立即收到当前状态（断线重连友好）。
//   - 支持流式拼接 chunk、最终整段替换、手动清空三种写入模式。
//
// 安全：目前按用户选择不加认证（方案 A），监听 0.0.0.0 允许局域网访问。
// 如需升级安全级别，可在 NewServer 中加一个 token 参数并在 handler 里校验。
package broadcast

import (
	"Q-Solver/pkg/logger"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// DefaultPort 是 broadcaster 默认监听端口。
// 选一个不太容易冲突的端口（5 位数、非常见服务端口）。
const DefaultPort = 47821

// event 是推给 SSE 客户端的一条事件。
type event struct {
	Kind    string `json:"kind"`    // "start" | "chunk" | "final" | "clear" | "ping"
	Content string `json:"content"` // chunk 内容 或 final 整段 或 空
	Seq     int    `json:"seq"`     // 递增序号，便于前端判定乱序/丢包
}

// Server 是 broadcaster 的核心。线程安全。
type Server struct {
	port int
	srv  *http.Server

	mu          sync.RWMutex
	currentText string // 当前答案的累积内容（最终完成后等于 final 整段）
	finalized   bool   // 当前答案是否已完成
	seq         int

	subs  map[chan event]struct{}
	subMu sync.Mutex

	running bool
}

// NewServer 创建但不启动 server。port=0 时使用默认端口。
func NewServer(port int) *Server {
	if port <= 0 {
		port = DefaultPort
	}
	return &Server{
		port: port,
		subs: make(map[chan event]struct{}),
	}
}

// Start 启动 HTTP server（异步）。多次调用无副作用。
func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/stream", s.handleStream)
	mux.HandleFunc("/current", s.handleCurrent)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	s.srv = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Printf("[broadcast] HTTP server 监听端口 %d (局域网访问: http://<本机IP>:%d/)", s.port, s.port)
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Printf("[broadcast] HTTP server 错误: %v", err)
		}
	}()

	// 定期发送 ping 保活（部分代理/浏览器超过一定时间无数据会断开 SSE）
	go s.pingLoop()

	return nil
}

// Stop 优雅关闭 server 和所有 SSE 客户端。
func (s *Server) Stop() {
	s.mu.Lock()
	running := s.running
	s.running = false
	srv := s.srv
	s.mu.Unlock()

	if !running || srv == nil {
		return
	}

	// 关闭所有订阅通道
	s.subMu.Lock()
	for ch := range s.subs {
		close(ch)
	}
	s.subs = make(map[chan event]struct{})
	s.subMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	logger.Println("[broadcast] HTTP server 已停止")
}

// LocalAddresses 返回当前机器上可能的局域网 IP 列表（供前端显示给用户）。
func (s *Server) LocalAddresses() []string {
	var out []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil {
				continue
			}
			out = append(out, fmt.Sprintf("http://%s:%d/", ip4.String(), s.port))
		}
	}
	return out
}

// Port 返回监听端口。
func (s *Server) Port() int {
	return s.port
}

// ---- 写入 API：由 solution 层在流式回调中调用 ----

// StartNewAnswer 开始一段新答案，重置累积缓冲。
func (s *Server) StartNewAnswer() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.currentText = ""
	s.finalized = false
	s.seq++
	seq := s.seq
	s.mu.Unlock()
	s.broadcast(event{Kind: "start", Seq: seq})
}

// AppendChunk 追加一段流式增量内容。
func (s *Server) AppendChunk(chunk string) {
	if s == nil || chunk == "" {
		return
	}
	s.mu.Lock()
	s.currentText += chunk
	s.seq++
	seq := s.seq
	s.mu.Unlock()
	s.broadcast(event{Kind: "chunk", Content: chunk, Seq: seq})
}

// FinalizeAnswer 用最终整段替换当前答案并标记完成。
// 对于 LLM 返回和流式累积不一致的情况（某些 provider 会出现），以 final 为准。
func (s *Server) FinalizeAnswer(full string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.currentText = full
	s.finalized = true
	s.seq++
	seq := s.seq
	s.mu.Unlock()
	s.broadcast(event{Kind: "final", Content: full, Seq: seq})
}

// Clear 清空当前答案（用于 panic 退出清理）。
func (s *Server) Clear() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.currentText = ""
	s.finalized = false
	s.seq++
	seq := s.seq
	s.mu.Unlock()
	s.broadcast(event{Kind: "clear", Seq: seq})
}

// ---- 内部：订阅和分发 ----

func (s *Server) subscribe() chan event {
	ch := make(chan event, 64)
	s.subMu.Lock()
	s.subs[ch] = struct{}{}
	s.subMu.Unlock()
	return ch
}

func (s *Server) unsubscribe(ch chan event) {
	s.subMu.Lock()
	if _, ok := s.subs[ch]; ok {
		delete(s.subs, ch)
		close(ch)
	}
	s.subMu.Unlock()
}

func (s *Server) broadcast(ev event) {
	s.subMu.Lock()
	for ch := range s.subs {
		// 非阻塞：缓冲满了就丢弃（宁可让慢速客户端丢数据也别卡主线）
		select {
		case ch <- ev:
		default:
		}
	}
	s.subMu.Unlock()
}

func (s *Server) pingLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.RLock()
		running := s.running
		s.mu.RUnlock()
		if !running {
			return
		}
		s.broadcast(event{Kind: "ping"})
	}
}

// ---- HTTP Handlers ----

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(indexHTML))
}

func (s *Server) handleCurrent(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	payload := struct {
		Content   string `json:"content"`
		Finalized bool   `json:"finalized"`
		Seq       int    `json:"seq"`
	}{s.currentText, s.finalized, s.seq}
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // 防止反向代理缓冲

	// 首帧：把当前累积内容立刻推给新客户端
	s.mu.RLock()
	initEv := event{
		Kind:    "final",
		Content: s.currentText,
		Seq:     s.seq,
	}
	if !s.finalized {
		initEv.Kind = "final" // 仍发 final 类型让前端直接替换，避免 chunk 拼接错误
	}
	s.mu.RUnlock()
	writeSSE(w, initEv)
	flusher.Flush()

	ch := s.subscribe()
	defer s.unsubscribe(ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(w, ev)
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, ev event) {
	b, _ := json.Marshal(ev)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
}
