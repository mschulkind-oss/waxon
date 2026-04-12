package pdf

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mschulkind-oss/waxon/internal/format"
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

func TestInjectPrintCSS(t *testing.T) {
	html := `<html><head><title>Test</title></head><body></body></html>`
	result := injectPrintCSS(html)

	if !strings.Contains(result, "page-break-after") {
		t.Error("missing page-break-after in print CSS")
	}
	if !strings.Contains(result, "WebSocket") {
		t.Error("missing WebSocket disable script")
	}
	if !strings.Contains(result, "</head>") {
		t.Error("missing closing head tag")
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
