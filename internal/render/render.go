// Package render converts parsed slides into HTML.
package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mschulkind-oss/waxon/internal/format"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Options configures the renderer.
type Options struct {
	// ThemeOverride forces a different theme than the deck's frontmatter.
	// When set, the UI shows a banner indicating the override.
	ThemeOverride string

	// DeckPath is the path used to identify this deck on the server (for
	// websocket scoping and the comment-post API). It may be a filename, a
	// relative path, or empty for stand-alone renders (e.g. PDF export).
	DeckPath string

	// Decks is the list of sibling decks shown in the deck switcher. Empty
	// in single-file mode.
	Decks []DeckSummary

	// Standalone disables websocket reload + comment posting so the rendered
	// HTML doesn't try to phone home. Used by both PDF export and static
	// HTML export.
	Standalone bool

	// Print switches to the print-friendly layout (all slides pre-rendered
	// as stacked divs, one-per-page). Used by PDF export.
	Print bool

	// IncludeNotes, when true in an export, bundles speaker notes into the
	// rendered output so the viewer can open them alongside each slide.
	IncludeNotes bool

	// DeckDir is the filesystem directory of the .slides file being rendered.
	// It's used to resolve relative theme paths in `theme: ./foo.css`.
	DeckDir string
}

// DeckSummary is a lightweight reference to a deck on the same server.
type DeckSummary struct {
	Path       string `json:"path"`
	Title      string `json:"title"`
	Author     string `json:"author,omitempty"`
	Theme      string `json:"theme,omitempty"`
	SlideCount int    `json:"slideCount,omitempty"`
}

var md goldmark.Markdown

func init() {
	md = goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Typographer,
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"),
				highlighting.WithFormatOptions(),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
}

// RenderHTML renders a deck to a complete HTML page. When opts.Print is
// true, the renderer emits a PDF-friendly layout with all slides stacked
// vertically (used by the PDF exporter); otherwise it emits the
// interactive multi-deck UI. Standalone further disables server features
// (websocket reload, comment posting) so the output works offline.
func RenderHTML(deck *format.Deck, opts Options) (string, error) {
	if opts.Print {
		return renderStandalone(deck, opts)
	}

	theme := deck.Meta.Theme
	themeOverridden := false
	if opts.ThemeOverride != "" && opts.ThemeOverride != theme {
		theme = opts.ThemeOverride
		themeOverridden = true
	}

	var themeCSSInline template.CSS
	if isThemePath(theme) {
		css, err := resolveThemePath(theme, opts.DeckDir)
		if err != nil {
			return "", err
		}
		themeCSSInline = css
	}

	jsDeck, err := buildJSDeck(deck)
	if err != nil {
		return "", err
	}
	jsDeckJSON, err := json.Marshal(jsDeck)
	if err != nil {
		return "", err
	}

	decksJSON, err := json.Marshal(opts.Decks)
	if err != nil {
		return "", err
	}

	themesJSON, err := json.Marshal(AllThemes())
	if err != nil {
		return "", err
	}

	stateJSON, err := json.Marshal(struct {
		Path            string `json:"path"`
		DeckTheme       string `json:"deckTheme"`
		ActiveTheme     string `json:"activeTheme"`
		ThemeOverridden bool   `json:"themeOverridden"`
		Standalone      bool   `json:"standalone"`
	}{
		Path:            opts.DeckPath,
		DeckTheme:       deck.Meta.Theme,
		ActiveTheme:     theme,
		ThemeOverridden: themeOverridden,
		Standalone:      opts.Standalone,
	})
	if err != nil {
		return "", err
	}

	data := templateData{
		Title:           deck.Meta.Title,
		Author:          deck.Meta.Author,
		Theme:           theme,
		Aspect:          deck.Meta.Aspect,
		Footer:          deck.Meta.Footer,
		FooterLeft:      deck.Meta.FooterLeft,
		FooterRight:     deck.Meta.FooterRight,
		Transition:      deck.Meta.Transition,
		TerminalVariant: deck.Meta.TerminalVariant,
		TerminalEffects: deck.Meta.TerminalEffects,
		ThemeCSSInline:  themeCSSInline,
		Fonts:           deck.Meta.Fonts,
		DeckJSON:        template.JS(jsDeckJSON),
		DecksJSON:       template.JS(decksJSON),
		StateJSON:       template.JS(stateJSON),
		ThemesJSON:      template.JS(themesJSON),
		TotalSlides:     len(deck.Slides),
	}

	var buf bytes.Buffer
	if err := pageTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}

	return buf.String(), nil
}

// RenderSlideHTML renders a single slide's markdown content to HTML.
func RenderSlideHTML(content string) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(content), &buf); err != nil {
		return "", fmt.Errorf("render markdown: %w", err)
	}
	return buf.String(), nil
}

// jsSlide is the per-slide payload sent to the browser.
type jsSlide struct {
	Index      int              `json:"index"`
	ID         string           `json:"id,omitempty"`
	HTML       string           `json:"html"`
	Notes      []string         `json:"notes"`
	AINotes    []string         `json:"aiNotes"`
	Comments   []format.Comment `json:"comments"`
	Variants   []jsVariant      `json:"variants"`
	Pauses     int              `json:"pauses"`
	Bg         string           `json:"bg,omitempty"`
	BgImage    string           `json:"bgImage,omitempty"`
	Class      string           `json:"class,omitempty"`
	Valign     string           `json:"valign,omitempty"`
	Transition string           `json:"transition,omitempty"`
}

type jsVariant struct {
	Name     string           `json:"name"`
	HTML     string           `json:"html"`
	Notes    []string         `json:"notes"`
	AINotes  []string         `json:"aiNotes"`
	Comments []format.Comment `json:"comments"`
	Pauses   int              `json:"pauses"`
}

func buildJSDeck(deck *format.Deck) ([]jsSlide, error) {
	out := make([]jsSlide, 0, len(deck.Slides))
	for _, s := range deck.Slides {
		html, err := RenderSlideHTML(s.Content)
		if err != nil {
			return nil, err
		}
		js := jsSlide{
			Index:    s.Index,
			ID:       s.ID,
			HTML:     html,
			Notes:    nilToEmpty(s.Notes),
			AINotes:  nilToEmpty(s.AINotes),
			Comments: commentsOrEmpty(s.Comments),
			Pauses:   s.Pauses,
		}
		if s.SlideOpts != nil {
			js.Bg = s.SlideOpts.Background
			js.BgImage = s.SlideOpts.BgImage
			js.Class = s.SlideOpts.Class
			js.Valign = s.SlideOpts.Valign
			js.Transition = s.SlideOpts.Transition
		}
		for _, v := range s.Variants {
			vhtml, err := RenderSlideHTML(v.Content)
			if err != nil {
				return nil, err
			}
			js.Variants = append(js.Variants, jsVariant{
				Name:     v.Name,
				HTML:     vhtml,
				Notes:    nilToEmpty(v.Notes),
				AINotes:  nilToEmpty(v.AINotes),
				Comments: commentsOrEmpty(v.Comments),
				Pauses:   v.Pauses,
			})
		}
		if js.Variants == nil {
			js.Variants = []jsVariant{}
		}
		out = append(out, js)
	}
	return out, nil
}

func nilToEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func commentsOrEmpty(c []format.Comment) []format.Comment {
	if c == nil {
		return []format.Comment{}
	}
	return c
}

type templateData struct {
	Title           string
	Author          string
	Theme           string
	Aspect          string
	Footer          string
	FooterLeft      string
	FooterRight     string
	Transition      string
	TerminalVariant string
	TerminalEffects bool
	// ThemeCSSInline, when non-empty, overrides the built-in theme lookup
	// with raw CSS loaded from disk. Used by `theme: ./foo.css` paths.
	ThemeCSSInline template.CSS
	Fonts          []string
	DeckJSON       template.JS
	DecksJSON      template.JS
	StateJSON      template.JS
	ThemesJSON     template.JS
	TotalSlides    int
}

// ThemeCSS returns the CSS for a given theme name.
// The themes package replaces this at init time.
var ThemeCSS = func(theme string) template.CSS {
	return ""
}

// isThemePath reports whether a theme value looks like a relative path
// to a .css file rather than a bare theme name. We only recognize .css
// suffixes so a theme named `foo` is never misinterpreted.
func isThemePath(theme string) bool {
	return strings.HasSuffix(theme, ".css")
}

// resolveThemePath loads the CSS file referenced by a `theme: ./foo.css`
// frontmatter value, resolved relative to deckDir. Returns (css, nil) on
// success, or ("", err) if deckDir is empty, the path escapes deckDir,
// or the file can't be read. Absolute paths are rejected.
func resolveThemePath(theme, deckDir string) (template.CSS, error) {
	if deckDir == "" {
		return "", fmt.Errorf("theme %q is a path but no deck directory is set", theme)
	}
	if filepath.IsAbs(theme) {
		return "", fmt.Errorf("theme path %q must be relative to the deck", theme)
	}
	clean := filepath.Clean(filepath.FromSlash(theme))
	full := filepath.Join(deckDir, clean)
	data, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("read theme %s: %w", theme, err)
	}
	return template.CSS(expandBuiltinImports(string(data))), nil
}

// builtinImportRe matches `@import "builtin:<name>";` or `@import 'builtin:<name>';`.
// Whitespace around the directive and a trailing semicolon are tolerated.
var builtinImportRe = regexp.MustCompile(`(?m)^[ \t]*@import[ \t]+["']builtin:([a-zA-Z0-9_-]+)["'][ \t]*;?[ \t]*$`)

// expandBuiltinImports replaces `@import "builtin:<name>";` lines in a
// custom theme CSS body with the full CSS of the built-in theme named
// <name>. An unknown built-in is replaced with a CSS comment so the user
// can see what went wrong in the rendered output. This preprocessing
// happens once at render time; the custom theme can still add its own
// rules after the import block.
func expandBuiltinImports(css string) string {
	return builtinImportRe.ReplaceAllStringFunc(css, func(match string) string {
		m := builtinImportRe.FindStringSubmatch(match)
		if len(m) != 2 {
			return match
		}
		name := m[1]
		body := ThemeCSS(name)
		if body == "" {
			return fmt.Sprintf("/* @import builtin:%s — unknown theme */", name)
		}
		return string(body)
	})
}

// ThemeEntry is a lightweight view of a theme for the picker panel.
type ThemeEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CSS         string `json:"css"`
}

// AllThemes returns every built-in theme. The themes package replaces
// this at init time; before then it returns nil.
var AllThemes = func() []ThemeEntry {
	return nil
}

// renderStandalone produces a print-friendly HTML page that pre-renders all
// slides as static <div class="slide"> elements stacked vertically. The PDF
// exporter relies on this layout: it loads the page in headless Chromium,
// waits for `.deck` to appear, then prints to PDF. No JavaScript is required.
//
// If opts.ThemeOverride is set it replaces the deck theme. If opts.Variant
// names a variant on a slide, that variant's body is used in place of the
// main slide body for that slide.
func renderStandalone(deck *format.Deck, opts Options) (string, error) {
	theme := deck.Meta.Theme
	if opts.ThemeOverride != "" {
		theme = opts.ThemeOverride
	}

	var themeCSSInline template.CSS
	if isThemePath(theme) {
		css, err := resolveThemePath(theme, opts.DeckDir)
		if err != nil {
			return "", err
		}
		themeCSSInline = css
	}

	type printSlide struct {
		Index   int
		ID      string
		Class   string
		Bg      string
		BgImage string
		Valign  string
		HTML    template.HTML
	}
	slides := make([]printSlide, 0, len(deck.Slides))
	for i, s := range deck.Slides {
		body := s.Content
		html, err := RenderSlideHTML(body)
		if err != nil {
			return "", err
		}
		ps := printSlide{Index: i, ID: s.ID, HTML: template.HTML(html)}
		if s.SlideOpts != nil {
			ps.Class = s.SlideOpts.Class
			ps.Bg = s.SlideOpts.Background
			ps.BgImage = s.SlideOpts.BgImage
			ps.Valign = s.SlideOpts.Valign
		}
		slides = append(slides, ps)
	}

	data := struct {
		Title           string
		Theme           string
		Transition      string
		TerminalVariant string
		TerminalEffects bool
		Footer          string
		FooterLeft      string
		FooterRight     string
		ThemeCSSInline  template.CSS
		Fonts           []string
		Slides          []printSlide
	}{
		Title:           deck.Meta.Title,
		Theme:           theme,
		Transition:      deck.Meta.Transition,
		TerminalVariant: deck.Meta.TerminalVariant,
		TerminalEffects: deck.Meta.TerminalEffects,
		Footer:          deck.Meta.Footer,
		FooterLeft:      deck.Meta.FooterLeft,
		FooterRight:     deck.Meta.FooterRight,
		ThemeCSSInline:  themeCSSInline,
		Fonts:           deck.Meta.Fonts,
		Slides:          slides,
	}

	var buf bytes.Buffer
	if err := printTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render print template: %w", err)
	}
	return buf.String(), nil
}

// RenderIndex renders a directory listing of decks for multi-deck serve mode.
func RenderIndex(decks []DeckSummary) (string, error) {
	data := struct {
		Decks []DeckSummary
		JSON  template.JS
	}{
		Decks: decks,
	}
	jsonBytes, err := json.Marshal(decks)
	if err != nil {
		return "", err
	}
	data.JSON = template.JS(jsonBytes)

	var buf bytes.Buffer
	if err := indexTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render index: %w", err)
	}
	return buf.String(), nil
}

var (
	pageTmpl  *template.Template
	indexTmpl *template.Template
	printTmpl *template.Template
)

