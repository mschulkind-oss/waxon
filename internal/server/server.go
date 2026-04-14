// Package server implements the HTTP server with WebSocket live reload.
//
// The server has two modes:
//
//   - Single-file: Config.File points at a .slides file. The server serves
//     just that one deck and `/` renders it directly.
//   - Directory:   Config.File points at a directory. The server scans for
//     *.slides files recursively, serves a deck index at `/`, and serves
//     individual decks at `/d/<relative-path>`.
//
// Both modes share the same set of routes and the same WebSocket reload
// mechanism. Reload notifications are scoped per deck path so editing one
// deck only refreshes browsers viewing that deck.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/fsnotify/fsnotify"
	"github.com/mschulkind-oss/waxon/internal/format"
	"github.com/mschulkind-oss/waxon/internal/render"
)

// Config holds server configuration.
type Config struct {
	// File is either a .slides file or a directory containing them.
	File          string
	Port          string
	Bind          string
	ThemeOverride string
	NoOpen        bool
	Logger        *log.Logger
}

// Server serves slides with live reload.
//
// The legacy fields (deck, html) are kept so existing tests and the
// agent-context handler keep working. In single-file mode they point at the
// only deck; in directory mode they track the most-recently-loaded deck so
// `s.GetDeck()` is never nil.
type Server struct {
	cfg     Config
	rootDir string // absolute directory we watch
	isDir   bool   // true when Config.File was a directory
	soloRel string // single-file mode: relative path of the lone deck

	mu    sync.RWMutex
	decks map[string]*deckEntry // key = relative path under rootDir

	// Legacy "current" pointers — populated for back-compat with tests and
	// existing handlers (agent-context).
	deck *format.Deck
	html string

	// Per-deck client sets for scoped websocket reloads. The empty key ""
	// holds clients that connected without a ?path filter and so receive
	// every reload.
	clientsMu sync.Mutex
	clients   map[string]map[chan struct{}]struct{}

	// Per-deck write locks serialize comment-post writes against a single
	// .slides file so two browsers commenting simultaneously can't race
	// AddComment's read-modify-write. Keyed by relative deck path. Locks
	// are created lazily and never garbage-collected (decks are typically
	// few and long-lived).
	writeLocksMu sync.Mutex
	writeLocks   map[string]*sync.Mutex

	logger *log.Logger
}

// deckWriteLock returns the per-deck write mutex, creating it on first use.
func (s *Server) deckWriteLock(relPath string) *sync.Mutex {
	s.writeLocksMu.Lock()
	defer s.writeLocksMu.Unlock()
	if s.writeLocks == nil {
		s.writeLocks = make(map[string]*sync.Mutex)
	}
	if l, ok := s.writeLocks[relPath]; ok {
		return l
	}
	l := &sync.Mutex{}
	s.writeLocks[relPath] = l
	return l
}

// deckEntry caches a parsed + rendered deck.
type deckEntry struct {
	relPath string
	absPath string
	deck    *format.Deck
	html    string
	title   string
}

// New creates a new Server. The Config.File path may point at a .slides
// file or a directory containing .slides files.
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

	abs, err := filepath.Abs(cfg.File)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", cfg.File, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", cfg.File, err)
	}

	s := &Server{
		cfg:     cfg,
		clients: make(map[string]map[chan struct{}]struct{}),
		decks:   make(map[string]*deckEntry),
		logger:  cfg.Logger,
	}

	if info.IsDir() {
		s.isDir = true
		s.rootDir = abs
	} else {
		s.rootDir = filepath.Dir(abs)
		s.soloRel = filepath.Base(abs)
	}

	if err := s.reload(); err != nil {
		return nil, fmt.Errorf("initial load: %w", err)
	}

	return s, nil
}

func getPort() string {
	if p := os.Getenv("WK_PORT"); p != "" {
		return p
	}
	return "8080"
}

