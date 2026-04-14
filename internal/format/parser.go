package format

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// noteRe matches <!-- note: ... -->
	noteRe = regexp.MustCompile(`<!--\s*note:\s*(.*?)\s*-->`)
	// aiRe matches <!-- ai: ... -->
	aiRe = regexp.MustCompile(`<!--\s*ai:\s*(.*?)\s*-->`)
	// commentRe matches <!-- comment(@author): text -->
	// Author allows letters, digits, underscore, and hyphen so names like
	// "mary-jane" round-trip cleanly through the writer.
	commentRe = regexp.MustCompile(`<!--\s*comment\(@([\w-]+)\):\s*(.*?)\s*-->`)
	// pauseRe matches <!-- pause -->
	pauseRe = regexp.MustCompile(`<!--\s*pause\s*-->`)
	// slideOptsRe matches <!-- slide: key=val, key=val -->
	slideOptsRe = regexp.MustCompile(`<!--\s*slide:\s*(.*?)\s*-->`)
	// variantRe matches ---variant: name
	variantRe = regexp.MustCompile(`^---variant:\s*(\S+)\s*$`)
	// slideSepRe matches a slide separator, optionally with an ID:
	//   ---
	//   --- #some-id
	// The ID allows URL fragments and agent-addressable anchors without
	// relying on slide index numbers that shift when content is inserted.
	slideSepRe = regexp.MustCompile(`^---(?:\s+#([\w-]+))?\s*$`)
)

// colorPalette is the closed set of color utility classes recognized in
// .color{text} and .color text syntax. Unknown names are left as literal
// text so a stray ".error{foo}" in prose doesn't silently vanish.
var colorPalette = map[string]struct{}{
	"red":    {},
	"green":  {},
	"yellow": {},
	"blue":   {},
	"aqua":   {},
	"dim":    {},
}

// badgePalette is the closed set of badge background color classes for
// .badge-color{text} inline pills. Same set as colorPalette minus `dim`
// (dim as a background isn't a useful visual pop).
var badgePalette = map[string]struct{}{
	"red":    {},
	"green":  {},
	"yellow": {},
	"blue":   {},
	"aqua":   {},
}

// Parse parses a .slides file into a Deck.
func Parse(input string) (*Deck, error) {
	meta, body, err := splitFrontmatter(input)
	if err != nil {
		return nil, err
	}

	deck := &Deck{Meta: meta}
	deck.Slides = parseSlides(body)
	return deck, nil
}

// splitFrontmatter extracts YAML frontmatter and the remaining body.
func splitFrontmatter(input string) (Meta, string, error) {
	var meta Meta

	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "---") {
		return meta, input, nil
	}

	// Find the closing ---
	rest := trimmed[3:]
	yamlContent, body, found := strings.Cut(rest, "\n---")
	if !found {
		return meta, input, nil
	}

	if err := yaml.Unmarshal([]byte(yamlContent), &meta); err != nil {
		return meta, "", err
	}

	// Set defaults
	if meta.Aspect == "" {
		meta.Aspect = "16:9"
	}
	if meta.Theme == "" {
		meta.Theme = "default"
	}

	return meta, body, nil
}

