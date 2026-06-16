package render

import (
	"html/template"
	"os"
	"path/filepath"
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

func TestRenderHTMLPresenterReserve(t *testing.T) {
	deck := testDeck()
	deck.Meta.PresenterReserve = "bottom-right"
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The marker class is applied to the live slide element...
	if !strings.Contains(html, `class="slide presenter-reserve"`) {
		t.Error("missing presenter-reserve class on render-main")
	}
	// ...and re-added by render() JS, which wipes className on every nav.
	if !strings.Contains(html, "classList.add('presenter-reserve')") {
		t.Error("render() JS does not re-apply presenter-reserve after className wipe")
	}
	// The debug-overlay toggle ('R') is wired only when a reserve is declared.
	if !strings.Contains(html, "reserveDebug = !reserveDebug") {
		t.Error("missing reserve-debug toggle")
	}
	if !strings.Contains(html, ".slide.presenter-reserve.reserve-debug::after") {
		t.Error("missing reserve-debug overlay CSS")
	}
	// CSS var fallback is present so themes need not define it.
	if !strings.Contains(html, "--presenter-reserve-w") {
		t.Error("missing presenter-reserve CSS variable fallback")
	}
	// The OPT-IN helper rule exists (reserve is a marker; .reserve-pad clears).
	if !strings.Contains(html, ".slide.reserve-pad {") {
		t.Error("missing opt-in .reserve-pad helper rule")
	}
	// The marker itself must NOT auto-pad content (it's hand-composed).
	if strings.Contains(html, ".slide.presenter-reserve {\n  padding-right") {
		t.Error("presenter-reserve should not auto-pad content (marker only)")
	}
}

func TestRenderHTMLPresenterReserveOff(t *testing.T) {
	deck := testDeck() // PresenterReserve == "" (off)
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(html, `class="slide presenter-reserve"`) {
		t.Error("presenter-reserve class leaked onto slide when option off")
	}
	if strings.Contains(html, "classList.add('presenter-reserve')") {
		t.Error("presenter-reserve JS re-add leaked when option off")
	}
	if strings.Contains(html, "reserveDebug = !reserveDebug") {
		t.Error("reserve-debug toggle leaked when option off")
	}
}

func TestRenderHTMLPresenterReserveUnknownValue(t *testing.T) {
	deck := testDeck()
	deck.Meta.PresenterReserve = "top-left" // not shipped — should be treated as off
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(html, `class="slide presenter-reserve"`) {
		t.Error("unknown presenter-reserve value should not activate the reserve")
	}
}

func TestRenderPrintPresenterReserve(t *testing.T) {
	deck := testDeck()
	deck.Meta.PresenterReserve = "bottom-right"
	html, err := RenderHTML(deck, Options{Print: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Every slide div in the print markup carries the class (one per slide).
	if got := strings.Count(html, "slide presenter-reserve"); got < len(deck.Slides) {
		t.Errorf("print slides missing presenter-reserve class: got %d, want >= %d", got, len(deck.Slides))
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

func TestRenderHTMLThemePathInline(t *testing.T) {
	dir := t.TempDir()
	themeCSS := ".slide h1 { color: hotpink; } /* SENTINEL-CUSTOM-THEME */"
	if err := os.WriteFile(filepath.Join(dir, "custom.css"), []byte(themeCSS), 0o644); err != nil {
		t.Fatalf("write theme: %v", err)
	}

	deck := testDeck()
	deck.Meta.Theme = "./custom.css"

	html, err := RenderHTML(deck, Options{DeckDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(html, "SENTINEL-CUSTOM-THEME") {
		t.Error("custom theme CSS not inlined in rendered HTML")
	}
}

func TestRenderHTMLThemePathPrint(t *testing.T) {
	dir := t.TempDir()
	themeCSS := ".slide { background: lime; } /* SENTINEL-PRINT-THEME */"
	if err := os.WriteFile(filepath.Join(dir, "print.css"), []byte(themeCSS), 0o644); err != nil {
		t.Fatalf("write theme: %v", err)
	}

	deck := testDeck()
	deck.Meta.Theme = "./print.css"

	html, err := RenderHTML(deck, Options{DeckDir: dir, Print: true, Standalone: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(html, "SENTINEL-PRINT-THEME") {
		t.Error("custom theme CSS not inlined in print render")
	}
}

func TestExpandBuiltinImports(t *testing.T) {
	ThemeCSS = func(theme string) template.CSS {
		if theme == "minimal" {
			return template.CSS(":root { --minimal: 1; /* BUILTIN-MINIMAL */ }")
		}
		return ""
	}
	t.Cleanup(func() {
		ThemeCSS = func(string) template.CSS { return "" }
	})

	in := `@import "builtin:minimal";
.custom { color: red; }`
	out := expandBuiltinImports(in)
	if !strings.Contains(out, "BUILTIN-MINIMAL") {
		t.Errorf("expected builtin CSS to be inlined, got: %q", out)
	}
	if !strings.Contains(out, ".custom { color: red; }") {
		t.Errorf("expected user CSS to remain, got: %q", out)
	}
	if strings.Contains(out, "@import") {
		t.Errorf("expected @import line to be replaced, got: %q", out)
	}
}

func TestExpandBuiltinImportsUnknown(t *testing.T) {
	ThemeCSS = func(string) template.CSS { return "" }
	t.Cleanup(func() {
		ThemeCSS = func(string) template.CSS { return "" }
	})
	out := expandBuiltinImports(`@import "builtin:nope";`)
	if !strings.Contains(out, "unknown theme") {
		t.Errorf("expected unknown-theme comment, got: %q", out)
	}
}

func TestRenderHTMLFonts(t *testing.T) {
	deck := testDeck()
	deck.Meta.Fonts = []string{
		"https://fonts.googleapis.com/css2?family=Inter",
		"https://example.com/foo.css",
	}
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, href := range deck.Meta.Fonts {
		want := `<link rel="stylesheet" href="` + href + `">`
		if !strings.Contains(html, want) {
			t.Errorf("missing font link %q", want)
		}
	}
}

func TestRenderHTMLFontsPrint(t *testing.T) {
	deck := testDeck()
	deck.Meta.Fonts = []string{"https://fonts.googleapis.com/css2?family=Inter"}
	html, err := RenderHTML(deck, Options{Print: true, Standalone: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(html, `href="https://fonts.googleapis.com/css2?family=Inter"`) {
		t.Error("font link missing in print render")
	}
}

func TestRenderHTMLPrintSlideClass(t *testing.T) {
	deck := testDeck()
	deck.Slides[0].SlideOpts = &format.SlideOpts{Class: "no-chrome"}
	html, err := RenderHTML(deck, Options{Print: true, Standalone: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(html, `class="slide no-chrome"`) {
		t.Error("expected no-chrome class on slide in print output")
	}
}

func TestRenderHTMLPrintSlideBg(t *testing.T) {
	deck := testDeck()
	deck.Slides[0].SlideOpts = &format.SlideOpts{Background: "#112233"}
	html, err := RenderHTML(deck, Options{Print: true, Standalone: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(html, "background: #112233") {
		t.Error("expected slide background in print output")
	}
}

func TestRenderHTMLThemePathMissingDeckDir(t *testing.T) {
	deck := testDeck()
	deck.Meta.Theme = "./missing.css"
	_, err := RenderHTML(deck, Options{})
	if err == nil {
		t.Fatal("expected error for theme path without DeckDir")
	}
}
