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

	"github.com/mschulkind-oss/waxon/internal/format"
	"github.com/mschulkind-oss/waxon/internal/pdf"
	"github.com/mschulkind-oss/waxon/internal/server"
	"github.com/mschulkind-oss/waxon/internal/themes"
	"github.com/spf13/cobra"

	// Ensure themes init() registers CSS callback
	_ "github.com/mschulkind-oss/waxon/internal/themes"
)

var version = "dev"

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "waxon",
		Short:   "A slide deck toolkit built for the mind meld between human and agent",
		Version: version,
	}

	cmd.AddCommand(serveCmd())
	cmd.AddCommand(exportCmd())
	cmd.AddCommand(newCmd())
	cmd.AddCommand(themesCmd())
	cmd.AddCommand(commentCmd())
	cmd.AddCommand(agentContextCmd())

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
		Args:  cobra.ExactArgs(1),
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

	cmd.Flags().StringVar(&port, "port", "", "HTTP server port (default $SM_PORT or 8080)")
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
		Args:  cobra.ExactArgs(1),
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

			fmt.Fprintf(cmd.OutOrStdout(), "Exporting to %s...\n", output)
			if err := pdf.Export(ctx, deck, pdf.Options{
				Output:        output,
				ThemeOverride: theme,
				Variant:       variant,
				Pages:         pages,
			}); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", output)
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
		Args:  cobra.ExactArgs(1),
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

			fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", filename)
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
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			all := themes.All()
			out := cmd.OutOrStdout()

			if jsonOutput {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(all)
			}

			w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "THEME\tDESCRIPTION")
			for _, t := range all {
				fmt.Fprintf(w, "%s\t%s\n", t.Name, t.Description)
			}
			return w.Flush()
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
		Short: "View and manage comments",
		Args:  cobra.ExactArgs(1),
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
				fmt.Fprintln(out, "No comments found.")
				return nil
			}

			for _, c := range comments {
				fmt.Fprintf(out, "Slide %d — @%s: %s\n", c.Slide, c.Comment.Author, c.Comment.Text)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&add, "add", "", "Add a comment")
	cmd.Flags().IntVar(&slide, "slide", 0, "Filter by slide number")
	cmd.Flags().StringVar(&author, "author", "", "Filter by author")
	cmd.Flags().IntVar(&resolve, "resolve", 0, "Mark a comment as resolved")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

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
		Short: "Emit structured context for agents",
		Args:  cobra.ExactArgs(1),
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