// parseSlides splits the body into slides on --- delimiters and extracts metadata.
func parseSlides(body string) []Slide {
	// Split on lines that are exactly "---" or "--- #slide-id" (but not
	// ---variant:). A trailing `#id` on a separator is an ID for the
	// slide that FOLLOWS — IDs identify targets, and the first slide can
	// be named by placing a separator line above it.
	lines := strings.Split(body, "\n")
	var slides []Slide
	var current []string
	index := 0
	pendingID := ""

	flush := func() {
		if len(current) > 0 || index == 0 {
			raw := strings.Join(current, "\n")
			raw = strings.TrimSpace(raw)
			if raw != "" || index == 0 {
				slide := buildSlide(index, raw)
				slide.ID = pendingID
				pendingID = ""
				if slide.Content != "" || len(slide.Variants) > 0 {
					slides = append(slides, slide)
					index++
				}
			}
			current = nil
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := slideSepRe.FindStringSubmatch(trimmed); m != nil {
			flush()
			if m[1] != "" {
				pendingID = m[1]
			}
		} else {
			current = append(current, line)
		}
	}
	flush()

	return slides
}

// buildSlide creates a Slide from raw content, extracting directives and variants.
func buildSlide(index int, raw string) Slide {
	// First, split off any variants
	mainContent, variants := splitVariants(raw)

	slide := Slide{
		Index: index,
	}

	// Extract directives from main content, then apply the fence and color
	// passes. Order matters: directives are stripped first (so pause
	// sentinels sit in the DOM child stream at block level), then fence
	// blocks (which emit block-level HTML containers around markdown), then
	// color transforms (which emit inline HTML spans inside the remaining
	// prose, including spans nested inside fence body markdown).
	slide.Content, slide.Notes, slide.AINotes, slide.Comments, slide.Pauses, slide.SlideOpts = extractDirectives(mainContent)
	slide.Content = applyFenceBlocks(slide.Content)
	slide.Content = applyMidSlideHR(slide.Content)
	slide.Content = applyColorTransforms(slide.Content)

	// Extract directives from each variant body so reviewers can comment
	// on a specific alternative.
	for _, v := range variants {
		content, notes, aiNotes, comments, pauses, _ := extractDirectives(v.Content)
		content = applyFenceBlocks(content)
		content = applyMidSlideHR(content)
		content = applyColorTransforms(content)
		slide.Variants = append(slide.Variants, Variant{
			Name:     v.Name,
			Content:  content,
			Notes:    notes,
			AINotes:  aiNotes,
			Comments: comments,
			Pauses:   pauses,
		})
	}

	return slide
}

// applyFenceBlocks runs all ::: fence transforms in a single left-to-right
// pass so we avoid repeatedly splitting and rejoining the line slice for
// each fence type. The opening marker on a line decides which transform
// handles the block.
func applyFenceBlocks(content string) string {
	return strings.Join(applyFenceBlocksToLines(strings.Split(content, "\n")), "\n")
}

// applyFenceBlocksToLines is the line-level fence transform used by both
// the top-level slide content pass and by fence handlers that need to
// process nested fences inside their body (e.g. :::stat inside :::grid).
func applyFenceBlocksToLines(lines []string) []string {
	var out []string
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, ":::") {
			out = append(out, lines[i])
			i++
			continue
		}
		name, args := splitFenceOpener(trimmed)
		handler, ok := fenceHandlers[name]
		if !ok {
			// Not a recognized fence — leave the line as-is; it might be
			// a code fence (```...) or literal text.
			out = append(out, lines[i])
			i++
			continue
		}
		block, consumed, err := handler(args, lines[i+1:])
		if err != nil {
			out = append(out, fmt.Sprintf(
				`<div class="waxon-error">waxon-%s error: %s</div>`,
				name, err.Error(),
			))
			i += 1 + consumed
			continue
		}
		out = append(out, block...)
		i += 1 + consumed
	}
	return out
}

// splitFenceOpener parses ":::name arg1 arg2" into its name and argument
// string. Returns ("", "") if the line isn't a recognized fence opener.
func splitFenceOpener(line string) (name, args string) {
	if !strings.HasPrefix(line, ":::") {
		return "", ""
	}
	rest := strings.TrimSpace(line[3:])
	if rest == "" {
		return "", ""
	}
	parts := strings.SplitN(rest, " ", 2)
	name = parts[0]
	if len(parts) == 2 {
		args = strings.TrimSpace(parts[1])
	}
	return name, args
}

// fenceHandler is the signature every ::: fence block implements. Each
// handler receives the arg string from the opener line and the lines after
// the opener; it returns the emitted HTML lines and the number of source
// lines it consumed (including the closing :::).
type fenceHandler func(args string, lines []string) (block []string, consumed int, err error)

// fenceHandlers is populated in init() because wrapBlock → applyFenceBlocksToLines
// needs to read this map, and the handlers stored in it call wrapBlock —
// Go's package-level init analysis rejects a cycle through a var literal,
// so we break it by deferring the assignment.
var fenceHandlers map[string]fenceHandler

func init() {
	fenceHandlers = map[string]fenceHandler{
		"compare":   handleCompareFence,
		"card":      handleCardFence,
		"card-left": handleCardLeftFence,
		"grid":      handleGridFence,
		"flow":      handleFlowFence,
		"timeline":  handleTimelineFence,
		"quote":     handleQuoteFence,
		"stat":      handleStatFence,
	}
}

// splitVariants separates main slide content from ---variant: sections.
func splitVariants(raw string) (string, []Variant) {
	lines := strings.Split(raw, "\n")
	var mainLines []string
	var variants []Variant
	var currentVariantName string
	var currentVariantLines []string
	inVariant := false

	for _, line := range lines {
		if m := variantRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			// Flush current variant if any
			if inVariant {
				variants = append(variants, Variant{
					Name:    currentVariantName,
					Content: strings.TrimSpace(strings.Join(currentVariantLines, "\n")),
				})
			}
			currentVariantName = m[1]
			currentVariantLines = nil
			inVariant = true
		} else if inVariant {
			currentVariantLines = append(currentVariantLines, line)
		} else {
			mainLines = append(mainLines, line)
		}
	}

	// Flush last variant
	if inVariant {
		variants = append(variants, Variant{
			Name:    currentVariantName,
			Content: strings.TrimSpace(strings.Join(currentVariantLines, "\n")),
		})
	}

	return strings.TrimSpace(strings.Join(mainLines, "\n")), variants
}

