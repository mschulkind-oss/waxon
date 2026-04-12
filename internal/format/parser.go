package format

import (
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
	commentRe = regexp.MustCompile(`<!--\s*comment\(@(\w+)\):\s*(.*?)\s*-->`)
	// pauseRe matches <!-- pause -->
	pauseRe = regexp.MustCompile(`<!--\s*pause\s*-->`)
	// slideOptsRe matches <!-- slide: key=val, key=val -->
	slideOptsRe = regexp.MustCompile(`<!--\s*slide:\s*(.*?)\s*-->`)
	// variantRe matches ---variant: name
	variantRe = regexp.MustCompile(`^---variant:\s*(\S+)\s*$`)
)

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
	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		return meta, input, nil
	}

	yamlContent := rest[:idx]
	body := rest[idx+4:] // skip \n---

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
	// Split on lines that are exactly "---" (not ---variant:)
	lines := strings.Split(body, "\n")
	var slides []Slide
	var current []string
	index := 0

	flush := func() {
		if len(current) > 0 || index == 0 {
			raw := strings.Join(current, "\n")
			raw = strings.TrimSpace(raw)
			if raw != "" || index == 0 {
				slide := buildSlide(index, raw)
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
		if trimmed == "---" {
			flush()
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
		Index:    index,
		Variants: variants,
	}

	// Extract directives from main content
	slide.Content, slide.Notes, slide.AINotes, slide.Comments, slide.Pauses, slide.SlideOpts = extractDirectives(mainContent)

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

	// Remove all directives from content
	cleaned = noteRe.ReplaceAllString(content, "")
	cleaned = aiRe.ReplaceAllString(cleaned, "")
	cleaned = commentRe.ReplaceAllString(cleaned, "")
	cleaned = pauseRe.ReplaceAllString(cleaned, "")
	cleaned = slideOptsRe.ReplaceAllString(cleaned, "")

	// Clean up extra blank lines
	cleaned = collapseBlankLines(cleaned)
	cleaned = strings.TrimSpace(cleaned)

	return
}

// parseSlideOpts parses "bg=#fff, class=centered" into SlideOpts.
func parseSlideOpts(raw string) *SlideOpts {
	opts := &SlideOpts{}
	pairs := strings.Split(raw, ",")
	for _, pair := range pairs {
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
