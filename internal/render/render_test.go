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

	// Slide HTML is delivered via JSON inside a <script> tag, so the
	// html/template JS-context escaper rewrites < and > as \u003c / \u003e.
	if !strings.Contains(html, `\u003ch1`) {
		t.Error("missing h1 from markdown JSON payload")
	}
	if !strings.Contains(html, "Welcome to the test.") {
		t.Error("missing paragraph text")
	}
	if !strings.Contains(html, `\u003cli\u003ePoint one\u003c/li\u003e`) {
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

func TestRenderHTMLDeckJSON(t *testing.T) {
	deck := testDeck()
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Slides are delivered via inline JSON to the browser.
	if !strings.Contains(html, `"index":0`) {
		t.Error("deck JSON not embedded")
	}
	if !strings.Contains(html, "Speaker note") {
		t.Error("notes should appear in JSON")
	}
}

func TestRenderHTMLBannerThemeOverride(t *testing.T) {
	deck := testDeck()
	html, err := RenderHTML(deck, Options{ThemeOverride: "vibrant"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(html, `"themeOverridden":true`) {
		t.Error("theme-override flag not set in state JSON")
	}
}

func TestRenderHTMLVariantsInJSON(t *testing.T) {
	deck := testDeck()
	deck.Slides[0].Variants = []format.Variant{
		{Name: "alt", Content: "# Alt\n\nDifferent."},
	}
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(html, `"name":"alt"`) {
		t.Error("variant name missing from JSON")
	}
}

func TestRenderHTMLDecksList(t *testing.T) {
	deck := testDeck()
	html, err := RenderHTML(deck, Options{
		DeckPath: "cats.slides",
		Decks: []DeckSummary{
			{Path: "cats.slides", Title: "Cats"},
			{Path: "dogs.slides", Title: "Dogs"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(html, `"path":"dogs.slides"`) {
		t.Error("deck switcher list missing")
	}
	if !strings.Contains(html, `"path":"cats.slides"`) {
		t.Error("current deck path missing")
	}
}

func TestRenderHTMLPaletteCSS(t *testing.T) {
	deck := testDeck()
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, class := range []string{".red", ".green", ".yellow", ".blue", ".aqua", ".dim"} {
		if !strings.Contains(html, ".slide "+class) {
			t.Errorf("missing palette class %q in base CSS", class)
		}
	}
	for _, v := range []string{"--color-red", "--color-green", "--color-yellow", "--color-blue", "--color-aqua"} {
		if !strings.Contains(html, v) {
			t.Errorf("missing CSS custom property %q", v)
		}
	}
}

func TestRenderHTMLCompareCSS(t *testing.T) {
	deck := testDeck()
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(html, ".waxon-compare") {
		t.Error("missing .waxon-compare base CSS")
	}
	if !strings.Contains(html, ".waxon-compare-pane") {
		t.Error("missing .waxon-compare-pane base CSS")
	}
}

func TestRenderHTMLSlideIDInJSON(t *testing.T) {
	deck := testDeck()
	deck.Slides[0].ID = "intro"
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(html, `"id":"intro"`) {
		t.Error("slide ID missing from JSON payload")
	}
}

func TestRenderIndex(t *testing.T) {
	html, err := RenderIndex([]DeckSummary{
		{Path: "cats.slides", Title: "Cats"},
		{Path: "dogs.slides", Title: "Dogs"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(html, "/d/cats.slides") {
		t.Error("missing cats link")
	}
	if !strings.Contains(html, "/d/dogs.slides") {
		t.Error("missing dogs link")
	}
}
