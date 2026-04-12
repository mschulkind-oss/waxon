// Package render converts parsed slides into HTML.
package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"github.com/mschulkind-oss/waxon/internal/format"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Options configures the renderer.
type Options struct {
	ThemeOverride string // Override theme from frontmatter
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

// RenderHTML renders a deck to a complete HTML page.
func RenderHTML(deck *format.Deck, opts Options) (string, error) {
	theme := deck.Meta.Theme
	if opts.ThemeOverride != "" {
		theme = opts.ThemeOverride
	}

	slidesHTML, err := renderSlides(deck.Slides)
	if err != nil {
		return "", err
	}

	notesJSON, err := json.Marshal(collectNotes(deck.Slides))
	if err != nil {
		return "", err
	}

	data := templateData{
		Title:           deck.Meta.Title,
		Author:          deck.Meta.Author,
		Theme:           theme,
		Aspect:          deck.Meta.Aspect,
		Footer:          deck.Meta.Footer,
		TerminalVariant: deck.Meta.TerminalVariant,
		TerminalEffects: deck.Meta.TerminalEffects,
		SlidesHTML:      slidesHTML,
		SlidesJSON:      template.JS(notesJSON),
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

func renderSlides(slides []format.Slide) ([]template.HTML, error) {
	result := make([]template.HTML, 0, len(slides))
	for _, s := range slides {
		html, err := RenderSlideHTML(s.Content)
		if err != nil {
			return nil, err
		}
		result = append(result, template.HTML(html))
	}
	return result, nil
}

func collectNotes(slides []format.Slide) [][]string {
	notes := make([][]string, len(slides))
	for i, s := range slides {
		notes[i] = s.Notes
		if notes[i] == nil {
			notes[i] = []string{}
		}
	}
	return notes
}

type templateData struct {
	Title           string
	Author          string
	Theme           string
	Aspect          string
	Footer          string
	TerminalVariant string
	TerminalEffects bool
	SlidesHTML      []template.HTML
	SlidesJSON      template.JS
	TotalSlides     int
}

// ThemeCSS returns the CSS for a given theme name.
// The themes package replaces this at init time.
var ThemeCSS = func(theme string) template.CSS {
	return ""
}

var pageTmpl *template.Template

func init() {
	pageTmpl = template.Must(template.New("page").Funcs(template.FuncMap{
		"themeCSS": ThemeCSS,
	}).Parse(pageTemplate))
}

const pageTemplate = `<!DOCTYPE html>
<html lang="en" data-theme="{{.Theme}}"{{if .TerminalVariant}} data-terminal-variant="{{.TerminalVariant}}"{{end}}>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
{{themeCSS .Theme}}
</style>
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
  height: 100%;
  overflow: hidden;
  background: var(--slide-bg);
  color: var(--slide-fg);
  font-family: var(--font-body);
  font-size: clamp(16px, 2.5vmin, 28px);
  line-height: 1.5;
}

.deck {
  width: 100vw;
  height: 100vh;
  position: relative;
}

.slide {
  display: none;
  position: absolute;
  inset: 0;
  padding: var(--slide-padding);
  overflow: auto;
}

.slide.active { display: flex; flex-direction: column; justify-content: center; }

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

.slide code {
  font-family: var(--font-mono);
  font-size: 0.9em;
}

.slide :not(pre) > code {
  background: rgba(0,0,0,0.2);
  padding: 0.15em 0.4em;
  border-radius: 4px;
}

.slide table {
  border-collapse: collapse;
  margin-bottom: 1em;
  width: auto;
}

.slide th, .slide td {
  border: 1px solid rgba(255,255,255,0.2);
  padding: 0.5em 1em;
  text-align: left;
}

.slide th { background: rgba(255,255,255,0.05); font-weight: 600; }

.slide blockquote {
  border-left: 4px solid var(--accent);
  padding-left: 1em;
  margin-bottom: 0.8em;
  opacity: 0.85;
  font-style: italic;
}

.slide img { max-width: 100%; max-height: 60vh; object-fit: contain; }

.footer {
  position: fixed;
  bottom: 1vmin;
  left: var(--slide-padding);
  right: var(--slide-padding);
  display: flex;
  justify-content: space-between;
  font-size: 0.6em;
  opacity: 0.5;
}

.progress {
  position: fixed;
  bottom: 0;
  left: 0;
  height: 3px;
  background: var(--accent);
  transition: width 0.3s ease;
}

{{if .TerminalEffects}}
@keyframes scanline {
  0% { transform: translateY(-100%); }
  100% { transform: translateY(100vh); }
}
.deck::after {
  content: '';
  position: fixed;
  inset: 0;
  background: repeating-linear-gradient(
    transparent 0px,
    transparent 2px,
    rgba(0,0,0,0.03) 2px,
    rgba(0,0,0,0.03) 4px
  );
  pointer-events: none;
  z-index: 9999;
}
{{end}}
</style>
</head>
<body>
<div class="deck" id="deck">
{{range $i, $html := .SlidesHTML}}
<div class="slide{{if eq $i 0}} active{{end}}" data-index="{{$i}}">
<div class="slide-content">{{$html}}</div>
</div>
{{end}}
</div>

{{if .Footer}}
<div class="footer">
<span>{{.Footer}}</span>
<span class="slide-counter"></span>
</div>
{{end}}
<div class="progress" id="progress"></div>

<script>
(function() {
  'use strict';
  var current = 0;
  var total = {{.TotalSlides}};
  var notes = {{.SlidesJSON}};
  var slides = document.querySelectorAll('.slide');
  var progress = document.getElementById('progress');

  function go(n) {
    if (n < 0 || n >= total) return;
    slides[current].classList.remove('active');
    current = n;
    slides[current].classList.add('active');
    progress.style.width = ((current + 1) / total * 100) + '%';
    var counter = document.querySelector('.slide-counter');
    if (counter) counter.textContent = (current + 1) + ' / ' + total;
    window.location.hash = '#' + (current + 1);
  }

  function next() { go(current + 1); }
  function prev() { go(current - 1); }

  document.addEventListener('keydown', function(e) {
    switch(e.key) {
      case 'ArrowRight': case ' ': case 'Enter': e.preventDefault(); next(); break;
      case 'ArrowLeft': case 'Backspace': e.preventDefault(); prev(); break;
      case 'f': if (!e.ctrlKey && !e.metaKey) { document.documentElement.requestFullscreen?.(); } break;
      case 'Escape': document.exitFullscreen?.(); break;
      case 'Home': e.preventDefault(); go(0); break;
      case 'End': e.preventDefault(); go(total - 1); break;
    }
    if (e.key >= '1' && e.key <= '9') {
      var n = parseInt(e.key) - 1;
      if (n < total) go(n);
    }
  });

  // Hash navigation
  var hash = parseInt(window.location.hash.slice(1));
  if (hash > 0 && hash <= total) go(hash - 1);

  // Live reload via WebSocket
  function connectWS() {
    var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    var ws = new WebSocket(proto + '//' + location.host + '/ws');
    ws.onmessage = function(e) {
      if (e.data === 'reload') {
        location.reload();
      }
    };
    ws.onclose = function() {
      setTimeout(connectWS, 1000);
    };
  }
  connectWS();

  // Initial state
  progress.style.width = ((current + 1) / total * 100) + '%';
  var counter = document.querySelector('.slide-counter');
  if (counter) counter.textContent = (current + 1) + ' / ' + total;
})();
</script>
</body>
</html>`