// extractDirectives pulls notes, ai notes, comments, pauses, and slide opts from content.
func extractDirectives(content string) (cleaned string, notes, aiNotes []string, comments []Comment, pauses int, opts *SlideOpts) {
	// Extract notes
	for _, m := range noteRe.FindAllStringSubmatch(content, -1) {
		notes = append(notes, m[1])
	}

	// Extract AI notes
	for _, m := range aiRe.FindAllStringSubmatch(content, -1) {
		aiNotes = append(aiNotes, m[1])
	}

	// Extract comments
	for _, m := range commentRe.FindAllStringSubmatch(content, -1) {
		comments = append(comments, Comment{Author: m[1], Text: m[2]})
	}

	// Count pauses
	pauses = len(pauseRe.FindAllString(content, -1))

	// Extract slide opts
	if m := slideOptsRe.FindStringSubmatch(content); m != nil {
		opts = parseSlideOpts(m[1])
	}

	// Remove all directives from content. Pauses are replaced with a
	// sentinel <div> so the renderer can split the slide body at the
	// original marker positions for progressive reveal — without this,
	// we would only know *how many* pauses there were, not *where*.
	cleaned = noteRe.ReplaceAllString(content, "")
	cleaned = aiRe.ReplaceAllString(cleaned, "")
	cleaned = commentRe.ReplaceAllString(cleaned, "")
	cleaned = pauseRe.ReplaceAllString(cleaned, "\n\n<div class=\"waxon-pause\"></div>\n\n")
	cleaned = slideOptsRe.ReplaceAllString(cleaned, "")

	// Clean up extra blank lines
	cleaned = collapseBlankLines(cleaned)
	cleaned = strings.TrimSpace(cleaned)

	return
}

// parseSlideOpts parses "bg=#fff, class=centered" into SlideOpts.
// It also accepts bare tokens (no `key=` prefix) as class shorthands —
// e.g. `<!-- slide: no-chrome -->` is equivalent to `class=no-chrome`.
// Multiple bare tokens are concatenated space-separated.
func parseSlideOpts(raw string) *SlideOpts {
	opts := &SlideOpts{}
	var extraClasses []string
	for pair := range strings.SplitSeq(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			extraClasses = append(extraClasses, pair)
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "bg":
			opts.Background = val
		case "class":
			opts.Class = val
		}
	}
	if len(extraClasses) > 0 {
		if opts.Class == "" {
			opts.Class = strings.Join(extraClasses, " ")
		} else {
			opts.Class = opts.Class + " " + strings.Join(extraClasses, " ")
		}
	}
	if opts.Background == "" && opts.Class == "" {
		return nil
	}
	return opts
}

// collapseBlankLines reduces runs of 3+ blank lines to 2.
func collapseBlankLines(s string) string {
	re := regexp.MustCompile(`\n{3,}`)
	return re.ReplaceAllString(s, "\n\n")
}

// ---------- Fence-block helpers ----------
//
// Every fence below emits HTML that relies on CommonMark type-6 HTML block
// behavior: a blank line after an opening <div> ends the HTML block, so
// inner markdown is parsed as usual, and the closing </div> starts a new
// HTML block. That's why the emitted slices have careful blank lines
// around inner bodies.

// fenceSection holds a sub-block inside a fence, delimited by a `::name`
// marker line. `args` is whatever followed the marker name on the same
// line (used for per-section palette hints like `::left red`). `body`
// holds the raw markdown lines between this marker and the next.
type fenceSection struct {
	name string
	args string
	body []string
}

// readFenceSections scans `lines` starting at the line AFTER the fence
// opener and until the matching `:::` close, collecting `::section`
// sub-blocks. It returns the list of sections, the number of source lines
// consumed (including the closing :::), and an error on nested fence,
// unterminated block, or an unrecognized opening `:::` inside.
//
// Content that appears BEFORE the first `::section` marker goes into an
// anonymous section with name "" — that lets fences like :::card (which
// has no sub-sections) and :::quote (where the attribution is an
// optional tail section) both use the same primitive.
func readFenceSections(fenceName string, lines []string) ([]fenceSection, int, error) {
	var sections []fenceSection
	current := fenceSection{name: ""}
	consumed := 0
	closed := false
	// depth tracks nested ::: fences inside this one so `:::stat` inside
	// `:::grid` / `::col` doesn't terminate the outer grid on its closing
	// `:::`. Inner section markers (`::col`) only apply at depth 0.
	depth := 0
	for i, line := range lines {
		consumed = i + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == ":::" {
			if depth == 0 {
				closed = true
				break
			}
			depth--
			current.body = append(current.body, line)
			continue
		}
		if strings.HasPrefix(trimmed, ":::") {
			// Opening of a nested fence — only count it as nesting if
			// it is a recognized fence name, so an unrelated `:::foo`
			// that would otherwise pass through as literal text doesn't
			// throw our depth tracking off.
			name, _ := splitFenceOpener(trimmed)
			if _, ok := fenceHandlers[name]; ok {
				depth++
				current.body = append(current.body, line)
				continue
			}
			current.body = append(current.body, line)
			continue
		}
		if depth == 0 && strings.HasPrefix(trimmed, "::") {
			// Flush current section if it has anything (body or non-empty name).
			if current.name != "" || len(current.body) > 0 {
				sections = append(sections, current)
			}
			rest := strings.TrimSpace(trimmed[2:])
			parts := strings.SplitN(rest, " ", 2)
			current = fenceSection{name: parts[0]}
			if len(parts) == 2 {
				current.args = strings.TrimSpace(parts[1])
			}
			continue
		}
		current.body = append(current.body, line)
	}
	if !closed {
		return nil, consumed, fmt.Errorf("unterminated :::%s (missing closing :::)", fenceName)
	}
	if current.name != "" || len(current.body) > 0 {
		sections = append(sections, current)
	}
	return sections, consumed, nil
}

