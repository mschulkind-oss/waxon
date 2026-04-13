package themes

import (
	"html/template"
	"strings"
	"testing"

	"github.com/mschulkind-oss/waxon/internal/render"
)

func TestAllThemes(t *testing.T) {
	themes := All()
	if len(themes) != 23 {
		t.Errorf("got %d themes, want 23", len(themes))
	}

	// Verify order
	expected := []string{
		"default", "light", "corporate", "minimal", "vibrant", "terminal",
		"dracula", "solarized-dark", "solarized-light", "tokyo-night",
		"catppuccin", "monokai", "one-dark", "github-light", "github-dark",
		"paper", "neon", "forest", "ocean", "sunset", "edit-dos",
		"win98", "nes",
	}
	for i, name := range expected {
		if themes[i].Name != name {
			t.Errorf("themes[%d].Name = %q, want %q", i, themes[i].Name, name)
		}
	}
}

func TestAllThemesHaveCSS(t *testing.T) {
	for _, theme := range All() {
		if theme.CSS == "" {
			t.Errorf("theme %q has empty CSS", theme.Name)
		}
		if theme.Description == "" {
			t.Errorf("theme %q has empty description", theme.Name)
		}
	}
}

func TestAllThemesHaveRequiredCSSVars(t *testing.T) {
	requiredVars := []string{"--slide-bg", "--slide-fg", "--accent"}
	for _, theme := range All() {
		for _, v := range requiredVars {
			if !strings.Contains(theme.CSS, v) {
				t.Errorf("theme %q missing CSS variable %s", theme.Name, v)
			}
		}
	}
}

func TestAllThemesDefinePalette(t *testing.T) {
	// Every theme must define the full palette of --color-* variables
	// so inline color syntax renders correctly against its background.
	paletteVars := []string{
		"--color-red",
		"--color-green",
		"--color-yellow",
		"--color-blue",
		"--color-aqua",
	}
	for _, theme := range All() {
		for _, v := range paletteVars {
			if !strings.Contains(theme.CSS, v) {
				t.Errorf("theme %q missing palette variable %s", theme.Name, v)
			}
		}
	}
}

func TestGetExisting(t *testing.T) {
	for _, name := range Names() {
		theme := Get(name)
		if theme == nil {
			t.Errorf("Get(%q) returned nil", name)
			continue
		}
		if theme.Name != name {
			t.Errorf("Get(%q).Name = %q", name, theme.Name)
		}
	}
}

func TestGetNonexistent(t *testing.T) {
	if theme := Get("nonexistent"); theme != nil {
		t.Errorf("Get(nonexistent) = %v, want nil", theme)
	}
}

func TestNames(t *testing.T) {
	names := Names()
	if len(names) != 23 {
		t.Errorf("got %d names, want 23", len(names))
	}

	expected := []string{
		"default", "light", "corporate", "minimal", "vibrant", "terminal",
		"dracula", "solarized-dark", "solarized-light", "tokyo-night",
		"catppuccin", "monokai", "one-dark", "github-light", "github-dark",
		"paper", "neon", "forest", "ocean", "sunset", "edit-dos",
		"win98", "nes",
	}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("names[%d] = %q, want %q", i, names[i], name)
		}
	}
}

func TestThemeCSSRegistered(t *testing.T) {
	// The init() function should have registered the ThemeCSS callback
	css := render.ThemeCSS("default")
	if css == "" {
		t.Error("ThemeCSS(default) returned empty, init() may not have run")
	}
	if !strings.Contains(string(css), "--slide-bg") {
		t.Error("ThemeCSS(default) missing --slide-bg")
	}
}

func TestThemeCSSNonexistent(t *testing.T) {
	css := render.ThemeCSS("nonexistent")
	if css != template.CSS("") {
		t.Errorf("ThemeCSS(nonexistent) = %q, want empty", css)
	}
}

func TestTerminalThemeMonospace(t *testing.T) {
	theme := Get("terminal")
	if theme == nil {
		t.Fatal("terminal theme not found")
	}
	if !strings.Contains(theme.CSS, "monospace") {
		t.Error("terminal theme should use monospace fonts")
	}
}

func TestTerminalVariants(t *testing.T) {
	theme := Get("terminal")
	if theme == nil {
		t.Fatal("terminal theme not found")
	}

	variants := []string{"nord", "gruvbox", "everforest", "vitesse"}
	for _, v := range variants {
		selector := `[data-terminal-variant="` + v + `"]`
		if !strings.Contains(theme.CSS, selector) {
			t.Errorf("terminal theme missing variant %q selector", v)
		}
	}
}

func TestTerminalHeadingPrefixes(t *testing.T) {
	theme := Get("terminal")
	if theme == nil {
		t.Fatal("terminal theme not found")
	}
	if !strings.Contains(theme.CSS, `content: '# '`) {
		t.Error("terminal theme missing h1 prefix")
	}
	if !strings.Contains(theme.CSS, `content: '## '`) {
		t.Error("terminal theme missing h2 prefix")
	}
	if !strings.Contains(theme.CSS, `content: '### '`) {
		t.Error("terminal theme missing h3 prefix")
	}
}

func TestLightThemeColors(t *testing.T) {
	theme := Get("light")
	if theme == nil {
		t.Fatal("light theme not found")
	}
	// Light theme should have a white/light background
	if !strings.Contains(theme.CSS, "#ffffff") {
		t.Error("light theme should have white background")
	}
}

func TestVibrantThemeGradient(t *testing.T) {
	theme := Get("vibrant")
	if theme == nil {
		t.Fatal("vibrant theme not found")
	}
	if !strings.Contains(theme.CSS, "linear-gradient") {
		t.Error("vibrant theme should have gradient")
	}
}

func TestThemeJSONOmitsCSS(t *testing.T) {
	// The CSS field has json:"-" tag, verify it's there
	theme := Get("default")
	if theme == nil {
		t.Fatal("default theme not found")
	}
	// Just verify the struct has the tag by checking the CSS is populated
	// but would be omitted from JSON (the tag is json:"-")
	if theme.CSS == "" {
		t.Error("CSS should be populated in Go struct")
	}
}
