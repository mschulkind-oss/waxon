package render

import (
	"strings"
	"testing"

	"github.com/mschulkind-oss/waxon/internal/format"
)

func TestZoomControlsPresentInInteractivePage(t *testing.T) {
	deck, err := format.Parse("---\ntitle: t\n---\n\n# Hello\n\nworld\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	html, err := RenderHTML(deck, Options{}) // Print:false => interactive page (what `serve` sends)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	wants := []string{
		`data-action="zoom-out"`,
		`data-action="zoom-in"`,
		`data-action="zoom-reset"`,
		`id="zoom-level"`,
		`action === 'zoom-in'`,
		`action === 'zoom-out'`,
		`action === 'zoom-reset'`,
		`function zoomIn`,
		`getElementById('zoom-level')`,
		`localStorage.setItem('waxon-zoom'`,
		`var(--waxon-zoom`,
	}
	for _, w := range wants {
		if !strings.Contains(html, w) {
			t.Errorf("interactive page missing %q", w)
		}
	}
	// and the print/PDF page must NOT carry the toolbar
	phtml, err := RenderHTML(deck, Options{Print: true})
	if err != nil {
		t.Fatalf("render print: %v", err)
	}
	if strings.Contains(phtml, `data-action="zoom-in"`) {
		t.Errorf("print page should not contain the zoom toolbar")
	}
}

func TestToolbarToggleWiredInInteractivePage(t *testing.T) {
	deck, err := format.Parse("---\ntitle: t\n---\n\n# Hello\n\nworld\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	wants := []string{
		`.fab.fab-hidden`,    // the hidden-state CSS
		`function toggleFab`, // the toggle handler
		`case 'H':`,          // Shift+H keybinding
		`fullscreenchange`,   // auto-hide on fullscreen
		`localStorage.setItem('waxon-fab-hidden'`, // persisted preference
		`<kbd>Shift</kbd>+<kbd>H</kbd>`,           // documented in help overlay
	}
	for _, w := range wants {
		if !strings.Contains(html, w) {
			t.Errorf("interactive page missing toolbar-toggle wiring %q", w)
		}
	}
}

func TestPanelShadowOnlyWhenOpen(t *testing.T) {
	deck, err := format.Parse("---\ntitle: t\n---\n\n# Hello\n\nworld\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	html, err := RenderHTML(deck, Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// The leftward panel shadow must be gated on .panel.open, not the base
	// .panel rule, or it lingers as a phantom strip after the panel closes.
	if !strings.Contains(html, ".panel.open {") {
		t.Errorf("expected a .panel.open rule")
	}
	// crude but effective: the box-shadow line must sit inside the .open block.
	openIdx := strings.Index(html, ".panel.open {")
	shadowIdx := strings.Index(html, "box-shadow: -8px 0 24px")
	baseIdx := strings.Index(html, ".panel {")
	if shadowIdx < 0 || openIdx < 0 || baseIdx < 0 {
		t.Fatalf("missing expected rules (open=%d shadow=%d base=%d)", openIdx, shadowIdx, baseIdx)
	}
	if shadowIdx < openIdx {
		t.Errorf("panel box-shadow appears before .panel.open — it is on the base rule and will linger when closed")
	}
}