// trimBlankEdges strips leading and trailing blank lines from a body slice
// while preserving blank lines in the middle. Used to keep the emitted
// HTML tight around fence wrappers.
func trimBlankEdges(body []string) []string {
	start, end := 0, len(body)
	for start < end && strings.TrimSpace(body[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(body[end-1]) == "" {
		end--
	}
	return body[start:end]
}

// paletteClass returns the class name to add for a fence's color hint, or
// "" if the hint is empty or not in the closed palette.
func paletteClass(hint string) string {
	if hint == "" {
		return ""
	}
	if _, ok := colorPalette[hint]; ok {
		return hint
	}
	return ""
}

// wrapBlock emits a `<div class="cls">` ... `</div>` wrapper around the
// given body lines, with blank lines inserted at the edges so goldmark
// parses the interior as markdown rather than as raw HTML. The body is
// recursively passed through the fence transform so nested fences
// (e.g. :::stat inside a :::grid ::col) get processed.
func wrapBlock(cls string, body []string) []string {
	body = applyFenceBlocksToLines(body)
	out := []string{fmt.Sprintf(`<div class="%s">`, cls), ""}
	out = append(out, trimBlankEdges(body)...)
	out = append(out, "", `</div>`)
	return out
}

// ---------- :::compare ----------

func handleCompareFence(_ string, lines []string) ([]string, int, error) {
	sections, consumed, err := readFenceSections("compare", lines)
	if err != nil {
		return nil, consumed, err
	}
	var left, right *fenceSection
	for i := range sections {
		switch sections[i].name {
		case "left":
			left = &sections[i]
		case "right":
			right = &sections[i]
		}
	}
	if left == nil || right == nil {
		return nil, consumed, fmt.Errorf("compare requires both ::left and ::right panes")
	}
	out := []string{`<div class="waxon-compare">`}
	out = append(out, comparePaneHTML("left", left.args, left.body)...)
	out = append(out, comparePaneHTML("right", right.args, right.body)...)
	out = append(out, `</div>`)
	return out, consumed, nil
}

func comparePaneHTML(side, colorHint string, body []string) []string {
	classes := "waxon-compare-pane waxon-compare-" + side
	if c := paletteClass(colorHint); c != "" {
		classes += " " + c
	}
	return wrapBlock(classes, body)
}

// ---------- :::card ----------

func handleCardFence(args string, lines []string) ([]string, int, error) {
	sections, consumed, err := readFenceSections("card", lines)
	if err != nil {
		return nil, consumed, err
	}
	classes := "waxon-card"
	if c := paletteClass(args); c != "" {
		classes += " " + c
	}
	var body []string
	for _, s := range sections {
		body = append(body, s.body...)
	}
	return wrapBlock(classes, body), consumed, nil
}

func handleCardLeftFence(args string, lines []string) ([]string, int, error) {
	sections, consumed, err := readFenceSections("card-left", lines)
	if err != nil {
		return nil, consumed, err
	}
	classes := "waxon-card waxon-card-left"
	if c := paletteClass(args); c != "" {
		classes += " " + c
	}
	var body []string
	for _, s := range sections {
		body = append(body, s.body...)
	}
	return wrapBlock(classes, body), consumed, nil
}

// ---------- :::grid ----------

// handleGridFence emits a CSS grid with either N linear columns or an NxM
// matrix. `args` is "3" (three columns) or "2x2" (two columns, two rows)
// or the literal string passed by the author — we just stash it into a
// data attribute and let CSS read it, which keeps the parser lean and
// lets themes experiment with layouts.
func handleGridFence(args string, lines []string) ([]string, int, error) {
	sections, consumed, err := readFenceSections("grid", lines)
	if err != nil {
		return nil, consumed, err
	}
	cols, rows, layoutErr := parseGridLayout(args)
	if layoutErr != nil {
		return nil, consumed, layoutErr
	}

	// Children are ::col or ::cell — treat them identically; grid just
	// needs an ordered list of panes.
	var out []string
	style := fmt.Sprintf("grid-template-columns: repeat(%d, 1fr);", cols)
	if rows > 0 {
		style += fmt.Sprintf(" grid-template-rows: repeat(%d, 1fr);", rows)
	}
	out = append(out, fmt.Sprintf(
		`<div class="waxon-grid" style="%s">`, style,
	))
	for _, s := range sections {
		if s.name != "col" && s.name != "cell" {
			continue
		}
		classes := "waxon-grid-cell"
		if c := paletteClass(s.args); c != "" {
			classes += " " + c
		}
		cell := wrapBlock(classes, s.body)
		out = append(out, cell...)
	}
	out = append(out, `</div>`)
	return out, consumed, nil
}

// parseGridLayout reads the grid arg string and returns (cols, rows). If
// rows is 0, the grid only pins columns and lets content flow naturally.
func parseGridLayout(args string) (int, int, error) {
	if args == "" {
		return 0, 0, fmt.Errorf("grid needs a size like `3` or `2x2`")
	}
	if strings.Contains(args, "x") {
		var c, r int
		if _, err := fmt.Sscanf(args, "%dx%d", &c, &r); err != nil {
			return 0, 0, fmt.Errorf("grid size %q not in CxR form", args)
		}
		if c <= 0 || r <= 0 {
			return 0, 0, fmt.Errorf("grid size %q must be positive", args)
		}
		return c, r, nil
	}
	var c int
	if _, err := fmt.Sscanf(args, "%d", &c); err != nil || c <= 0 {
		return 0, 0, fmt.Errorf("grid size %q not a positive integer", args)
	}
	return c, 0, nil
}

// ---------- :::flow ----------

// handleFlowFence emits a linear row/column of boxes connected by arrows.
// The body is one or more lines of `[A] --> [B] --> [C]` (or `-.->` for
// dashed). Branching and labeled arrows are out of scope for the first
// cut — authors with those needs fall back to raw HTML, as they did
// before this fence existed.
func handleFlowFence(args string, lines []string) ([]string, int, error) {
	sections, consumed, err := readFenceSections("flow", lines)
	if err != nil {
		return nil, consumed, err
	}
	orientation := "horizontal"
	if args == "vertical" {
		orientation = "vertical"
	}
	// Collect all body lines from all sections (flow has no ::sub-markers).
	var bodyLines []string
	for _, s := range sections {
		bodyLines = append(bodyLines, s.body...)
	}

	nodes, arrows, flowErr := parseFlowBody(bodyLines)
	if flowErr != nil {
		return nil, consumed, flowErr
	}
	if len(nodes) == 0 {
		return nil, consumed, fmt.Errorf("flow has no [box] nodes")
	}
	classes := "waxon-flow waxon-flow-" + orientation
	out := []string{fmt.Sprintf(`<div class="%s">`, classes)}
	for i, node := range nodes {
		nodeClasses := "waxon-flow-node"
		if c := paletteClass(node.color); c != "" {
			nodeClasses += " " + c
		}
		out = append(out, fmt.Sprintf(
			`<div class="%s">%s</div>`, nodeClasses, htmlEscape(node.text),
		))
		if i < len(arrows) {
			switch arrows[i] {
			case "divider":
				out = append(out, `<div class="waxon-flow-divider">/</div>`)
			default:
				arrowClass := "waxon-flow-arrow"
				if arrows[i] == "dashed" {
					arrowClass += " waxon-flow-arrow-dashed"
				}
				glyph := flowArrowGlyph(orientation)
				out = append(out, fmt.Sprintf(
					`<div class="%s">%s</div>`, arrowClass, glyph,
				))
			}
		}
	}
	out = append(out, `</div>`)
	return out, consumed, nil
}

type flowNode struct {
	text  string
	color string
}

// parseFlowBody scans one or more lines of `[box] --> [box] -.-> [box]`
// and returns the parsed nodes + the arrow styles between them. Optional
// `.color[text]` prefix sets the box color from the palette.
func parseFlowBody(lines []string) ([]flowNode, []string, error) {
	var nodes []flowNode
	var arrows []string
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		i := 0
		for i < len(text) {
			if text[i] == ' ' || text[i] == '\t' {
				i++
				continue
			}
			// Optional palette prefix: .color[...]
			color := ""
			if text[i] == '.' {
				j := i + 1
				for j < len(text) && isClassChar(text[j]) {
					j++
				}
				if j > i+1 && j < len(text) && text[j] == '[' {
					candidate := text[i+1 : j]
					if _, ok := colorPalette[candidate]; ok {
						color = candidate
						i = j
					}
				}
			}
			if i >= len(text) || text[i] != '[' {
				// Arrow segment?
				end, arrowKind, ok := parseFlowArrow(text, i)
				if ok {
					arrows = append(arrows, arrowKind)
					i = end
					continue
				}
				// A lone `/` between boxes marks parallel/alternate paths
				// (e.g. `[A] --> [B] / [C]`). Renders as a plain divider.
				if text[i] == '/' {
					arrows = append(arrows, "divider")
					i++
					continue
				}
				return nil, nil, fmt.Errorf("flow: unexpected %q at column %d", string(text[i]), i)
			}
			// Read box text until matching `]`.
			j := i + 1
			for j < len(text) && text[j] != ']' {
				j++
			}
			if j >= len(text) {
				return nil, nil, fmt.Errorf("flow: unterminated [box]")
			}
			nodes = append(nodes, flowNode{
				text:  strings.TrimSpace(text[i+1 : j]),
				color: color,
			})
			i = j + 1
		}
	}
	return nodes, arrows, nil
}

// parseFlowArrow matches `-->` or `-.->` starting at position i and
// returns the end index, the arrow kind ("solid" or "dashed"), and
// whether a match was found.
func parseFlowArrow(text string, i int) (int, string, bool) {
	if strings.HasPrefix(text[i:], "-.->") {
		return i + 4, "dashed", true
	}
	if strings.HasPrefix(text[i:], "-->") {
		return i + 3, "solid", true
	}
	return i, "", false
}

func flowArrowGlyph(orientation string) string {
	if orientation == "vertical" {
		return "&darr;"
	}
	return "&rarr;"
}

// ---------- :::timeline ----------

func handleTimelineFence(args string, lines []string) ([]string, int, error) {
	sections, consumed, err := readFenceSections("timeline", lines)
	if err != nil {
		return nil, consumed, err
	}
	orientation := "horizontal"
	if args == "vertical" {
		orientation = "vertical"
	}
	out := []string{fmt.Sprintf(
		`<div class="waxon-timeline waxon-timeline-%s">`, orientation,
	)}
	for _, s := range sections {
		// Timeline entries use :: with the label as the argument (e.g.
		// `:: 2020` or `:: .green 2025`). readFenceSections parsed the
		// first token as name; join it back with args for the label.
		if s.name == "" && len(s.body) == 0 {
			continue
		}
		label := s.name
		if s.args != "" {
			label += " " + s.args
		}
		label = strings.TrimSpace(label)
		labelColor := ""
		if strings.HasPrefix(label, ".") {
			// `.green 2025` → color=green, label=2025
			end := 1
			for end < len(label) && isClassChar(label[end]) {
				end++
			}
			if end > 1 && end < len(label) && label[end] == ' ' {
				candidate := label[1:end]
				if _, ok := colorPalette[candidate]; ok {
					labelColor = candidate
					label = strings.TrimSpace(label[end+1:])
				}
			}
		}
		labelClasses := "waxon-timeline-label"
		if labelColor != "" {
			labelClasses += " " + labelColor
		}
		out = append(out, `<div class="waxon-timeline-entry">`)
		out = append(out, `<div class="waxon-timeline-marker"></div>`)
		out = append(out, fmt.Sprintf(
			`<div class="%s">%s</div>`, labelClasses, htmlEscape(label),
		))
		out = append(out, `<div class="waxon-timeline-body">`, "")
		out = append(out, trimBlankEdges(s.body)...)
		out = append(out, "", `</div>`, `</div>`)
	}
	out = append(out, `</div>`)
	return out, consumed, nil
}

// ---------- :::quote ----------

func handleQuoteFence(args string, lines []string) ([]string, int, error) {
	sections, consumed, err := readFenceSections("quote", lines)
	if err != nil {
		return nil, consumed, err
	}
	classes := "waxon-quote"
	if c := paletteClass(args); c != "" {
		classes += " " + c
	}
	var body []string
	var attribution []string
	for _, s := range sections {
		switch s.name {
		case "":
			body = append(body, s.body...)
		case "by":
			// `::by CTO, SSA` — the args hold the attribution line.
			line := s.args
			if line == "" && len(s.body) > 0 {
				line = strings.TrimSpace(strings.Join(s.body, " "))
			}
			if line != "" {
				attribution = append(attribution, line)
			}
		}
	}
	out := []string{fmt.Sprintf(`<div class="%s">`, classes), ""}
	out = append(out, trimBlankEdges(body)...)
	if len(attribution) > 0 {
		out = append(out, "", fmt.Sprintf(
			`<div class="waxon-quote-by">— %s</div>`,
			htmlEscape(strings.Join(attribution, " ")),
		))
	}
	out = append(out, "", `</div>`)
	return out, consumed, nil
}

// ---------- :::stat ----------

func handleStatFence(args string, lines []string) ([]string, int, error) {
	sections, consumed, err := readFenceSections("stat", lines)
	if err != nil {
		return nil, consumed, err
	}
	classes := "waxon-stat"
	if c := paletteClass(args); c != "" {
		classes += " " + c
	}
	var number, label, context []string
	for _, s := range sections {
		switch s.name {
		case "":
			number = append(number, s.body...)
		case "label":
			if s.args != "" {
				label = append(label, s.args)
			}
			label = append(label, s.body...)
		case "context":
			if s.args != "" {
				context = append(context, s.args)
			}
			context = append(context, s.body...)
		}
	}
	numberText := strings.TrimSpace(strings.Join(number, " "))
	labelText := strings.TrimSpace(strings.Join(label, " "))
	contextText := strings.TrimSpace(strings.Join(context, " "))
	if numberText == "" {
		return nil, consumed, fmt.Errorf("stat needs a number before ::label")
	}
	out := []string{fmt.Sprintf(`<div class="%s">`, classes)}
	out = append(out, fmt.Sprintf(
		`<div class="waxon-stat-number">%s</div>`, htmlEscape(numberText),
	))
	if labelText != "" {
		out = append(out, fmt.Sprintf(
			`<div class="waxon-stat-label">%s</div>`, htmlEscape(labelText),
		))
	}
	if contextText != "" {
		out = append(out, fmt.Sprintf(
			`<div class="waxon-stat-context">%s</div>`, htmlEscape(contextText),
		))
	}
	out = append(out, `</div>`)
	return out, consumed, nil
}

// ---------- Mid-slide horizontal rule ----------

// applyMidSlideHR replaces standalone `----` (4+ dashes) lines with an
// explicit HTML <hr> block so goldmark doesn't interpret them as setext
// heading underlines on the previous line. The slide separator is still
// exactly `---`; slideSepRe enforces end-of-line, so `----` never reaches
// this transform as a separator.
func applyMidSlideHR(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !isMidSlideHRLine(trimmed) {
			continue
		}
		// To avoid turning a setext underline into an HR, require the
		// previous non-fence line to be blank. The `---` (exactly three)
		// case is already captured by the slide separator; we only handle
		// 4+ here.
		if i > 0 && strings.TrimSpace(lines[i-1]) != "" {
			continue
		}
		lines[i] = `<hr class="waxon-hr" />`
	}
	return strings.Join(lines, "\n")
}

