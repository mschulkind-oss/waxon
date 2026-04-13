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

func TestParseVariantDirectives(t *testing.T) {
	// Each variant body carries its own notes/comments/ai-notes/pauses so
	// reviewers can leave feedback on a specific alternative without
	// affecting the main slide or its siblings.
	input := `---
title: "Variant Directives"
---

# Original
Main body.
<!-- comment(@alice): main needs a hook -->

---variant: short
# Short
<!-- pause -->
Just one line.
<!-- comment(@bob): love this version -->
<!-- ai: shorter form for time-pressed audiences -->

---variant: long
# Long
<!-- note: read this slowly -->
Lots of detail.
<!-- comment(@bob): too dense, cut it -->
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(deck.Slides) != 1 {
		t.Fatalf("got %d slides, want 1", len(deck.Slides))
	}
	s := deck.Slides[0]
	if len(s.Comments) != 1 || s.Comments[0].Author != "alice" {
		t.Errorf("main comments = %+v", s.Comments)
	}
	if len(s.Variants) != 2 {
		t.Fatalf("got %d variants, want 2", len(s.Variants))
	}

	// short variant
	short := s.Variants[0]
	if short.Name != "short" {
		t.Errorf("variants[0].name = %q", short.Name)
	}
	if short.Pauses != 1 {
		t.Errorf("short variant pauses = %d, want 1", short.Pauses)
	}
	if len(short.Comments) != 1 || short.Comments[0].Author != "bob" {
		t.Errorf("short variant comments = %+v", short.Comments)
	}
	if len(short.AINotes) != 1 {
		t.Errorf("short variant aiNotes = %+v", short.AINotes)
	}
	if contains(short.Content, "comment(@bob)") {
		t.Error("variant content should be cleaned of directives")
	}

	// long variant
	long := s.Variants[1]
	if len(long.Notes) != 1 {
		t.Errorf("long variant notes = %+v", long.Notes)
	}
	if len(long.Comments) != 1 || long.Comments[0].Text != "too dense, cut it" {
		t.Errorf("long variant comments = %+v", long.Comments)
	}

	// Confirm comments are scoped — bob's comments did NOT leak into the
	// main slide body.
	for _, c := range s.Comments {
		if c.Author == "bob" {
			t.Error("variant comment leaked into main slide")
		}
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

// ---------- Slide IDs ----------

func TestParseSlideIDSeparator(t *testing.T) {
	input := `---
title: "IDs"
---

# First

Content.

--- #intro

# Intro

Hello.

--- #call-to-action

# CTA

Sign up now.
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(deck.Slides) != 3 {
		t.Fatalf("got %d slides, want 3", len(deck.Slides))
	}
	if deck.Slides[0].ID != "" {
		t.Errorf("first slide ID = %q, want empty", deck.Slides[0].ID)
	}
	if deck.Slides[1].ID != "intro" {
		t.Errorf("slide[1].ID = %q, want intro", deck.Slides[1].ID)
	}
	if deck.Slides[2].ID != "call-to-action" {
		t.Errorf("slide[2].ID = %q, want call-to-action", deck.Slides[2].ID)
	}
}

func TestParseSlideIDWithDigitsAndHyphens(t *testing.T) {
	input := `---
title: "T"
---

# First

--- #step-2a

# Step 2a

--- #final_step

# Final
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(deck.Slides) != 3 {
		t.Fatalf("got %d slides, want 3", len(deck.Slides))
	}
	if deck.Slides[1].ID != "step-2a" {
		t.Errorf("slide[1].ID = %q", deck.Slides[1].ID)
	}
	if deck.Slides[2].ID != "final_step" {
		t.Errorf("slide[2].ID = %q", deck.Slides[2].ID)
	}
}

func TestParseBareSeparatorStillWorks(t *testing.T) {
	input := `---
title: "T"
---

# A

---

# B
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(deck.Slides) != 2 {
		t.Fatalf("got %d slides", len(deck.Slides))
	}
	for _, s := range deck.Slides {
		if s.ID != "" {
			t.Errorf("slide %d unexpectedly got ID %q", s.Index, s.ID)
		}
	}
}

// ---------- Inline color ----------

func TestInlineColorSimple(t *testing.T) {
	out := applyInlineColor(`alpha .red{bravo} charlie`)
	want := `alpha <span class="red">bravo</span> charlie`
	if out != want {
		t.Errorf("got %q\nwant %q", out, want)
	}
}

func TestInlineColorAdjacent(t *testing.T) {
	out := applyInlineColor(`.red{a}.blue{b}`)
	want := `<span class="red">a</span><span class="blue">b</span>`
	if out != want {
		t.Errorf("got %q\nwant %q", out, want)
	}
}

func TestInlineColorNested(t *testing.T) {
	out := applyInlineColor(`.red{.blue{inner}}`)
	want := `<span class="red"><span class="blue">inner</span></span>`
	if out != want {
		t.Errorf("got %q\nwant %q", out, want)
	}
}

func TestInlineColorNestedBraces(t *testing.T) {
	// Curly braces inside the payload should not prematurely close the span.
	out := applyInlineColor(`.red{foo {bar} baz}`)
	want := `<span class="red">foo {bar} baz</span>`
	if out != want {
		t.Errorf("got %q\nwant %q", out, want)
	}
}