// reload re-scans the root directory (or just the single file) and re-renders
// every deck. It is safe to call repeatedly; failures on individual decks are
// logged but do not abort the whole reload — except in single-file mode,
// where a parse error on the lone deck is returned to the caller.
func (s *Server) reload() error {
	paths, err := s.discoverDecks()
	if err != nil {
		return err
	}

	newDecks := make(map[string]*deckEntry, len(paths))
	var firstErr error

	for _, rel := range paths {
		entry, err := s.loadDeck(rel)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			s.logger.Printf("load %s: %v", rel, err)
			continue
		}
		newDecks[rel] = entry
	}

	// In single-file mode any error must be surfaced — there's only one
	// deck and the caller (likely tests or the CLI) needs to know.
	if !s.isDir && firstErr != nil {
		return firstErr
	}

	// Re-render now that we know the full deck list, so each page embeds
	// the up-to-date sibling list in the deck switcher.
	summaries := s.summariesFromMap(newDecks)
	for rel, entry := range newDecks {
		html, err := render.RenderHTML(entry.deck, render.Options{
			ThemeOverride: s.cfg.ThemeOverride,
			DeckPath:      rel,
			Decks:         summaries,
			DeckDir:       filepath.Dir(entry.absPath),
		})
		if err != nil {
			s.logger.Printf("render %s: %v", rel, err)
			continue
		}
		entry.html = html
	}

	s.mu.Lock()
	s.decks = newDecks
	// Pick a "current" deck for legacy fields. Prefer the single-file
	// target if any, else the alphabetically first deck.
	var picked *deckEntry
	if s.soloRel != "" {
		picked = newDecks[s.soloRel]
	}
	if picked == nil {
		for _, e := range newDecks {
			if picked == nil || e.relPath < picked.relPath {
				picked = e
			}
		}
	}
	if picked != nil {
		s.deck = picked.deck
		s.html = picked.html
	} else {
		s.deck = nil
		s.html = ""
	}
	s.mu.Unlock()

	return nil
}

// reloadDeck refreshes a single deck (used after a comment POST).
func (s *Server) reloadDeck(relPath string) error {
	relPath = filepath.ToSlash(relPath)
	entry, err := s.loadDeck(relPath)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.decks[relPath] = entry
	summaries := s.summariesFromMap(s.decks)
	s.mu.Unlock()

	html, err := render.RenderHTML(entry.deck, render.Options{
		ThemeOverride: s.cfg.ThemeOverride,
		DeckPath:      relPath,
		Decks:         summaries,
		DeckDir:       filepath.Dir(entry.absPath),
	})
	if err != nil {
		return fmt.Errorf("render %s: %w", relPath, err)
	}

	s.mu.Lock()
	entry.html = html
	if relPath == s.soloRel || s.deck == nil {
		s.deck = entry.deck
		s.html = entry.html
	}
	s.mu.Unlock()

	return nil
}

// discoverDecks returns relative paths (slash-separated) of all *.slides
// files under the configured root, or just the single file in solo mode.
func (s *Server) discoverDecks() ([]string, error) {
	if !s.isDir {
		return []string{s.soloRel}, nil
	}
	var out []string
	err := filepath.WalkDir(s.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip dot dirs to avoid walking .git etc.
			name := d.Name()
			if path != s.rootDir && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(d.Name()) != ".slides" {
			return nil
		}
		rel, err := filepath.Rel(s.rootDir, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", s.rootDir, err)
	}
	return out, nil
}

// loadDeck reads + parses one .slides file by its relative path.
func (s *Server) loadDeck(relPath string) (*deckEntry, error) {
	abs, err := s.resolveAbs(relPath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", relPath, err)
	}
	deck, err := format.Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", relPath, err)
	}
	title := deck.Meta.Title
	if title == "" {
		title = relPath
	}
	return &deckEntry{
		relPath: relPath,
		absPath: abs,
		deck:    deck,
		title:   title,
	}, nil
}

// resolveAbs joins the relative path against rootDir and verifies that the
// result stays inside it (i.e. defends against `../` traversal).
func (s *Server) resolveAbs(relPath string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(relPath))
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("path must be relative: %s", relPath)
	}
	abs := filepath.Join(s.rootDir, clean)
	rel, err := filepath.Rel(s.rootDir, abs)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") || rel == ".." {
		return "", fmt.Errorf("path escapes root: %s", relPath)
	}
	return abs, nil
}

func (s *Server) summariesFromMap(m map[string]*deckEntry) []render.DeckSummary {
	if !s.isDir {
		return nil
	}
	out := make([]render.DeckSummary, 0, len(m))
	for _, e := range m {
		out = append(out, summaryFromEntry(e))
	}
	// Sort for stable display.
	sortSummaries(out)
	return out
}