// isMidSlideHRLine reports whether a line is a mid-slide horizontal rule
// marker: 4+ consecutive `-`, `*`, or `_` characters with nothing else.
func isMidSlideHRLine(s string) bool {
	if len(s) < 4 {
		return false
	}
	c := s[0]
	if c != '-' && c != '*' && c != '_' {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] != c {
			return false
		}
	}
	return true
}

// htmlEscape escapes `&<>"` in text we emit directly inside an HTML tag.
// The fence handlers that escape are ones where the source line is a
// label or plain text — markdown bodies go through goldmark and don't
// need this.
func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	)
	return r.Replace(s)
}

// ---------- Color transforms ----------

// applyColorTransforms walks the content line by line, tracking fenced
// code block state, and rewrites:
//
//	.color text at start of line  →  <span class="color">text</span>
//	.color{text} anywhere in line →  <span class="color">text</span>
//
// for the closed palette {red, green, yellow, blue, aqua, dim}. Unknown
// class names are left untouched so a stray ".error{foo}" in prose is
// preserved literally. Inline backtick code spans and fenced code blocks
// are skipped — color syntax inside a code example must survive verbatim.
func applyColorTransforms(content string) string {
	lines := strings.Split(content, "\n")
	inFence := false
	var fenceMarker string
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		// Detect fence open/close: ``` or ~~~ (three or more).
		if marker := fenceMarkerOf(trimmed); marker != "" {
			if !inFence {
				inFence = true
				fenceMarker = marker
			} else if strings.HasPrefix(trimmed, fenceMarker) {
				inFence = false
				fenceMarker = ""
			}
			continue
		}
		if inFence {
			continue
		}
		lines[i] = transformLineColors(line)
	}
	return strings.Join(lines, "\n")
}

