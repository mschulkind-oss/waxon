package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/mschulkind-oss/waxon/internal/format"
	"github.com/mschulkind-oss/waxon/internal/pdf"
	"github.com/mschulkind-oss/waxon/internal/server"
	"github.com/mschulkind-oss/waxon/internal/themes"
	"github.com/mschulkind-oss/waxon/internal/version"
	"github.com/spf13/cobra"

	// Ensure themes init() registers CSS callback
	_ "github.com/mschulkind-oss/waxon/internal/themes"
)

// Shared color styles
var (
	bold    = color.New(color.Bold)
	dim     = color.New(color.Faint)
	accent  = color.New(color.FgHiMagenta, color.Bold)
	success = color.New(color.FgHiGreen)
	info    = color.New(color.FgHiCyan)
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "waxon",
		Short: "A slide deck toolkit built for the mind meld between human and agent",
		Long: accent.Sprint("waxon") + ` — slides for the mind meld between human and agent.

A text-first slide deck toolkit. Write slides in Markdown, present them
in the browser, export to PDF. Designed so humans and AI agents can
collaborate on the same ` + bold.Sprint(".slides") + ` file.

` + dim.Sprint("Quick start:") + `
  ` + info.Sprint("waxon new my-talk") + `          Create a new deck
  ` + info.Sprint("waxon serve my-talk.slides") + `  Live preview with hot reload
  ` + info.Sprint("waxon export my-talk.slides") + ` Export to PDF

` + dim.Sprint("Agent workflow:") + `
  ` + info.Sprint("waxon agent-context deck.slides") + ` | your-agent "improve the flow"
  ` + info.Sprint("waxon comment deck.slides --json") + ` | your-agent "resolve these"

` + dim.Sprint("Available themes:") + ` default, light, corporate, minimal, vibrant, terminal
` + dim.Sprint("Run") + ` ` + info.Sprint("waxon themes") + ` ` + dim.Sprint("to see them all."),
		Version: version.String(),
	}

	cmd.SetVersionTemplate(accent.Sprint("waxon") + " {{.Version}}\n")

	cmd.AddCommand(serveCmd())
	cmd.AddCommand(exportCmd())
	cmd.AddCommand(newCmd())
	cmd.AddCommand(themesCmd())
	cmd.AddCommand(commentCmd())
	cmd.AddCommand(agentContextCmd())
	cmd.AddCommand(serviceCmd())

	return cmd
}

