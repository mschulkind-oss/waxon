package pdf

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mschulkind-oss/waxon/internal/format"
	"github.com/mschulkind-oss/waxon/internal/render"
)

// HTMLOptions configures static HTML export.
type HTMLOptions struct {
	Output        string // Output file path
	ThemeOverride string // Override theme from frontmatter
	IncludeNotes  bool   // Bundle speaker notes into the export
}

// ExportHTML renders a deck to a single self-contained HTML file that uses
// the full interactive viewer but has server features (websocket reload,
// comment posting) disabled so it works offline.
func ExportHTML(deck *format.Deck, opts HTMLOptions) error {
	theme := deck.Meta.Theme
	if opts.ThemeOverride != "" {
		theme = opts.ThemeOverride
	}

	html, err := render.RenderHTML(deck, render.Options{
		ThemeOverride: theme,
		Standalone:    true,
		IncludeNotes:  opts.IncludeNotes,
	})
	if err != nil {
		return fmt.Errorf("render HTML: %w", err)
	}

	output := opts.Output
	if output == "" {
		output = "output.html"
	}
	if dir := filepath.Dir(output); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}
	}
	if err := os.WriteFile(output, []byte(html), 0644); err != nil {
		return fmt.Errorf("write HTML: %w", err)
	}
	return nil
}