func TestInlineColorUnbalanced(t *testing.T) {
	in := `.red{unclosed`
	out := applyInlineColor(in)
	if out != in {
		t.Errorf("unbalanced inline color should remain literal; got %q", out)
	}
}

func TestInlineColorUnknownPalette(t *testing.T) {
	in := `.error{oops}`
	out := applyInlineColor(in)
	if out != in {
		t.Errorf("unknown palette class should remain literal; got %q", out)
	}
}

func TestInlineColorAllPalette(t *testing.T) {
	for _, class := range []string{"red", "green", "yellow", "blue", "aqua", "dim"} {
		in := "." + class + "{x}"
		want := `<span class="` + class + `">x</span>`
		if got := applyInlineColor(in); got != want {
			t.Errorf("class %q: got %q, want %q", class, got, want)
		}
	}
}

func TestInlineColorInsideFencedCode(t *testing.T) {
	input := "before .red{hit}\n\n```\n.red{miss}\n```\n\nafter .blue{hit2}"
	out := applyColorTransforms(input)
	if !contains(out, `<span class="red">hit</span>`) {
		t.Error("should transform outside fence")
	}
	if contains(out, `<span class="red">miss</span>`) {
		t.Error("should not transform inside fenced code block")
	}
	if !contains(out, `<span class="blue">hit2</span>`) {
		t.Error("should resume transforming after fence close")
	}
}

func TestInlineColorInsideBacktickSpan(t *testing.T) {
	input := "see `.red{untouched}` vs .green{touched}"
	out := applyColorTransforms(input)
	if !contains(out, "`.red{untouched}`") {
		t.Errorf("backtick code span should be preserved, got %q", out)
	}
	if !contains(out, `<span class="green">touched</span>`) {
		t.Errorf("outside backticks should transform, got %q", out)
	}
}

// ---------- Line-level color ----------

func TestLineColorStartOfLine(t *testing.T) {
	input := ".red This whole line is red"
	out := applyColorTransforms(input)
	want := `<span class="red">This whole line is red</span>`
	if out != want {
		t.Errorf("got %q\nwant %q", out, want)
	}
}

func TestLineColorNotMidLine(t *testing.T) {
	input := "prefix .red not at start"
	out := applyColorTransforms(input)
	if out != input {
		t.Errorf("mid-line .red should not wrap; got %q", out)
	}
}

func TestLineColorNotInsideFence(t *testing.T) {
	input := "outside\n```\n.red inside\n```\n.red after"
	out := applyColorTransforms(input)
	if contains(out, `<span class="red">inside</span>`) {
		t.Error("line color should not fire inside fenced code")
	}
	if !contains(out, `<span class="red">after</span>`) {
		t.Errorf("line color should fire after fence, got %q", out)
	}
}

func TestLineColorUnknownPalette(t *testing.T) {
	input := ".error something"
	out := applyColorTransforms(input)
	if out != input {
		t.Errorf("unknown palette should be literal; got %q", out)
	}
}

// ---------- Compare fence blocks ----------

func TestCompareHappyPath(t *testing.T) {
	input := `:::compare
::left red
Left body
::right green
Right body
:::`
	out := applyCompareBlocks(input)
	if !contains(out, `<div class="waxon-compare">`) {
		t.Error("missing waxon-compare wrapper")
	}
	if !contains(out, `<div class="waxon-compare-pane waxon-compare-left red">`) {
		t.Errorf("missing left pane with red class, got %q", out)
	}
	if !contains(out, `<div class="waxon-compare-pane waxon-compare-right green">`) {
		t.Errorf("missing right pane with green class, got %q", out)
	}
	if !contains(out, "Left body") || !contains(out, "Right body") {
		t.Errorf("bodies missing, got %q", out)
	}
}

func TestCompareWithoutPaneColors(t *testing.T) {
	input := `:::compare
::left
A
::right
B
:::`
	out := applyCompareBlocks(input)
	if !contains(out, `<div class="waxon-compare-pane waxon-compare-left">`) {
		t.Errorf("left pane without color should have no palette class, got %q", out)
	}
}

func TestCompareUnterminated(t *testing.T) {
	input := `:::compare
::left
unfinished`
	out := applyCompareBlocks(input)
	if !contains(out, `waxon-error`) {
		t.Errorf("unterminated compare should emit error banner, got %q", out)
	}
}

func TestCompareNested(t *testing.T) {
	input := `:::compare
::left
A
:::compare
::right
B
:::`
	out := applyCompareBlocks(input)
	if !contains(out, `waxon-error`) {
		t.Errorf("nested compare should emit error, got %q", out)
	}
}

func TestCompareEndToEnd(t *testing.T) {
	// Verify the full parser pipeline threads compare + color together.
	input := `---
title: "E2E"
---

# Slide

:::compare
::left red
.yellow{warn}
::right green
ok
:::
`
	deck, err := Parse(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(deck.Slides) != 1 {
		t.Fatalf("got %d slides", len(deck.Slides))
	}
	c := deck.Slides[0].Content
	if !contains(c, `<div class="waxon-compare">`) {
		t.Errorf("missing waxon-compare, got %q", c)
	}
	if !contains(c, `<span class="yellow">warn</span>`) {
		t.Errorf("color transforms should run inside compare pane, got %q", c)
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
