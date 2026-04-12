// Package pdf exports slide decks to PDF using headless Chromium.
package pdf

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/mschulkind-oss/waxon/internal/format"
	"github.com/mschulkind-oss/waxon/internal/render"
)

// Options configures PDF export.
type Options struct {
	Output        string // Output file path
	ThemeOverride string // Override theme from frontmatter
	Variant       string // Choose specific variant
	Pages         string // Page range (e.g., "1-5", "3,7,9")
}

// Export renders a deck to PDF.
func Export(ctx context.Context, deck *format.Deck, opts Options) error {
	theme := deck.Meta.Theme
	if opts.ThemeOverride != "" {
		theme = opts.ThemeOverride
	}

	html, err := render.RenderHTML(deck, render.Options{
		ThemeOverride: theme,
	})
	if err != nil {
		return fmt.Errorf("render HTML: %w", err)
	}

	// Inject print-specific CSS that shows all slides
	html = injectPrintCSS(html)

	// Serve HTML on a local ephemeral port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	})

	srv := &http.Server{Handler: mux}
	go func() {
		// http.ErrServerClosed is the expected outcome when defer srv.Close() runs.
		_ = srv.Serve(listener)
	}()
	defer srv.Close()

	addr := fmt.Sprintf("http://%s/", listener.Addr().String())

	// Configure chromedp
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx,
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Flag("disable-dev-shm-usage", true),
	)
	defer allocCancel()

	chromCtx, chromCancel := chromedp.NewContext(allocCtx)
	defer chromCancel()

	// Set a timeout for the whole operation
	chromCtx, timeoutCancel := context.WithTimeout(chromCtx, 60*time.Second)
	defer timeoutCancel()

	// Calculate page dimensions based on aspect ratio
	paperWidth, paperHeight := paperSize(deck.Meta.Aspect)

	var pdfBuf []byte
	err = chromedp.Run(chromCtx,
		chromedp.Navigate(addr),
		// Wait for the deck to render
		chromedp.WaitVisible(".deck", chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithPreferCSSPageSize(true).
				WithPaperWidth(paperWidth).
				WithPaperHeight(paperHeight).
				WithMarginTop(0).
				WithMarginBottom(0).
				WithMarginLeft(0).
				WithMarginRight(0).
				Do(ctx)
			return err
		}),
	)
	if err != nil {
		return fmt.Errorf("chromedp: %w", err)
	}

	output := opts.Output
	if output == "" {
		// Default: same name as input with .pdf extension
		output = "output.pdf"
	}

	// Ensure output directory exists
	if dir := filepath.Dir(output); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}
	}

	if err := os.WriteFile(output, pdfBuf, 0644); err != nil {
		return fmt.Errorf("write PDF: %w", err)
	}

	return nil
}

// paperSize returns width and height in inches for the given aspect ratio.
func paperSize(aspect string) (float64, float64) {
	// Default to 16:9 landscape at 10 inches wide
	width := 10.0
	height := 5.625 // 10 * 9/16

	parts := strings.SplitN(aspect, ":", 2)
	if len(parts) == 2 {
		var w, h float64
		if _, err := fmt.Sscanf(parts[0], "%f", &w); err == nil {
			if _, err := fmt.Sscanf(parts[1], "%f", &h); err == nil {
				if w > 0 && h > 0 {
					height = width * h / w
					// Round to 3 decimal places
					height = math.Round(height*1000) / 1000
				}
			}
		}
	}

	return width, height
}

// injectPrintCSS adds CSS that lays out all slides for print (one per page).
func injectPrintCSS(html string) string {
	printCSS := `
<style>
/* PDF export: show all slides, one per page */
@media print {
  .slide { display: flex !important; flex-direction: column; justify-content: center; break-after: page; page-break-after: always; position: relative !important; }
  .slide.active { display: flex !important; }
  .footer { position: relative !important; }
  .progress { display: none !important; }
}
/* For PDF rendering: show all slides stacked */
body { overflow: visible !important; height: auto !important; }
.deck { height: auto !important; position: relative !important; }
.slide {
  display: flex !important;
  flex-direction: column;
  justify-content: center;
  position: relative !important;
  width: 100vw;
  height: 100vh;
  break-after: page;
  page-break-after: always;
}
.progress { display: none !important; }
</style>
<!-- WebSocket disabled for PDF export -->
<script>window.WebSocket = undefined;</script>
`
	// Inject before </head>
	return strings.Replace(html, "</head>", printCSS+"</head>", 1)
}