func serveCmd() *cobra.Command {
	var (
		port      string
		bind      string
		theme     string
		noOpen    bool
		presenter bool
	)

	cmd := &cobra.Command{
		Use:   "serve <file.slides>",
		Short: "Start a live-preview server with hot reload",
		Long: bold.Sprint("Start a live-preview server with hot reload.") + `

Opens your slides in the browser and watches the .slides file for changes.
Every save triggers an instant reload via WebSocket — no manual refresh needed.

` + dim.Sprint("Keyboard controls in the browser:") + `
  Right/Space/Enter  Next slide or reveal next pause
  Left/Backspace     Previous slide
  f                  Toggle fullscreen
  1-9                Jump to slide N
  Home/End           First/last slide

` + dim.Sprint("Examples:") + `
  ` + info.Sprint("waxon serve deck.slides") + `
  ` + info.Sprint("waxon serve deck.slides --theme terminal") + `
  ` + info.Sprint("waxon serve deck.slides --port 3000 --no-open") + `

` + dim.Sprint("For agents:") + `
  Start the server in the background, then use ` + info.Sprint("waxon agent-context") + `
  to read the deck structure. Edit the .slides file directly — the
  browser will reload automatically.`,
		Example: `  waxon serve deck.slides
  waxon serve deck.slides --theme terminal --port 3000
  waxon serve my-talk.slides --no-open --bind 127.0.0.1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer cancel()

			s, err := server.New(server.Config{
				File:          args[0],
				Port:          port,
				Bind:          bind,
				ThemeOverride: theme,
				NoOpen:        noOpen,
			})
			if err != nil {
				return err
			}

			_ = presenter // TODO: presenter mode

			return s.ListenAndServe(ctx)
		},
	}

	cmd.Flags().StringVar(&port, "port", "", "HTTP server port (default $WK_PORT or 8080)")
	cmd.Flags().StringVar(&bind, "bind", "0.0.0.0", "Bind address")
	cmd.Flags().StringVar(&theme, "theme", "", "Override the theme from frontmatter")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Don't auto-open browser")
	cmd.Flags().BoolVar(&presenter, "presenter", false, "Start in presenter mode")

	return cmd
}

func exportCmd() *cobra.Command {
	var (
		output  string
		theme   string
		variant string
		pages   string
	)

	cmd := &cobra.Command{
		Use:   "export <file.slides>",
		Short: "Export to PDF",
		Long: bold.Sprint("Export slides to PDF using headless Chromium.") + `

Renders your deck to pixel-perfect PDF, one slide per page, preserving
theme styling, syntax highlighting, and layout.

` + dim.Sprint("Output defaults to the input filename with a .pdf extension.") + `

` + dim.Sprint("Examples:") + `
  ` + info.Sprint("waxon export deck.slides") + `                    → deck.pdf
  ` + info.Sprint("waxon export deck.slides -o talk.pdf") + `        → talk.pdf
  ` + info.Sprint("waxon export deck.slides --theme minimal") + `    Override theme

` + dim.Sprint("For agents:") + `
  Export after editing to verify visual output. Combine with
  ` + info.Sprint("waxon agent-context") + ` to read the deck, make changes,
  then export to confirm the result.`,
		Example: `  waxon export deck.slides
  waxon export deck.slides -o presentation.pdf
  waxon export deck.slides --theme corporate -o quarterly.pdf`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}

			deck, err := format.Parse(string(data))
			if err != nil {
				return err
			}

			// Default output name
			if output == "" {
				base := strings.TrimSuffix(filepath.Base(args[0]), filepath.Ext(args[0]))
				output = base + ".pdf"
			}

			_ = variant // TODO: variant selection
			_ = pages   // TODO: page range filtering

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer cancel()

			fmt.Fprintf(cmd.OutOrStdout(), "%s Exporting to %s...\n", info.Sprint("→"), bold.Sprint(output))
			if err := pdf.Export(ctx, deck, pdf.Options{
				Output:        output,
				ThemeOverride: theme,
				Variant:       variant,
				Pages:         pages,
			}); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s Wrote %s\n", success.Sprint("✓"), bold.Sprint(output))
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (default <name>.pdf)")
	cmd.Flags().StringVar(&theme, "theme", "", "Override the theme")
	cmd.Flags().StringVar(&variant, "variant", "", "Choose specific variants by name")
	cmd.Flags().StringVar(&pages, "pages", "", "Page range (e.g., 1-5, 3,7,9)")

	return cmd
}

func newCmd() *cobra.Command {
	var (
		theme string
		ratio string
	)

	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a new .slides file from a starter template",
		Long: bold.Sprint("Create a new .slides file from a starter template.") + `

Generates a ready-to-edit deck with YAML frontmatter, a title slide,
an agenda slide, and a closing slide. Start editing immediately or
run ` + info.Sprint("waxon serve") + ` to preview.

` + dim.Sprint("The .slides extension is added automatically if omitted.") + `

` + dim.Sprint("Examples:") + `
  ` + info.Sprint("waxon new my-talk") + `                    → my-talk.slides
  ` + info.Sprint("waxon new quarterly --theme corporate") + ` → quarterly.slides
  ` + info.Sprint("waxon new demo --theme terminal --ratio 4:3") + `

` + dim.Sprint("For agents:") + `
  Use this to scaffold a new deck, then edit the generated .slides
  file directly. The format is YAML frontmatter + Markdown.`,
		Example: `  waxon new my-talk
  waxon new quarterly-update --theme corporate
  waxon new demo --theme terminal --ratio 4:3`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			filename := name
			if !strings.HasSuffix(filename, ".slides") {
				filename += ".slides"
			}

			if _, err := os.Stat(filename); err == nil {
				return fmt.Errorf("%s already exists", filename)
			}

			content := fmt.Sprintf(`---
title: "%s"
author: ""
theme: %s
aspect: "%s"
footer: "%s"
---

# %s

Welcome to your presentation.

---

# Agenda

- Topic one
- Topic two
- Topic three

---

# Thank You

Questions?
`, name, theme, ratio, name, name)

			if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s Created %s\n", success.Sprint("✓"), bold.Sprint(filename))
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", dim.Sprint("Next steps:"))
			fmt.Fprintf(cmd.OutOrStdout(), "  %s  Edit the file\n", info.Sprint("$EDITOR "+filename))
			fmt.Fprintf(cmd.OutOrStdout(), "  %s   Live preview\n", info.Sprint("waxon serve "+filename))
			return nil
		},
	}

	cmd.Flags().StringVar(&theme, "theme", "default", "Set the initial theme")
	cmd.Flags().StringVar(&ratio, "ratio", "16:9", "Set the aspect ratio")

	return cmd
}

func themesCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "themes",
		Short: "List available themes",
		Long: bold.Sprint("List all built-in themes.") + `

Themes control colors, fonts, and styling for your slides. Set the theme
in your .slides frontmatter:

  ` + dim.Sprint("---") + `
  ` + dim.Sprint("theme: terminal") + `
  ` + dim.Sprint("---") + `

Or override at runtime:
  ` + info.Sprint("waxon serve deck.slides --theme vibrant") + `
  ` + info.Sprint("waxon export deck.slides --theme corporate") + `

` + dim.Sprint("The terminal theme supports sub-variants:") + ` nord, gruvbox, everforest, vitesse
  Set via ` + dim.Sprint("terminal_variant: nord") + ` in frontmatter.

` + dim.Sprint("For agents:") + `
  Use ` + info.Sprint("--json") + ` for machine-readable output.`,
		Example: `  waxon themes
  waxon themes --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			all := themes.All()
			out := cmd.OutOrStdout()

			if jsonOutput {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(all)
			}

			w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
			fmt.Fprintf(w, "%s\t%s\n", bold.Sprint("THEME"), bold.Sprint("DESCRIPTION"))
			for _, t := range all {
				fmt.Fprintf(w, "%s\t%s\n", accent.Sprint(t.Name), t.Description)
			}
			w.Flush()

			fmt.Fprintf(out, "\n%s Use %s in frontmatter, or %s at runtime.\n",
				dim.Sprint("Tip:"),
				info.Sprint("theme: <name>"),
				info.Sprint("--theme <name>"),
			)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func commentCmd() *cobra.Command {
	var (
		add     string
		slide   int
		author  string
		resolve int
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "comment <file.slides>",
		Short: "View and manage comments in a .slides file",
		Long: bold.Sprint("View and manage comments in a .slides file.") + `

Comments live inside the .slides file as ` + dim.Sprint("<!-- comment(@author): text -->") + `
directives. They survive version control, diffs, and agent handoffs.

` + dim.Sprint("List comments:") + `
  ` + info.Sprint("waxon comment deck.slides") + `
  ` + info.Sprint("waxon comment deck.slides --author alice") + `
  ` + info.Sprint("waxon comment deck.slides --slide 3") + `

` + dim.Sprint("Add a comment:") + `
  ` + info.Sprint("waxon comment deck.slides --add \"needs data\" --slide 2 --author me") + `

` + dim.Sprint("For agents:") + `
  Use ` + info.Sprint("--json") + ` to get structured output. Comments are a collaboration
  channel — agents can read comments to understand what humans want
  changed, and add their own to explain what they did.

  Read:  ` + info.Sprint("waxon comment deck.slides --json") + `
  Write: ` + info.Sprint("waxon comment deck.slides --add \"done\" --slide 2 --author agent") + `

  Or edit the .slides file directly — comments are just HTML comments.`,
		Example: `  waxon comment deck.slides
  waxon comment deck.slides --json
  waxon comment deck.slides --author alice --slide 3
  waxon comment deck.slides --add "needs more data" --slide 2 --author me`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}

			deck, err := format.Parse(string(data))
			if err != nil {
				return err
			}

			// Add a comment
			if add != "" {
				if slide < 1 || slide > len(deck.Slides) {
					return fmt.Errorf("--slide must be between 1 and %d", len(deck.Slides))
				}
				comment := fmt.Sprintf("<!-- comment(@%s): %s -->", author, add)
				return appendToSlide(args[0], string(data), slide, comment)
			}

			// Resolve a comment
			if cmd.Flags().Changed("resolve") {
				_ = resolve
				return fmt.Errorf("--resolve not yet implemented")
			}

			// List comments
			type commentEntry struct {
				Slide   int            `json:"slide"`
				Comment format.Comment `json:"comment"`
			}
			var comments []commentEntry
			for _, s := range deck.Slides {
				for _, c := range s.Comments {
					if author != "" && c.Author != author {
						continue
					}
					if slide > 0 && s.Index+1 != slide {
						continue
					}
					comments = append(comments, commentEntry{
						Slide:   s.Index + 1,
						Comment: c,
					})
				}
			}

			out := cmd.OutOrStdout()
			if jsonOut {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(comments)
			}

			if len(comments) == 0 {
				fmt.Fprintln(out, dim.Sprint("No comments found."))
				return nil
			}

			for _, c := range comments {
				fmt.Fprintf(out, "%s %s: %s\n",
					info.Sprintf("Slide %d", c.Slide),
					accent.Sprintf("@%s", c.Comment.Author),
					c.Comment.Text,
				)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&add, "add", "", "Add a comment to the file")
	cmd.Flags().IntVar(&slide, "slide", 0, "Filter by or target slide number (1-indexed)")
	cmd.Flags().StringVar(&author, "author", "", "Filter by author or set author for --add")
	cmd.Flags().IntVar(&resolve, "resolve", 0, "Mark a comment as resolved")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON (agent-friendly)")

	return cmd
}

func appendToSlide(filename, content string, slideNum int, directive string) error {
	// Split into frontmatter and body
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return fmt.Errorf("could not parse file structure")
	}

	frontmatter := parts[0] + "---" + parts[1] + "---"
	body := parts[2]

	// Split body on slide separators
	slides := strings.Split(body, "\n---\n")
	if slideNum < 1 || slideNum > len(slides) {
		return fmt.Errorf("slide %d out of range", slideNum)
	}

	// Append directive to the target slide
	slides[slideNum-1] = strings.TrimRight(slides[slideNum-1], "\n") + "\n" + directive + "\n"

	newBody := strings.Join(slides, "\n---\n")
	return os.WriteFile(filename, []byte(frontmatter+newBody), 0644)
}

func agentContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent-context <file.slides>",
		Short: "Emit structured JSON context for AI agents",
		Long: bold.Sprint("Emit structured JSON context for AI agents.") + `

Outputs the full parsed structure of a .slides file as JSON: metadata,
slides, speaker notes, AI notes, comments, variants, and directives.

This is the primary interface for agents to ` + bold.Sprint("read") + ` a deck. Pipe it into
any agent or LLM:

  ` + info.Sprint("waxon agent-context deck.slides | claude \"improve the narrative flow\"") + `
  ` + info.Sprint("waxon agent-context deck.slides | jq '.deck.slides[].content'") + `

` + dim.Sprint("JSON structure:") + `
  {
    "file": "/absolute/path/to/deck.slides",
    "deck": {
      "meta": { "title", "author", "theme", "aspect", ... },
      "slides": [
        { "index", "content", "notes", "ai_notes", "comments", "variants", ... }
      ]
    }
  }

` + dim.Sprint("For agents:") + `
  1. Read the deck:  ` + info.Sprint("waxon agent-context deck.slides") + `
  2. Edit the .slides file directly (it's just Markdown)
  3. Add ` + dim.Sprint("<!-- ai: your reasoning here -->") + ` to explain changes
  4. Add ` + dim.Sprint("<!-- comment(@agent): message -->") + ` to communicate with the human
  5. If ` + info.Sprint("waxon serve") + ` is running, the browser refreshes automatically`,
		Example: `  waxon agent-context deck.slides
  waxon agent-context deck.slides | jq '.deck.meta'
  waxon agent-context deck.slides | claude "suggest improvements"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}

			deck, err := format.Parse(string(data))
			if err != nil {
				return err
			}

			// Include file path in output
			absPath, _ := filepath.Abs(args[0])
			type contextOutput struct {
				File string       `json:"file"`
				Deck *format.Deck `json:"deck"`
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(contextOutput{
				File: absPath,
				Deck: deck,
			})
		},
	}

	return cmd
}
