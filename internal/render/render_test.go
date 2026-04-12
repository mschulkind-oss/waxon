package render

import (
	"strings"
	"testing"

	"github.com/mschulkind-oss/waxon/internal/format"
)

func testDeck() *format.Deck {
	return &format.Deck{
		Meta: format.Meta{
			Title:  "Test Deck",
			Author: "Tester",
			Theme:  "default",
			Aspect: "16:9",
			Footer: "Footer Text",
		},
		Slides: []format.Slide{
			{
				Index:   0,
				Content: "# Hello\n\nWelcome to the test.",
				Notes:   []string{"Speaker note"},
			},
			{
				Index:   1,
				Content: "## Second Slide\n\n- Point one\n- Point two",
			},
		},
	}
}

func TestRenderHTML(t *testing.T) {
	deck := testDeck()
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check basic structure
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("missing DOCTYPE")
	}
	if !strings.Contains(html, "<title>Test Deck</title>") {
		t.Error("missing title")
	}
	if !strings.Contains(html, "Footer Text") {
		t.Error("missing footer")
	}
	if !strings.Contains(html, `data-theme="default"`) {
		t.Error("missing theme attribute")
	}
}

func TestRenderHTMLSlideContent(t *testing.T) {
	deck := testDeck()
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check slide content is rendered as HTML
	if !strings.Contains(html, "<h1") {
		t.Error("missing h1 from markdown")
	}
	if !strings.Contains(html, "Welcome to the test.") {
		t.Error("missing paragraph text")
	}
	if !strings.Contains(html, "<li>Point one</li>") {
		t.Error("missing list item")
	}
}

func TestRenderHTMLThemeOverride(t *testing.T) {
	deck := testDeck()
	html, err := RenderHTML(deck, Options{ThemeOverride: "terminal"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, `data-theme="terminal"`) {
		t.Error("theme override not applied")
	}
}

func TestRenderHTMLTerminalVariant(t *testing.T) {
	deck := testDeck()
	deck.Meta.TerminalVariant = "nord"
	deck.Meta.TerminalEffects = true
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, `data-terminal-variant="nord"`) {
		t.Error("missing terminal variant")
	}
	if !strings.Contains(html, "scanline") {
		t.Error("missing terminal effects")
	}
}

func TestRenderHTMLNoTerminalEffects(t *testing.T) {
	deck := testDeck()
	deck.Meta.TerminalEffects = false
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(html, "scanline") {
		t.Error("should not have terminal effects")
	}
}

func TestRenderHTMLWebSocket(t *testing.T) {
	deck := testDeck()
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "WebSocket") {
		t.Error("missing WebSocket live reload script")
	}
	if !strings.Contains(html, "/ws") {
		t.Error("missing /ws endpoint reference")
	}
}

func TestRenderHTMLKeyboardNav(t *testing.T) {
	deck := testDeck()
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "ArrowRight") {
		t.Error("missing keyboard navigation")
	}
	if !strings.Contains(html, "ArrowLeft") {
		t.Error("missing keyboard navigation")
	}
}

func TestRenderHTMLProgress(t *testing.T) {
	deck := testDeck()
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "progress") {
		t.Error("missing progress bar")
	}
}

func TestRenderHTMLNotes(t *testing.T) {
	deck := testDeck()
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "Speaker note") {
		t.Error("notes should be embedded in JSON")
	}
}

func TestRenderSlideHTML(t *testing.T) {
	html, err := RenderSlideHTML("# Hello\n\nWorld **bold**")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "<h1") {
		t.Error("missing h1")
	}
	if !strings.Contains(html, "<strong>bold</strong>") {
		t.Error("missing bold")
	}
}

func TestRenderSlideHTMLCode(t *testing.T) {
	html, err := RenderSlideHTML("```go\nfunc main() {}\n```")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// chroma generates spans with classes or inline styles
	if !strings.Contains(html, "<pre") {
		t.Error("missing pre block")
	}
}

func TestRenderSlideHTMLTable(t *testing.T) {
	html, err := RenderSlideHTML("| A | B |\n|---|---|\n| 1 | 2 |")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "<table") {
		t.Error("missing table")
	}
}

func TestRenderHTMLNoFooter(t *testing.T) {
	deck := testDeck()
	deck.Meta.Footer = ""
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(html, `class="footer"`) {
		t.Error("should not render footer div when footer is empty")
	}
}

func TestRenderHTMLSlideCount(t *testing.T) {
	deck := testDeck()
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "var total") {
		t.Error("missing var total")
	}
}

func TestRenderHTMLEmptyDeck(t *testing.T) {
	deck := &format.Deck{
		Meta:   format.Meta{Title: "Empty"},
		Slides: []format.Slide{},
	}
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "var total") {
		t.Error("missing var total in empty deck")
	}
}

func TestCollectNotes(t *testing.T) {
	slides := []format.Slide{
		{Notes: []string{"note 1", "note 2"}},
		{Notes: nil},
		{Notes: []string{"note 3"}},
	}
	notes := collectNotes(slides)
	if len(notes) != 3 {
		t.Fatalf("got %d note sets, want 3", len(notes))
	}
	if len(notes[0]) != 2 {
		t.Errorf("notes[0] has %d entries", len(notes[0]))
	}
	if len(notes[1]) != 0 {
		t.Errorf("notes[1] should be empty, got %d", len(notes[1]))
	}
}

func TestRenderHTMLActiveSlide(t *testing.T) {
	deck := testDeck()
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First slide should have active class
	if !strings.Contains(html, `class="slide active"`) {
		t.Error("first slide should be active")
	}
}
