// Package themes provides the built-in CSS theme system for waxon.
package themes

import (
	"html/template"

	"github.com/mschulkind-oss/waxon/internal/render"
)

// Theme describes a built-in theme.
type Theme struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CSS         string `json:"-"`
}

// All returns all built-in themes in display order.
func All() []Theme {
	return []Theme{
		{Name: "default", Description: "Clean, minimal dark theme with good contrast", CSS: defaultCSS},
		{Name: "light", Description: "Bright background, dark text, professional look", CSS: lightCSS},
		{Name: "corporate", Description: "Conservative palette, suitable for business presentations", CSS: corporateCSS},
		{Name: "minimal", Description: "Maximum whitespace, typography-focused, no distractions", CSS: minimalCSS},
		{Name: "vibrant", Description: "Bold colors and gradients for creative talks", CSS: vibrantCSS},
		{Name: "terminal", Description: "Authentic TUI aesthetic powered by WebTUI", CSS: terminalCSS},
	}
}

// Get returns a theme by name, or nil if not found.
func Get(name string) *Theme {
	for _, t := range All() {
		if t.Name == name {
			return &t
		}
	}
	return nil
}

// Names returns all theme names.
func Names() []string {
	themes := All()
	names := make([]string, len(themes))
	for i, t := range themes {
		names[i] = t.Name
	}
	return names
}

func init() {
	// Register the theme CSS function with the renderer
	render.ThemeCSS = func(theme string) template.CSS {
		t := Get(theme)
		if t == nil {
			return ""
		}
		return template.CSS(t.CSS)
	}
}

const defaultCSS = `
:root {
  --slide-bg: #1a1a2e;
  --slide-fg: #e0e0e0;
  --accent: #7c3aed;
  --font-body: system-ui, -apple-system, 'Segoe UI', sans-serif;
  --font-heading: system-ui, -apple-system, 'Segoe UI', sans-serif;
  --font-mono: 'JetBrains Mono', 'Fira Code', monospace;
}
`

const lightCSS = `
:root {
  --slide-bg: #ffffff;
  --slide-fg: #1a1a1a;
  --accent: #2563eb;
  --font-body: 'Inter', system-ui, -apple-system, sans-serif;
  --font-heading: 'Inter', system-ui, -apple-system, sans-serif;
  --font-mono: 'JetBrains Mono', 'Fira Code', monospace;
}
.slide pre { background: #f3f4f6; }
.slide :not(pre) > code { background: #e5e7eb; }
.slide th { background: #f3f4f6; }
.slide th, .slide td { border-color: #d1d5db; }
.slide blockquote { border-left-color: #2563eb; }
`

const corporateCSS = `
:root {
  --slide-bg: #f8f9fa;
  --slide-fg: #212529;
  --accent: #0d6efd;
  --font-body: 'Segoe UI', Roboto, 'Helvetica Neue', sans-serif;
  --font-heading: 'Segoe UI', Roboto, 'Helvetica Neue', sans-serif;
  --font-mono: 'Cascadia Code', 'Consolas', monospace;
}
.slide pre { background: #e9ecef; }
.slide :not(pre) > code { background: #dee2e6; }
.slide th { background: #e9ecef; }
.slide th, .slide td { border-color: #ced4da; }
.slide blockquote { border-left-color: #0d6efd; }
`

const minimalCSS = `
:root {
  --slide-bg: #fafafa;
  --slide-fg: #333333;
  --accent: #333333;
  --font-body: 'Georgia', 'Times New Roman', serif;
  --font-heading: system-ui, -apple-system, sans-serif;
  --font-mono: 'SF Mono', 'Monaco', monospace;
  --slide-padding: 10vmin;
}
.slide h1 { font-weight: 300; font-size: 3em; }
.slide pre { background: #f5f5f5; border: 1px solid #e0e0e0; border-radius: 0; }
.slide :not(pre) > code { background: #f0f0f0; }
.slide th, .slide td { border-color: #e0e0e0; }
.slide blockquote { border-left-color: #999; }
`