func init() {
	// Wrap ThemeCSS in a closure so the template looks up the current
	// value at execute time. The themes package replaces ThemeCSS in its
	// own init(), which may run after this init() — capturing the bare
	// variable would lock in the empty stub.
	themeCSSFn := func(theme string) template.CSS { return ThemeCSS(theme) }
	incFn := func(i int) int { return i + 1 }
	pageFooterFn := func(tpl string, n, total int) string {
		if tpl == "" {
			return ""
		}
		r := strings.NewReplacer(
			"{n}", fmt.Sprintf("%d", n),
			"{page}", fmt.Sprintf("%d", n),
			"{total}", fmt.Sprintf("%d", total),
		)
		return r.Replace(tpl)
	}
	funcs := template.FuncMap{
		"themeCSS":   themeCSSFn,
		"inc":        incFn,
		"pageFooter": pageFooterFn,
	}
	pageTmpl = template.Must(template.New("page").Funcs(funcs).Parse(pageTemplate))
	indexTmpl = template.Must(template.New("index").Parse(indexTemplate))
	printTmpl = template.Must(template.New("print").Funcs(funcs).Parse(printTemplate))
}

// MarshalDeckListJSON is a small convenience for callers that want the same
// JSON encoding the renderer uses (used by tests).
func MarshalDeckListJSON(decks []DeckSummary) string {
	b, _ := json.Marshal(decks)
	return strings.TrimSpace(string(b))
}

