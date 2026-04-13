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

	// Extract directives from main content, then apply the compare-fence
	// and color passes. Order matters: directives are stripped first (so
	// pause sentinels sit in the DOM child stream at block level), then
	// compare blocks (which emit block-level HTML containers around
	// markdown), then color transforms (which emit inline HTML spans
	// inside the remaining prose).
	slide.Content, slide.Notes, slide.AINotes, slide.Comments, slide.Pauses, slide.SlideOpts = extractDirectives(mainContent)
	slide.Content = applyCompareBlocks(slide.Content)
	slide.Content = applyColorTransforms(slide.Content)

	// Extract directives from each variant body so reviewers can comment
	// on a specific alternative.
	for _, v := range variants {
		content, notes, aiNotes, comments, pauses, _ := extractDirectives(v.Content)
		content = applyCompareBlocks(content)
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
func parseSlideOpts(raw string) *SlideOpts {
	opts := &SlideOpts{}
	for pair := range strings.SplitSeq(raw, ",") {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
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

// ---------- Compare fence blocks ----------

// applyCompareBlocks transforms :::compare / ::left / ::right / ::: fence
// blocks into the HTML div structure the renderer styles. The emitted HTML
// relies on CommonMark type-6 HTML block behavior: a blank line after an
// opening <div> ends the HTML block, so inner markdown is parsed as usual,
// and the closing </div> starts a new HTML block. That's why the emitted
// text has careful blank lines around the pane bodies.
//
// Nested :::compare is not supported and emits a visible error div in
// place of the broken block so the author sees exactly where it failed.
func applyCompareBlocks(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) == ":::compare" {
			block, consumed, err := parseCompareBlock(lines[i+1:])
			if err != nil {
				out = append(out, fmt.Sprintf(
					`<div class="waxon-error">waxon-compare error: %s</div>`,
					err.Error(),
				))
				i += 1 + consumed
				continue
			}
			out = append(out, block...)
			i += 1 + consumed
			continue
		}
		out = append(out, lines[i])
		i++
	}
	return strings.Join(out, "\n")
}

// parseCompareBlock reads lines starting *after* the opening :::compare
// marker until it finds the closing :::. Returns the emitted HTML lines,
// the number of source lines consumed (including the closing :::), and
// any error found along the way.
func parseCompareBlock(lines []string) ([]string, int, error) {
	var leftColor, rightColor string
	var leftBody, rightBody []string
	side := ""
	closed := false
	consumed := 0
	for i, line := range lines {
		consumed = i + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == ":::compare" {
			return nil, consumed, fmt.Errorf("nested :::compare is not supported")
		}
		if trimmed == ":::" {
			closed = true
			break
		}
		if strings.HasPrefix(trimmed, "::left") {
			side = "left"
			leftColor = strings.TrimSpace(strings.TrimPrefix(trimmed, "::left"))
			continue
		}
		if strings.HasPrefix(trimmed, "::right") {
			side = "right"
			rightColor = strings.TrimSpace(strings.TrimPrefix(trimmed, "::right"))
			continue
		}
		switch side {
		case "left":
			leftBody = append(leftBody, line)
		case "right":
			rightBody = append(rightBody, line)
		default:
			// Content before ::left — ignored (commonly whitespace).
		}
	}
	if !closed {
		return nil, consumed, fmt.Errorf("unterminated :::compare (missing closing :::)")
	}

	out := []string{`<div class="waxon-compare">`}
	out = append(out, paneHTML("left", leftColor, leftBody)...)
	out = append(out, paneHTML("right", rightColor, rightBody)...)
	out = append(out, `</div>`)
	return out, consumed, nil
}

// paneHTML emits the HTML lines for a single compare pane. Blank lines
// around the pane body are mandatory: CommonMark type-6 HTML blocks end at
// a blank line, so the inner body must be separated from the wrapper div
// to be parsed as markdown instead of raw HTML.
func paneHTML(side, color string, body []string) []string {
	classes := "waxon-compare-pane waxon-compare-" + side
	if _, ok := colorPalette[color]; ok {
		classes += " " + color
	}
	out := []string{fmt.Sprintf(`<div class="%s">`, classes), ""}
	// Trim leading/trailing blank lines in the body so we don't emit
	// excessive whitespace, but preserve inner blanks.
	start, end := 0, len(body)
	for start < end && strings.TrimSpace(body[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(body[end-1]) == "" {
		end--
	}
	out = append(out, body[start:end]...)
	out = append(out, "", `</div>`)
	return out
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

// applyInlineColor rewrites all `.color{text}` occurrences in a line.
// Uses a balanced-brace walker so nested braces `.red{a {b} c}` are
// handled. On unbalanced or unknown class, leaves text as-is.
func applyInlineColor(text string) string {
	var out strings.Builder
	i := 0
	for i < len(text) {
		// Look for `.<class>{`.
		if text[i] != '.' {
			out.WriteByte(text[i])
			i++
			continue
		}
		// Scan class name.
		j := i + 1
		for j < len(text) && isClassChar(text[j]) {
			j++
		}
		if j == i+1 || j >= len(text) || text[j] != '{' {
			out.WriteByte(text[i])
			i++
			continue
		}
		class := text[i+1 : j]
		if _, ok := colorPalette[class]; !ok {
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
		fmt.Fprintf(&out, `<span class="%s">%s</span>`, class, inner)
		i = k + 1
	}
	return out.String()
}

// isClassChar reports whether c can appear in a palette class name. We
// keep it strict — lowercase ASCII only — so ".red." in prose doesn't
// accidentally match ".red.more".
func isClassChar(c byte) bool {
	return c >= 'a' && c <= 'z'
}