// fenceMarkerOf returns "```" or "~~~" if line opens/closes a fenced block,
// otherwise "". It requires three or more of the same character, matching
// CommonMark behavior.
func fenceMarkerOf(line string) string {
	for _, marker := range []string{"```", "~~~"} {
		if strings.HasPrefix(line, marker) {
			// Require any trailing chars to be info-string, not more ticks
			// interleaved — but any pure run of 3+ is fine.
			return marker
		}
	}
	return ""
}

// transformLineColors applies both line-level and inline color transforms
// to a single line, preserving backtick-delimited code spans. It first
// splits the line into alternating (unprotected, backtick) segments,
// then transforms only unprotected segments, then reassembles.
func transformLineColors(line string) string {
	segments := splitBackticks(line)
	for i, seg := range segments {
		if seg.code {
			continue
		}
		text := seg.text
		// Line-level color: only applies to the first unprotected segment
		// AND only when the segment sits at the start of the line (column
		// 0). A later segment, or one preceded by whitespace that isn't
		// at the real start, must not trigger.
		if i == 0 {
			if rewritten, ok := tryLineLevelColor(text); ok {
				text = rewritten
			}
		}
		text = applyInlineColor(text)
		segments[i].text = text
	}
	var out strings.Builder
	for _, seg := range segments {
		if seg.code {
			out.WriteByte('`')
			out.WriteString(seg.text)
			out.WriteByte('`')
		} else {
			out.WriteString(seg.text)
		}
	}
	return out.String()
}

