package server

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/mschulkind-oss/waxon/internal/format"
)

const testSlides = `---
title: "Test"
theme: default
---

# Slide 1

Hello world.

---

# Slide 2

Goodbye.
`

func writeTestFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.slides")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func testServer(t *testing.T) (*Server, string) {
	t.Helper()
	path := writeTestFile(t, testSlides)
	s, err := New(Config{
		File:   path,
		Port:   "0",
		Logger: log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}

func TestNewServer(t *testing.T) {
	s, _ := testServer(t)
	if s.deck == nil {
		t.Error("deck should be loaded")
	}
	if s.html == "" {
		t.Error("html should be rendered")
	}
}

func TestNewServerBadFile(t *testing.T) {
	_, err := New(Config{
		File:   "/nonexistent/file.slides",
		Logger: log.New(io.Discard, "", 0),
	})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestNewServerBadYAML(t *testing.T) {
	path := writeTestFile(t, "---\ntitle: [broken\n---\n# Slide")
	_, err := New(Config{
		File:   path,
		Logger: log.New(io.Discard, "", 0),
	})
	if err == nil {
		t.Error("expected error for bad YAML")
	}
}

func TestHandleSlides(t *testing.T) {
	s, _ := testServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("content-type = %q", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Hello world") {
		t.Error("body should contain slide content")
	}
}

func TestHandleSlides404(t *testing.T) {
	s, _ := testServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestHandleAgentContext(t *testing.T) {
	s, _ := testServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/context")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("content-type = %q", ct)
	}

	var deck format.Deck
	if err := json.NewDecoder(resp.Body).Decode(&deck); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if deck.Meta.Title != "Test" {
		t.Errorf("title = %q", deck.Meta.Title)
	}
	if len(deck.Slides) != 2 {
		t.Errorf("slides = %d, want 2", len(deck.Slides))
	}
}

func TestWebSocket(t *testing.T) {
	s, _ := testServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Trigger a reload notification
	s.notifyClients()

	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}

	if string(msg) != "reload" {
		t.Errorf("got %q, want %q", msg, "reload")
	}
}

func TestReload(t *testing.T) {
	s, path := testServer(t)

	// Verify initial state
	if !strings.Contains(s.html, "Hello world") {
		t.Error("initial HTML should contain slide content")
	}

	// Write updated file
	updated := strings.Replace(testSlides, "Hello world", "Updated content", 1)
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		t.Fatal(err)
	}

	// Trigger reload
	if err := s.reload(); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s.html, "Updated content") {
		t.Error("HTML should contain updated content after reload")
	}
}

func TestGetDeck(t *testing.T) {
	s, _ := testServer(t)
	deck := s.GetDeck()
	if deck == nil {
		t.Fatal("GetDeck returned nil")
	}
	if deck.Meta.Title != "Test" {
		t.Errorf("title = %q", deck.Meta.Title)
	}
}

func TestGetPort(t *testing.T) {
	// Default port
	os.Unsetenv("SM_PORT")
	if p := getPort(); p != "8080" {
		t.Errorf("default port = %q, want 8080", p)
	}

	// SM_PORT override
	os.Setenv("SM_PORT", "3000")
	defer os.Unsetenv("SM_PORT")
	if p := getPort(); p != "3000" {
		t.Errorf("SM_PORT = %q, want 3000", p)
	}
}

func TestClientManagement(t *testing.T) {
	s, _ := testServer(t)

	ch := make(chan struct{}, 1)
	s.addClient(ch)

	s.mu.RLock()
	count := len(s.clients)
	s.mu.RUnlock()
	if count != 1 {
		t.Errorf("client count = %d, want 1", count)
	}

	s.removeClient(ch)
	s.mu.RLock()
	count = len(s.clients)
	s.mu.RUnlock()
	if count != 0 {
		t.Errorf("client count = %d, want 0", count)
	}
}

func TestNotifyClientsNonBlocking(t *testing.T) {
	s, _ := testServer(t)

	// Add a client with a full channel
	ch := make(chan struct{}, 1)
	ch <- struct{}{} // fill it
	s.addClient(ch)

	// Should not block
	done := make(chan struct{})
	go func() {
		s.notifyClients()
		close(done)
	}()

	select {
	case <-done:
		// good
	case <-time.After(time.Second):
		t.Error("notifyClients blocked")
	}
}

func TestWatchContext(t *testing.T) {
	s, _ := testServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Watch should return quickly
	done := make(chan error, 1)
	go func() {
		done <- s.Watch(ctx)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("watch: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Watch did not return after context cancel")
	}
}

func TestWatchFileChange(t *testing.T) {
	s, path := testServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register a client to receive notifications
	ch := make(chan struct{}, 1)
	s.addClient(ch)
	defer s.removeClient(ch)

	// Start watcher
	go s.Watch(ctx)
	time.Sleep(200 * time.Millisecond) // let watcher start

	// Modify the file
	updated := strings.Replace(testSlides, "Hello world", "File changed", 1)
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for notification (debounce is 100ms)
	select {
	case <-ch:
		// Success — we got a reload notification
		if !strings.Contains(s.html, "File changed") {
			t.Error("HTML should contain updated content")
		}
	case <-time.After(3 * time.Second):
		t.Error("did not receive reload notification")
	}
}

func TestListenAndServe(t *testing.T) {
	path := writeTestFile(t, testSlides)
	s, err := New(Config{
		File:   path,
		Port:   "0",
		Bind:   "127.0.0.1",
		Logger: log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.ListenAndServe(ctx)
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Cancel to trigger shutdown
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("ListenAndServe: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("ListenAndServe did not shut down")
	}
}
