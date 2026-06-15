package server

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleDeckServesStaticAsset(t *testing.T) {
	dir := t.TempDir()
	// a deck + a nested asset, mirroring episodes/assets/pic.png
	os.WriteFile(filepath.Join(dir, "deck.slides"), []byte("---\ntitle: t\n---\n\n# Hi\n\n![x](assets/pic.png)\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "assets"), 0755)
	os.WriteFile(filepath.Join(dir, "assets", "pic.png"), []byte("\x89PNG\r\n\x1a\nFAKE"), 0644)

	s, err := New(Config{File: dir, Logger: log.New(os.Stderr, "", 0)})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// asset request through the /d/ handler
	req := httptest.NewRequest("GET", "/d/assets/pic.png", nil)
	rec := httptest.NewRecorder()
	s.handleDeck(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("asset: got %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got[:4] != "\x89PNG" {
		t.Errorf("asset body not served (got %q)", got[:8])
	}
	// traversal must still be blocked
	req2 := httptest.NewRequest("GET", "/d/../../etc/passwd", nil)
	rec2 := httptest.NewRecorder()
	s.handleDeck(rec2, req2)
	if rec2.Code == http.StatusOK {
		t.Errorf("path traversal not blocked")
	}
}
