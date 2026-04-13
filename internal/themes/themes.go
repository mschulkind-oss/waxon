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
		{Name: "dracula", Description: "Iconic purple-on-dark with pink accents", CSS: draculaCSS},
		{Name: "solarized-dark", Description: "Solarized palette tuned for low-light reading", CSS: solarizedDarkCSS},
		{Name: "solarized-light", Description: "Solarized palette in daylight mode", CSS: solarizedLightCSS},
		{Name: "tokyo-night", Description: "Tokyo Night colors — deep blues and electric accents", CSS: tokyoNightCSS},
		{Name: "catppuccin", Description: "Catppuccin Mocha — soothing pastel dark theme", CSS: catppuccinCSS},
		{Name: "monokai", Description: "Classic Monokai with its signature green and pink", CSS: monokaiCSS},
		{Name: "one-dark", Description: "Atom One Dark — calm slate with cool highlights", CSS: oneDarkCSS},
		{Name: "github-light", Description: "GitHub's light theme — familiar, readable, neutral", CSS: githubLightCSS},
		{Name: "github-dark", Description: "GitHub's dark theme — high-contrast, work-ready", CSS: githubDarkCSS},
		{Name: "paper", Description: "Newsprint aesthetic — serif type on warm off-white", CSS: paperCSS},
		{Name: "neon", Description: "Cyberpunk neon glow on near-black", CSS: neonCSS},
		{Name: "forest", Description: "Mossy greens and bark browns for an outdoors feel", CSS: forestCSS},
		{Name: "ocean", Description: "Deep teals and seafoam for a calm aquatic palette", CSS: oceanCSS},
		{Name: "sunset", Description: "Warm orange-to-violet gradient backdrop", CSS: sunsetCSS},
		{Name: "edit-dos", Description: "MS-DOS EDIT.COM — blue + yellow box-drawing, 80×25 vibes", CSS: editDosCSS},
		{Name: "win98", Description: "Windows 98 — silver chrome, navy title bars, beveled 3D dialogs", CSS: win98CSS},
		{Name: "nes", Description: "8-bit NES — Press Start 2P pixel font on chunky pixel borders", CSS: nesCSS},
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
	// Register the theme listing used by the in-browser theme picker.
	render.AllThemes = func() []render.ThemeEntry {
		themes := All()
		out := make([]render.ThemeEntry, len(themes))
		for i, t := range themes {
			out[i] = render.ThemeEntry{
				Name:        t.Name,
				Description: t.Description,
				CSS:         t.CSS,
			}
		}
		return out
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
  --color-red: #ef4444;
  --color-green: #22c55e;
  --color-yellow: #eab308;
  --color-blue: #60a5fa;
  --color-aqua: #22d3ee;
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
  --color-red: #dc2626;
  --color-green: #16a34a;
  --color-yellow: #ca8a04;
  --color-blue: #2563eb;
  --color-aqua: #0891b2;
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
  --color-red: #dc3545;
  --color-green: #198754;
  --color-yellow: #ffc107;
  --color-blue: #0d6efd;
  --color-aqua: #0dcaf0;
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
  --color-red: #7a2e2e;
  --color-green: #2e5a2e;
  --color-yellow: #8a6d1e;
  --color-blue: #2e4a7a;
  --color-aqua: #2e6a7a;
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
  --color-red: #f97316;
  --color-green: #84cc16;
  --color-yellow: #facc15;
  --color-blue: #6366f1;
  --color-aqua: #06b6d4;
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
  --color-red: #f38ba8;
  --color-green: #a6e3a1;
  --color-yellow: #f9e2af;
  --color-blue: #89b4fa;
  --color-aqua: #94e2d5;
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

const draculaCSS = `
:root {
  --slide-bg: #282a36;
  --slide-fg: #f8f8f2;
  --accent: #bd93f9;
  --font-body: 'Fira Sans', system-ui, -apple-system, sans-serif;
  --font-heading: 'Fira Sans', system-ui, -apple-system, sans-serif;
  --font-mono: 'Fira Code', 'JetBrains Mono', monospace;
  --color-red: #ff5555;
  --color-green: #50fa7b;
  --color-yellow: #f1fa8c;
  --color-blue: #bd93f9;
  --color-aqua: #8be9fd;
}
.slide h1 { color: #bd93f9; }
.slide h2 { color: #ff79c6; }
.slide h3 { color: #8be9fd; }
.slide pre { background: #21222c; border: 1px solid #44475a; }
.slide :not(pre) > code { background: #44475a; color: #ff79c6; }
.slide blockquote { border-left-color: #bd93f9; color: #f8f8f2cc; }
.slide a { color: #8be9fd; }
.slide th { background: #44475a; }
.slide th, .slide td { border-color: #44475a; }
.progress { background: linear-gradient(90deg, #bd93f9, #ff79c6); }
`

const solarizedDarkCSS = `
:root {
  --slide-bg: #002b36;
  --slide-fg: #93a1a1;
  --accent: #268bd2;
  --font-body: 'Source Sans 3', system-ui, sans-serif;
  --font-heading: 'Source Sans 3', system-ui, sans-serif;
  --font-mono: 'Source Code Pro', 'JetBrains Mono', monospace;
  --color-red: #dc322f;
  --color-green: #859900;
  --color-yellow: #b58900;
  --color-blue: #268bd2;
  --color-aqua: #2aa198;
}
.slide h1 { color: #b58900; }
.slide h2 { color: #cb4b16; }
.slide h3 { color: #2aa198; }
.slide pre { background: #073642; border: 1px solid #586e75; }
.slide :not(pre) > code { background: #073642; color: #d33682; }
.slide blockquote { border-left-color: #268bd2; color: #93a1a1; }
.slide a { color: #268bd2; }
.slide th { background: #073642; }
.slide th, .slide td { border-color: #586e75; }
.progress { background: #b58900; }
`

const solarizedLightCSS = `
:root {
  --slide-bg: #fdf6e3;
  --slide-fg: #586e75;
  --accent: #268bd2;
  --font-body: 'Source Sans 3', system-ui, sans-serif;
  --font-heading: 'Source Sans 3', system-ui, sans-serif;
  --font-mono: 'Source Code Pro', 'JetBrains Mono', monospace;
  --color-red: #dc322f;
  --color-green: #859900;
  --color-yellow: #b58900;
  --color-blue: #268bd2;
  --color-aqua: #2aa198;
}
.slide h1 { color: #b58900; }
.slide h2 { color: #cb4b16; }
.slide h3 { color: #2aa198; }
.slide pre { background: #eee8d5; border: 1px solid #93a1a1; }
.slide :not(pre) > code { background: #eee8d5; color: #d33682; }
.slide blockquote { border-left-color: #268bd2; color: #657b83; }
.slide a { color: #268bd2; }
.slide th { background: #eee8d5; }
.slide th, .slide td { border-color: #93a1a1; }
.progress { background: #b58900; }
`

const tokyoNightCSS = `
:root {
  --slide-bg: #1a1b26;
  --slide-fg: #c0caf5;
  --accent: #7aa2f7;
  --font-body: 'Inter', system-ui, sans-serif;
  --font-heading: 'Inter', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', 'Fira Code', monospace;
  --color-red: #f7768e;
  --color-green: #9ece6a;
  --color-yellow: #e0af68;
  --color-blue: #7aa2f7;
  --color-aqua: #7dcfff;
}
.slide h1 { color: #7aa2f7; }
.slide h2 { color: #bb9af7; }
.slide h3 { color: #7dcfff; }
.slide pre { background: #16161e; border: 1px solid #292e42; }
.slide :not(pre) > code { background: #292e42; color: #f7768e; }
.slide blockquote { border-left-color: #7aa2f7; color: #a9b1d6; }
.slide a { color: #7dcfff; }
.slide th { background: #16161e; }
.slide th, .slide td { border-color: #292e42; }
.progress { background: linear-gradient(90deg, #7aa2f7, #bb9af7); }
`

const catppuccinCSS = `
:root {
  --slide-bg: #1e1e2e;
  --slide-fg: #cdd6f4;
  --accent: #89b4fa;
  --font-body: 'Inter', system-ui, sans-serif;
  --font-heading: 'Inter', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', 'Fira Code', monospace;
  --color-red: #f38ba8;
  --color-green: #a6e3a1;
  --color-yellow: #f9e2af;
  --color-blue: #89b4fa;
  --color-aqua: #94e2d5;
}
.slide h1 { color: #f5c2e7; }
.slide h2 { color: #cba6f7; }
.slide h3 { color: #94e2d5; }
.slide pre { background: #181825; border: 1px solid #45475a; }
.slide :not(pre) > code { background: #313244; color: #f38ba8; }
.slide blockquote { border-left-color: #89b4fa; color: #bac2de; }
.slide a { color: #89b4fa; }
.slide th { background: #181825; }
.slide th, .slide td { border-color: #45475a; }
.progress { background: linear-gradient(90deg, #f5c2e7, #cba6f7); }
`

const monokaiCSS = `
:root {
  --slide-bg: #272822;
  --slide-fg: #f8f8f2;
  --accent: #a6e22e;
  --font-body: 'Source Sans 3', system-ui, sans-serif;
  --font-heading: 'Source Sans 3', system-ui, sans-serif;
  --font-mono: 'Monaco', 'Menlo', monospace;
  --color-red: #f92672;
  --color-green: #a6e22e;
  --color-yellow: #e6db74;
  --color-blue: #66d9ef;
  --color-aqua: #a1efe4;
}
.slide h1 { color: #a6e22e; }
.slide h2 { color: #f92672; }
.slide h3 { color: #66d9ef; }
.slide pre { background: #1e1f1c; border: 1px solid #49483e; }
.slide :not(pre) > code { background: #3e3d32; color: #fd971f; }
.slide blockquote { border-left-color: #a6e22e; color: #f8f8f2cc; }
.slide a { color: #66d9ef; }
.slide th { background: #1e1f1c; }
.slide th, .slide td { border-color: #49483e; }
.progress { background: linear-gradient(90deg, #a6e22e, #f92672); }
`

const oneDarkCSS = `
:root {
  --slide-bg: #282c34;
  --slide-fg: #abb2bf;
  --accent: #61afef;
  --font-body: 'Inter', system-ui, sans-serif;
  --font-heading: 'Inter', system-ui, sans-serif;
  --font-mono: 'Fira Code', 'JetBrains Mono', monospace;
  --color-red: #e06c75;
  --color-green: #98c379;
  --color-yellow: #e5c07b;
  --color-blue: #61afef;
  --color-aqua: #56b6c2;
}
.slide h1 { color: #61afef; }
.slide h2 { color: #c678dd; }
.slide h3 { color: #98c379; }
.slide pre { background: #21252b; border: 1px solid #3e4451; }
.slide :not(pre) > code { background: #3e4451; color: #e06c75; }
.slide blockquote { border-left-color: #61afef; color: #9ca3af; }
.slide a { color: #61afef; }
.slide th { background: #21252b; }
.slide th, .slide td { border-color: #3e4451; }
.progress { background: #61afef; }
`

const githubLightCSS = `
:root {
  --slide-bg: #ffffff;
  --slide-fg: #1f2328;
  --accent: #0969da;
  --font-body: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  --font-heading: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  --font-mono: 'SFMono-Regular', 'Cascadia Code', Consolas, monospace;
  --color-red: #cf222e;
  --color-green: #1a7f37;
  --color-yellow: #9a6700;
  --color-blue: #0969da;
  --color-aqua: #1f883d;
}
.slide h1 { color: #1f2328; border-bottom: 1px solid #d1d9e0; padding-bottom: 0.3em; }
.slide h2 { color: #1f2328; }
.slide pre { background: #f6f8fa; border: 1px solid #d1d9e0; }
.slide :not(pre) > code { background: #afb8c133; color: #1f2328; }
.slide blockquote { border-left-color: #d1d9e0; color: #59636e; }
.slide a { color: #0969da; }
.slide th { background: #f6f8fa; }
.slide th, .slide td { border-color: #d1d9e0; }
.progress { background: #0969da; }
`

const githubDarkCSS = `
:root {
  --slide-bg: #0d1117;
  --slide-fg: #c9d1d9;
  --accent: #58a6ff;
  --font-body: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  --font-heading: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  --font-mono: 'SFMono-Regular', 'Cascadia Code', Consolas, monospace;
  --color-red: #ff7b72;
  --color-green: #7ee787;
  --color-yellow: #d29922;
  --color-blue: #58a6ff;
  --color-aqua: #79c0ff;
}
.slide h1 { color: #f0f6fc; border-bottom: 1px solid #21262d; padding-bottom: 0.3em; }
.slide h2 { color: #f0f6fc; }
.slide pre { background: #161b22; border: 1px solid #30363d; }
.slide :not(pre) > code { background: #6e768166; color: #c9d1d9; }
.slide blockquote { border-left-color: #30363d; color: #8b949e; }
.slide a { color: #58a6ff; }
.slide th { background: #161b22; }
.slide th, .slide td { border-color: #30363d; }
.progress { background: #58a6ff; }
`

const paperCSS = `
:root {
  --slide-bg: #f5f0e8;
  --slide-fg: #2b2520;
  --accent: #8b4513;
  --font-body: 'Lora', 'Georgia', 'Times New Roman', serif;
  --font-heading: 'Playfair Display', 'Georgia', serif;
  --font-mono: 'Courier Prime', 'Courier New', monospace;
  --slide-padding: 8vmin;
  --color-red: #9a3e2e;
  --color-green: #4a6a1a;
  --color-yellow: #b58900;
  --color-blue: #2f4a6a;
  --color-aqua: #3a7a7a;
}
.slide h1 { font-weight: 700; font-style: italic; color: #2b2520; }
.slide h2 { font-weight: 700; color: #4a3a2a; }
.slide pre { background: #ebe4d6; border: 1px solid #c9bea5; }
.slide :not(pre) > code { background: #ebe4d6; color: #8b4513; }
.slide blockquote { border-left: 3px solid #8b4513; color: #4a3a2a; font-style: italic; }
.slide a { color: #8b4513; text-decoration: underline; }
.slide th { background: #ebe4d6; }
.slide th, .slide td { border-color: #c9bea5; }
.progress { background: #8b4513; }
`

const neonCSS = `
:root {
  --slide-bg: #0a001a;
  --slide-fg: #e0e0ff;
  --accent: #00ffff;
  --font-body: 'Rajdhani', 'Orbitron', system-ui, sans-serif;
  --font-heading: 'Orbitron', 'Rajdhani', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', 'Fira Code', monospace;
  --color-red: #ff0080;
  --color-green: #00ff40;
  --color-yellow: #ffff00;
  --color-blue: #0080ff;
  --color-aqua: #00ffff;
}
body { background: #0a001a; }
.slide h1 {
  color: #00ffff;
  text-shadow: 0 0 10px #00ffff, 0 0 20px #00ffff, 0 0 40px #00ffff80;
  letter-spacing: 0.05em;
}
.slide h2 {
  color: #ff00ff;
  text-shadow: 0 0 8px #ff00ff, 0 0 16px #ff00ff80;
}
.slide h3 { color: #ffff00; text-shadow: 0 0 6px #ffff00; }
.slide pre { background: #14002a; border: 1px solid #00ffff; box-shadow: 0 0 12px #00ffff40; }
.slide :not(pre) > code { background: #1a0030; color: #ff00ff; }
.slide blockquote { border-left: 2px solid #ff00ff; color: #ff00ffcc; }
.slide a { color: #00ffff; text-shadow: 0 0 6px #00ffff; }
.slide th { background: #14002a; }
.slide th, .slide td { border-color: #00ffff80; }
.progress { background: linear-gradient(90deg, #00ffff, #ff00ff); box-shadow: 0 0 12px #00ffff; }
`

const forestCSS = `
:root {
  --slide-bg: #1b2a1f;
  --slide-fg: #d4e0c5;
  --accent: #8fbc4a;
  --font-body: 'Lora', 'Georgia', serif;
  --font-heading: 'Merriweather', 'Georgia', serif;
  --font-mono: 'JetBrains Mono', 'Fira Code', monospace;
  --color-red: #c45a3a;
  --color-green: #8fbc4a;
  --color-yellow: #d4a35a;
  --color-blue: #6a90b5;
  --color-aqua: #5a9a8a;
}
.slide h1 { color: #c5e08b; }
.slide h2 { color: #8fbc4a; }
.slide h3 { color: #a8d97c; }
.slide pre { background: #15211a; border: 1px solid #3a4d2f; }
.slide :not(pre) > code { background: #2a3a26; color: #d4a35a; }
.slide blockquote { border-left-color: #8fbc4a; color: #b5c7a3; }
.slide a { color: #c5e08b; }
.slide th { background: #15211a; }
.slide th, .slide td { border-color: #3a4d2f; }
.progress { background: #8fbc4a; }
`

const oceanCSS = `
:root {
  --slide-bg: #0a2540;
  --slide-fg: #d4e8f5;
  --accent: #4fc3f7;
  --font-body: 'Inter', system-ui, sans-serif;
  --font-heading: 'Inter', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', 'Fira Code', monospace;
  --color-red: #ff6b6b;
  --color-green: #51cf66;
  --color-yellow: #ffd43b;
  --color-blue: #4dabf7;
  --color-aqua: #80deea;
}
.slide h1 { color: #80deea; }
.slide h2 { color: #4fc3f7; }
.slide h3 { color: #4dd0e1; }
.slide pre { background: #061a2e; border: 1px solid #1e3a5f; }
.slide :not(pre) > code { background: #14304d; color: #80deea; }
.slide blockquote { border-left-color: #4fc3f7; color: #a8c8dc; }
.slide a { color: #80deea; }
.slide th { background: #061a2e; }
.slide th, .slide td { border-color: #1e3a5f; }
.progress { background: linear-gradient(90deg, #4fc3f7, #80deea); }
`

const sunsetCSS = `
:root {
  --slide-bg: #2a0a3a;
  --slide-fg: #fff5e1;
  --accent: #ff9a3c;
  --font-body: 'Inter', system-ui, sans-serif;
  --font-heading: 'Inter', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', 'Fira Code', monospace;
  --color-red: #ff6b6b;
  --color-green: #b5e87e;
  --color-yellow: #ffd57a;
  --color-blue: #6a9bd5;
  --color-aqua: #9cd8e3;
}
body {
  background: linear-gradient(135deg, #2a0a3a 0%, #6b1f4a 40%, #c44569 70%, #ff9a3c 100%);
}
.slide h1 { color: #ffd57a; text-shadow: 0 2px 12px rgba(255, 154, 60, 0.4); }
.slide h2 { color: #ff9a3c; }
.slide h3 { color: #ffb86b; }
.slide pre { background: rgba(0, 0, 0, 0.45); border: 1px solid rgba(255, 154, 60, 0.3); }
.slide :not(pre) > code { background: rgba(0, 0, 0, 0.35); color: #ffd57a; }
.slide blockquote { border-left-color: #ff9a3c; color: #fff5e1cc; }
.slide a { color: #ffd57a; }
.slide th { background: rgba(0, 0, 0, 0.35); }
.slide th, .slide td { border-color: rgba(255, 154, 60, 0.3); }
.progress { background: linear-gradient(90deg, #ff9a3c, #c44569); }
`

// editDosCSS evokes the MS-DOS EDIT.COM / QBasic editor: IBM VGA blue
// background (#0000AA), bright white body text, yellow chrome accents,
// and box-drawing frames rendered via ::before pseudo-elements on the
// slide container. Monospace only — no web fonts fetched.
const editDosCSS = `
:root {
  --slide-bg: #0000aa;
  --slide-fg: #ffffff;
  --accent: #ffff55;
  --font-body: 'Perfect DOS VGA 437', 'IBM Plex Mono', 'Cascadia Code', 'Courier New', monospace;
  --font-heading: 'Perfect DOS VGA 437', 'IBM Plex Mono', 'Cascadia Code', 'Courier New', monospace;
  --font-mono: 'Perfect DOS VGA 437', 'IBM Plex Mono', 'Cascadia Code', 'Courier New', monospace;
  --color-red: #ff5555;
  --color-green: #55ff55;
  --color-yellow: #ffff55;
  --color-blue: #5555ff;
  --color-aqua: #55ffff;
  --color-dim: #aaaaaa;
  --slide-padding: 3.5vmin;
}
body {
  background: #0000aa;
  font-feature-settings: "calt" 0, "liga" 0;
}
.slide {
  /* Double-ruled frame in the classic IBM CP437 style. The outer border
   * is the main window; the inner ::before draws a title bar region. */
  border: 2px solid #ffffff;
  box-shadow:
    inset 0 0 0 4px #0000aa,
    inset 0 0 0 5px #ffffff,
    0 0 0 2px #000000;
  padding: calc(var(--slide-padding) + 1.5em) var(--slide-padding) var(--slide-padding);
  background:
    linear-gradient(#ffff55, #ffff55) 0 0 / 100% 1.6em no-repeat,
    #0000aa;
}
.slide::before {
  content: '─[ FILE ]── EDIT ── SEARCH ── VIEW ── OPTIONS ── HELP ──────';
  position: absolute;
  top: 0.15em;
  left: 0.6em;
  right: 0.6em;
  height: 1.4em;
  line-height: 1.4em;
  color: #000000;
  font-family: var(--font-mono);
  font-size: 0.7em;
  letter-spacing: 0.05em;
  pointer-events: none;
  white-space: nowrap;
  overflow: hidden;
}
.slide h1 {
  color: #ffff55;
  background: #0000aa;
  text-shadow: 2px 2px 0 #000000;
  border-bottom: 1px dashed #ffff55;
  padding-bottom: 0.2em;
  margin-bottom: 0.5em;
  font-size: 2em;
}
.slide h2 { color: #55ffff; border-bottom: 1px dotted #55ffff; padding-bottom: 0.15em; font-size: 1.6em; }
.slide h3 { color: #55ff55; font-size: 1.25em; }
.slide p, .slide li { color: #ffffff; }
.slide strong { color: #ffff55; }
.slide em { color: #55ffff; font-style: normal; text-decoration: underline; }
.slide a { color: #55ffff; text-decoration: underline; }
.slide ul { list-style: none; padding-left: 1em; }
.slide ul li::before { content: '► '; color: #ffff55; margin-right: 0.3em; }
.slide ol { padding-left: 1.8em; color: #ffff55; }
.slide ol li { color: #ffffff; }
.slide pre {
  background: #000000;
  color: #55ff55;
  border: 1px solid #ffffff;
  border-radius: 0;
  padding: 0.6em 1em;
  box-shadow: inset 0 0 0 1px #0000aa;
}
.slide :not(pre) > code {
  background: #000000;
  color: #ffff55;
  padding: 0 0.3em;
  border-radius: 0;
  border: 1px solid #55ffff;
}
.slide blockquote {
  border-left: 3px double #ffff55;
  color: #aaaaaa;
  background: color-mix(in srgb, #000000 40%, transparent);
  padding: 0.4em 0.8em;
  font-style: normal;
}
.slide table { border-collapse: collapse; }
.slide th {
  background: #ffff55;
  color: #0000aa;
  border: 1px solid #ffffff;
  padding: 0.3em 0.7em;
}
.slide td {
  border: 1px solid #55ffff;
  padding: 0.25em 0.7em;
}
.slide hr, .slide .waxon-hr {
  border: none;
  border-top: 1px solid #55ffff;
  opacity: 1;
}
.progress {
  background: #ffff55;
  box-shadow: 0 0 0 1px #ffffff;
}
.footer, .footer-counter {
  color: #ffff55;
  opacity: 0.85;
}
/* Card / compare / stat blocks get the signature DOS dialog border. */
.slide .waxon-card,
.slide .waxon-compare-pane,
.slide .waxon-stat,
.slide .waxon-quote,
.slide .waxon-timeline,
.slide .waxon-flow,
.slide .waxon-grid-cell {
  border: 1px solid #ffffff;
  box-shadow: inset 0 0 0 1px #0000aa, 1px 1px 0 #000000;
  background: color-mix(in srgb, #000000 30%, transparent);
}
.slide .waxon-card-left {
  border-left: 3px double #ffff55;
}
.slide .waxon-badge { border-radius: 0; border-width: 1px; }
`

// win98CSS — inspired by 98.css (github.com/jdan/98.css), MIT licensed.
// Pixel-beveled dialog chrome, navy title bar, silver canvas.
const win98CSS = `
:root {
  --slide-bg: #c0c0c0;
  --slide-fg: #000000;
  --accent: #000080;
  --font-body: 'Pixelated MS Sans Serif', 'MS Sans Serif', Tahoma, 'Microsoft Sans Serif', Arial, sans-serif;
  --font-heading: 'Pixelated MS Sans Serif', 'MS Sans Serif', Tahoma, Arial, sans-serif;
  --font-mono: 'Perfect DOS VGA 437', 'Lucida Console', 'Courier New', monospace;
  --color-red: #aa0000;
  --color-green: #008000;
  --color-yellow: #808000;
  --color-blue: #000080;
  --color-aqua: #008080;
  --color-dim: #808080;
  --slide-padding: 3vmin;
}
body { background: #008080; } /* classic teal desktop */
.slide {
  /* Beveled silver dialog: highlight top+left, shadow right+bottom. */
  background: #c0c0c0;
  color: #000;
  border: 1px solid #0a0a0a;
  box-shadow:
    inset -1px -1px #0a0a0a,
    inset 1px 1px #dfdfdf,
    inset -2px -2px #808080,
    inset 2px 2px #ffffff;
  padding: calc(var(--slide-padding) + 1.8em) var(--slide-padding) var(--slide-padding);
  border-radius: 0;
}
.slide::before {
  /* Navy title bar — the iconic Win98 gradient. */
  content: '■  Slide.exe';
  position: absolute;
  top: 3px; left: 3px; right: 3px;
  height: 1.5em;
  line-height: 1.5em;
  padding: 0 0.4em;
  background: linear-gradient(90deg, #000080 0%, #1084d0 100%);
  color: #ffffff;
  font-family: var(--font-heading);
  font-size: 0.75em;
  font-weight: bold;
  letter-spacing: 0.02em;
  white-space: nowrap;
  overflow: hidden;
  pointer-events: none;
}
.slide::after {
  /* Fake min/max/close buttons on the right edge of the title bar. */
  content: '_ □ ×';
  position: absolute;
  top: 3px;
  right: 0.6em;
  height: 1.5em;
  line-height: 1.3em;
  color: #ffffff;
  font-family: var(--font-heading);
  font-size: 0.7em;
  letter-spacing: 0.35em;
  pointer-events: none;
}
.slide h1 { color: #000080; font-weight: bold; font-size: 2em; border-bottom: 1px solid #808080; padding-bottom: 0.2em; }
.slide h2 { color: #000000; font-weight: bold; font-size: 1.5em; }
.slide h3 { color: #000000; font-weight: bold; font-size: 1.2em; }
.slide p, .slide li { color: #000000; }
.slide strong { color: #000080; }
.slide a { color: #0000ee; text-decoration: underline; }
.slide ul li::marker { color: #000080; }
.slide pre {
  background: #ffffff;
  color: #000000;
  border: 1px solid #0a0a0a;
  border-radius: 0;
  box-shadow: inset -1px -1px #dfdfdf, inset 1px 1px #808080;
  padding: 0.6em 0.9em;
  font-family: var(--font-mono);
}
.slide :not(pre) > code {
  background: #ffffff;
  color: #000000;
  border: 1px solid #808080;
  padding: 0 0.25em;
  border-radius: 0;
  font-family: var(--font-mono);
}
.slide blockquote {
  background: #ffffff;
  border: 1px solid #0a0a0a;
  border-left: 4px solid #000080;
  color: #000000;
  box-shadow: inset -1px -1px #dfdfdf, inset 1px 1px #c0c0c0;
  padding: 0.5em 0.9em;
  font-style: normal;
}
.slide table { border-collapse: collapse; }
.slide th {
  background: #c0c0c0;
  color: #000000;
  border: 1px solid #0a0a0a;
  box-shadow: inset -1px -1px #808080, inset 1px 1px #ffffff;
  padding: 0.35em 0.8em;
  font-weight: bold;
}
.slide td { border: 1px solid #808080; padding: 0.3em 0.8em; background: #ffffff; }
.slide hr, .slide .waxon-hr {
  border: none;
  height: 2px;
  background: none;
  box-shadow: 0 1px #808080, 0 2px #ffffff;
}
.slide .waxon-card,
.slide .waxon-compare-pane,
.slide .waxon-stat,
.slide .waxon-quote,
.slide .waxon-timeline,
.slide .waxon-flow,
.slide .waxon-grid-cell {
  background: #c0c0c0;
  color: #000;
  border: 1px solid #0a0a0a;
  border-radius: 0;
  box-shadow:
    inset -1px -1px #0a0a0a,
    inset 1px 1px #dfdfdf,
    inset -2px -2px #808080,
    inset 2px 2px #ffffff;
}
.slide .waxon-badge {
  background: #c0c0c0;
  color: #000;
  border: 1px solid #0a0a0a;
  border-radius: 0;
  box-shadow: inset -1px -1px #808080, inset 1px 1px #ffffff;
}
.progress { background: #000080; }
.footer, .footer-counter { color: #000080; }
`

// nesCSS — inspired by NES.css (github.com/nostalgic-css/NES.css), MIT licensed.
// 8-bit pixel borders using stepped box-shadows to avoid SVG border-image.
const nesCSS = `
@import url('https://fonts.googleapis.com/css2?family=Press+Start+2P&display=swap');
:root {
  --slide-bg: #212529;
  --slide-fg: #ffffff;
  --accent: #209cee;
  --font-body: 'Press Start 2P', 'Courier New', monospace;
  --font-heading: 'Press Start 2P', 'Courier New', monospace;
  --font-mono: 'Press Start 2P', 'Courier New', monospace;
  --color-red: #e76e55;
  --color-green: #92cc41;
  --color-yellow: #f7d51d;
  --color-blue: #209cee;
  --color-aqua: #7ad6f0;
  --color-dim: #adafbc;
  --slide-padding: 4vmin;
}
body { background: #0c0d10; }
.slide {
  background: #212529;
  color: #ffffff;
  border: none;
  border-radius: 0;
  /* 4px pixel-stepped border built from box-shadows. */
  box-shadow:
    0 -4px #ffffff, 0 4px #ffffff,
    -4px 0 #ffffff, 4px 0 #ffffff,
    0 -8px #212529, 0 8px #212529,
    -8px 0 #212529, 8px 0 #212529,
    -4px -4px 0 4px #ffffff,
    4px -4px 0 4px #ffffff,
    -4px 4px 0 4px #ffffff,
    4px 4px 0 4px #ffffff;
  margin: 8px;
  font-size: 0.78em; /* Press Start 2P is chunky — shrink baseline */
  line-height: 1.8;
}
.slide h1 { color: #f7d51d; font-size: 1.6em; text-shadow: 3px 3px 0 #000000; margin-bottom: 0.8em; }
.slide h2 { color: #209cee; font-size: 1.2em; text-shadow: 2px 2px 0 #000000; }
.slide h3 { color: #92cc41; font-size: 1em; }
.slide p, .slide li { color: #ffffff; }
.slide strong { color: #f7d51d; }
.slide em { color: #7ad6f0; font-style: normal; }
.slide a { color: #209cee; text-decoration: underline; text-underline-offset: 4px; }
.slide ul { list-style: none; padding-left: 1.5em; }
.slide ul li { position: relative; }
.slide ul li::before {
  content: '';
  position: absolute;
  left: -1.2em;
  top: 0.5em;
  width: 0.6em;
  height: 0.6em;
  background: #92cc41;
  box-shadow: 0 0 0 2px #000000;
}
.slide ol { padding-left: 2em; }
.slide ol li::marker { color: #f7d51d; font-weight: bold; }
.slide pre {
  background: #000000;
  color: #92cc41;
  border: none;
  border-radius: 0;
  padding: 1em;
  box-shadow:
    0 -4px #ffffff, 0 4px #ffffff,
    -4px 0 #ffffff, 4px 0 #ffffff;
  margin: 8px;
  font-family: 'Courier New', monospace;
  font-size: 0.9em;
}
.slide :not(pre) > code {
  background: #000000;
  color: #f7d51d;
  padding: 0.1em 0.4em;
  border-radius: 0;
  font-family: 'Courier New', monospace;
}
.slide blockquote {
  background: #1b1e22;
  color: #adafbc;
  border: none;
  padding: 0.8em 1.2em;
  margin: 8px 16px;
  box-shadow:
    0 -4px #7ad6f0, 0 4px #7ad6f0,
    -4px 0 #7ad6f0, 4px 0 #7ad6f0;
  font-style: normal;
}
.slide table { border-collapse: separate; border-spacing: 0; }
.slide th {
  background: #209cee;
  color: #ffffff;
  padding: 0.5em 0.9em;
  border: 2px solid #ffffff;
}
.slide td {
  background: #212529;
  color: #ffffff;
  padding: 0.5em 0.9em;
  border: 2px solid #ffffff;
}
.slide hr, .slide .waxon-hr {
  border: none;
  height: 4px;
  background:
    repeating-linear-gradient(90deg, #ffffff 0 8px, transparent 8px 16px);
}
.slide .waxon-card,
.slide .waxon-compare-pane,
.slide .waxon-stat,
.slide .waxon-quote,
.slide .waxon-timeline,
.slide .waxon-flow,
.slide .waxon-grid-cell {
  background: #1b1e22;
  color: #ffffff;
  border: none;
  border-radius: 0;
  padding: 1em;
  box-shadow:
    0 -4px #ffffff, 0 4px #ffffff,
    -4px 0 #ffffff, 4px 0 #ffffff;
  margin: 8px;
}
.slide .waxon-badge {
  background: #f7d51d;
  color: #000000;
  border-radius: 0;
  border: 2px solid #000000;
  padding: 0.2em 0.5em;
  font-weight: normal;
}
.progress { background: #92cc41; box-shadow: 0 0 0 2px #ffffff; }
.footer, .footer-counter { color: #7ad6f0; }
`