type segment struct {
	text string
	code bool // true if text is the *interior* of a backtick span (without delimiters)
}

// splitBackticks chops a line into alternating prose/code segments. It
// handles single-backtick inline code only — that's the dominant form in
// prose, and multi-backtick escapes are rare enough that we degrade to
// literal passthrough for them.
func splitBackticks(line string) []segment {
	var out []segment
	var buf strings.Builder
	inCode := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '`' {
			out = append(out, segment{text: buf.String(), code: inCode})
			buf.Reset()
			inCode = !inCode
			continue
		}
		buf.WriteByte(c)
	}
	// Tail — if we ended inside an unclosed code span, treat the whole
	// unclosed remainder as prose (we emitted the open backtick as a
	// split point, so recover by putting the text back as prose).
	out = append(out, segment{text: buf.String(), code: inCode})
	return out
}

// tryLineLevelColor handles `.color text` at start of line. Returns the
// wrapped version and true on success, or the original and false if the
// line doesn't match.
func tryLineLevelColor(text string) (string, bool) {
	if len(text) < 2 || text[0] != '.' {
		return text, false
	}
	// Find end of class name (letters only — palette is ascii).
	end := 1
	for end < len(text) && isClassChar(text[end]) {
		end++
	}
	if end == 1 {
		return text, false
	}
	class := text[1:end]
	if _, ok := colorPalette[class]; !ok {
		return text, false
	}
	// Must be followed by a space and then non-empty content. `.red` alone
	// or `.red{…}` falls through to the inline pass.
	if end >= len(text) || text[end] != ' ' {
		return text, false
	}
	rest := text[end+1:]
	if rest == "" {
		return text, false
	}
	return fmt.Sprintf(`<span class="%s">%s</span>`, class, rest), true
}

