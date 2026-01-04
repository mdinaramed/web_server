package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"
)

type Server struct {
	mu         sync.Mutex
	data       map[string]string
	requests   int
	shutdownCh chan struct{}
}

func NewServer() *Server {
	return &Server{
		data:       make(map[string]string),
		shutdownCh: make(chan struct{}),
	}
}

func (s *Server) incRequests() {
	s.mu.Lock()
	s.requests++
	s.mu.Unlock()
}

// Handle POST /data
func (s *Server) postDataHandler(w http.ResponseWriter, r *http.Request) {
	s.incRequests()

	var payload map[string]string
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		http.Error(w, "failed json", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	for k, v := range payload {
		s.data[k] = v
	}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Handle GET /data
func (s *Server) getDataHandler(w http.ResponseWriter, r *http.Request) {
	s.incRequests()

	s.mu.Lock()
	copyData := make(map[string]string)
	for k, v := range s.data {
		copyData[k] = v
	}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(copyData)
}

// Handle DELETE /data/{key}
func (s *Server) deleteDataHandler(w http.ResponseWriter, r *http.Request) {
	s.incRequests()

	key := r.PathValue("key")
	if key == "" {
		http.Error(w, "missing key", http.StatusNotFound)
		return
	}
	s.mu.Lock()
	_, exists := s.data[key]
	if exists {
		delete(s.data, key)
	}
	s.mu.Unlock()
	if !exists {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"deleted": key})
}

// Handle GET /stats
func (s *Server) statsHandler(w http.ResponseWriter, r *http.Request) {
	s.incRequests()

	s.mu.Lock()
	req := s.requests
	size := len(s.data)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"requests": req,
		"db_size":  size,
	})
}

// Background worker to log server status
func (s *Server) startBackgroundWorker() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			req := s.requests
			size := len(s.data)
			s.mu.Unlock()
			fmt.Printf("Current requests: %d, Database size: %d\n", req, size)
		case <-s.shutdownCh:
			fmt.Println("Worker stopped")
			return
		}
	}
}

func main() {
	server := NewServer()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /data", server.postDataHandler)
	mux.HandleFunc("GET /data", server.getDataHandler)
	mux.HandleFunc("DELETE /data/{key}", server.deleteDataHandler)
	mux.HandleFunc("GET /stats", server.statsHandler)

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go server.startBackgroundWorker()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	go func() {
		<-stop
		fmt.Println("\nShutting down server...")
		close(server.shutdownCh)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
	}()

	fmt.Println("Server starting on :8080")
	err := httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		fmt.Println("Server error:", err)
	}
	fmt.Println("Server stopped")
}