// summaryFromEntry projects a deckEntry into the lightweight DeckSummary used
// by the index page and the JSON listing endpoint.
func summaryFromEntry(e *deckEntry) render.DeckSummary {
	sum := render.DeckSummary{Path: e.relPath, Title: e.title}
	if e.deck != nil {
		sum.Author = e.deck.Meta.Author
		sum.Theme = e.deck.Meta.Theme
		sum.SlideCount = len(e.deck.Slides)
	}
	return sum
}

func sortSummaries(s []render.DeckSummary) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1].Path > s[j].Path; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// notifyClients sends a reload signal to every WebSocket client. Used by the
// directory watcher and tests.
func (s *Server) notifyClients() {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	for _, set := range s.clients {
		for ch := range set {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}
}

// notifyDeck sends a reload signal only to clients viewing the given deck
// path (and to global clients with no path filter).
func (s *Server) notifyDeck(relPath string) {
	relPath = filepath.ToSlash(relPath)
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	for key, set := range s.clients {
		if key != "" && key != relPath {
			continue
		}
		for ch := range set {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}
}

// addClient registers a new WebSocket client scoped to the given deck path.
// An empty path means "any deck".
func (s *Server) addClient(ch chan struct{}) {
	s.addScopedClient("", ch)
}
func (s *Server) addScopedClient(scope string, ch chan struct{}) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	set := s.clients[scope]
	if set == nil {
		set = make(map[chan struct{}]struct{})
		s.clients[scope] = set
	}
	set[ch] = struct{}{}
}

// removeClient unregisters a WebSocket client. The scope is unknown so we
// search every set.
func (s *Server) removeClient(ch chan struct{}) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	for key, set := range s.clients {
		if _, ok := set[ch]; ok {
			delete(set, ch)
			if len(set) == 0 {
				delete(s.clients, key)
			}
			return
		}
	}
}

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/d/", s.handleDeck)
	mux.HandleFunc("/api/context", s.handleAgentContext)
	mux.HandleFunc("/api/decks", s.handleDecksList)
	mux.HandleFunc("/api/comment", s.handleCommentPost)
	mux.HandleFunc("/ws", s.handleWS)
	return mux
}

// handleRoot serves the deck index in directory mode, or the single deck in
// file mode.
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !s.isDir {
		s.mu.RLock()
		html := s.html
		s.mu.RUnlock()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
		return
	}

	s.mu.RLock()
	summaries := s.summariesFromMap(s.decks)
	s.mu.RUnlock()

	html, err := render.RenderIndex(summaries)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}

// handleDeck serves an individual deck at /d/<relative-path>.
func (s *Server) handleDeck(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/d/")
	if rel == "" {
		http.NotFound(w, r)
		return
	}
	rel = filepath.ToSlash(rel)
	s.mu.RLock()
	entry, ok := s.decks[rel]
	html := ""
	if ok {
		html = entry.html
	}
	s.mu.RUnlock()

	if !ok {
		http.NotFound(w, r)
		return
	}
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

	scope := filepath.ToSlash(r.URL.Query().Get("path"))
	ch := make(chan struct{}, 1)
	s.addScopedClient(scope, ch)
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

func (s *Server) handleDecksList(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	summaries := s.summariesFromMap(s.decks)
	if !s.isDir {
		// Even in single-file mode, expose the lone deck so the JSON
		// shape is uniform.
		summaries = make([]render.DeckSummary, 0, len(s.decks))
		for _, e := range s.decks {
			summaries = append(summaries, summaryFromEntry(e))
		}
	}
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(summaries); err != nil {
		s.logger.Printf("decks-list encode: %v", err)
	}
}

// handleCommentPost accepts a JSON body and appends a comment to the target
// .slides file. The body shape:
//
//	{ "path": "cats.slides", "slide": 4, "variant": "", "author": "alice", "text": "needs a citation" }
//
// `slide` is 1-indexed (matching the UI). `variant` is optional and targets
// a specific variant body within the slide.
func (s *Server) handleCommentPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Path    string `json:"path"`
		Slide   int    `json:"slide"`
		Variant string `json:"variant"`
		Author  string `json:"author"`
		Text    string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Slide < 1 {
		http.Error(w, "slide must be >= 1", http.StatusBadRequest)
		return
	}

	// Resolve which deck to write to. Empty path means the default deck.
	rel := filepath.ToSlash(req.Path)
	if rel == "" {
		if s.soloRel == "" {
			http.Error(w, "path is required in directory mode", http.StatusBadRequest)
			return
		}
		rel = s.soloRel
	}

	s.mu.RLock()
	entry, ok := s.decks[rel]
	s.mu.RUnlock()
	if !ok {
		http.Error(w, "unknown deck: "+rel, http.StatusNotFound)
		return
	}

	// Serialize concurrent writes against this deck file. Two browsers
	// posting simultaneously must not race the read-modify-write inside
	// AddComment, and the reloadDeck refresh must observe the post. The
	// lock lives on the server (not the entry) because reloadDeck replaces
	// the entry pointer in s.decks on every refresh.
	lock := s.deckWriteLock(rel)
	lock.Lock()
	defer lock.Unlock()

	if err := format.AddComment(entry.absPath, req.Slide-1, req.Variant, req.Author, req.Text); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.reloadDeck(rel); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.notifyDeck(rel)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":   true,
		"path": rel,
	})
}

