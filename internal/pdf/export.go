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
	DeckDir       string // Directory of the source .slides file (for relative theme paths)
}

// Export renders a deck to PDF.
func Export(ctx context.Context, deck *format.Deck, opts Options) error {
	theme := deck.Meta.Theme
	if opts.ThemeOverride != "" {
		theme = opts.ThemeOverride
	}

	html, err := render.RenderHTML(deck, render.Options{
		ThemeOverride: theme,
		Standalone:    true,
		Print:         true,
		DeckDir:       opts.DeckDir,
	})
	if err != nil {
		return fmt.Errorf("render HTML: %w", err)
	}

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

	// Calculate page dimensions based on aspect ratio
	paperWidth, paperHeight := paperSize(deck.Meta.Aspect)

	pdfBuf, err := renderPDFWithRetry(ctx, addr, paperWidth, paperHeight)
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

// renderPDFWithRetry launches headless Chromium, navigates to the local
// server, and prints the deck to PDF. It retries on transient Chromium
// startup failures — most notably the
// `Check failed: ThreadCache::IsValid(tcache)` PartitionAlloc crash that
// fires sporadically on GitHub Actions runners. Each attempt gets a
// fresh allocator and browser context so a crashed subprocess doesn't
// poison the retry.
func renderPDFWithRetry(ctx context.Context, addr string, paperWidth, paperHeight float64) ([]byte, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		buf, err := renderPDFOnce(ctx, addr, paperWidth, paperHeight)
		if err == nil {
			return buf, nil
		}
		lastErr = err
		// Only retry the specific "chrome failed to start" / PartitionAlloc
		// crash class. Legitimate errors (template issues, navigation
		// timeouts on the server side) should fail fast.
		if !isChromeStartupFailure(err) {
			return nil, err
		}
		// Respect the caller's context — if it has been cancelled, stop.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Small backoff so the OS has time to reap the crashed process
		// and release any partially-mapped memory.
		time.Sleep(time.Duration(attempt) * 250 * time.Millisecond)
	}
	return nil, lastErr
}

// renderPDFOnce performs a single headless-Chromium launch, navigation,
// and PrintToPDF call. All chromedp resources (allocator + browser
// context) are scoped to this function so the caller can retry cleanly.
func renderPDFOnce(ctx context.Context, addr string, paperWidth, paperHeight float64) ([]byte, error) {
	// `SchedulerLoopQuarantine` is a PartitionAlloc feature whose
	// assertion (`ThreadCache::IsValid`) fires sporadically in CI-style
	// environments. We disable it plus the rest of the PartitionAlloc
	// experimental knobs so the crash class is eliminated at its source
	// rather than relying solely on retries.
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx,
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-features", "SchedulerLoopQuarantine,PartitionAllocBackupRefPtr,PartitionAllocPCScan,PartitionAllocMemoryTagging"),
		chromedp.Flag("disable-crash-reporter", true),
		chromedp.Flag("no-zygote", true),
		chromedp.Flag("single-process", false),
	)
	defer allocCancel()

	chromCtx, chromCancel := chromedp.NewContext(allocCtx)
	defer chromCancel()

	chromCtx, timeoutCancel := context.WithTimeout(chromCtx, 60*time.Second)
	defer timeoutCancel()

	var pdfBuf []byte
	err := chromedp.Run(chromCtx,
		chromedp.Navigate(addr),
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
		return nil, err
	}
	return pdfBuf, nil
}

// isChromeStartupFailure returns true for error strings that match
// transient Chromium launch failures we know to be safe to retry.
// Matches the "chrome failed to start" prefix that chromedp emits when
// the subprocess exits before the DevTools endpoint comes up, and the
// specific PartitionAlloc check-fail that has been seen on CI.
func isChromeStartupFailure(err error) bool {
	msg := err.Error()
	markers := []string{
		"chrome failed to start",
		"ThreadCache::IsValid",
		"scheduler_loop_quarantine_support",
		"websocket: bad handshake",
	}
	for _, m := range markers {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return false
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
