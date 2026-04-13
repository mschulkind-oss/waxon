package pdf

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mschulkind-oss/waxon/internal/format"
	"github.com/mschulkind-oss/waxon/internal/render"
	_ "github.com/mschulkind-oss/waxon/internal/themes"
)

func testDeck() *format.Deck {
	return &format.Deck{
		Meta: format.Meta{
			Title:  "Test PDF",
			Author: "Tester",
			Theme:  "default",
			Aspect: "16:9",
		},
		Slides: []format.Slide{
			{Index: 0, Content: "# Hello\n\nFirst slide."},
			{Index: 1, Content: "# World\n\nSecond slide."},
		},
	}
}

func TestExport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PDF export test in short mode")
	}

	dir := t.TempDir()
	output := filepath.Join(dir, "test.pdf")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := Export(ctx, testDeck(), Options{
		Output: output,
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	// Verify file exists and is a PDF
	info, err := os.Stat(output)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() < 1000 {
		t.Errorf("PDF too small: %d bytes", info.Size())
	}

	// Check PDF header
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "%PDF") {
		t.Error("file does not start with PDF header")
	}
}

func TestExportWithThemeOverride(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PDF export test in short mode")
	}

	dir := t.TempDir()
	output := filepath.Join(dir, "themed.pdf")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := Export(ctx, testDeck(), Options{
		Output:        output,
		ThemeOverride: "terminal",
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	info, err := os.Stat(output)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() < 1000 {
		t.Errorf("PDF too small: %d bytes", info.Size())
	}
}

func TestExportCreatesSubdirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PDF export test in short mode")
	}

	dir := t.TempDir()
	output := filepath.Join(dir, "sub", "dir", "test.pdf")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := Export(ctx, testDeck(), Options{
		Output: output,
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if _, err := os.Stat(output); err != nil {
		t.Fatal("output file not created in subdirectory")
	}
}

func TestPaperSize16x9(t *testing.T) {
	w, h := paperSize("16:9")
	if w != 10.0 {
		t.Errorf("width = %f, want 10.0", w)
	}
	if h != 5.625 {
		t.Errorf("height = %f, want 5.625", h)
	}
}

func TestPaperSize4x3(t *testing.T) {
	w, h := paperSize("4:3")
	if w != 10.0 {
		t.Errorf("width = %f, want 10.0", w)
	}
	if h != 7.5 {
		t.Errorf("height = %f, want 7.5", h)
	}
}

func TestPaperSizeInvalid(t *testing.T) {
	w, h := paperSize("invalid")
	if w != 10.0 || h != 5.625 {
		t.Errorf("invalid aspect should default to 16:9, got %fx%f", w, h)
	}
}

func TestPaperSizeEmpty(t *testing.T) {
	w, h := paperSize("")
	if w != 10.0 || h != 5.625 {
		t.Errorf("empty aspect should default to 16:9, got %fx%f", w, h)
	}
}

func TestPrintRender(t *testing.T) {
	// The PDF exporter asks the renderer for a print-friendly page directly.
	// This test guards that contract: the print HTML must contain the .deck
	// wrapper chromedp waits on, must page-break each slide, and must NOT
	// include the live-reload websocket script.
	html, err := render.RenderHTML(testDeck(), render.Options{Standalone: true, Print: true})
	if err != nil {
		t.Fatalf("RenderHTML print: %v", err)
	}
	if !strings.Contains(html, `class="deck"`) {
		t.Error("missing .deck wrapper")
	}
	if !strings.Contains(html, "page-break-after") {
		t.Error("missing page-break-after")
	}
	if strings.Contains(html, "new WebSocket") {
		t.Error("print page must not open a websocket")
	}
}

func TestStandaloneHTMLRender(t *testing.T) {
	// Static HTML export uses the interactive viewer with Standalone=true.
	// It must still be self-contained (no websocket, no comment-post calls)
	// so it works when opened from file:// or a bare webserver.
	html, err := render.RenderHTML(testDeck(), render.Options{Standalone: true})
	if err != nil {
		t.Fatalf("RenderHTML standalone: %v", err)
	}
	// Interactive viewer uses #app / .slide, not the .deck print wrapper.
	if !strings.Contains(html, `id="app"`) {
		t.Error("missing interactive viewer")
	}
	// The WS reconnection loop is gated on !state.standalone — confirm the
	// state flag is set so the browser never tries to connect.
	if !strings.Contains(html, `"standalone":true`) {
		t.Error("standalone HTML must mark state.standalone=true")
	}
}

func TestExportContextCanceled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PDF export test in short mode")
	}

	dir := t.TempDir()
	output := filepath.Join(dir, "cancel.pdf")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := Export(ctx, testDeck(), Options{
		Output: output,
	})
	if err == nil {
		t.Error("expected error for canceled context")
	}
}
