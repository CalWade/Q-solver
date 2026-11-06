package main

import (
	"Q-Solver/internal/harness"
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	address := flag.String("addr", "127.0.0.1:4317", "loopback address")
	flag.Parse()
	host, _, err := net.SplitHostPort(*address)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		log.Fatal("harness must listen on a loopback IP")
	}
	handler, cleanup, err := harness.Temporary()
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()
	server := &http.Server{Addr: *address, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	log.Printf("isolated harness at http://%s (simulated OS ports)", *address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Print(err)
		os.Exit(1)
	}
}
