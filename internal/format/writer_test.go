package format

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const writerSample = `---
title: "Sample"
theme: default
---

# Slide 0

Hello.

---

# Slide 1

Body.

---variant: alt
# Slide 1 Alt

Different body.

---

# Slide 2

End.
`

func TestSanitizeAuthor(t *testing.T) {
	cases := map[string]string{
		"alice":           "alice",
		"  Alice  ":       "Alice",
		"alice@example":   "alice-example",
		"alice bob":       "alice-bob",
		"!!alice!!":       "alice",
		"alice/../../bob": "alice-bob",
		"  ":              "",
	}
	for in, want := range cases {
		if got := sanitizeAuthor(in); got != want {
			t.Errorf("sanitizeAuthor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeCommentText(t *testing.T) {
	cases := map[string]string{
		"hello":                "hello",
		"  hello  ":            "hello",
		"line one\nline two":   "line one line two",
		"line one\r\nline two": "line one line two",
		"close --> early":      "close --&gt; early",
		"a\t\tb     c":         "a b c",
		"":                     "",
	}
	for in, want := range cases {
		if got := sanitizeCommentText(in); got != want {
			t.Errorf("sanitizeCommentText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInsertCommentMainBody(t *testing.T) {
	out, err := insertCommentInSource(writerSample, 0, "", "alice", "great opener")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<!-- comment(@alice): great opener -->") {
		t.Errorf("comment not inserted:\n%s", out)
	}
	// Make sure it landed in slide 0 (before the first ---), not later.
	idxComment := strings.Index(out, "comment(@alice)")
	idxSep := strings.Index(out[strings.Index(out, "Hello."):], "\n---")
	if idxComment == -1 || idxComment > strings.Index(out, "Hello.")+idxSep {
		t.Errorf("comment landed past slide 0: idxComment=%d", idxComment)
	}
}

func TestInsertCommentTargetVariant(t *testing.T) {
	out, err := insertCommentInSource(writerSample, 1, "alt", "bob", "tighten this")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<!-- comment(@bob): tighten this -->") {
		t.Errorf("variant comment not inserted:\n%s", out)
	}
	// Confirm the comment is INSIDE the variant section, not in the main
	// slide body.
	variantStart := strings.Index(out, "---variant: alt")
	commentIdx := strings.Index(out, "comment(@bob)")
	if commentIdx < variantStart {
		t.Errorf("comment landed before variant: variantStart=%d commentIdx=%d", variantStart, commentIdx)
	}
}

func TestInsertCommentUnknownVariant(t *testing.T) {
	_, err := insertCommentInSource(writerSample, 1, "ghost", "bob", "x")
	if err == nil {
		t.Error("expected error for unknown variant")
	}
}

func TestInsertCommentSlideOutOfRange(t *testing.T) {
	_, err := insertCommentInSource(writerSample, 99, "", "bob", "x")
	if err == nil {
		t.Error("expected error for out-of-range slide index")
	}
}

func TestInsertCommentEmptyAuthor(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "x.slides")
	if err := os.WriteFile(path, []byte(writerSample), 0644); err != nil {
		t.Fatal(err)
	}
	if err := AddComment(path, 0, "", "  ", "x"); err == nil {
		t.Error("expected error for empty author")
	}
}

func TestInsertCommentEmptyText(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "x.slides")
	if err := os.WriteFile(path, []byte(writerSample), 0644); err != nil {
		t.Fatal(err)
	}
	if err := AddComment(path, 0, "", "alice", "   "); err == nil {
		t.Error("expected error for empty text")
	}
}

func TestAddCommentRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "x.slides")
	if err := os.WriteFile(path, []byte(writerSample), 0644); err != nil {
		t.Fatal(err)
	}

	if err := AddComment(path, 1, "", "alice", "needs an example"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Re-parse and confirm the comment is on slide 1's main body.
	deck, err := Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(deck.Slides) != 3 {
		t.Fatalf("got %d slides, want 3", len(deck.Slides))
	}
	if len(deck.Slides[1].Comments) == 0 {
		t.Fatal("slide 1 should have a comment")
	}
	c := deck.Slides[1].Comments[0]
	if c.Author != "alice" || c.Text != "needs an example" {
		t.Errorf("got comment %+v", c)
	}
}

func TestAddCommentToVariantRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "x.slides")
	if err := os.WriteFile(path, []byte(writerSample), 0644); err != nil {
		t.Fatal(err)
	}

	if err := AddComment(path, 1, "alt", "bob", "tighten the wording"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	deck, err := Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(deck.Slides[1].Variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(deck.Slides[1].Variants))
	}
	v := deck.Slides[1].Variants[0]
	if len(v.Comments) == 0 {
		t.Fatal("variant should have a comment")
	}
	if v.Comments[0].Author != "bob" {
		t.Errorf("variant comment author = %q", v.Comments[0].Author)
	}
	// And the main slide body must NOT have picked it up.
	if len(deck.Slides[1].Comments) != 0 {
		t.Error("variant comment leaked into main body")
	}
}
