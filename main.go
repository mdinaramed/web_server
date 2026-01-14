package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	s.requests++
}

func (s *Server) postDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload map[string]string
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	for k, v := range payload {
		s.data[k] = v
	}
	s.incRequests()
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) getDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	s.incRequests()
	copyData := make(map[string]string)
	for k, v := range s.data {
		copyData[k] = v
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(copyData)
}

func (s *Server) deleteDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 || parts[3] == "" {
		http.Error(w, "Key not specified", http.StatusBadRequest)
		return
	}
	key := parts[3]

	s.mu.Lock()
	s.incRequests()
	_, ok := s.data[key]
	if ok {
		delete(s.data, key)
	}
	s.mu.Unlock()

	if !ok {
		http.Error(w, "Key not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"deleted": key})
}

func (s *Server) statsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	s.incRequests()
	stats := map[string]int{
		"total_requests": s.requests,
		"db_size":        len(s.data),
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) startBackgroundWorker() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			fmt.Printf("Current Requests: %d, Database size: %d\n", s.requests, len(s.data))
			s.mu.Unlock()
		case <-s.shutdownCh:
			fmt.Println("Worker Stopped")
			return
		}
	}
}

func main() {
	server := NewServer()
	mux := http.NewServeMux()

	mux.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))

	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			server.postDataHandler(w, r)
			return
		}
		if r.Method == http.MethodGet {
			server.getDataHandler(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/data/", server.deleteDataHandler)
	mux.HandleFunc("/api/stats", server.statsHandler)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/index":
			http.ServeFile(w, r, "views/index.html")
		case "/data":
			http.ServeFile(w, r, "views/data.html")
		case "/stats":
			http.ServeFile(w, r, "views/stats.html")
		default:
			http.NotFound(w, r)
		}
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go server.startBackgroundWorker()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	go func() {
		fmt.Println("Server started at http://localhost:8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("Server error:", err)
		}
	}()

	<-stop
	fmt.Println("\nShutting down server...")
	close(server.shutdownCh)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)

	fmt.Println("Server exited properly")
}
