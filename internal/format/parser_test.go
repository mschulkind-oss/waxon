package format

import (
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	input := `---
title: "Test Deck"
author: "Alice"
theme: terminal
aspect: "4:3"
footer: "ACME Corp"
terminal-variant: nord
terminal-effects: true
---

# Slide 1
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if deck.Meta.Title != "Test Deck" {
		t.Errorf("title = %q, want %q", deck.Meta.Title, "Test Deck")
	}
	if deck.Meta.Author != "Alice" {
		t.Errorf("author = %q, want %q", deck.Meta.Author, "Alice")
	}
	if deck.Meta.Theme != "terminal" {
		t.Errorf("theme = %q, want %q", deck.Meta.Theme, "terminal")
	}
	if deck.Meta.Aspect != "4:3" {
		t.Errorf("aspect = %q, want %q", deck.Meta.Aspect, "4:3")
	}
	if deck.Meta.Footer != "ACME Corp" {
		t.Errorf("footer = %q, want %q", deck.Meta.Footer, "ACME Corp")
	}
	if deck.Meta.TerminalVariant != "nord" {
		t.Errorf("terminal-variant = %q, want %q", deck.Meta.TerminalVariant, "nord")
	}
	if !deck.Meta.TerminalEffects {
		t.Error("terminal-effects should be true")
	}
}

func TestParseDefaults(t *testing.T) {
	input := `---
title: "Minimal"
---

# Hello
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deck.Meta.Theme != "default" {
		t.Errorf("default theme = %q, want %q", deck.Meta.Theme, "default")
	}
	if deck.Meta.Aspect != "16:9" {
		t.Errorf("default aspect = %q, want %q", deck.Meta.Aspect, "16:9")
	}
}

func TestParseNoFrontmatter(t *testing.T) {
	input := `# Just a slide

Some content.
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deck.Slides) != 1 {
		t.Fatalf("got %d slides, want 1", len(deck.Slides))
	}
	if deck.Meta.Theme != "" {
		t.Errorf("theme should be empty without frontmatter, got %q", deck.Meta.Theme)
	}
}

func TestParseMultipleSlides(t *testing.T) {
	input := `---
title: "Multi"
---

# Slide 1

Content one.

---

# Slide 2

Content two.

---

# Slide 3

Content three.
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deck.Slides) != 3 {
		t.Fatalf("got %d slides, want 3", len(deck.Slides))
	}
	for i, s := range deck.Slides {
		if s.Index != i {
			t.Errorf("slide %d has index %d", i, s.Index)
		}
	}
}

func TestParseSpeakerNotes(t *testing.T) {
	input := `---
title: "Notes"
---

# Slide 1

Content here.

<!-- note: Remember to pause -->
<!-- note: Second note -->
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := deck.Slides[0]
	if len(s.Notes) != 2 {
		t.Fatalf("got %d notes, want 2", len(s.Notes))
	}
	if s.Notes[0] != "Remember to pause" {
		t.Errorf("note[0] = %q", s.Notes[0])
	}
	if s.Notes[1] != "Second note" {
		t.Errorf("note[1] = %q", s.Notes[1])
	}
	// Notes should be stripped from content
	if contains(s.Content, "<!-- note:") {
		t.Error("content should not contain note directives")
	}
}

func TestParseAINotes(t *testing.T) {
	input := `---
title: "AI"
---

# Slide 1

Content.

<!-- ai: Keep this high-level -->
<!-- ai: Source from Q3 report -->
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := deck.Slides[0]
	if len(s.AINotes) != 2 {
		t.Fatalf("got %d ai notes, want 2", len(s.AINotes))
	}
	if s.AINotes[0] != "Keep this high-level" {
		t.Errorf("ai_note[0] = %q", s.AINotes[0])
	}
}

func TestParseComments(t *testing.T) {
	input := `---
title: "Comments"
---

# Slide 1

Growth rate is 15%.

<!-- comment(@alice): Use Q3 numbers? -->
<!-- comment(@bob): Yes, update it -->
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := deck.Slides[0]
	if len(s.Comments) != 2 {
		t.Fatalf("got %d comments, want 2", len(s.Comments))
	}
	if s.Comments[0].Author != "alice" {
		t.Errorf("comment[0].author = %q", s.Comments[0].Author)
	}
	if s.Comments[0].Text != "Use Q3 numbers?" {
		t.Errorf("comment[0].text = %q", s.Comments[0].Text)
	}
	if s.Comments[1].Author != "bob" {
		t.Errorf("comment[1].author = %q", s.Comments[1].Author)
	}
}

func TestParsePauses(t *testing.T) {
	input := `---
title: "Pauses"
---

# Three Things

- First
<!-- pause -->
- Second
<!-- pause -->
- Third
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := deck.Slides[0]
	if s.Pauses != 2 {
		t.Errorf("pauses = %d, want 2", s.Pauses)
	}
	if contains(s.Content, "<!-- pause") {
		t.Error("content should not contain pause directives")
	}
}