const pageTemplate = `<!DOCTYPE html>
<html lang="en" data-theme="{{.Theme}}"{{if .TerminalVariant}} data-terminal-variant="{{.TerminalVariant}}"{{end}}>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
/*
 * Layered architecture:
 *  - The "slide" class is shared with the print template and the 20 themes,
 *    so theme rules like ".slide h1 { color: ... }" apply identically in
 *    the live UI and PDF export.
 *  - Chrome (banner, panels, FAB, help overlay) lives in its own --chrome-*
 *    variable space and uses fixed-px sizing so it never scales with the
 *    slide font-size.
 */
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

:root {
  --slide-bg: #1a1a2e;
  --slide-fg: #e0e0e0;
  --accent: #7c3aed;
  --font-body: system-ui, -apple-system, sans-serif;
  --font-heading: system-ui, -apple-system, sans-serif;
  --font-mono: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace;
  --slide-padding: 5vmin;

  /* Chrome variables — fixed-px, theme-aware */
  --chrome-fg: var(--slide-fg);
  --chrome-bg: color-mix(in srgb, var(--slide-bg) 88%, #000 12%);
  --chrome-border: color-mix(in srgb, var(--slide-fg) 18%, transparent);
  --chrome-hover: color-mix(in srgb, var(--accent) 25%, transparent);
  --chrome-active: color-mix(in srgb, var(--accent) 40%, transparent);
  --chrome-font: 13px;
  --chrome-font-sm: 12px;
  --chrome-font-lg: 15px;
  --backdrop: color-mix(in srgb, var(--slide-bg) 70%, #000 60%);
}

html, body {
  height: 100%;
  overflow: hidden;
  background: var(--slide-bg);
  color: var(--slide-fg);
  font-family: var(--font-body);
  font-size: calc(clamp(16px, 2.5vmin, 28px) * var(--waxon-zoom, 1));
  line-height: 1.5;
}

.app {
  width: 100vw;
  height: 100vh;
  position: relative;
  display: flex;
  flex-direction: column;
  outline: none;
  /* Clip the off-screen panels so they don't extend document scrollWidth;
   * without this, focus() on a panel descendant scrolls the body 420px and
   * the hidden panels become visible. */
  overflow: hidden;
}

/* ---------- Banner ---------- */
.banner {
  display: none;
  align-items: center;
  gap: 0.6em;
  padding: 8px 14px;
  background: color-mix(in srgb, var(--accent) 14%, var(--chrome-bg));
  color: var(--chrome-fg);
  font-size: var(--chrome-font);
  border-bottom: 1px solid color-mix(in srgb, var(--accent) 45%, transparent);
  z-index: 100;
}
.banner.active { display: flex; }
.banner .label { font-weight: 600; opacity: 0.85; }
.banner .pill {
  background: color-mix(in srgb, var(--accent) 30%, transparent);
  border: 1px solid color-mix(in srgb, var(--accent) 50%, transparent);
  padding: 2px 8px;
  border-radius: 999px;
  font-size: var(--chrome-font-sm);
  font-family: var(--font-mono);
}
.banner .spacer { flex: 1; }
.banner button {
  background: transparent;
  border: 1px solid var(--chrome-border);
  color: inherit;
  padding: 3px 10px;
  border-radius: 4px;
  font: inherit;
  font-size: var(--chrome-font-sm);
  cursor: pointer;
}
.banner button:hover { background: color-mix(in srgb, var(--chrome-fg) 10%, transparent); }
.banner button + button { margin-left: 4px; }

/* ---------- Deck area ---------- */
.deck-area { flex: 1; position: relative; min-height: 0; display: flex; }
.deck-area.compare { gap: 8px; background: color-mix(in srgb, var(--slide-fg) 20%, transparent); }

.pane {
  flex: 1;
  position: relative;
  overflow: hidden;
  background: var(--slide-bg);
}
.pane.compare-pane {
  outline: 1px solid color-mix(in srgb, var(--accent) 45%, transparent);
  outline-offset: -1px;
}
.pane-label {
  position: absolute;
  top: 8px;
  left: 50%;
  transform: translateX(-50%);
  background: color-mix(in srgb, var(--slide-bg) 70%, #000);
  color: var(--chrome-fg);
  padding: 3px 10px;
  border-radius: 4px;
  font-size: var(--chrome-font-sm);
  z-index: 10;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  border: 1px solid var(--chrome-border);
}

/*
 * .slide is shared with the print template and the 20 themes target it
 * directly. Don't rename without updating themes.go.
 */
.slide {
  position: absolute;
  inset: 0;
  padding: var(--slide-padding);
  overflow: auto;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

/* Base typography is wrapped in :where() so theme CSS with bare element
 * selectors wins without needing a .slide prefix (#41). */
:where(.slide) h1 { font-size: 2.5em; font-family: var(--font-heading); color: var(--accent); margin-bottom: 0.5em; }
:where(.slide) h2 { font-size: 1.8em; font-family: var(--font-heading); color: var(--accent); margin-bottom: 0.4em; }
:where(.slide) h3 { font-size: 1.3em; font-family: var(--font-heading); margin-bottom: 0.3em; }
:where(.slide) p { margin-bottom: 0.8em; }
:where(.slide) ul, :where(.slide) ol { margin-left: 1.5em; margin-bottom: 0.8em; }
:where(.slide) li { margin-bottom: 0.3em; }
/* Ordered list markers inherit accent color by default (#47) */
:where(.slide) ol > li::marker { color: var(--list-marker-color, var(--accent)); }
:where(.slide) pre {
  background: color-mix(in srgb, var(--slide-fg) 8%, transparent);
  border: 1px solid color-mix(in srgb, var(--slide-fg) 15%, transparent);
  border-radius: 6px;
  padding: 1em;
  overflow-x: auto;
  margin-bottom: 1em;
  font-family: var(--font-mono);
  font-size: 0.85em;
  line-height: 1.4;
}
:where(.slide) code { font-family: var(--font-mono); font-size: 0.9em; }
:where(.slide) :not(pre) > code {
  background: color-mix(in srgb, var(--slide-fg) 12%, transparent);
  padding: 0.15em 0.4em;
  border-radius: 4px;
}
:where(.slide) table { border-collapse: collapse; margin-bottom: 1em; }
:where(.slide) th, :where(.slide) td {
  border: 1px solid color-mix(in srgb, var(--slide-fg) 22%, transparent);
  padding: 0.5em 1em;
  text-align: left;
}
:where(.slide) th[align], :where(.slide) td[align] { text-align: inherit; }
:where(.slide) th { background: color-mix(in srgb, var(--slide-fg) 10%, transparent); font-weight: 600; }
:where(.slide) blockquote {
  border-left: 4px solid var(--accent);
  padding-left: 1em;
  margin-bottom: 0.8em;
  font-style: italic;
}
:where(.slide) img { max-width: 100%; max-height: calc(100vh - 16vmin); object-fit: contain; }

/* Slide-level vertical alignment (#49, #54) */
.slide[data-valign="top"] { justify-content: flex-start; }
.slide[data-valign="center"], .slide[data-valign="middle"] { justify-content: center; }
.slide[data-valign="bottom"] { justify-content: flex-end; }

/* Pause / progressive reveal: hide everything inside .slide that has the
 * .waxon-hidden class. JS adds/removes this on direct children at every
 * pause boundary so '<!-- pause -->' authors get progressive reveal. */
:where(.slide) .waxon-hidden { visibility: hidden; }

/* The parser replaces <!-- pause --> directives with this sentinel so the
 * renderer knows *where* the pauses were. It's a marker only — never visible.
 * goldmark treats the sentinel div as a block element, which splits any
 * enclosing <ul>/<ol> into segments at the pause boundary; re-tighten the
 * vertical spacing so progressively revealed lists look continuous. */
:where(.slide) .waxon-pause { display: none; }
:where(.slide) .waxon-pause + ul, :where(.slide) .waxon-pause + ol { margin-top: -0.3em; }

/* ---------- Color palette utility classes ----------
 * Emitted by the parser's .color{text} and .color text transforms. Themes
 * override the --color-* custom properties to match their own palettes;
 * the fallbacks below only kick in when a theme forgets to set them. */
.slide .red    { color: var(--color-red,    #ef4444); }
.slide .green  { color: var(--color-green,  #22c55e); }
.slide .yellow { color: var(--color-yellow, #eab308); }
.slide .blue   { color: var(--color-blue,   #3b82f6); }
.slide .aqua   { color: var(--color-aqua,   #06b6d4); }
.slide .dim    { opacity: 0.6; }

/* ---------- Compare layout ----------
 * Emitted by the parser's :::compare fence transform. Two panes side by
 * side, stacking to a single column below 700px so the split doesn't
 * compress to illegibility on narrow screens. Border color uses the
 * palette class on the pane itself (.red / .green / etc.) via currentColor. */
:where(.slide) .waxon-compare {
  display: flex;
  flex-direction: row;
  gap: 1.5em;
  margin: 1em 0;
  align-items: stretch;
}
:where(.slide) .waxon-compare-pane {
  flex: 1 1 0;
  padding: 1em 1.2em;
  border: 2px solid currentColor;
  border-radius: 6px;
  color: var(--slide-fg);
}
:where(.slide) .waxon-compare-pane.red    { border-color: var(--color-red,    #ef4444); }
:where(.slide) .waxon-compare-pane.green  { border-color: var(--color-green,  #22c55e); }
:where(.slide) .waxon-compare-pane.yellow { border-color: var(--color-yellow, #eab308); }
:where(.slide) .waxon-compare-pane.blue   { border-color: var(--color-blue,   #3b82f6); }
:where(.slide) .waxon-compare-pane.aqua   { border-color: var(--color-aqua,   #06b6d4); }
:where(.slide) .waxon-compare-pane h1,
:where(.slide) .waxon-compare-pane h2,
:where(.slide) .waxon-compare-pane h3 { margin-top: 0; }
@media (max-width: 700px) {
  :where(.slide) .waxon-compare { flex-direction: column; }
}

/* ---------- Parse error banner ----------
 * Visible block the parser emits when it can't process a fenced block
 * (e.g. nested :::compare). Authors see exactly where the failure is. */
:where(.slide) .waxon-error {
  background: color-mix(in srgb, #ef4444 20%, transparent);
  color: #fca5a5;
  border: 1px solid #ef4444;
  border-radius: 4px;
  padding: 0.5em 0.8em;
  font-family: var(--font-mono);
  font-size: 0.85em;
}

/* ---------- Cards ----------
 * :::card / :::card <color> / :::card-left. A bordered container for a
 * title + metric + subtitle. Left-border variant drops the full border
 * for a blockquote-like affordance. */
:where(.slide) .waxon-card {
  border: 1px solid var(--foreground2, currentColor);
  border-radius: 6px;
  padding: 0.8em 1.2em;
  margin: 0.8em 0;
  color: var(--slide-fg);
}
:where(.slide) .waxon-card.red    { border-color: var(--color-red,    #ef4444); --card-color: var(--color-red,    #ef4444); }
:where(.slide) .waxon-card.green  { border-color: var(--color-green,  #22c55e); --card-color: var(--color-green,  #22c55e); }
:where(.slide) .waxon-card.yellow { border-color: var(--color-yellow, #eab308); --card-color: var(--color-yellow, #eab308); }
:where(.slide) .waxon-card.blue   { border-color: var(--color-blue,   #3b82f6); --card-color: var(--color-blue,   #3b82f6); }
:where(.slide) .waxon-card.aqua   { border-color: var(--color-aqua,   #06b6d4); --card-color: var(--color-aqua,   #06b6d4); }
/* Card headings inherit the card's accent color automatically (#43). */
:where(.slide) .waxon-card h1,
:where(.slide) .waxon-card h2,
:where(.slide) .waxon-card h3 { color: var(--card-color, inherit); }
:where(.slide) .waxon-card > :first-child { margin-top: 0; }
:where(.slide) .waxon-card > :last-child  { margin-bottom: 0; }
/* Card size shortcuts (#48). width=NN% modifier sets inline style directly. */
:where(.slide) .waxon-card.waxon-card-small  { max-width: 18em; }
:where(.slide) .waxon-card.waxon-card-medium { max-width: 28em; }
:where(.slide) .waxon-card.waxon-card-large  { max-width: 42em; }
:where(.slide) .waxon-card-left {
  border: none;
  border-left: 4px solid var(--foreground2, currentColor);
  border-radius: 0;
  padding-left: 1em;
}
:where(.slide) .waxon-card-left.red    { border-left-color: var(--color-red,    #ef4444); }
:where(.slide) .waxon-card-left.green  { border-left-color: var(--color-green,  #22c55e); }
:where(.slide) .waxon-card-left.yellow { border-left-color: var(--color-yellow, #eab308); }
:where(.slide) .waxon-card-left.blue   { border-left-color: var(--color-blue,   #3b82f6); }
:where(.slide) .waxon-card-left.aqua   { border-left-color: var(--color-aqua,   #06b6d4); }

/* ---------- Grid layout ----------
 * :::grid 3 / :::grid 2x2. Uses inline grid-template-columns from the
 * parser so the number of columns isn't baked into CSS. */
:where(.slide) .waxon-grid {
  display: grid;
  gap: 1em;
  margin: 1em 0;
}
:where(.slide) .waxon-grid-cell {
  padding: 0.8em 1em;
  border: 1px solid var(--foreground2, currentColor);
  border-radius: 6px;
}
:where(.slide) .waxon-grid-cell.red    { border-color: var(--color-red,    #ef4444); border-top: 3px solid var(--color-red,    #ef4444); }
:where(.slide) .waxon-grid-cell.green  { border-color: var(--color-green,  #22c55e); border-top: 3px solid var(--color-green,  #22c55e); }
:where(.slide) .waxon-grid-cell.yellow { border-color: var(--color-yellow, #eab308); border-top: 3px solid var(--color-yellow, #eab308); }
:where(.slide) .waxon-grid-cell.blue   { border-color: var(--color-blue,   #3b82f6); border-top: 3px solid var(--color-blue,   #3b82f6); }
:where(.slide) .waxon-grid-cell.aqua   { border-color: var(--color-aqua,   #06b6d4); border-top: 3px solid var(--color-aqua,   #06b6d4); }
:where(.slide) .waxon-grid-cell > :first-child { margin-top: 0; }
:where(.slide) .waxon-grid-cell > :last-child  { margin-bottom: 0; }

/* ---------- Flow diagrams ----------
 * :::flow horizontal / :::flow vertical. Linear chain of boxes with
 * arrow glyphs between them. Branching and labeled arrows are not yet
 * supported — authors with those needs still reach for raw HTML. */
:where(.slide) .waxon-flow {
  display: flex;
  gap: 0.6em;
  align-items: center;
  margin: 1em 0;
  flex-wrap: wrap;
}
:where(.slide) .waxon-flow-vertical {
  flex-direction: column;
  align-items: stretch;
}
:where(.slide) .waxon-flow-node {
  padding: 0.6em 1em;
  border: 2px solid currentColor;
  border-radius: 4px;
  font-family: var(--font-mono);
  white-space: nowrap;
  text-align: center;
}
/* :::flow wide gives every node a uniform minimum width so flows with
 * varying label lengths line up cleanly instead of looking jagged. */
:where(.slide) .waxon-flow-wide .waxon-flow-node { min-width: 8em; }
:where(.slide) .waxon-flow-wide.waxon-flow-vertical .waxon-flow-node { min-width: 12em; }
/* :::flow tall / :::flow boxes — pipeline-style tall boxes (#45, #55).
 * Allow multi-line wrapping and give every node a uniform minimum height
 * so short/long labels render as consistent rectangular boxes. */
:where(.slide) .waxon-flow-tall .waxon-flow-node,
:where(.slide) .waxon-flow-boxes .waxon-flow-node {
  white-space: normal;
  min-height: 4em;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 0.8em 1em;
}
:where(.slide) .waxon-flow-node.red    { border-color: var(--color-red,    #ef4444); color: var(--color-red,    #ef4444); }
:where(.slide) .waxon-flow-node.green  { border-color: var(--color-green,  #22c55e); color: var(--color-green,  #22c55e); }
:where(.slide) .waxon-flow-node.yellow { border-color: var(--color-yellow, #eab308); color: var(--color-yellow, #eab308); }
:where(.slide) .waxon-flow-node.blue   { border-color: var(--color-blue,   #3b82f6); color: var(--color-blue,   #3b82f6); }
:where(.slide) .waxon-flow-node.aqua   { border-color: var(--color-aqua,   #06b6d4); color: var(--color-aqua,   #06b6d4); }
:where(.slide) .waxon-flow-arrow {
  font-size: 1.6em;
  opacity: 0.6;
  padding: 0 0.2em;
}
:where(.slide) .waxon-flow-arrow-dashed { opacity: 0.4; font-style: italic; }
:where(.slide) .waxon-flow-divider {
  font-size: 1.6em;
  opacity: 0.5;
  padding: 0 0.4em;
  font-family: var(--font-mono);
}

/* ---------- Timeline ----------
 * :::timeline horizontal / :::timeline vertical with :: entries. Each
 * entry gets a dot marker and label above its content body. */
:where(.slide) .waxon-timeline {
  display: flex;
  gap: 1.2em;
  margin: 1em 0;
}
:where(.slide) .waxon-timeline-horizontal { flex-direction: row; flex-wrap: wrap; }
:where(.slide) .waxon-timeline-vertical   { flex-direction: column; }
:where(.slide) .waxon-timeline-entry {
  flex: 1 1 0;
  display: flex;
  flex-direction: column;
  gap: 0.3em;
  position: relative;
  padding-top: 0.8em;
}
:where(.slide) .waxon-timeline-horizontal .waxon-timeline-entry {
  border-top: 2px solid var(--foreground2, currentColor);
}
:where(.slide) .waxon-timeline-vertical .waxon-timeline-entry {
  border-left: 2px solid var(--foreground2, currentColor);
  padding-left: 1em;
  padding-top: 0;
}
:where(.slide) .waxon-timeline-marker {
  position: absolute;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--accent, currentColor);
}
:where(.slide) .waxon-timeline-horizontal .waxon-timeline-marker {
  top: -6px;
  left: 0;
}
:where(.slide) .waxon-timeline-vertical .waxon-timeline-marker {
  top: 0;
  left: -6px;
}
:where(.slide) .waxon-timeline-label {
  font-family: var(--font-mono);
  font-weight: 600;
  font-size: 0.9em;
}
:where(.slide) .waxon-timeline-label.red    { color: var(--color-red,    #ef4444); }
:where(.slide) .waxon-timeline-label.green  { color: var(--color-green,  #22c55e); }
:where(.slide) .waxon-timeline-label.yellow { color: var(--color-yellow, #eab308); }
:where(.slide) .waxon-timeline-label.blue   { color: var(--color-blue,   #3b82f6); }
:where(.slide) .waxon-timeline-label.aqua   { color: var(--color-aqua,   #06b6d4); }
:where(.slide) .waxon-timeline-body > :first-child { margin-top: 0; }
:where(.slide) .waxon-timeline-body > :last-child  { margin-bottom: 0; }

/* ---------- Quote block ----------
 * :::quote with optional ::by attribution. Theme-aware border color. */
:where(.slide) .waxon-quote {
  border-left: 4px solid var(--accent, currentColor);
  padding: 0.4em 1em;
  margin: 1em 0;
  font-style: italic;
  font-size: 1.1em;
}
:where(.slide) .waxon-quote.red    { border-left-color: var(--color-red,    #ef4444); }
:where(.slide) .waxon-quote.green  { border-left-color: var(--color-green,  #22c55e); }
:where(.slide) .waxon-quote.yellow { border-left-color: var(--color-yellow, #eab308); }
:where(.slide) .waxon-quote.blue   { border-left-color: var(--color-blue,   #3b82f6); }
:where(.slide) .waxon-quote.aqua   { border-left-color: var(--color-aqua,   #06b6d4); }
:where(.slide) .waxon-quote > :first-child { margin-top: 0; }
:where(.slide) .waxon-quote-by {
  margin-top: 0.4em;
  font-size: 0.8em;
  font-style: normal;
  opacity: 0.6;
}

/* ---------- Stat block ----------
 * :::stat <color> with ::label / ::context. Big centered number. */
:where(.slide) .waxon-stat {
  text-align: center;
  margin: 1.2em 0;
  padding: 0.5em 0;
}
:where(.slide) .waxon-stat-number {
  font-size: 4em;
  font-weight: 700;
  line-height: 1;
  font-family: var(--font-mono);
}
:where(.slide) .waxon-stat.red    .waxon-stat-number { color: var(--color-red,    #ef4444); }
:where(.slide) .waxon-stat.green  .waxon-stat-number { color: var(--color-green,  #22c55e); }
:where(.slide) .waxon-stat.yellow .waxon-stat-number { color: var(--color-yellow, #eab308); }
:where(.slide) .waxon-stat.blue   .waxon-stat-number { color: var(--color-blue,   #3b82f6); }
:where(.slide) .waxon-stat.aqua   .waxon-stat-number { color: var(--color-aqua,   #06b6d4); }
:where(.slide) .waxon-stat-label {
  font-size: 1.1em;
  margin-top: 0.2em;
  opacity: 0.85;
}
:where(.slide) .waxon-stat-context {
  font-size: 0.8em;
  margin-top: 0.4em;
  opacity: 0.6;
}
/* Stats nested inside a grid cell shrink so the big number fits the
 * narrow column. Without this, a 4em number overflows a 3-col grid. */
:where(.slide) .waxon-grid-cell .waxon-stat { margin: 0.4em 0; padding: 0; }
:where(.slide) .waxon-grid-cell .waxon-stat-number { font-size: 2.4em; }
:where(.slide) .waxon-grid-cell .waxon-stat-label { font-size: 0.95em; }
:where(.slide) .waxon-grid-cell .waxon-stat-context { font-size: 0.75em; }

/* ---------- Badge pills ----------
 * Inline .badge-green{SHIPPED} renders as a rounded pill with a tinted
 * background. The color class is applied alongside .waxon-badge so theme
 * palette vars drive the background. */
:where(.slide) .waxon-badge {
  display: inline-block;
  padding: 0.05em 0.5em;
  border-radius: 999px;
  font-family: var(--font-mono);
  font-size: 0.75em;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  vertical-align: middle;
  line-height: 1.6;
}
:where(.slide) .waxon-badge.red    { background: color-mix(in srgb, var(--color-red,    #ef4444) 25%, transparent); color: var(--color-red,    #ef4444); border: 1px solid var(--color-red,    #ef4444); }
:where(.slide) .waxon-badge.green  { background: color-mix(in srgb, var(--color-green,  #22c55e) 25%, transparent); color: var(--color-green,  #22c55e); border: 1px solid var(--color-green,  #22c55e); }
:where(.slide) .waxon-badge.yellow { background: color-mix(in srgb, var(--color-yellow, #eab308) 25%, transparent); color: var(--color-yellow, #eab308); border: 1px solid var(--color-yellow, #eab308); }
:where(.slide) .waxon-badge.blue   { background: color-mix(in srgb, var(--color-blue,   #3b82f6) 25%, transparent); color: var(--color-blue,   #3b82f6); border: 1px solid var(--color-blue,   #3b82f6); }
:where(.slide) .waxon-badge.aqua   { background: color-mix(in srgb, var(--color-aqua,   #06b6d4) 25%, transparent); color: var(--color-aqua,   #06b6d4); border: 1px solid var(--color-aqua,   #06b6d4); }

/* ---------- Mid-slide horizontal rule ----------
 * The parser emits <hr class="waxon-hr"/> for standalone 4+ dash lines
 * so goldmark doesn't interpret them as setext underlines. */
:where(.slide) .waxon-hr {
  border: none;
  border-top: 1px solid var(--foreground2, currentColor);
  opacity: 0.4;
  margin: 1em 0;
}
:where(.slide) .waxon-hr.waxon-hr-dashed { border-top-style: dashed; }
:where(.slide) .waxon-hr.waxon-hr-dotted { border-top-style: dotted; }

/* ---------- Columns (#39) ----------
 * :::columns N wraps its body in a CSS multi-column container. Each
 * block-level child flows into the next column like a newspaper. */
:where(.slide) .waxon-columns > * { break-inside: avoid; }
:where(.slide) .waxon-columns ul,
:where(.slide) .waxon-columns ol { margin-top: 0; }

/* ---------- Footnote (#40) ----------
 * :::footnote renders small, dim annotation text anchored below the main
 * content. Authors use it for source citations and disclaimers. */
:where(.slide) .waxon-footnote {
  font-size: 0.7em;
  opacity: 0.6;
  margin-top: auto;
  padding-top: 0.8em;
  font-style: italic;
}

/* ---------- Compare brackets (#52) ----------
 * :::compare brackets replaces the full continuous pane border with
 * corner-bracket borders at each corner using CSS masks. */
:where(.slide) .waxon-compare-brackets .waxon-compare-pane {
  border: none;
  position: relative;
}
:where(.slide) .waxon-compare-brackets .waxon-compare-pane::before,
:where(.slide) .waxon-compare-brackets .waxon-compare-pane::after {
  content: '';
  position: absolute;
  width: 1.2em;
  height: 1.2em;
  border: 2px solid var(--foreground2, currentColor);
  pointer-events: none;
}
:where(.slide) .waxon-compare-brackets .waxon-compare-pane::before {
  top: 0; left: 0;
  border-right: none;
  border-bottom: none;
}
:where(.slide) .waxon-compare-brackets .waxon-compare-pane::after {
  bottom: 0; right: 0;
  border-left: none;
  border-top: none;
}

/* ---------- Slide transitions ----------
 * Applied via [data-transition="fade"] on the top-level wrapper. CSS
 * drives the crossfade; JS just swaps slide content on navigation. The
 * animation replays when the slide's inner HTML is replaced because we
 * toggle the attribute on a short delay from the render pipeline. */
[data-transition="fade"] .slide {
  animation: waxon-slide-fade 240ms ease-out;
}
@keyframes waxon-slide-fade {
  from { opacity: 0; }
  to   { opacity: 1; }
}

.footer {
  position: absolute;
  bottom: 1vmin;
  left: var(--slide-padding);
  right: calc(var(--slide-padding) + 280px);
  font-size: 0.6em;
  opacity: 0.6;
  pointer-events: none;
  display: flex;
  gap: 1em;
  align-items: baseline;
}
.footer > .footer-left { flex: 1 1 auto; text-align: left; }
.footer > .footer-center { flex: 1 1 auto; text-align: center; }
.footer > .footer-right { flex: 1 1 auto; text-align: right; }
.footer > :empty { display: none; }
.footer-counter {
  position: absolute;
  bottom: 1vmin;
  left: 50%;
  transform: translateX(-50%);
  font-size: 0.6em;
  opacity: 0.6;
  pointer-events: none;
  font-family: var(--font-mono);
}

.progress {
  position: absolute;
  bottom: 0;
  left: 0;
  height: 4px;
  background: var(--accent);
  box-shadow: 0 0 8px color-mix(in srgb, var(--accent) 50%, transparent);
  transition: width 0.3s ease;
  z-index: 50;
}

/* ---------- Floating action bar ---------- */
.fab {
  position: absolute;
  bottom: 1.5vmin;
  right: 1.5vmin;
  display: flex;
  gap: 6px;
  z-index: 250; /* above panels so panels don't hide it */
  background: var(--chrome-bg);
  border: 1px solid var(--chrome-border);
  border-radius: 8px;
  padding: 4px;
  box-shadow: 0 4px 16px color-mix(in srgb, #000 30%, transparent);
  transition: right 0.18s ease-out;
}
/* When a right-side panel is open, slide the FAB left so its buttons
 * don't overlap the panel content. Panel width matches .panel rule. */
.app:has(.panel.open) .fab {
  right: calc(min(420px, 88vw) + 1.5vmin);
}
.fab .group { display: flex; gap: 2px; }
.fab .divider {
  width: 1px;
  background: var(--chrome-border);
  margin: 4px 2px;
  align-self: stretch;
}
.fab button {
  background: transparent;
  color: var(--chrome-fg);
  border: 1px solid transparent;
  border-radius: 5px;
  padding: 6px 10px;
  font: inherit;
  font-size: var(--chrome-font);
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
  letter-spacing: 0.02em;
}
.fab button:hover { background: var(--chrome-hover); }
.fab button.active {
  background: var(--chrome-active);
  border-color: var(--accent);
  color: var(--chrome-fg);
}
.fab button kbd {
  background: color-mix(in srgb, var(--chrome-fg) 12%, transparent);
  border: 1px solid var(--chrome-border);
  padding: 1px 6px;
  border-radius: 3px;
  font-family: var(--font-mono);
  font-size: var(--chrome-font-sm);
  line-height: 1;
}

/* ---------- Side panels ---------- */
.panel {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: min(420px, 88vw);
  background: var(--chrome-bg);
  color: var(--chrome-fg);
  border-left: 1px solid var(--chrome-border);
  transform: translateX(100%);
  transition: transform 0.18s ease-out;
  display: flex;
  flex-direction: column;
  z-index: 200;
  font-size: var(--chrome-font);
  box-shadow: -8px 0 24px color-mix(in srgb, #000 35%, transparent);
}
.panel.open { transform: translateX(0); }
.panel-header {
  padding: 12px 16px;
  border-bottom: 1px solid var(--chrome-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 600;
  font-size: var(--chrome-font-lg);
}
.panel-header .close {
  background: none;
  border: none;
  color: inherit;
  font-size: 22px;
  line-height: 1;
  cursor: pointer;
  padding: 0 6px;
  border-radius: 4px;
}
.panel-header .close:hover { background: var(--chrome-hover); }
.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 14px 16px;
}
.panel-body h3 {
  font-size: var(--chrome-font-sm);
  text-transform: uppercase;
  letter-spacing: 0.07em;
  opacity: 0.7;
  margin: 18px 0 6px;
  border-bottom: 1px solid var(--chrome-border);
  padding-bottom: 4px;
}
.panel-body h3:first-child { margin-top: 0; }
.panel-body .empty {
  opacity: 0.6;
  font-style: italic;
  padding: 12px 0;
}

.comment-item, .variant-item, .deck-item {
  padding: 10px 12px;
  margin-bottom: 6px;
  border-left: 3px solid var(--chrome-border);
  border-radius: 4px;
  background: color-mix(in srgb, var(--chrome-fg) 4%, transparent);
}
.comment-item .meta {
  font-size: var(--chrome-font-sm);
  opacity: 0.75;
  margin-bottom: 4px;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.comment-item .meta .author { color: var(--accent); font-weight: 600; }
.comment-item .meta .scope {
  background: color-mix(in srgb, var(--chrome-fg) 10%, transparent);
  padding: 1px 6px;
  border-radius: 3px;
}
.comment-item .text { white-space: pre-wrap; word-wrap: break-word; }
.comment-item.clickable { cursor: pointer; }
.comment-item.clickable:hover { background: var(--chrome-hover); }

.variant-item { cursor: pointer; }
.variant-item:hover { background: var(--chrome-hover); }
.variant-item.active {
  border-left-color: var(--accent);
  background: var(--chrome-active);
}
.variant-item .name { font-weight: 600; font-family: var(--font-mono); font-size: var(--chrome-font); }
.variant-item .preview { opacity: 0.7; font-size: var(--chrome-font-sm); margin-top: 4px; line-height: 1.4; }
.variant-item .compare-from {
  background: color-mix(in srgb, var(--accent) 30%, transparent);
  border: 1px solid var(--accent);
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 3px;
  margin-left: 6px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.deck-item { cursor: pointer; text-decoration: none; color: inherit; display: block; }
.deck-item:hover { background: var(--chrome-hover); }
.deck-item.active { border-left-color: var(--accent); background: var(--chrome-active); }
.deck-item .title { font-weight: 600; }
.deck-item .path { opacity: 0.6; font-size: var(--chrome-font-sm); font-family: var(--font-mono); margin-top: 2px; }

/* ---------- Theme picker ----------
 * Each swatch uses its own scoped style block (injected by JS) to preview
 * the theme's --slide-bg / --slide-fg / --accent without affecting the
 * live slide. Click a swatch to swap the active theme. */
.theme-item {
  cursor: pointer;
  display: block;
  padding: 10px 12px;
  margin-bottom: 6px;
  border: 1px solid var(--chrome-border);
  border-radius: 6px;
  transition: border-color 0.12s, background 0.12s;
}
.theme-item:hover { border-color: var(--accent); }
.theme-item.active {
  border-color: var(--accent);
  background: var(--chrome-active);
}
.theme-item .name {
  font-weight: 600;
  font-family: var(--font-mono);
  font-size: var(--chrome-font);
  margin-bottom: 3px;
}
.theme-item .desc {
  opacity: 0.7;
  font-size: var(--chrome-font-sm);
  line-height: 1.35;
  margin-bottom: 8px;
}
.theme-swatch {
  display: flex;
  height: 28px;
  border-radius: 4px;
  overflow: hidden;
  border: 1px solid var(--chrome-border);
  font-family: var(--font-mono);
  font-size: 10px;
  align-items: stretch;
}
.theme-swatch > span {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  letter-spacing: 0.03em;
}

/* ---------- Speaker notes panel ---------- */
.notes-current {
  padding: 14px 16px;
  border: 1px solid var(--chrome-border);
  border-radius: 6px;
  margin-bottom: 12px;
  background: color-mix(in srgb, var(--accent) 6%, transparent);
}
.notes-current h3 {
  font-size: var(--chrome-font);
  font-weight: 600;
  margin: 0 0 10px;
  color: var(--accent);
}
.notes-current .note {
  padding: 8px 10px;
  margin-bottom: 6px;
  background: var(--chrome-bg);
  border-left: 2px solid var(--accent);
  border-radius: 3px;
  font-size: var(--chrome-font);
  line-height: 1.45;
  white-space: pre-wrap;
  word-wrap: break-word;
}
.notes-current .note.ai {
  border-left-color: color-mix(in srgb, var(--accent) 40%, var(--chrome-fg) 60%);
  opacity: 0.88;
}
.notes-current .note .tag {
  display: inline-block;
  padding: 1px 5px;
  margin-right: 4px;
  font-size: 9px;
  font-family: var(--font-mono);
  text-transform: uppercase;
  background: color-mix(in srgb, var(--accent) 25%, transparent);
  color: var(--accent);
  border-radius: 3px;
  vertical-align: middle;
}
.notes-next {
  padding: 10px 14px;
  border: 1px dashed var(--chrome-border);
  border-radius: 6px;
  margin-bottom: 14px;
  opacity: 0.75;
}
.notes-next h4 {
  font-size: var(--chrome-font-sm);
  font-weight: 500;
  margin: 0;
  color: var(--chrome-fg);
}
.notes-outline-heading {
  font-size: var(--chrome-font-sm);
  font-weight: 600;
  margin: 0 0 6px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  opacity: 0.7;
}
.notes-outline-item {
  display: flex;
  gap: 8px;
  padding: 6px 10px;
  border-radius: 4px;
  font-size: var(--chrome-font-sm);
  cursor: pointer;
  align-items: center;
}
.notes-outline-item:hover { background: var(--chrome-hover); }
.notes-outline-item.active {
  background: var(--chrome-active);
  border-left: 2px solid var(--accent);
  padding-left: 8px;
}
.notes-outline-item .n {
  font-family: var(--font-mono);
  opacity: 0.55;
  min-width: 22px;
  text-align: right;
}
.notes-outline-item .title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.notes-outline-item .count {
  font-family: var(--font-mono);
  font-size: 10px;
  padding: 1px 5px;
  background: color-mix(in srgb, var(--accent) 20%, transparent);
  color: var(--accent);
  border-radius: 8px;
}

.comment-form {
  padding: 12px 16px;
  border-top: 1px solid var(--chrome-border);
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: color-mix(in srgb, var(--chrome-fg) 3%, transparent);
}
.comment-form label {
  font-size: var(--chrome-font-sm);
  opacity: 0.75;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.comment-form input, .comment-form textarea, .comment-form select {
  background: color-mix(in srgb, var(--slide-bg) 70%, #000 30%);
  border: 1px solid var(--chrome-border);
  color: var(--chrome-fg);
  padding: 7px 10px;
  border-radius: 4px;
  font: inherit;
  font-size: var(--chrome-font);
  font-family: var(--font-body);
}
.comment-form textarea { min-height: 5em; resize: vertical; }
.comment-form input:focus, .comment-form textarea:focus, .comment-form select:focus {
  outline: 2px solid var(--accent);
  outline-offset: -1px;
  border-color: transparent;
}
.comment-form button {
  background: var(--accent);
  color: #fff;
  border: none;
  padding: 9px 14px;
  border-radius: 4px;
  font: inherit;
  font-weight: 600;
  font-size: var(--chrome-font);
  cursor: pointer;
}
.comment-form button:hover:not(:disabled) { filter: brightness(1.15); }
.comment-form button:disabled { opacity: 0.55; cursor: wait; }
.comment-form .target-locked {
  font-size: var(--chrome-font-sm);
  opacity: 0.7;
  margin-top: -4px;
}
.comment-form .status { min-height: 1.2em; font-size: var(--chrome-font-sm); }
.comment-form .status.error::before { content: "✗ "; color: #ff8a80; }
.comment-form .status.ok::before { content: "✓ "; color: #80ff9d; }
.comment-form .status.error { color: #ff8a80; }
.comment-form .status.ok { color: #80ff9d; }

/* ---------- Help overlay ---------- */
.help-overlay {
  position: fixed;
  inset: 0;
  background: var(--backdrop);
  display: none;
  align-items: center;
  justify-content: center;
  z-index: 500;
  padding: 4vmin;
}
.help-overlay.open { display: flex; }
.help-card {
  background: var(--chrome-bg);
  color: var(--chrome-fg);
  border: 1px solid var(--chrome-border);
  border-radius: 12px;
  padding: 28px;
  max-width: 720px;
  width: 100%;
  max-height: 90vh;
  overflow-y: auto;
  font-size: 14px;
  outline: none;
  box-shadow: 0 12px 40px color-mix(in srgb, #000 45%, transparent);
}
.help-card h2 {
  font-family: var(--font-heading);
  margin-bottom: 16px;
  color: var(--accent);
  font-size: 22px;
}
.help-card h3 {
  margin-top: 22px;
  margin-bottom: 8px;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  opacity: 0.75;
  border-bottom: 1px solid var(--chrome-border);
  padding-bottom: 4px;
}
.help-card table { width: 100%; border-collapse: collapse; }
.help-card td { padding: 6px 8px; vertical-align: top; }
.help-card td:first-child { width: 38%; }
.help-card kbd {
  display: inline-block;
  background: color-mix(in srgb, var(--chrome-fg) 12%, transparent);
  border: 1px solid var(--chrome-border);
  border-bottom-width: 2px;
  border-radius: 4px;
  padding: 2px 7px;
  font-family: var(--font-mono);
  font-size: 12px;
  margin-right: 4px;
  line-height: 1;
}
.help-card .hint {
  margin-top: 18px;
  padding: 10px 14px;
  background: color-mix(in srgb, var(--accent) 12%, transparent);
  border-left: 3px solid var(--accent);
  border-radius: 4px;
  font-size: 12px;
}

/* ---------- WS reconnect indicator ---------- */
.ws-status {
  position: absolute;
  top: 8px;
  left: 8px;
  background: color-mix(in srgb, #000 70%, transparent);
  color: #ffb74d;
  font-size: var(--chrome-font-sm);
  padding: 4px 10px;
  border-radius: 4px;
  border: 1px solid #ffb74d;
  z-index: 150;
  display: none;
  font-family: var(--font-mono);
}
.ws-status.show { display: block; }

{{if .TerminalEffects}}
/* Terminal scanline effect — scoped to the deck area so it never overlays
   chrome (banner, FAB, panels, help overlay). */
.deck-area.scanline::after {
  content: '';
  position: absolute;
  inset: 0;
  background: repeating-linear-gradient(
    transparent 0px,
    transparent 2px,
    rgba(0,0,0,0.04) 2px,
    rgba(0,0,0,0.04) 4px
  );
  pointer-events: none;
  z-index: 1;
}
{{end}}
</style>
{{range .Fonts}}<link rel="stylesheet" href="{{.}}">
{{end}}<style id="theme-css">
{{if .ThemeCSSInline}}{{.ThemeCSSInline}}{{else}}{{themeCSS .Theme}}{{end}}
</style>
</head>
<body{{if .Transition}} data-transition="{{.Transition}}"{{end}}>
<div class="app" id="app" tabindex="-1">

<div class="banner" id="banner" role="status" aria-live="polite">
  <span class="label" id="banner-label">Preview</span>
  <span id="banner-pills"></span>
  <span class="spacer"></span>
  <button id="banner-hide" type="button" aria-label="Hide this banner">Hide</button>
  <button id="banner-reset" type="button" aria-label="Reset to default view">Reset</button>
</div>

<div class="deck-area{{if .TerminalEffects}} scanline{{end}}" id="deck-area">
  <div class="pane" id="pane-main">
    <div class="pane-label" id="main-label" style="display:none;">main</div>
    <div class="slide" id="render-main"></div>
{{if or .Footer .FooterLeft .FooterRight}}    <div class="footer" id="footer-main">
      <div class="footer-left" data-footer-tpl="{{.FooterLeft}}">{{.FooterLeft}}</div>
      <div class="footer-center" data-footer-tpl="{{.Footer}}">{{.Footer}}</div>
      <div class="footer-right" data-footer-tpl="{{.FooterRight}}">{{.FooterRight}}</div>
    </div>
{{end}}    <div class="footer-counter" id="counter-main"></div>
  </div>
  <div class="pane compare-pane" id="pane-compare" style="display:none;">
    <div class="pane-label" id="compare-label">compare</div>
    <div class="slide" id="render-compare"></div>
  </div>
  <div class="progress" id="progress" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-label="Slide progress"></div>
  <div class="ws-status" id="ws-status">disconnected — reconnecting…</div>

  <div class="fab" role="toolbar" aria-label="Slide controls">
    <div class="group">
      <button type="button" data-action="variants" aria-label="Toggle variants panel (v)" title="Variants — v"><kbd>v</kbd> variants</button>
      <button type="button" data-action="comments" aria-label="Toggle comments panel (c)" title="Comments — c"><kbd>c</kbd> comments</button>
      <button type="button" data-action="decks" aria-label="Toggle decks panel (d)" title="Decks — d"><kbd>d</kbd> decks</button>
      <button type="button" data-action="themes" aria-label="Toggle themes panel (t)" title="Themes — t"><kbd>t</kbd> themes</button>
      <button type="button" data-action="notes" aria-label="Toggle speaker notes (N)" title="Speaker notes — Shift+N"><kbd>N</kbd> notes</button>
    </div>
    <div class="divider"></div>
    <div class="group">
      <button type="button" data-action="compare" aria-pressed="false" aria-label="Toggle compare mode (x)" title="Compare main vs variant — x"><kbd>x</kbd> compare</button>
      <button type="button" data-action="help" aria-label="Show keyboard shortcuts (?)" title="Keyboard shortcuts — ?"><kbd>?</kbd></button>
    </div>
  </div>
</div>

<aside class="panel" id="panel-variants" data-panel="variants" role="dialog" aria-modal="false" aria-labelledby="variants-title" hidden>
  <div class="panel-header">
    <span id="variants-title">Variants — slide <span id="variants-slide-num">1</span></span>
    <button class="close" type="button" aria-label="Close variants panel">×</button>
  </div>
  <div class="panel-body" id="variants-body"></div>
</aside>

<aside class="panel" id="panel-comments" data-panel="comments" role="dialog" aria-modal="false" aria-labelledby="comments-title" hidden>
  <div class="panel-header">
    <span id="comments-title">Comments</span>
    <button class="close" type="button" aria-label="Close comments panel">×</button>
  </div>
  <div class="panel-body" id="comments-body"></div>
  <form class="comment-form" id="comment-form">
    <label>
      Author
      <input type="text" id="comment-author" placeholder="your name" autocomplete="username" required>
    </label>
    <label>
      Target slide
      <select id="comment-target" aria-describedby="target-locked"></select>
    </label>
    <div class="target-locked" id="target-locked" hidden>Target locked while you're composing — click <em>Discard</em> to release.</div>
    <label>
      Comment
      <textarea id="comment-text" placeholder="leave a note for collaborators…" required></textarea>
    </label>
    <button type="submit" id="comment-submit">Add comment</button>
    <div class="status" id="comment-status" aria-live="polite"></div>
  </form>
</aside>

<aside class="panel" id="panel-decks" data-panel="decks" role="dialog" aria-modal="false" aria-labelledby="decks-title" hidden>
  <div class="panel-header">
    <span id="decks-title">Switch deck</span>
    <button class="close" type="button" aria-label="Close decks panel">×</button>
  </div>
  <div class="panel-body" id="decks-body"></div>
</aside>

<aside class="panel" id="panel-themes" data-panel="themes" role="dialog" aria-modal="false" aria-labelledby="themes-title" hidden>
  <div class="panel-header">
    <span id="themes-title">Themes</span>
    <button class="close" type="button" aria-label="Close themes panel">×</button>
  </div>
  <div class="panel-body" id="themes-body"></div>
</aside>

<aside class="panel" id="panel-notes" data-panel="notes" role="dialog" aria-modal="false" aria-labelledby="notes-title" hidden>
  <div class="panel-header">
    <span id="notes-title">Speaker notes</span>
    <button class="close" type="button" aria-label="Close notes panel">×</button>
  </div>
  <div class="panel-body" id="notes-body"></div>
</aside>

<div class="help-overlay" id="help-overlay" role="dialog" aria-modal="true" aria-labelledby="help-title">
  <div class="help-card" tabindex="-1">
    <h2 id="help-title">Keyboard shortcuts</h2>
    <h3>Navigation</h3>
    <table>
      <tr><td><kbd>→</kbd><kbd>Space</kbd><kbd>Enter</kbd></td><td>Next slide / reveal next pause</td></tr>
      <tr><td><kbd>←</kbd><kbd>Backspace</kbd></td><td>Previous slide / hide last reveal</td></tr>
      <tr><td><kbd>Shift</kbd>+<kbd>→</kbd></td><td>Jump to next slide (skip reveals)</td></tr>
      <tr><td><kbd>Shift</kbd>+<kbd>←</kbd></td><td>Jump to previous slide (skip reveals)</td></tr>
      <tr><td><kbd>n</kbd></td><td>Next slide (alias)</td></tr>
      <tr><td><kbd>p</kbd></td><td>Previous slide (alias)</td></tr>
      <tr><td><kbd>Home</kbd> / <kbd>g</kbd> <kbd>g</kbd></td><td>Jump to first slide</td></tr>
      <tr><td><kbd>End</kbd> / <kbd>G</kbd></td><td>Jump to last slide</td></tr>
      <tr><td><kbd>1</kbd>–<kbd>9</kbd> · <kbd>0</kbd></td><td>Jump to slide 1–9 (0 = slide 10)</td></tr>
    </table>
    <h3>Panels</h3>
    <table>
      <tr><td><kbd>v</kbd></td><td>Toggle variants panel</td></tr>
      <tr><td><kbd>c</kbd></td><td>Toggle comments panel</td></tr>
      <tr><td><kbd>d</kbd></td><td>Toggle decks panel</td></tr>
      <tr><td><kbd>t</kbd></td><td>Toggle themes panel</td></tr>
      <tr><td><kbd>Shift</kbd>+<kbd>N</kbd></td><td>Toggle speaker notes panel</td></tr>
      <tr><td><kbd>?</kbd></td><td>Toggle this help</td></tr>
      <tr><td><kbd>Esc</kbd></td><td>Close help / panel / exit fullscreen</td></tr>
    </table>
    <h3>Variants &amp; view</h3>
    <table>
      <tr><td><kbd>]</kbd> / <kbd>[</kbd></td><td>Cycle next / previous variant on this slide</td></tr>
      <tr><td><kbd>x</kbd></td><td>Toggle compare mode (main vs active variant)</td></tr>
      <tr><td><kbd>T</kbd></td><td>Cycle to next theme (shift+t)</td></tr>
      <tr><td><kbd>+</kbd> / <kbd>-</kbd></td><td>Zoom in / out (reflows content)</td></tr>
      <tr><td><kbd>r</kbd></td><td>Reset variant, compare, zoom</td></tr>
      <tr><td><kbd>f</kbd></td><td>Toggle fullscreen</td></tr>
    </table>
    <div class="hint">
      Click a comment in the Comments panel to jump to that slide. The
      target dropdown locks while you're composing so navigation can't
      misroute your post — press <em>Discard</em> on the form to release it.
    </div>
  </div>
</div>

</div>

<script>
(function() {
  'use strict';

  var deck = {{.DeckJSON}};
  var decks = {{.DecksJSON}};
  var themes = {{.ThemesJSON}};
  var state = {{.StateJSON}};

  var total = deck.length;
  if (total === 0) {
    document.getElementById('render-main').innerHTML =
      '<p style="opacity:.7;font-style:italic;">This deck has no slides.</p>';
    return;
  }
  var current = 0;
  // activeVariant[i] = "" for main, or variant name string
  var activeVariant = {};
  var compareMode = false;
  // pauseStep[i] = how many pauses are revealed on slide i. 0 = none.
  // Range: 0..slide.pauses. The slide HTML is rendered into chunks split
  // at the original pause markers, and chunks beyond pauseStep get the
  // .waxon-hidden class.
  var pauseStep = {};
  var openPanel = null;
  var lastG = 0;
  var bannerHidden = false;
  // Set when the user is composing a comment, freezing the target dropdown.
  var commentComposing = false;
  // Tracks the slide+variant the comment was started on.
  var commentTarget = null;

  function $(id) { return document.getElementById(id); }
  function $$(sel, root) { return Array.prototype.slice.call((root || document).querySelectorAll(sel)); }

  var app = $('app');
  var renderMain = $('render-main');
  var renderCompare = $('render-compare');
  var paneCompare = $('pane-compare');
  var paneMain = $('pane-main');
  var deckArea = $('deck-area');
  var progress = $('progress');
  var counter = $('counter-main');
  var banner = $('banner');
  var bannerPills = $('banner-pills');
  var bannerLabel = $('banner-label');
  var bannerReset = $('banner-reset');
  var bannerHide = $('banner-hide');
  var helpOverlay = $('help-overlay');
  var helpCard = helpOverlay.querySelector('.help-card');
  var compareLabel = $('compare-label');
  var mainLabel = $('main-label');
  var wsStatus = $('ws-status');
  var commentForm = $('comment-form');
  var commentSubmit = $('comment-submit');
  var commentStatus = $('comment-status');
  var commentTextEl = $('comment-text');
  var commentAuthorEl = $('comment-author');
  var commentTargetEl = $('comment-target');
  var commentTargetLocked = $('target-locked');

  var lastFocusBeforePanel = null;

  function getSlide(i) { return deck[i] || deck[0]; }

  function updateFooter(n, total) {
    var footer = document.getElementById('footer-main');
    if (!footer) return;
    var regions = footer.querySelectorAll('[data-footer-tpl]');
    for (var i = 0; i < regions.length; i++) {
      var tpl = regions[i].getAttribute('data-footer-tpl') || '';
      regions[i].textContent = tpl
        .replace(/\{n\}/g, n)
        .replace(/\{page\}/g, n)
        .replace(/\{total\}/g, total);
    }
  }

  function activeView(i) {
    var s = getSlide(i);
    var variantName = activeVariant[i] || '';
    if (!variantName) return { kind: 'main', slide: s, html: s.html, pauses: s.pauses, name: '' };
    for (var k = 0; k < s.variants.length; k++) {
      if (s.variants[k].name === variantName) {
        return { kind: 'variant', slide: s, variant: s.variants[k], html: s.variants[k].html, pauses: s.variants[k].pauses, name: variantName };
      }
    }
    activeVariant[i] = '';
    return { kind: 'main', slide: s, html: s.html, pauses: s.pauses, name: '' };
  }

  // Parser replaces <!-- pause --> directives with <div class="waxon-pause">
  // sentinels, which survive markdown rendering. We walk the slide's top-level
  // children and assign each to a group — group 0 is pre-pause content, group
  // 1 is everything between pause #1 and pause #2, etc. Groups > step are
  // hidden. The sentinel div itself is never visible (CSS display:none).
  function applyPauseReveal(container, pauses, step) {
    if (!pauses || pauses <= 0) return;
    var children = Array.prototype.filter.call(container.children, function(el) {
      return el.tagName !== 'STYLE';
    });
    if (children.length === 0) return;
    var group = 0;
    children.forEach(function(el) {
      if (el.classList && el.classList.contains('waxon-pause')) {
        group++;
        return;
      }
      if (group > step) el.classList.add('waxon-hidden');
    });
  }

  function totalPauses(i) {
    var v = activeView(i);
    return v.pauses || 0;
  }

  function render() {
    var view = activeView(current);
    renderMain.innerHTML = view.html;
    // Retrigger CSS transitions (like [data-transition="fade"]) that run
    // on the .slide element. Removing and re-adding the class forces the
    // browser to restart any animation declared on .slide, so each
    // navigation re-plays the effect instead of running it once on load.
    renderMain.className = '';
    void renderMain.offsetWidth;
    renderMain.classList.add('slide');
    if (view.slide && view.slide.class) {
      view.slide.class.split(/\s+/).forEach(function(c) {
        if (c) renderMain.classList.add(c);
      });
    }
    if (view.slide && view.slide.bgImage) {
      renderMain.style.backgroundImage = view.slide.bgImage;
      renderMain.style.backgroundSize = 'cover';
      renderMain.style.backgroundPosition = 'center';
    } else {
      renderMain.style.backgroundImage = '';
      renderMain.style.backgroundSize = '';
      renderMain.style.backgroundPosition = '';
    }
    if (view.slide && view.slide.bg) {
      renderMain.style.background = view.slide.bg;
    } else if (!(view.slide && view.slide.bgImage)) {
      renderMain.style.background = '';
    }
    if (view.slide && view.slide.valign) {
      renderMain.setAttribute('data-valign', view.slide.valign);
    } else {
      renderMain.removeAttribute('data-valign');
    }
    if (view.slide && view.slide.transition) {
      renderMain.setAttribute('data-transition', view.slide.transition);
    } else {
      renderMain.removeAttribute('data-transition');
    }
    if (view.slide && view.slide.id) {
      renderMain.setAttribute('data-slide-id', view.slide.id);
    } else {
      renderMain.removeAttribute('data-slide-id');
    }
    applyPauseReveal(renderMain, view.pauses, pauseStep[current] || 0);

    progress.style.width = ((current + 1) / total * 100) + '%';
    progress.setAttribute('aria-valuenow', String(Math.round((current + 1) / total * 100)));
    if (counter) counter.textContent = (current + 1) + ' / ' + total;
    updateFooter(current + 1, total);

    if (compareMode) {
      var s = getSlide(current);
      if (s.variants.length > 0) {
        var rightHtml, rightLabel;
        if (view.kind === 'main') {
          // Compare main vs the first variant (or active selection if any).
          rightHtml = s.variants[0].html;
          rightLabel = 'variant: ' + s.variants[0].name;
        } else {
          // We're viewing a specific variant: compare against main.
          rightHtml = s.html;
          rightLabel = 'main';
        }
        renderCompare.innerHTML = rightHtml;
        compareLabel.textContent = rightLabel;
        mainLabel.style.display = '';
        mainLabel.textContent = view.kind === 'main' ? 'main' : ('variant: ' + view.name);
        paneCompare.style.display = '';
        deckArea.classList.add('compare');
      } else {
        // No variants — show an explanatory empty pane instead of silently
        // collapsing back to single view.
        renderCompare.innerHTML =
          '<p style="opacity:.7;font-style:italic;padding:1em;">' +
          'This slide has no variants to compare. Add one with ' +
          '<code>---variant: name</code>.</p>';
        compareLabel.textContent = 'no variants';
        mainLabel.style.display = '';
        mainLabel.textContent = 'main';
        paneCompare.style.display = '';
        deckArea.classList.add('compare');
      }
    } else {
      paneCompare.style.display = 'none';
      mainLabel.style.display = 'none';
      deckArea.classList.remove('compare');
    }

    updateBanner();
    updateFabActive();
    if (openPanel === 'variants') renderVariantsPanel();
    if (openPanel === 'comments') renderCommentsPanel();
    if (openPanel === 'notes') renderNotesPanel();
    refreshCommentTargets();

    // Prefer the named slide ID when one is set, so named anchors like
    // "#intro" survive re-renders and aren't overwritten with position
    // numbers. Position-based hashes remain the fallback.
    var hashBase = (view.slide && view.slide.id) ? view.slide.id : String(current + 1);
    var hash = '#' + hashBase;
    if (activeVariant[current]) hash += '/' + encodeURIComponent(activeVariant[current]);
    if (location.hash !== hash) history.replaceState(null, '', hash);
  }

  function updateBanner() {
    var pills = [];
    if (state.themeOverridden) {
      pills.push('theme: <span class="pill">' + escapeHTML(state.activeTheme) + '</span>');
    }
    if (activeVariant[current]) {
      pills.push('variant: <span class="pill">' + escapeHTML(activeVariant[current]) + '</span>');
    }
    if (compareMode) {
      pills.push('<span class="pill">compare</span>');
    }
    if (pills.length === 0 || bannerHidden) {
      banner.classList.remove('active');
      bannerPills.innerHTML = '';
    } else {
      banner.classList.add('active');
      bannerLabel.textContent = 'Preview';
      bannerPills.innerHTML = pills.join(' · ');
    }
  }

  function updateFabActive() {
    $$('.fab button').forEach(function(b) {
      var action = b.getAttribute('data-action');
      var active = false;
      if (action === 'compare') {
        active = compareMode;
        b.setAttribute('aria-pressed', active ? 'true' : 'false');
      } else if (action === openPanel) {
        active = true;
      } else if (action === 'help') {
        active = helpOverlay.classList.contains('open');
      }
      b.classList.toggle('active', active);
    });
  }

  bannerReset.addEventListener('click', function() {
    activeVariant = {};
    compareMode = false;
    bannerHidden = false;
    if (state.themeOverridden) {
      // Theme override is server-side; we can't undo it client-side, but
      // tell the user clearly.
      commentStatus.textContent = '';
      banner.classList.add('active');
      bannerLabel.textContent = 'Theme override is server-side';
      bannerPills.innerHTML = 'restart waxon without --theme to clear';
      return;
    }
    render();
  });

  bannerHide.addEventListener('click', function() {
    bannerHidden = true;
    updateBanner();
  });

  function escapeHTML(s) {
    return String(s).replace(/[&<>"']/g, function(c) {
      return ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c];
    });
  }

  function go(n) {
    if (n < 0) n = 0;
    if (n >= total) n = total - 1;
    if (n === current) return;
    current = n;
    pauseStep[current] = 0;
    render();
  }
  function next() {
    var pauses = totalPauses(current);
    var step = pauseStep[current] || 0;
    if (step < pauses) {
      pauseStep[current] = step + 1;
      render();
      return;
    }
    if (current >= total - 1) return; // end of deck — no-op intentionally
    go(current + 1);
  }
  function prev() {
    var step = pauseStep[current] || 0;
    if (step > 0) {
      pauseStep[current] = step - 1;
      render();
      return;
    }
    if (current <= 0) return;
    go(current - 1);
  }

  function cycleVariant(dir) {
    var s = getSlide(current);
    if (s.variants.length === 0) {
      flashBanner('No variants on this slide');
      return;
    }
    var names = [''].concat(s.variants.map(function(v) { return v.name; }));
    var cur = activeVariant[current] || '';
    var i = names.indexOf(cur);
    if (i === -1) i = 0;
    i = (i + dir + names.length) % names.length;
    activeVariant[current] = names[i];
    pauseStep[current] = 0;
    render();
  }

  function setVariant(name) {
    activeVariant[current] = name || '';
    pauseStep[current] = 0;
    render();
  }

  function flashBanner(msg) {
    banner.classList.add('active');
    bannerLabel.textContent = msg;
    bannerPills.innerHTML = '';
    setTimeout(function() {
      bannerLabel.textContent = 'Preview';
      updateBanner();
    }, 1500);
  }

  function togglePanel(name) {
    if (openPanel === name) {
      closePanels();
      return;
    }
    closePanels();
    var el = document.querySelector('[data-panel="' + name + '"]');
    if (el) {
      lastFocusBeforePanel = document.activeElement;
      el.removeAttribute('hidden');
      el.classList.add('open');
      openPanel = name;
      if (name === 'variants') renderVariantsPanel();
      if (name === 'comments') renderCommentsPanel();
      if (name === 'decks') renderDecksPanel();
      if (name === 'themes') renderThemesPanel();
      if (name === 'notes') renderNotesPanel();
      // Move focus into the panel header for screen reader users.
      // preventScroll: hidden panels extend the app's scrollWidth and a
      // plain focus() scrolls the app 420px left, exposing the hidden ones.
      var firstFocusable = el.querySelector('button, [href], input, select, textarea');
      if (firstFocusable) firstFocusable.focus({ preventScroll: true });
      app.scrollLeft = 0;
      updateFabActive();
    }
  }
  function closePanels() {
    $$('.panel.open').forEach(function(el) {
      el.classList.remove('open');
      // hide attribute set after transition; do immediately for a11y
      el.setAttribute('hidden', '');
    });
    openPanel = null;
    if (lastFocusBeforePanel && lastFocusBeforePanel.focus) {
      lastFocusBeforePanel.focus({ preventScroll: true });
    } else {
      app.focus({ preventScroll: true });
    }
    app.scrollLeft = 0;
    updateFabActive();
  }
  function toggleHelp() {
    if (helpOverlay.classList.contains('open')) closeHelp();
    else openHelp();
  }
  function openHelp() {
    helpOverlay.classList.add('open');
    helpCard.focus();
    updateFabActive();
  }
  function closeHelp() {
    helpOverlay.classList.remove('open');
    app.focus();
    updateFabActive();
  }

  function renderVariantsPanel() {
    var body = $('variants-body');
    var num = $('variants-slide-num');
    num.textContent = (current + 1);
    var s = getSlide(current);
    var html = '';
    var cur = activeVariant[current] || '';
    var fromMark = compareMode ? '<span class="compare-from">left</span>' : '';
    html += '<div class="variant-item ' + (cur === '' ? 'active' : '') + '" data-variant="" tabindex="0">' +
      '<div class="name">main' + (cur === '' ? fromMark : '') + '</div>' +
      '<div class="preview">' + escapeHTML(stripTags(s.html).slice(0, 120)) + '</div>' +
      '</div>';
    if (s.variants.length === 0) {
      html += '<p class="empty">No variants on this slide. Add one with <code>---variant: name</code>.</p>';
    } else {
      s.variants.forEach(function(v) {
        var isActive = cur === v.name;
        html += '<div class="variant-item ' + (isActive ? 'active' : '') + '" data-variant="' + escapeHTML(v.name) + '" tabindex="0">' +
          '<div class="name">' + escapeHTML(v.name) + (isActive ? fromMark : '') + '</div>' +
          '<div class="preview">' + escapeHTML(stripTags(v.html).slice(0, 120)) + '</div>' +
          '</div>';
      });
    }
    body.innerHTML = html;
    body.querySelectorAll('.variant-item').forEach(function(el) {
      el.addEventListener('click', function() {
        setVariant(el.getAttribute('data-variant'));
      });
      el.addEventListener('keydown', function(e) {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          setVariant(el.getAttribute('data-variant'));
        }
      });
    });
  }

  function stripTags(html) {
    var d = document.createElement('div');
    d.innerHTML = html;
    return (d.textContent || '').replace(/\s+/g, ' ').trim();
  }

  function slideTitle(s) {
    var d = document.createElement('div');
    d.innerHTML = s.html;
    var h = d.querySelector('h1, h2, h3');
    return h ? h.textContent.trim() : '';
  }

  function renderNotesPanel() {
    // Show notes for the active slide in a "now speaking" block, followed
    // by a condensed outline of the whole deck so the presenter can
    // preview what's coming.
    var body = $('notes-body');
    var s = getSlide(current);
    var html = '';
    var notes = (s.notes || []).slice();
    var aiNotes = (s.aiNotes || []).slice();
    // If a variant is active, prefer its notes.
    var activeV = activeVariant[current] || '';
    if (activeV) {
      for (var i = 0; i < s.variants.length; i++) {
        if (s.variants[i].name === activeV) {
          notes = (s.variants[i].notes || []).slice();
          aiNotes = (s.variants[i].aiNotes || []).slice();
          break;
        }
      }
    }
    var t = slideTitle(s);
    html += '<div class="notes-current">';
    html += '<h3>Now speaking — slide ' + (current + 1) + (t ? ' · ' + escapeHTML(t) : '') + '</h3>';
    if (notes.length === 0 && aiNotes.length === 0) {
      html += '<p class="empty">No notes on this slide.</p>';
    } else {
      notes.forEach(function(n) {
        html += '<div class="note">' + escapeHTML(n) + '</div>';
      });
      aiNotes.forEach(function(n) {
        html += '<div class="note ai"><span class="tag">ai</span> ' + escapeHTML(n) + '</div>';
      });
    }
    html += '</div>';
    // Up next: slide + 1 title only, for context.
    if (current + 1 < total) {
      var nx = getSlide(current + 1);
      var nt = slideTitle(nx);
      html += '<div class="notes-next">';
      html += '<h4>Up next — slide ' + (current + 2) + (nt ? ' · ' + escapeHTML(nt) : '') + '</h4>';
      html += '</div>';
    }
    // Full deck outline with inline notes count so presenters can jump.
    html += '<h4 class="notes-outline-heading">Deck outline</h4>';
    html += '<div class="notes-outline">';
    deck.forEach(function(sl) {
      var title = slideTitle(sl) || '(untitled)';
      var count = (sl.notes || []).length + (sl.aiNotes || []).length;
      var active = sl.index === current ? ' active' : '';
      html += '<div class="notes-outline-item clickable' + active + '" data-slide="' + (sl.index + 1) + '">' +
        '<span class="n">' + (sl.index + 1) + '</span>' +
        '<span class="title">' + escapeHTML(title) + '</span>' +
        (count > 0 ? '<span class="count">' + count + '</span>' : '') +
        '</div>';
    });
    html += '</div>';
    body.innerHTML = html;
    body.querySelectorAll('.notes-outline-item.clickable').forEach(function(el) {
      el.addEventListener('click', function() {
        var slide = parseInt(el.getAttribute('data-slide'), 10) - 1;
        if (slide >= 0 && slide < total) go(slide);
      });
    });
  }

  function renderCommentsPanel() {
    var body = $('comments-body');
    var html = '';
    var totalComments = 0;
    deck.forEach(function(s) {
      var entries = [];
      s.comments.forEach(function(c) {
        entries.push({ author: c.author, text: c.text, scope: 'main', variant: '' });
      });
      s.variants.forEach(function(v) {
        v.comments.forEach(function(c) {
          entries.push({ author: c.author, text: c.text, scope: 'variant: ' + v.name, variant: v.name });
        });
      });
      if (entries.length === 0) return;
      totalComments += entries.length;
      var t = slideTitle(s);
      html += '<h3>Slide ' + (s.index + 1) + (t ? ' — ' + escapeHTML(t) : '') + '</h3>';
      entries.forEach(function(e) {
        html += '<div class="comment-item clickable" data-slide="' + (s.index + 1) + '" data-variant="' + escapeHTML(e.variant) + '">' +
          '<div class="meta">' +
            '<span class="author">@' + escapeHTML(e.author) + '</span>' +
            '<span class="scope">' + escapeHTML(e.scope) + '</span>' +
          '</div>' +
          '<div class="text">' + escapeHTML(e.text) + '</div>' +
          '</div>';
      });
    });
    if (totalComments === 0) {
      html = '<p class="empty">No comments yet. Use the form below to add the first one.</p>';
    }
    body.innerHTML = html;
    body.querySelectorAll('.comment-item.clickable').forEach(function(el) {
      el.addEventListener('click', function() {
        var slide = parseInt(el.getAttribute('data-slide'), 10) - 1;
        var variant = el.getAttribute('data-variant') || '';
        if (slide >= 0 && slide < total) {
          if (variant) activeVariant[slide] = variant;
          go(slide);
        }
      });
    });
    var saved = localStorage.getItem('waxon-author');
    if (saved && !commentAuthorEl.value) commentAuthorEl.value = saved;
  }

  function refreshCommentTargets() {
    if (commentComposing) {
      // Don't rebuild — the user is composing for a frozen target.
      return;
    }
    var sel = commentTargetEl;
    var s = getSlide(current);
    var html = '';
    html += '<option value="' + (current + 1) + '|">Slide ' + (current + 1) + ' — main</option>';
    s.variants.forEach(function(v) {
      html += '<option value="' + (current + 1) + '|' + escapeHTML(v.name) + '">' +
        'Slide ' + (current + 1) + ' — variant: ' + escapeHTML(v.name) + '</option>';
    });
    sel.innerHTML = html;
    var cur = activeVariant[current] || '';
    sel.value = (current + 1) + '|' + cur;
  }

  function renderDecksPanel() {
    var body = $('decks-body');
    if (!decks || decks.length === 0) {
      body.innerHTML = '<p class="empty">Single-deck mode. Pass a directory to <code>waxon serve</code> to switch between decks here.</p>';
      return;
    }
    var html = '';
    decks.forEach(function(d) {
      var active = (d.path === state.path) ? ' active' : '';
      // Preserve the slide hash when switching decks for cross-deck compare.
      html += '<a class="deck-item' + active + '" href="/d/' + encodeURI(d.path) + location.hash + '">' +
        '<div class="title">' + escapeHTML(d.title || d.path) + '</div>' +
        '<div class="path">' + escapeHTML(d.path) + '</div>' +
        '</a>';
    });
    body.innerHTML = html;
  }

  // ---------- Theme picker ----------
  // Injects the clicked theme's CSS into #theme-css, so the active slide
  // re-themes instantly without a page reload. Persisted per-browser via
  // localStorage so reloads keep the user's selection.
  var themeStyleEl = document.getElementById('theme-css');
  function applyTheme(name) {
    if (!themes) return;
    for (var i = 0; i < themes.length; i++) {
      if (themes[i].name === name) {
        themeStyleEl.textContent = themes[i].css;
        state.activeTheme = name;
        state.themeOverridden = (name !== state.deckTheme);
        document.documentElement.setAttribute('data-theme', name);
        try { localStorage.setItem('waxon-theme-override', state.themeOverridden ? name : ''); }
        catch (e) {}
        updateBanner();
        if (openPanel === 'themes') renderThemesPanel();
        return;
      }
    }
  }
  function renderThemesPanel() {
    var body = $('themes-body');
    if (!themes || themes.length === 0) {
      body.innerHTML = '<p class="empty">No themes registered.</p>';
      return;
    }
    var html = '';
    themes.forEach(function(t) {
      var active = (t.name === state.activeTheme) ? ' active' : '';
      // Swatch: pull the theme's --slide-bg / --slide-fg / --accent from its
      // CSS by sniffing the declaration text. Good enough for a preview.
      function pick(re, fallback) {
        var m = t.css.match(re);
        return m ? m[1].trim() : fallback;
      }
      var bg = pick(/--slide-bg:\s*([^;]+);/, '#1a1a2e');
      var fg = pick(/--slide-fg:\s*([^;]+);/, '#e0e0e0');
      var accent = pick(/--accent:\s*([^;]+);/, '#7c3aed');
      html += '<div class="theme-item' + active + '" data-theme="' + escapeHTML(t.name) + '" tabindex="0" role="button">' +
        '<div class="name">' + escapeHTML(t.name) + '</div>' +
        '<div class="desc">' + escapeHTML(t.description || '') + '</div>' +
        '<div class="theme-swatch">' +
          '<span style="background:' + escapeHTML(bg) + ';color:' + escapeHTML(fg) + ';">Aa</span>' +
          '<span style="background:' + escapeHTML(accent) + ';color:' + escapeHTML(bg) + ';">h1</span>' +
          '<span style="background:' + escapeHTML(fg) + ';color:' + escapeHTML(bg) + ';">code</span>' +
        '</div>' +
      '</div>';
    });
    body.innerHTML = html;
    $$('.theme-item', body).forEach(function(el) {
      el.addEventListener('click', function() { applyTheme(el.getAttribute('data-theme')); });
      el.addEventListener('keydown', function(e) {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          applyTheme(el.getAttribute('data-theme'));
        }
      });
    });
  }
  function cycleTheme(dir) {
    if (!themes || themes.length === 0) return;
    var i = 0;
    for (var k = 0; k < themes.length; k++) {
      if (themes[k].name === state.activeTheme) { i = k; break; }
    }
    i = (i + dir + themes.length) % themes.length;
    applyTheme(themes[i].name);
  }
  // Restore persisted theme override on load.
  try {
    var storedTheme = localStorage.getItem('waxon-theme-override');
    if (storedTheme) applyTheme(storedTheme);
  } catch (e) {}

  // ---------- Zoom ----------
  // Scales the page font-size via --waxon-zoom. Slide content uses em-based
  // sizing so headings, bullets, and spacing all grow in step.
  var zoomLevel = 1;
  try {
    var storedZoom = parseFloat(localStorage.getItem('waxon-zoom') || '1');
    if (storedZoom > 0.3 && storedZoom < 4) zoomLevel = storedZoom;
  } catch (e) {}
  function applyZoom() {
    document.documentElement.style.setProperty('--waxon-zoom', String(zoomLevel));
    try { localStorage.setItem('waxon-zoom', String(zoomLevel)); } catch (e) {}
  }
  function zoomIn()    { zoomLevel = Math.min(3, Math.round((zoomLevel + 0.1) * 100) / 100); applyZoom(); flashBanner('Zoom ' + Math.round(zoomLevel * 100) + '%'); }
  function zoomOut()   { zoomLevel = Math.max(0.5, Math.round((zoomLevel - 0.1) * 100) / 100); applyZoom(); flashBanner('Zoom ' + Math.round(zoomLevel * 100) + '%'); }
  function zoomReset() { zoomLevel = 1; applyZoom(); flashBanner('Zoom 100%'); }
  applyZoom();

  // ---------- Comment composing lifecycle ----------
  function startComposing() {
    if (commentComposing) return;
    commentComposing = true;
    commentTargetEl.disabled = true;
    commentTargetLocked.removeAttribute('hidden');
    // Capture the target right now so future navigation can't change it.
    commentTarget = commentTargetEl.value;
  }
  function stopComposing() {
    commentComposing = false;
    commentTargetEl.disabled = false;
    commentTargetLocked.setAttribute('hidden', '');
    commentTarget = null;
    refreshCommentTargets();
  }
  commentTextEl.addEventListener('input', function() {
    if (commentTextEl.value.trim() !== '') startComposing();
  });
  commentTextEl.addEventListener('focus', function() {
    if (commentTextEl.value.trim() !== '') startComposing();
  });

  // ---------- Comment posting ----------
  commentForm.addEventListener('submit', function(e) {
    e.preventDefault();
    commentStatus.textContent = '';
    commentStatus.className = 'status';
    if (state.standalone) {
      commentStatus.textContent = 'Comment posting is disabled in standalone mode.';
      commentStatus.className = 'status error';
      return;
    }
    var author = commentAuthorEl.value.trim();
    var text = commentTextEl.value.trim();
    if (!author) {
      commentStatus.textContent = 'Author is required.';
      commentStatus.className = 'status error';
      commentAuthorEl.focus();
      return;
    }
    if (!text) {
      commentStatus.textContent = 'Comment text is required.';
      commentStatus.className = 'status error';
      commentTextEl.focus();
      return;
    }
    var target = commentTarget || commentTargetEl.value;
    var parts = target.split('|');
    var slideNum = parseInt(parts[0], 10);
    var variant = parts[1] || '';
    localStorage.setItem('waxon-author', author);
    commentSubmit.disabled = true;
    fetch('/api/comment', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        path: state.path,
        slide: slideNum,
        variant: variant,
        author: author,
        text: text,
      }),
    }).then(function(r) {
      if (!r.ok) return r.text().then(function(t) { throw new Error(t || ('HTTP ' + r.status)); });
      return r.json();
    }).then(function() {
      commentStatus.textContent = 'Posted. Reloading…';
      commentStatus.className = 'status ok';
      commentTextEl.value = '';
      stopComposing();
    }).catch(function(err) {
      commentStatus.textContent = err.message;
      commentStatus.className = 'status error';
    }).finally(function() {
      commentSubmit.disabled = false;
    });
  });

  // Floating action bar wiring
  $$('.fab button').forEach(function(b) {
    b.addEventListener('click', function() {
      var action = b.getAttribute('data-action');
      if (action === 'help') toggleHelp();
      else if (action === 'compare') { compareMode = !compareMode; render(); }
      else togglePanel(action);
    });
  });
  $$('.panel .close').forEach(function(b) {
    b.addEventListener('click', closePanels);
  });

  // Click backdrop of help overlay to close (but not the card itself)
  helpOverlay.addEventListener('click', function(e) {
    if (e.target === helpOverlay) closeHelp();
  });

  // Click on the deck-area refocuses the app root so keyboard nav resumes
  // even after a user clicked a link or anchor inside slide content.
  paneMain.addEventListener('click', function(e) {
    var t = e.target;
    if (t && (t.tagName === 'A' || t.tagName === 'BUTTON' || t.tagName === 'INPUT')) return;
    app.focus();
  });
  deckArea.addEventListener('mousedown', function(e) {
    var t = e.target;
    if (t && (t.tagName === 'A' || t.tagName === 'BUTTON' || t.tagName === 'INPUT')) return;
    // Defer focus so the click on a child doesn't fight us.
    setTimeout(function() { app.focus(); }, 0);
  });

  // Fullscreen toggle (true toggle, not one-way)
  function toggleFullscreen() {
    if (document.fullscreenElement) {
      document.exitFullscreen && document.exitFullscreen();
    } else {
      document.documentElement.requestFullscreen && document.documentElement.requestFullscreen();
    }
  }

  // Keyboard shortcuts
  document.addEventListener('keydown', function(e) {
    var t = e.target;
    var inField = t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || (t.tagName === 'SELECT' && !t.disabled));
    if (inField) {
      if (e.key === 'Escape') t.blur();
      return;
    }
    if (e.ctrlKey || e.metaKey || e.altKey) return;

    // Help has priority over everything when open.
    if (helpOverlay.classList.contains('open')) {
      if (e.key === 'Escape' || e.key === '?') { e.preventDefault(); closeHelp(); }
      return;
    }

    switch (e.key) {
      case 'ArrowRight': case ' ': case 'Enter':
        e.preventDefault();
        if (e.shiftKey) { go(current + 1); } else { next(); }
        return;
      case 'ArrowLeft': case 'Backspace':
        e.preventDefault();
        if (e.shiftKey) { go(current - 1); } else { prev(); }
        return;
      case 'Home': e.preventDefault(); go(0); return;
      case 'End': e.preventDefault(); go(total - 1); return;
      case 'f': e.preventDefault(); toggleFullscreen(); return;
      case 'Escape':
        if (openPanel) { closePanels(); return; }
        if (document.fullscreenElement) { document.exitFullscreen(); return; }
        return;
      case 'v': e.preventDefault(); togglePanel('variants'); return;
      case 'c': e.preventDefault(); togglePanel('comments'); return;
      case 'd': e.preventDefault(); togglePanel('decks'); return;
      case 't': e.preventDefault(); togglePanel('themes'); return;
      case 'T': e.preventDefault(); cycleTheme(1); return;
      case 'N': e.preventDefault(); togglePanel('notes'); return;
      case 'x': e.preventDefault(); compareMode = !compareMode; render(); return;
      case '[': e.preventDefault(); cycleVariant(-1); return;
      case ']': e.preventDefault(); cycleVariant(1); return;
      case 'n': e.preventDefault(); next(); return;
      case 'p': e.preventDefault(); prev(); return;
      case '+': case '=': e.preventDefault(); zoomIn(); return;
      case '-': case '_': e.preventDefault(); zoomOut(); return;
      case 'r':
        e.preventDefault();
        activeVariant = {};
        compareMode = false;
        bannerHidden = false;
        pauseStep = {};
        zoomReset();
        render();
        return;
      case '?': e.preventDefault(); toggleHelp(); return;
      case 'g':
        if (Date.now() - lastG < 500) { go(0); lastG = 0; }
        else lastG = Date.now();
        return;
      case 'G': go(total - 1); return;
    }
    if (e.key >= '1' && e.key <= '9') {
      var n = parseInt(e.key, 10) - 1;
      if (n < total) go(n);
    }
    if (e.key === '0') {
      // 0 = slide 10 (the only escape valve for two-digit slide numbers).
      if (total >= 10) go(9);
    }
  });

  // Hash navigation: #N, #N/variant-name, or #slide-id
  // A slide ID (non-numeric leading char) looks up a named slide. Numeric
  // hashes continue to address slides by 1-based position.
  function applyHash() {
    var h = location.hash.slice(1);
    if (!h) return;
    var parts = h.split('/');
    var head = parts[0];
    var n = parseInt(head, 10);
    if (!isNaN(n) && n > 0 && n <= total) {
      current = n - 1;
    } else if (head && /^[a-zA-Z_]/.test(head)) {
      for (var i = 0; i < total; i++) {
        if (deck[i] && deck[i].id === head) { current = i; break; }
      }
    }
    if (parts[1]) {
      try { activeVariant[current] = decodeURIComponent(parts[1]); }
      catch (err) { console.warn('bad hash variant:', err); }
    }
  }
  applyHash();
  window.addEventListener('hashchange', function() {
    var prev = current;
    applyHash();
    if (prev !== current || activeVariant[current]) render();
  });

  // Initial focus + render
  app.focus();
  render();

  // Live reload via WebSocket (scoped per deck) with exponential backoff.
  if (!state.standalone) {
    var wsBackoff = 500;
    var wsBackoffMax = 30000;
    function setWsConnected(ok) {
      wsStatus.classList.toggle('show', !ok);
    }
    function connectWS() {
      var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
      var url = proto + '//' + location.host + '/ws';
      if (state.path) url += '?path=' + encodeURIComponent(state.path);
      try {
        var ws = new WebSocket(url);
        ws.onopen = function() {
          setWsConnected(true);
          wsBackoff = 500;
        };
        ws.onmessage = function(e) {
          if (e.data === 'reload') location.reload();
        };
        ws.onclose = function() {
          setWsConnected(false);
          wsBackoff = Math.min(wsBackoffMax, Math.floor(wsBackoff * 1.7) + 200);
          setTimeout(connectWS, wsBackoff);
        };
        ws.onerror = function() { try { ws.close(); } catch (_) {} };
      } catch (err) {
        setWsConnected(false);
        wsBackoff = Math.min(wsBackoffMax, Math.floor(wsBackoff * 1.7) + 200);
        setTimeout(connectWS, wsBackoff);
      }
    }
    connectWS();
  }
})();
</script>
</body>
</html>`

const indexTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>waxon — decks</title>
<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
:root {
  --bg: #0f0f17;
  --bg-elev: #181826;
  --bg-elev-hi: #20202f;
  --fg: #e8e8f0;
  --fg-dim: #9090a8;
  --fg-faint: #606078;
  --accent: #7c5cff;
  --accent-hi: #a896ff;
  --border: rgba(255,255,255,0.08);
  --border-hi: rgba(255,255,255,0.16);
  --mono: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', ui-monospace, monospace;
}
html, body {
  background: var(--bg);
  color: var(--fg);
  font-family: system-ui, -apple-system, 'Segoe UI', sans-serif;
  font-size: 15px;
  line-height: 1.5;
  min-height: 100vh;
}
body { padding: 6vmin 4vmin; }
.wrap { max-width: 1100px; margin: 0 auto; }
header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  padding-bottom: 1.5em;
  margin-bottom: 2em;
  border-bottom: 1px solid var(--border);
  flex-wrap: wrap;
  gap: 1em;
}
.brand {
  display: flex;
  align-items: baseline;
  gap: 0.6em;
}
.brand h1 {
  font-family: var(--mono);
  font-size: 1.9em;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--fg);
}
.brand h1::before {
  content: '▌';
  color: var(--accent);
  margin-right: 0.15em;
}
.brand .subtitle {
  color: var(--fg-dim);
  font-size: 0.95em;
}
.count {
  font-family: var(--mono);
  font-size: 0.85em;
  color: var(--fg-dim);
  background: var(--bg-elev);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 0.35em 0.7em;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 1em;
}
a.deck {
  display: block;
  padding: 1.2em 1.3em;
  background: var(--bg-elev);
  border: 1px solid var(--border);
  border-radius: 8px;
  text-decoration: none;
  color: inherit;
  transition: border-color 0.15s, background 0.15s, transform 0.15s;
  position: relative;
  overflow: hidden;
}
a.deck::before {
  content: '';
  position: absolute;
  left: 0; top: 0; bottom: 0;
  width: 3px;
  background: var(--accent);
  opacity: 0.7;
}
a.deck:hover {
  background: var(--bg-elev-hi);
  border-color: var(--border-hi);
  transform: translateY(-1px);
}
a.deck:hover::before {
  background: var(--accent-hi);
  opacity: 1;
}
a.deck:focus-visible {
  outline: 2px solid var(--accent-hi);
  outline-offset: 2px;
}
.deck .title {
  font-size: 1.1em;
  font-weight: 600;
  color: var(--fg);
  margin-bottom: 0.25em;
  /* Allow long titles to wrap, but keep them tight. */
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.deck .author {
  font-size: 0.85em;
  color: var(--fg-dim);
  margin-bottom: 0.9em;
}
.deck .meta {
  display: flex;
  gap: 0.5em;
  flex-wrap: wrap;
  margin-bottom: 0.7em;
}
.deck .pill {
  font-family: var(--mono);
  font-size: 0.72em;
  padding: 0.2em 0.55em;
  border-radius: 3px;
  background: rgba(124, 92, 255, 0.12);
  color: var(--accent-hi);
  border: 1px solid rgba(124, 92, 255, 0.25);
}
.deck .pill.count {
  background: rgba(255,255,255,0.04);
  color: var(--fg-dim);
  border-color: var(--border);
}
.deck .path {
  font-family: var(--mono);
  font-size: 0.75em;
  color: var(--fg-faint);
  word-break: break-all;
}
.empty {
  padding: 4em 2em;
  text-align: center;
  color: var(--fg-dim);
  background: var(--bg-elev);
  border: 1px dashed var(--border-hi);
  border-radius: 8px;
}
.empty code {
  background: var(--bg-elev-hi);
  padding: 0.15em 0.45em;
  border-radius: 3px;
  font-family: var(--mono);
  color: var(--accent-hi);
}
footer {
  margin-top: 3em;
  padding-top: 1.5em;
  border-top: 1px solid var(--border);
  font-family: var(--mono);
  font-size: 0.78em;
  color: var(--fg-faint);
  display: flex;
  justify-content: space-between;
  gap: 1em;
  flex-wrap: wrap;
}
footer a { color: var(--fg-dim); text-decoration: none; }
footer a:hover { color: var(--accent-hi); }
</style>
</head>
<body>
<div class="wrap">
<header>
  <div class="brand">
    <h1>waxon</h1>
    <span class="subtitle">slide deck server</span>
  </div>
  <span class="count">{{len .Decks}} deck{{if ne (len .Decks) 1}}s{{end}}</span>
</header>
{{if .Decks}}
<div class="grid">
{{range .Decks}}
<a class="deck" href="/d/{{.Path}}">
  <div class="title">{{if .Title}}{{.Title}}{{else}}{{.Path}}{{end}}</div>
  {{if .Author}}<div class="author">by {{.Author}}</div>{{end}}
  <div class="meta">
    {{if .Theme}}<span class="pill">{{.Theme}}</span>{{end}}
    {{if .SlideCount}}<span class="pill count">{{.SlideCount}} slide{{if ne .SlideCount 1}}s{{end}}</span>{{end}}
  </div>
  <div class="path">{{.Path}}</div>
</a>
{{end}}
</div>
{{else}}
<div class="empty">
No <code>.slides</code> files found in this directory tree.
</div>
{{end}}
<footer>
  <span>waxon · text-first slide toolkit</span>
  <a href="/api/decks">/api/decks (json)</a>
</footer>
</div>
</body>
</html>`

// printTemplate is a JS-free, print-friendly layout used by the PDF exporter.
// Each slide becomes its own full-page <div class="slide">; the .deck wrapper
// matches the selector chromedp waits on (chromedp.WaitVisible(".deck", ...)).
const printTemplate = `<!DOCTYPE html>
<html lang="en" data-theme="{{.Theme}}"{{if .TerminalVariant}} data-terminal-variant="{{.TerminalVariant}}"{{end}}>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

:root {
  --slide-bg: #1a1a2e;
  --slide-fg: #e0e0e0;
  --accent: #7c3aed;
  --font-body: system-ui, -apple-system, sans-serif;
  --font-heading: system-ui, -apple-system, sans-serif;
  --font-mono: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace;
  --slide-padding: 5vmin;
}

html, body {
  background: var(--slide-bg);
  color: var(--slide-fg);
  font-family: var(--font-body);
  font-size: clamp(16px, 2.5vmin, 28px);
  line-height: 1.5;
}

.deck { width: 100vw; }

.slide {
  width: 100vw;
  min-height: 100vh;
  padding: var(--slide-padding);
  display: flex;
  flex-direction: column;
  justify-content: center;
  page-break-after: always;
  break-after: page;
  position: relative;
  overflow: visible;
}
.slide:last-child { page-break-after: auto; break-after: auto; }

.slide h1 { font-size: 2.5em; font-family: var(--font-heading); color: var(--accent); margin-bottom: 0.5em; }
.slide h2 { font-size: 1.8em; font-family: var(--font-heading); color: var(--accent); margin-bottom: 0.4em; }
.slide h3 { font-size: 1.3em; font-family: var(--font-heading); margin-bottom: 0.3em; }
.slide p { margin-bottom: 0.8em; }
.slide ul, .slide ol { margin-left: 1.5em; margin-bottom: 0.8em; }
.slide li { margin-bottom: 0.3em; }
.slide pre {
  background: rgba(0,0,0,0.3);
  border-radius: 8px;
  padding: 1em;
  overflow-x: auto;
  margin-bottom: 1em;
  font-family: var(--font-mono);
  font-size: 0.85em;
  line-height: 1.4;
}
.slide code { font-family: var(--font-mono); font-size: 0.9em; }
.slide :not(pre) > code {
  background: rgba(0,0,0,0.2);
  padding: 0.15em 0.4em;
  border-radius: 4px;
}
.slide table { border-collapse: collapse; margin-bottom: 1em; }
.slide th, .slide td { border: 1px solid rgba(255,255,255,0.2); padding: 0.5em 1em; text-align: left; }
.slide th { background: rgba(255,255,255,0.05); font-weight: 600; }
.slide blockquote {
  border-left: 4px solid var(--accent);
  padding-left: 1em;
  margin-bottom: 0.8em;
  opacity: 0.85;
  font-style: italic;
}
.slide img { max-width: 100%; max-height: 60vh; object-fit: contain; }

/* Progressive-reveal sentinel is never shown — PDF export always displays the
 * full slide content, so the marker is just hidden. Also re-tighten list
 * spacing when goldmark splits a <ul> around the sentinel. */
:where(.slide) .waxon-pause { display: none; }
:where(.slide) .waxon-pause + ul, :where(.slide) .waxon-pause + ol { margin-top: -0.3em; }

.footer {
  position: absolute;
  bottom: 1vmin;
  left: var(--slide-padding);
  right: var(--slide-padding);
  font-size: 0.6em;
  opacity: 0.5;
  display: flex;
  gap: 1em;
  align-items: baseline;
}
.footer > .footer-left { flex: 1 1 auto; text-align: left; }
.footer > .footer-center { flex: 1 1 auto; text-align: center; }
.footer > .footer-right { flex: 1 1 auto; text-align: right; }
.footer > :empty { display: none; }

/* Slide-level vertical alignment (#49, #54) */
.slide[data-valign="center"], .slide[data-valign="middle"] {
  display: flex;
  flex-direction: column;
  justify-content: center;
}
.slide[data-valign="bottom"] {
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
}

@media print {
  html, body { background: var(--slide-bg) !important; }
  .slide { page-break-after: always; }
}

{{if .TerminalEffects}}
.deck::after {
  content: '';
  position: fixed;
  inset: 0;
  background: repeating-linear-gradient(
    transparent 0px, transparent 2px,
    rgba(0,0,0,0.03) 2px, rgba(0,0,0,0.03) 4px
  );
  pointer-events: none;
}
{{end}}
</style>
{{range .Fonts}}<link rel="stylesheet" href="{{.}}">
{{end}}<style id="theme-css">
{{if .ThemeCSSInline}}{{.ThemeCSSInline}}{{else}}{{themeCSS .Theme}}{{end}}
</style>
</head>
<body>
<div class="deck"{{if .Transition}} data-transition="{{.Transition}}"{{end}}>
{{range $i, $s := .Slides}}
<div class="slide{{if $s.Class}} {{$s.Class}}{{end}}" data-index="{{$s.Index}}"{{if $s.ID}} id="{{$s.ID}}"{{end}}{{if $s.Valign}} data-valign="{{$s.Valign}}"{{end}}{{if $s.BgImage}} style="background-image: {{$s.BgImage}}; background-size: cover; background-position: center;{{if $s.Bg}} background-color: {{$s.Bg}};{{end}}"{{else if $s.Bg}} style="background: {{$s.Bg}}"{{end}}>
{{$s.HTML}}
{{if or $.Footer $.FooterLeft $.FooterRight}}<div class="footer">
<div class="footer-left">{{pageFooter $.FooterLeft (inc $i) (len $.Slides)}}</div>
<div class="footer-center">{{pageFooter $.Footer (inc $i) (len $.Slides)}}</div>
<div class="footer-right">{{pageFooter $.FooterRight (inc $i) (len $.Slides)}}</div>
</div>{{end}}
</div>
{{end}}
</div>
</body>
</html>`
