// Package server implements the HTTP server with WebSocket live reload.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/fsnotify/fsnotify"
	"github.com/mschulkind-oss/waxon/internal/format"
	"github.com/mschulkind-oss/waxon/internal/render"
)

// Config holds server configuration.
type Config struct {
	File          string
	Port          string
	Bind          string
	ThemeOverride string
	NoOpen        bool
	Logger        *log.Logger
}

// Server serves slides with live reload.
type Server struct {
	cfg     Config
	mu      sync.RWMutex
	deck    *format.Deck
	html    string
	clients map[chan struct{}]struct{}
	logger  *log.Logger
}

// New creates a new Server.
func New(cfg Config) (*Server, error) {
	if cfg.Port == "" {
		cfg.Port = getPort()
	}
	if cfg.Bind == "" {
		cfg.Bind = "0.0.0.0"
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(os.Stderr, "[waxon] ", log.LstdFlags)
	}

	s := &Server{
		cfg:     cfg,
		clients: make(map[chan struct{}]struct{}),
		logger:  cfg.Logger,
	}

	if err := s.reload(); err != nil {
		return nil, fmt.Errorf("initial load: %w", err)
	}

	return s, nil
}

func getPort() string {
	if p := os.Getenv("SM_PORT"); p != "" {
		return p
	}
	return "8080"
}

// reload reads the .slides file and re-renders HTML.
func (s *Server) reload() error {
	data, err := os.ReadFile(s.cfg.File)
	if err != nil {
		return fmt.Errorf("read %s: %w", s.cfg.File, err)
	}

	deck, err := format.Parse(string(data))
	if err != nil {
		return fmt.Errorf("parse %s: %w", s.cfg.File, err)
	}

	html, err := render.RenderHTML(deck, render.Options{
		ThemeOverride: s.cfg.ThemeOverride,
	})
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	s.mu.Lock()
	s.deck = deck
	s.html = html
	s.mu.Unlock()

	return nil
}

// notifyClients sends a reload signal to all WebSocket clients.
func (s *Server) notifyClients() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for ch := range s.clients {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// addClient registers a new WebSocket client.
func (s *Server) addClient(ch chan struct{}) {
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	s.mu.Unlock()
}

// removeClient unregisters a WebSocket client.
func (s *Server) removeClient(ch chan struct{}) {
	s.mu.Lock()
	delete(s.clients, ch)
	s.mu.Unlock()
}

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleSlides)
	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/api/context", s.handleAgentContext)
	return mux
}

func (s *Server) handleSlides(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	s.mu.RLock()
	html := s.html
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		s.logger.Printf("ws accept: %v", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ch := make(chan struct{}, 1)
	s.addClient(ch)
	defer s.removeClient(ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			err := conn.Write(ctx, websocket.MessageText, []byte("reload"))
			if err != nil {
				return
			}
		}
	}
}

func (s *Server) handleAgentContext(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	deck := s.deck
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(deck); err != nil {
		s.logger.Printf("agent-context encode: %v", err)
	}
}

// Watch starts watching the file for changes and triggers reload + notify.
func (s *Server) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer watcher.Close()

	// Watch the parent directory (more reliable than watching the file directly)
	dir := filepath.Dir(s.cfg.File)
	if err := watcher.Add(dir); err != nil {
		return fmt.Errorf("watch %s: %w", dir, err)
	}

	base := filepath.Base(s.cfg.File)
	var debounce <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if filepath.Base(event.Name) == base && (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) {
				debounce = time.After(100 * time.Millisecond)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			s.logger.Printf("watch error: %v", err)
		case <-debounce:
			if err := s.reload(); err != nil {
				s.logger.Printf("reload error: %v", err)
			} else {
				s.logger.Printf("reloaded %s", s.cfg.File)
				s.notifyClients()
			}
		}
	}
}

// ListenAndServe starts the server and file watcher.
func (s *Server) ListenAndServe(ctx context.Context) error {
	addr := s.cfg.Bind + ":" + s.cfg.Port
	srv := &http.Server{
		Addr:    addr,
		Handler: s.Handler(),
	}

	// Start file watcher in background
	go func() {
		if err := s.Watch(ctx); err != nil {
			s.logger.Printf("watcher: %v", err)
		}
	}()

	s.logger.Printf("serving %s at http://%s", s.cfg.File, addr)

	// Shutdown on context cancel
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// GetDeck returns the current deck (for agent-context).
func (s *Server) GetDeck() *format.Deck {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deck
}