func TestParseSlideOpts(t *testing.T) {
	input := `---
title: "Opts"
---

<!-- slide: bg=#1a1a2e, class=centered -->
# Special Slide

Custom background.
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := deck.Slides[0]
	if s.SlideOpts == nil {
		t.Fatal("slide opts should not be nil")
	}
	if s.SlideOpts.Background != "#1a1a2e" {
		t.Errorf("bg = %q, want %q", s.SlideOpts.Background, "#1a1a2e")
	}
	if s.SlideOpts.Class != "centered" {
		t.Errorf("class = %q, want %q", s.SlideOpts.Class, "centered")
	}
}

func TestParseVariants(t *testing.T) {
	input := `---
title: "Variants"
---

# Welcome
Standard opening.

---variant: welcome-bold
# Welcome to the Future
Bold opening.

---variant: welcome-minimal
# Welcome

---

# Second Slide
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deck.Slides) != 2 {
		t.Fatalf("got %d slides, want 2", len(deck.Slides))
	}

	s := deck.Slides[0]
	if len(s.Variants) != 2 {
		t.Fatalf("got %d variants, want 2", len(s.Variants))
	}
	if s.Variants[0].Name != "welcome-bold" {
		t.Errorf("variant[0].name = %q", s.Variants[0].Name)
	}
	if s.Variants[1].Name != "welcome-minimal" {
		t.Errorf("variant[1].name = %q", s.Variants[1].Name)
	}
	if !contains(s.Variants[0].Content, "Welcome to the Future") {
		t.Errorf("variant[0] content missing expected text")
	}
}

func TestParseAllDirectives(t *testing.T) {
	input := `---
title: "Everything"
---

<!-- slide: bg=#000, class=dark -->
# Big Slide

Main content.

<!-- note: Speaker note here -->
<!-- ai: Agent context here -->
<!-- comment(@reviewer): Looks good -->
<!-- pause -->

More content.
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := deck.Slides[0]
	if len(s.Notes) != 1 {
		t.Errorf("notes = %d, want 1", len(s.Notes))
	}
	if len(s.AINotes) != 1 {
		t.Errorf("ai notes = %d, want 1", len(s.AINotes))
	}
	if len(s.Comments) != 1 {
		t.Errorf("comments = %d, want 1", len(s.Comments))
	}
	if s.Pauses != 1 {
		t.Errorf("pauses = %d, want 1", s.Pauses)
	}
	if s.SlideOpts == nil {
		t.Error("slide opts should not be nil")
	}
	// Content should be clean
	if contains(s.Content, "<!--") {
		t.Errorf("content still contains directives: %q", s.Content)
	}
	if !contains(s.Content, "Main content.") {
		t.Error("content missing main text")
	}
	if !contains(s.Content, "More content.") {
		t.Error("content missing more text")
	}
}

func TestParseEmptyInput(t *testing.T) {
	deck, err := Parse("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deck.Slides) != 0 {
		t.Errorf("got %d slides, want 0", len(deck.Slides))
	}
}

func TestParseInvalidYAML(t *testing.T) {
	input := `---
title: [invalid yaml
---

# Slide
`
	_, err := Parse(input)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestSplitFrontmatterNoClosing(t *testing.T) {
	input := `---
title: "No closing"

# This is content, not frontmatter
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should treat entire thing as body (no valid frontmatter)
	if deck.Meta.Title != "" {
		t.Errorf("expected empty title, got %q", deck.Meta.Title)
	}
}

func TestParseSlideOptsPartial(t *testing.T) {
	input := `---
title: "Partial Opts"
---

<!-- slide: bg=#fff -->
# Slide

Content.
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := deck.Slides[0]
	if s.SlideOpts == nil {
		t.Fatal("slide opts should not be nil")
	}
	if s.SlideOpts.Background != "#fff" {
		t.Errorf("bg = %q, want %q", s.SlideOpts.Background, "#fff")
	}
	if s.SlideOpts.Class != "" {
		t.Errorf("class should be empty, got %q", s.SlideOpts.Class)
	}
}

func TestCollapseBlankLines(t *testing.T) {
	input := "a\n\n\n\n\nb"
	result := collapseBlankLines(input)
	if result != "a\n\nb" {
		t.Errorf("got %q, want %q", result, "a\n\nb")
	}
}

func TestParseSlideOptsEmpty(t *testing.T) {
	opts := parseSlideOpts("")
	if opts != nil {
		t.Error("expected nil for empty opts")
	}
}

func TestParseSlideOptsInvalidPair(t *testing.T) {
	opts := parseSlideOpts("noequals")
	if opts != nil {
		t.Error("expected nil for invalid pair")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
