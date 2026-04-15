package format

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Manifest is the deck.yaml composition format. A manifest references
// other slide files and combines them into a single deck. It is the
// minimum-author-touch path: the user names a theme, supplies vars,
// and lists which fragments to assemble — no waxon markup required in
// the manifest itself.
type Manifest struct {
	Title           string         `yaml:"title"`
	Author          string         `yaml:"author,omitempty"`
	Theme           string         `yaml:"theme,omitempty"`
	ThemeOverlay    string         `yaml:"theme-overlay,omitempty"`
	Themes          []string       `yaml:"themes,omitempty"`
	Aspect          string         `yaml:"aspect,omitempty"`
	Footer          string         `yaml:"footer,omitempty"`
	FooterLeft      string         `yaml:"footer-left,omitempty"`
	FooterRight     string         `yaml:"footer-right,omitempty"`
	Transition      string         `yaml:"transition,omitempty"`
	TerminalVariant string         `yaml:"terminal-variant,omitempty"`
	TerminalEffects bool           `yaml:"terminal-effects,omitempty"`
	Fonts           []string       `yaml:"fonts,omitempty"`
	Vars            map[string]any `yaml:"vars,omitempty"`
	Slides          []ManifestItem `yaml:"slides"`
}

// ManifestItem is a single entry in a manifest's slides: list. Exactly
// one of Include or File should be set: Include runs the referenced file
// through the include/template pipeline, File treats it as raw .slides
// markup with no preprocessing. Per-item Vars override top-level Vars
// for the entries they apply to.
type ManifestItem struct {
	Include string         `yaml:"include,omitempty"`
	File    string         `yaml:"file,omitempty"`
	Vars    map[string]any `yaml:"vars,omitempty"`
}

// LoadManifest reads a deck.yaml file and assembles a deck from its
// referenced slide fragments. The pipeline:
//
//  1. Read manifest YAML
//  2. For each slide entry, read the referenced file (relative to the
//     manifest's directory)
//  3. Run the include/template preprocessing pipeline on each fragment,
//     merging top-level vars + per-item vars + caller extraVars
//  4. Concatenate the fragments with `---` separators into a single
//     synthetic .slides body
//  5. Inject the manifest's frontmatter as the deck-level meta
//  6. Hand the assembled string to Parse() so the rest of the pipeline
//     (slide parsing, fences, etc.) is unchanged
func LoadManifest(path string, extraVars map[string]any) (*Deck, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", path, err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	if len(m.Slides) == 0 {
		return nil, fmt.Errorf("manifest %s has no slides", path)
	}

	dir := filepath.Dir(abs)
	var sb strings.Builder
	sb.WriteString("---\n")
	frontmatter, err := manifestFrontmatter(m)
	if err != nil {
		return nil, err
	}
	sb.WriteString(frontmatter)
	sb.WriteString("---\n\n")

	for i, item := range m.Slides {
		ref := item.Include
		raw := false
		if ref == "" {
			ref = item.File
			raw = true
		}
		if ref == "" {
			return nil, fmt.Errorf("manifest %s slide %d has neither include nor file", path, i)
		}
		full := ref
		if !filepath.IsAbs(ref) {
			full = filepath.Join(dir, filepath.FromSlash(ref))
		}
		body, err := os.ReadFile(full)
		if err != nil {
			return nil, fmt.Errorf("read slide %s: %w", ref, err)
		}
		fragment := stripFrontmatter(string(body))

		if !raw {
			vars := mergeVars(m.Vars, item.Vars, extraVars)
			expanded, err := expandIncludes(fragment, full, map[string]bool{})
			if err != nil {
				return nil, fmt.Errorf("slide %s: %w", ref, err)
			}
			rendered, err := renderTemplate(expanded, vars)
			if err != nil {
				return nil, fmt.Errorf("slide %s: %w", ref, err)
			}
			fragment = rendered
		}

		if i > 0 {
			sb.WriteString("\n\n---\n\n")
		}
		sb.WriteString(strings.TrimSpace(fragment))
		sb.WriteString("\n")
	}

	return Parse(sb.String())
}

// manifestFrontmatter serializes the manifest's deck-level meta back to
// YAML so it can be prepended to the synthetic body fed to Parse(). We
// write only the fields a downstream Meta cares about, in a stable
// order, to keep the synthetic output deterministic and debuggable.
func manifestFrontmatter(m Manifest) (string, error) {
	meta := Meta{
		Title:           m.Title,
		Author:          m.Author,
		Theme:           m.Theme,
		ThemeOverlay:    m.ThemeOverlay,
		Themes:          m.Themes,
		Aspect:          m.Aspect,
		Footer:          m.Footer,
		FooterLeft:      m.FooterLeft,
		FooterRight:     m.FooterRight,
		Transition:      m.Transition,
		TerminalVariant: m.TerminalVariant,
		TerminalEffects: m.TerminalEffects,
		Fonts:           m.Fonts,
	}
	out, err := yaml.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("marshal manifest meta: %w", err)
	}
	return string(out), nil
}

// mergeVars layers vars maps left-to-right (later wins). Used to combine
// manifest-level vars with per-item vars and CLI overrides without
// mutating any of the inputs.
func mergeVars(layers ...map[string]any) map[string]any {
	out := map[string]any{}
	for _, m := range layers {
		maps.Copy(out, m)
	}
	return out
}