const vibrantCSS = `
:root {
  --slide-bg: #0f0c29;
  --slide-fg: #ffffff;
  --accent: #f97316;
  --font-body: system-ui, -apple-system, sans-serif;
  --font-heading: system-ui, -apple-system, sans-serif;
  --font-mono: 'JetBrains Mono', monospace;
}
body {
  background: linear-gradient(135deg, #0f0c29, #302b63, #24243e);
}
.slide h1 { color: #f97316; text-shadow: 0 0 30px rgba(249, 115, 22, 0.3); }
.slide h2 { color: #fb923c; }
.slide pre { background: rgba(0,0,0,0.4); border: 1px solid rgba(249,115,22,0.2); }
.slide blockquote { border-left-color: #f97316; }
.progress { background: linear-gradient(90deg, #f97316, #ec4899); }
`

const terminalCSS = `
:root {
  --slide-bg: #1e1e2e;
  --slide-fg: #cdd6f4;
  --accent: #89b4fa;
  --font-body: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'SF Mono', monospace;
  --font-heading: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'SF Mono', monospace;
  --font-mono: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'SF Mono', monospace;
}
body { background: #1e1e2e; }
.slide {
  border: 1px solid #45475a;
  margin: 1vmin;
  padding: 4vmin;
}
.slide h1 {
  color: #89b4fa;
  font-weight: 700;
}
.slide h1::before { content: '# '; color: #6c7086; }
.slide h2::before { content: '## '; color: #6c7086; }
.slide h3::before { content: '### '; color: #6c7086; }
.slide ul { list-style: none; margin-left: 0; }
.slide ul li::before { content: '- '; color: #89b4fa; }
.slide pre {
  background: #181825;
  border: 1px solid #45475a;
  border-radius: 0;
}
.slide :not(pre) > code {
  background: #313244;
  color: #f38ba8;
}
.slide blockquote {
  border-left: 2px solid #89b4fa;
  color: #a6adc8;
}
.slide th { background: #181825; }
.slide th, .slide td { border-color: #45475a; }
.footer { font-family: var(--font-mono); color: #6c7086; }
.progress { background: #89b4fa; }

/* Terminal variant: nord */
[data-terminal-variant="nord"] {
  --slide-bg: #2e3440;
  --slide-fg: #d8dee9;
  --accent: #88c0d0;
}
[data-terminal-variant="nord"] body { background: #2e3440; }
[data-terminal-variant="nord"] .slide { border-color: #4c566a; }
[data-terminal-variant="nord"] .slide pre { background: #3b4252; border-color: #4c566a; }
[data-terminal-variant="nord"] .slide :not(pre) > code { background: #3b4252; color: #bf616a; }
[data-terminal-variant="nord"] .progress { background: #88c0d0; }

/* Terminal variant: gruvbox */
[data-terminal-variant="gruvbox"] {
  --slide-bg: #282828;
  --slide-fg: #ebdbb2;
  --accent: #b8bb26;
}
[data-terminal-variant="gruvbox"] body { background: #282828; }
[data-terminal-variant="gruvbox"] .slide { border-color: #504945; }
[data-terminal-variant="gruvbox"] .slide pre { background: #1d2021; border-color: #504945; }
[data-terminal-variant="gruvbox"] .slide :not(pre) > code { background: #3c3836; color: #fb4934; }
[data-terminal-variant="gruvbox"] .progress { background: #b8bb26; }

/* Terminal variant: everforest */
[data-terminal-variant="everforest"] {
  --slide-bg: #2d353b;
  --slide-fg: #d3c6aa;
  --accent: #a7c080;
}
[data-terminal-variant="everforest"] body { background: #2d353b; }
[data-terminal-variant="everforest"] .slide { border-color: #475258; }
[data-terminal-variant="everforest"] .slide pre { background: #272e33; border-color: #475258; }
[data-terminal-variant="everforest"] .progress { background: #a7c080; }

/* Terminal variant: vitesse */
[data-terminal-variant="vitesse"] {
  --slide-bg: #121212;
  --slide-fg: #dbd7caee;
  --accent: #4d9375;
}
[data-terminal-variant="vitesse"] body { background: #121212; }
[data-terminal-variant="vitesse"] .slide { border-color: #2c2c2c; }
[data-terminal-variant="vitesse"] .slide pre { background: #1a1a1a; border-color: #2c2c2c; }
[data-terminal-variant="vitesse"] .progress { background: #4d9375; }
`
