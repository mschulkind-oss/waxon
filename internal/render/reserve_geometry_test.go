package render

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/mschulkind-oss/waxon/internal/format"
)

// TestReserveBandKeepsContentInBox is an integration test (real headless
// Chrome) for the presenter-reserve band's core invariant:
//
//	At EVERY band level, every content element must stay inside the slide's
//	content box (the dashed frame). Specifically nothing may poke out the TOP
//	or LEFT of the padding box — that content is unreachable in a deck
//	(can't scroll up) and looks broken (title clipped off-screen).
//
// This guards the regression where dialing the band to a high level (e.g.
// 70%) made an overflowing slide center its overflow and push the <h1> title
// above the content box. The fix (justify-content: safe center + media-height
// cap) is what this test locks in.
//
// Skipped in -short mode; requires Chrome (same as the PDF export tests).
func TestReserveBandKeepsContentInBox(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser geometry test in short mode")
	}

	// A deck with a reserve declared and a deliberately TALL slide (long
	// transcript-style content) so the band squeeze forces overflow — the
	// exact condition that used to clip the title.
	src := "---\ntitle: Reserve Geometry\npresenter-reserve: bottom-right\n---\n\n" +
		"# A title that must never clip\n\n" +
		"Line one of body copy that is reasonably long so the block has real height.\n\n" +
		"Line two of body copy, also reasonably long, adding more vertical extent here.\n\n" +
		"Line three to push the content tall enough that a big bottom band forces overflow.\n\n" +
		"Line four keeps going so that at high band levels the content exceeds the shrunk box.\n\n" +
		"Line five, the last, ensures we are comfortably taller than the squeezed region.\n"
	deck, err := format.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	html, err := RenderHTML(deck, Options{}) // interactive page carries the reserve CSS + classes
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	}))
	defer srv.Close()

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-features", "SchedulerLoopQuarantine,PartitionAllocBackupRefPtr,PartitionAllocPCScan,PartitionAllocMemoryTagging"),
		chromedp.Flag("no-zygote", true),
		chromedp.WindowSize(1280, 720),
	)
	defer allocCancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, tcancel := context.WithTimeout(ctx, 60*time.Second)
	defer tcancel()

	// Measure, at each band level, the worst (most-negative) overflow of any
	// content child past the TOP or LEFT edge of the slide's content box.
	// A value < 0 means something pokes outside the dashed frame.
	measure := `(function(band){
		var s = document.querySelector('#render-main');
		s.style.setProperty('--reserve-live', String(band));
		if (band > 0) s.classList.add('reserve-live'); else s.classList.remove('reserve-live');
		void s.offsetWidth;
		var cs = getComputedStyle(s);
		var sr = s.getBoundingClientRect();
		var padT = parseFloat(cs.paddingTop), padL = parseFloat(cs.paddingLeft);
		var boxTop = sr.top + padT, boxLeft = sr.left + padL;
		var worstTop = 1e9, worstLeft = 1e9, offender = '';
		s.querySelectorAll('h1,h2,h3,p,.waxon-quote,.waxon-image,.waxon-grid,.waxon-compare').forEach(function(el){
			var r = el.getBoundingClientRect();
			if (r.width === 0 && r.height === 0) return;
			var dTop = r.top - boxTop;    // >=0 means inside
			var dLeft = r.left - boxLeft; // >=0 means inside
			if (dTop < worstTop) { worstTop = dTop; }
			if (dLeft < worstLeft) { worstLeft = dLeft; }
			if (dTop < -1 || dLeft < -1) offender = (el.tagName + '.' + (el.className||'')).slice(0,40);
		});
		return JSON.stringify({top: Math.round(worstTop), left: Math.round(worstLeft), offender: offender});
	})`

	if err := chromedp.Run(ctx,
		chromedp.Navigate(srv.URL),
		chromedp.WaitVisible("#render-main", chromedp.ByID),
	); err != nil {
		t.Skipf("chrome unavailable, skipping geometry test: %v", err)
	}

	// A couple of px tolerance for sub-pixel rounding / borders.
	const tol = -2.0
	for _, band := range []float64{0, 0.3, 0.5, 0.7, 1.0} {
		var raw string
		expr := fmt.Sprintf("%s(%g)", measure, band)
		if err := chromedp.Run(ctx, chromedp.Evaluate(expr, &raw)); err != nil {
			t.Fatalf("evaluate at band %g: %v", band, err)
		}
		var m struct {
			Top, Left float64
			Offender  string
		}
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatalf("decode at band %g: %v (raw=%s)", band, err, raw)
		}
		if m.Top < tol {
			t.Errorf("band %.0f%%: content clips the TOP of the content box by %.0fpx (offender %q) — title/content pushed off-screen",
				band*100, -m.Top, m.Offender)
		}
		if m.Left < tol {
			t.Errorf("band %.0f%%: content clips the LEFT of the content box by %.0fpx (offender %q)",
				band*100, -m.Left, m.Offender)
		}
	}
}
