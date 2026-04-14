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

// waitForClient polls the server's client registry until at least one
// WebSocket client is registered, or fails the test after 2s.
func waitForClient(t *testing.T, s *Server) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.clientsMu.Lock()
		var n int
		for _, set := range s.clients {
			n += len(set)
		}
		s.clientsMu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for WebSocket client to register")
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

	// Dial returns after the HTTP upgrade handshake, but the server goroutine
	// that handles the connection hasn't necessarily reached addScopedClient
	// yet. Wait until the client is registered before notifying, otherwise the
	// notify fires into an empty map and the Read below blocks until timeout.
	waitForClient(t, s)

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
	os.Unsetenv("WK_PORT")
	if p := getPort(); p != "8080" {
		t.Errorf("default port = %q, want 8080", p)
	}

	// WK_PORT override
	os.Setenv("WK_PORT", "4000")
	defer os.Unsetenv("WK_PORT")
	if p := getPort(); p != "4000" {
		t.Errorf("WK_PORT = %q, want 4000", p)
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

// --- directory-mode tests --------------------------------------------------

func writeDeckTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"cats.slides": `---
title: "Cats"
---
# Cat slide
Hello cats.
`,
		"dogs.slides": `---
title: "Dogs"
---
# Dog slide
Hello dogs.
`,
		"sub/birds.slides": `---
title: "Birds"
---
# Bird slide
Hello birds.
`,
	}
	for rel, body := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func dirTestServer(t *testing.T) *Server {
	t.Helper()
	dir := writeDeckTree(t)
	s, err := New(Config{
		File:   dir,
		Port:   "0",
		Logger: log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDirectoryModeDiscoversDecks(t *testing.T) {
	s := dirTestServer(t)
	if !s.isDir {
		t.Fatal("server should be in directory mode")
	}
	if len(s.decks) != 3 {
		t.Errorf("got %d decks, want 3", len(s.decks))
	}
	if _, ok := s.decks["cats.slides"]; !ok {
		t.Error("cats.slides not discovered")
	}
	if _, ok := s.decks["sub/birds.slides"]; !ok {
		t.Error("sub/birds.slides not discovered (recursion broken?)")
	}
}

func TestDirectoryRootServesIndex(t *testing.T) {
	s := dirTestServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	if !strings.Contains(html, "/d/cats.slides") {
		t.Error("index should link to /d/cats.slides")
	}
	if !strings.Contains(html, "/d/dogs.slides") {
		t.Error("index should link to /d/dogs.slides")
	}
	if !strings.Contains(html, "/d/sub/birds.slides") {
		t.Error("index should link to /d/sub/birds.slides")
	}
}

func TestDirectoryDeckRoute(t *testing.T) {
	s := dirTestServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/d/dogs.slides")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Hello dogs.") {
		t.Error("response should contain dogs deck content")
	}

	// 404 for unknown deck
	resp2, err := http.Get(ts.URL + "/d/missing.slides")
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 404 {
		t.Errorf("unknown deck status = %d, want 404", resp2.StatusCode)
	}
}

func TestDirectoryDecksListAPI(t *testing.T) {
	s := dirTestServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/decks")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var list []struct {
		Path  string `json:"path"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Errorf("got %d decks, want 3", len(list))
	}
}

func TestCommentPostAppendsAndReloads(t *testing.T) {
	s := dirTestServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	body := `{"path":"cats.slides","slide":1,"variant":"","author":"alice","text":"add a hook"}`
	resp, err := http.Post(ts.URL+"/api/comment", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body = %s", resp.StatusCode, out)
	}

	// File on disk should now contain the directive.
	rootDir := s.rootDir
	data, err := os.ReadFile(filepath.Join(rootDir, "cats.slides"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<!-- comment(@alice): add a hook -->") {
		t.Errorf("file missing comment directive:\n%s", data)
	}

	// And the cached deck should be updated.
	s.mu.RLock()
	cats := s.decks["cats.slides"]
	s.mu.RUnlock()
	if cats == nil || len(cats.deck.Slides[0].Comments) == 0 {
		t.Error("server cache not refreshed after comment post")
	}
}

func TestCommentPostBadInput(t *testing.T) {
	s := dirTestServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	cases := []struct {
		name string
		body string
		want int
	}{
		{"bad json", `not json`, 400},
		{"slide=0", `{"path":"cats.slides","slide":0,"author":"a","text":"x"}`, 400},
		{"unknown deck", `{"path":"missing.slides","slide":1,"author":"a","text":"x"}`, 404},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, err := http.Post(ts.URL+"/api/comment", "application/json", strings.NewReader(c.body))
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != c.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, c.want)
			}
		})
	}
}

func TestScopedWebSocketReload(t *testing.T) {
	s := dirTestServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsBase := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Subscribe one client to cats and one to dogs.
	connCats, _, err := websocket.Dial(ctx, wsBase+"?path=cats.slides", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connCats.Close(websocket.StatusNormalClosure, "")
	connDogs, _, err := websocket.Dial(ctx, wsBase+"?path=dogs.slides", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connDogs.Close(websocket.StatusNormalClosure, "")

	// Trigger a reload notification scoped to cats only.
	time.Sleep(50 * time.Millisecond) // let both subscriptions register
	s.notifyDeck("cats.slides")

	// Cats should receive "reload"; dogs should NOT (within a short window).
	readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
	defer readCancel()
	_, msg, err := connCats.Read(readCtx)
	if err != nil || string(msg) != "reload" {
		t.Errorf("cats got msg=%q err=%v", msg, err)
	}

	dogCtx, dogCancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer dogCancel()
	_, _, err = connDogs.Read(dogCtx)
	if err == nil {
		t.Error("dogs client received unexpected reload — scoping broken")
	}
}

func TestPathTraversalRejected(t *testing.T) {
	s := dirTestServer(t)
	if _, err := s.resolveAbs("../etc/passwd"); err == nil {
		t.Error("expected path traversal to be rejected")
	}
	if _, err := s.resolveAbs("/etc/passwd"); err == nil {
		t.Error("expected absolute path to be rejected")
	}
}

func TestSingleFileModeStillServesRoot(t *testing.T) {
	// Regression: in single-file mode `/` must serve the deck directly,
	// not the directory index.
	s, _ := testServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Hello world") {
		t.Error("single-file root should serve the deck content")
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