// applyInlineColor rewrites all `.color{text}` and `.badge-color{text}`
// occurrences in a line. Uses a balanced-brace walker so nested braces
// `.red{a {b} c}` are handled. On unbalanced or unknown class, leaves
// text as-is.
func applyInlineColor(text string) string {
	var out strings.Builder
	i := 0
	for i < len(text) {
		// Look for `.<class>{` or `.badge-<class>{`.
		if text[i] != '.' {
			out.WriteByte(text[i])
			i++
			continue
		}
		// Scan class name. Badge classes include a hyphen so we allow
		// `-` in the class scanner here — palette class matching below
		// still only accepts pure-ASCII names, so stray names like
		// `.foo-bar{...}` fall through unchanged.
		j := i + 1
		for j < len(text) && (isClassChar(text[j]) || text[j] == '-') {
			j++
		}
		if j == i+1 || j >= len(text) || text[j] != '{' {
			out.WriteByte(text[i])
			i++
			continue
		}
		class := text[i+1 : j]
		htmlClass, ok := resolveInlineClass(class)
		if !ok {
			out.WriteByte(text[i])
			i++
			continue
		}
		// Find matching close brace, counting nesting.
		depth := 1
		k := j + 1
		for k < len(text) && depth > 0 {
			switch text[k] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					goto matched
				}
			}
			k++
		}
		// Unbalanced — emit literally.
		out.WriteByte(text[i])
		i++
		continue
	matched:
		inner := text[j+1 : k]
		// Recurse: allow nested `.color{.other{x}}` to transform.
		inner = applyInlineColor(inner)
		fmt.Fprintf(&out, `<span class="%s">%s</span>`, htmlClass, inner)
		i = k + 1
	}
	return out.String()
}

// resolveInlineClass maps a source class name to the CSS class(es) emitted
// in the output span, or returns ok=false if the name isn't recognized.
// Plain palette names produce their own class; `badge-<color>` produces
// `waxon-badge <color>` so the badge pill CSS can style both the shape
// and the color in one rule set.
func resolveInlineClass(class string) (string, bool) {
	if _, ok := colorPalette[class]; ok {
		return class, true
	}
	if rest, found := strings.CutPrefix(class, "badge-"); found {
		if _, ok := badgePalette[rest]; ok {
			return "waxon-badge " + rest, true
		}
	}
	return "", false
}

// isClassChar reports whether c can appear in a palette class name. We
// keep it strict — lowercase ASCII only — so ".red." in prose doesn't
// accidentally match ".red.more".
func isClassChar(c byte) bool {
	return c >= 'a' && c <= 'z'
}