// Watch starts watching the root directory tree for *.slides changes and
// triggers a reload + scoped notify on each event.
func (s *Server) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer watcher.Close()

	// Add the root and all (non-dot) subdirectories so nested decks are
	// covered. fsnotify is non-recursive, so we walk once at startup and
	// re-walk on directory creation events.
	if err := s.addWatchTree(watcher, s.rootDir); err != nil {
		return err
	}

	debounceMap := make(map[string]*time.Timer)
	var debMu sync.Mutex

	scheduleReload := func(rel string) {
		debMu.Lock()
		defer debMu.Unlock()
		if t, ok := debounceMap[rel]; ok {
			t.Stop()
		}
		debounceMap[rel] = time.AfterFunc(100*time.Millisecond, func() {
			if rel == "" {
				if err := s.reload(); err != nil {
					s.logger.Printf("reload error: %v", err)
				} else {
					s.logger.Printf("reloaded %s", s.rootDir)
					s.notifyClients()
				}
				return
			}
			if err := s.reloadDeck(rel); err != nil {
				// Path may have been added/deleted — fall back to a
				// full reload.
				if err2 := s.reload(); err2 != nil {
					s.logger.Printf("reload error: %v", err2)
					return
				}
			}
			s.logger.Printf("reloaded %s", rel)
			s.notifyDeck(rel)
		})
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = s.addWatchTree(watcher, event.Name)
					scheduleReload("")
					continue
				}
			}
			if filepath.Ext(event.Name) != ".slides" {
				continue
			}
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) && !event.Has(fsnotify.Remove) && !event.Has(fsnotify.Rename) {
				continue
			}
			rel, err := filepath.Rel(s.rootDir, event.Name)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			// In single-file mode, only react to the one file.
			if s.soloRel != "" && rel != s.soloRel {
				continue
			}
			if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				scheduleReload("") // full reload to drop the entry
			} else {
				scheduleReload(rel)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			s.logger.Printf("watch error: %v", err)
		}
	}
}

// addWatchTree adds `root` and every non-dot subdirectory to the watcher.
func (s *Server) addWatchTree(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		if err := w.Add(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			s.logger.Printf("watch %s: %v", path, err)
		}
		return nil
	})
}

// ListenAndServe starts the server and file watcher.
func (s *Server) ListenAndServe(ctx context.Context) error {
	addr := s.cfg.Bind + ":" + s.cfg.Port
	srv := &http.Server{
		Addr:    addr,
		Handler: s.Handler(),
	}

	go func() {
		if err := s.Watch(ctx); err != nil {
			s.logger.Printf("watcher: %v", err)
		}
	}()

	if s.isDir {
		s.logger.Printf("serving %s (%d decks) at http://%s", s.rootDir, len(s.decks), addr)
	} else {
		s.logger.Printf("serving %s at http://%s", s.cfg.File, addr)
	}

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

// GetDeck returns the current default deck (for agent-context).
func (s *Server) GetDeck() *format.Deck {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deck
}
