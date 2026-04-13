package format

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// AddComment appends a `<!-- comment(@author): text -->` directive to a slide
// (or to a specific variant within a slide) inside a .slides file. The
// insertion is performed by line scanning instead of round-tripping through
// the parser, so unrelated formatting and whitespace are preserved.
//
// slideIndex is zero-based and counts only real slides (frontmatter is not a
// slide). variant is the variant name to target, or "" to target the main
// slide content.
//
// The author is sanitized to a single word (letters, digits, underscore).
// The text is sanitized so it cannot accidentally close the surrounding HTML
// comment.
func AddComment(filePath string, slideIndex int, variant, author, text string) error {
	if slideIndex < 0 {
		return fmt.Errorf("invalid slide index %d", slideIndex)
	}

	author = sanitizeAuthor(author)
	if author == "" {
		return fmt.Errorf("author is required")
	}

	text = sanitizeCommentText(text)
	if text == "" {
		return fmt.Errorf("comment text is required")
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filePath, err)
	}

	updated, err := insertCommentInSource(string(raw), slideIndex, variant, author, text)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("write %s: %w", filePath, err)
	}

	return nil
}

// insertCommentInSource is the pure-string core of AddComment, exposed
// separately so tests can exercise it without touching the filesystem.
func insertCommentInSource(source string, slideIndex int, variant, author, text string) (string, error) {
	directive := fmt.Sprintf("<!-- comment(@%s): %s -->", author, text)

	lines := strings.Split(source, "\n")

	// Locate the slide-body region by skipping the YAML frontmatter (if any)
	// and counting `---` separators that are not `---variant:`.
	bodyStart := skipFrontmatter(lines)

	// The body always implicitly starts with slide 0; each subsequent
	// `---` separator ends the current slide and starts the next.
	currentSlide := 0
	slideStart := bodyStart
	slideEnd := len(lines)
	found := false

	for i := bodyStart; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			if currentSlide == slideIndex {
				slideEnd = i
				found = true
				break
			}
			currentSlide++
			slideStart = i + 1
		}
	}

	if !found && currentSlide < slideIndex {
		return "", fmt.Errorf("slide index %d out of range (file has %d slides)", slideIndex, currentSlide+1)
	}
	// If we never explicitly closed the slide, it runs to EOF — slideStart
	// and slideEnd are already correct.

	// Find the insertion point inside the slide.
	insertAt := slideEnd
	if variant == "" {
		// Insert into the main slide body — i.e. before the first
		// `---variant:` if any, otherwise at slideEnd.
		variantStart := findVariantStart(lines, slideStart, slideEnd, "")
		if variantStart != -1 {
			insertAt = variantStart
		}
	} else {
		// Find the named variant within the slide range.
		vStart := findVariantStart(lines, slideStart, slideEnd, variant)
		if vStart == -1 {
			return "", fmt.Errorf("variant %q not found in slide %d", variant, slideIndex)
		}
		// Variant body runs until the next `---variant:` or the slide end.
		insertAt = findNextVariantOrEnd(lines, vStart+1, slideEnd)
	}

	// Trim trailing blank lines so the comment lands tight against the
	// existing content rather than after a gap.
	for insertAt > 0 && strings.TrimSpace(lines[insertAt-1]) == "" {
		insertAt--
	}

	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:insertAt]...)
	out = append(out, directive)
	out = append(out, lines[insertAt:]...)

	return strings.Join(out, "\n"), nil
}

// skipFrontmatter returns the line index immediately after the YAML
// frontmatter close. If there is no frontmatter, returns 0.
func skipFrontmatter(lines []string) int {
	if len(lines) == 0 {
		return 0
	}
	// Skip leading blank lines
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) || strings.TrimSpace(lines[i]) != "---" {
		return 0
	}
	// Find closing `---`
	for j := i + 1; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) == "---" {
			return j + 1
		}
	}
	return 0
}

var commentVariantLine = regexp.MustCompile(`^---variant:\s*(\S+)\s*$`)

// findVariantStart returns the line index of the first `---variant:` directive
// within [start, end). If name is non-empty, only that variant matches; if
// name is empty, any variant matches. Returns -1 if not found.
func findVariantStart(lines []string, start, end int, name string) int {
	if end > len(lines) {
		end = len(lines)
	}
	for i := start; i < end; i++ {
		m := commentVariantLine.FindStringSubmatch(strings.TrimSpace(lines[i]))
		if m == nil {
			continue
		}
		if name == "" || m[1] == name {
			return i
		}
	}
	return -1
}

// findNextVariantOrEnd returns the line index of the next `---variant:` line in
// [start, end), or end if none is found.
func findNextVariantOrEnd(lines []string, start, end int) int {
	if end > len(lines) {
		end = len(lines)
	}
	for i := start; i < end; i++ {
		if commentVariantLine.MatchString(strings.TrimSpace(lines[i])) {
			return i
		}
	}
	return end
}

var authorRe = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// sanitizeAuthor reduces an author string to safe characters. Spaces and
// punctuation collapse to a single hyphen so names like "Mary Jane" become
// "Mary-Jane" (case is preserved). The result must round-trip through the
// parser's commentRe, which allows [\w-]+.
func sanitizeAuthor(author string) string {
	author = strings.TrimSpace(author)
	author = authorRe.ReplaceAllString(author, "-")
	author = strings.Trim(author, "-")
	return author
}

// sanitizeCommentText collapses newlines and prevents premature comment
// termination by replacing any `-->` sequence with `--&gt;`.
func sanitizeCommentText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\r\n", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "-->", "--&gt;")
	// collapse runs of whitespace
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return text
}
