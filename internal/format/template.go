package format

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	ttemplate "text/template"

	"gopkg.in/yaml.v3"
)

// includeRe matches `<!-- include: relative/path.slides -->` directives.
// The path is captured trimmed of surrounding whitespace. Authors use
// these to compose decks from reusable fragments without copy-paste.
var includeRe = regexp.MustCompile(`<!--\s*include:\s*(.+?)\s*-->`)

// ParseFile reads a .slides file from disk and parses it through the full
// preprocessing pipeline:
//
//  1. Read the file
//  2. Recursively expand <!-- include: path --> directives, with paths
//     resolved relative to the including file and a circular-include guard
//  3. Extract `vars:` from the (raw, untemplated) frontmatter
//  4. Merge extraVars on top of frontmatter vars (extraVars wins)
//  5. Run text/template substitution on the entire post-include text so
//     {{.var}}, {{range}}, {{if}}, etc. are available everywhere
//  6. Hand the result to the existing in-memory Parse() so the rest of
//     the pipeline (slide split, fences, etc.) is unchanged
//
// Files that don't use any preprocessing features round-trip identically
// to plain Parse(string).
func ParseFile(path string, extraVars map[string]any) (*Deck, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", path, err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	expanded, err := expandIncludes(string(data), abs, map[string]bool{})
	if err != nil {
		return nil, err
	}

	vars, err := extractVars(expanded)
	if err != nil {
		return nil, err
	}
	for k, v := range extraVars {
		if vars == nil {
			vars = map[string]any{}
		}
		vars[k] = v
	}

	rendered, err := renderTemplate(expanded, vars)
	if err != nil {
		return nil, err
	}

	return Parse(rendered)
}

// expandIncludes walks the source text once, replacing every include
// directive with the contents of the referenced file (recursively, with
// the same expansion rules). Paths in include directives are resolved
// relative to the file that contains the directive — `current` is that
// file's absolute path, used both as the root for relative resolution
// and as a key in the visited set.
//
// A file that includes itself (directly or transitively) returns an
// error rather than infinite-looping. Frontmatter inside an included
// file is stripped so fragments can be authored as standalone .slides
// files but composed without their meta blocks bleeding in.
func expandIncludes(text, current string, visited map[string]bool) (string, error) {
	if visited[current] {
		return "", fmt.Errorf("circular include: %s", current)
	}
	visited[current] = true
	defer delete(visited, current)

	dir := filepath.Dir(current)
	var outErr error
	out := includeRe.ReplaceAllStringFunc(text, func(match string) string {
		if outErr != nil {
			return ""
		}
		m := includeRe.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		rel := strings.TrimSpace(m[1])
		if rel == "" {
			outErr = fmt.Errorf("include directive missing path")
			return ""
		}
		full := rel
		if !filepath.IsAbs(rel) {
			full = filepath.Join(dir, filepath.FromSlash(rel))
		}
		full, err := filepath.Abs(full)
		if err != nil {
			outErr = fmt.Errorf("resolve include %s: %w", rel, err)
			return ""
		}
		data, err := os.ReadFile(full)
		if err != nil {
			outErr = fmt.Errorf("read include %s: %w", rel, err)
			return ""
		}
		body := stripFrontmatter(string(data))
		nested, err := expandIncludes(body, full, visited)
		if err != nil {
			outErr = err
			return ""
		}
		// Surround the include with blank lines so an embedded `---`
		// separator sits cleanly between slides regardless of how the
		// directive was formatted in the parent.
		return "\n\n" + strings.TrimSpace(nested) + "\n\n"
	})
	if outErr != nil {
		return "", outErr
	}
	return out, nil
}

// stripFrontmatter removes a leading YAML frontmatter block from a
// fragment so an included file's meta doesn't end up in the middle of
// the parent deck. Behavior matches splitFrontmatter for non-fragments.
func stripFrontmatter(text string) string {
	trimmed := strings.TrimLeft(text, " \t\n")
	if !strings.HasPrefix(trimmed, "---") {
		return text
	}
	rest := trimmed[3:]
	_, body, found := strings.Cut(rest, "\n---")
	if !found {
		return text
	}
	return strings.TrimLeft(body, "\n")
}

// extractVars reads the frontmatter (if present) of the post-include text
// and returns the `vars:` map. Returns (nil, nil) when there's no
// frontmatter or when the vars key is absent. Errors propagate from the
// YAML parser.
func extractVars(text string) (map[string]any, error) {
	trimmed := strings.TrimLeft(text, " \t\n")
	if !strings.HasPrefix(trimmed, "---") {
		return nil, nil
	}
	rest := trimmed[3:]
	yamlContent, _, found := strings.Cut(rest, "\n---")
	if !found {
		return nil, nil
	}
	var probe struct {
		Vars map[string]any `yaml:"vars"`
	}
	if err := yaml.Unmarshal([]byte(yamlContent), &probe); err != nil {
		return nil, fmt.Errorf("parse frontmatter for vars: %w", err)
	}
	return probe.Vars, nil
}

// renderTemplate runs the text through Go's text/template with the given
// vars map. When vars is empty AND the text contains no `{{` action
// markers we skip parsing entirely so a deck that doesn't use templating
// stays byte-identical to the source.
//
// Errors from text/template are wrapped with a clear "template:" prefix
// so the user knows where to look. Undefined variables are surfaced as
// errors via Option("missingkey=error") instead of silent empty strings.
func renderTemplate(text string, vars map[string]any) (string, error) {
	if len(vars) == 0 && !strings.Contains(text, "{{") {
		return text, nil
	}
	tmpl, err := ttemplate.New("deck").
		Option("missingkey=error").
		Parse(text)
	if err != nil {
		return "", fmt.Errorf("template: %w", err)
	}
	data := map[string]any(vars)
	if data == nil {
		data = map[string]any{}
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template: %w", err)
	}
	return buf.String(), nil
}
