package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testSlides = `---
title: "Test Deck"
theme: default
---

# Slide 1

Hello world.

<!-- comment(@alice): needs more detail -->

---

# Slide 2

Goodbye.
`

func writeTestSlides(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.slides")
	if err := os.WriteFile(path, []byte(testSlides), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func executeCmd(args ...string) (string, error) {
	cmd := rootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestRootVersion(t *testing.T) {
	out, err := executeCmd("--version")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out, "dev") {
		t.Errorf("version output = %q", out)
	}
}

func TestRootHelp(t *testing.T) {
	out, err := executeCmd("--help")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out, "serve") {
		t.Error("help should mention serve")
	}
	if !strings.Contains(out, "export") {
		t.Error("help should mention export")
	}
	if !strings.Contains(out, "themes") {
		t.Error("help should mention themes")
	}
}

func TestThemesCmd(t *testing.T) {
	out, err := executeCmd("themes")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out, "default") {
		t.Error("should list default theme")
	}
	if !strings.Contains(out, "terminal") {
		t.Error("should list terminal theme")
	}
}

func TestThemesCmdJSON(t *testing.T) {
	out, err := executeCmd("themes", "--json")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	var themes []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(out), &themes); err != nil {
		t.Fatalf("JSON decode: %v\noutput: %s", err, out)
	}
	if len(themes) != 21 {
		t.Errorf("got %d themes, want 21", len(themes))
	}
}

func TestNewCmd(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	out, err := executeCmd("new", "my-talk")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out, "Created my-talk.slides") {
		t.Errorf("output = %q", out)
	}

	data, err := os.ReadFile(filepath.Join(dir, "my-talk.slides"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "title: \"my-talk\"") {
		t.Error("missing title")
	}
	if !strings.Contains(content, "theme: default") {
		t.Error("missing default theme")
	}
	if !strings.Contains(content, "16:9") {
		t.Error("missing default aspect ratio")
	}
}

func TestNewCmdCustomFlags(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	_, err := executeCmd("new", "my-talk", "--theme", "terminal", "--ratio", "4:3")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "my-talk.slides"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "theme: terminal") {
		t.Error("theme override not applied")
	}
	if !strings.Contains(content, "4:3") {
		t.Error("ratio override not applied")
	}
}

func TestNewCmdAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	os.WriteFile(filepath.Join(dir, "existing.slides"), []byte("x"), 0644)

	_, err := executeCmd("new", "existing")
	if err == nil {
		t.Error("expected error for existing file")
	}
}

func TestNewCmdWithExtension(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	_, err := executeCmd("new", "test.slides")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "test.slides")); err != nil {
		t.Error("file not created with .slides extension")
	}
}

func TestAgentContextCmd(t *testing.T) {
	path := writeTestSlides(t)

	out, err := executeCmd("agent-context", path)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	var result struct {
		File string `json:"file"`
		Deck struct {
			Meta struct {
				Title string `json:"title"`
			} `json:"meta"`
			Slides []any `json:"slides"`
		} `json:"deck"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("JSON decode: %v\noutput: %s", err, out)
	}
	if result.Deck.Meta.Title != "Test Deck" {
		t.Errorf("title = %q", result.Deck.Meta.Title)
	}
	if len(result.Deck.Slides) != 2 {
		t.Errorf("slides = %d, want 2", len(result.Deck.Slides))
	}
	if result.File == "" {
		t.Error("file path should be set")
	}
}

func TestAgentContextCmdBadFile(t *testing.T) {
	_, err := executeCmd("agent-context", "/nonexistent.slides")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestCommentCmdList(t *testing.T) {
	path := writeTestSlides(t)

	out, err := executeCmd("comment", path)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("should show alice's comment, got: %s", out)
	}
	if !strings.Contains(out, "needs more detail") {
		t.Errorf("should show comment text, got: %s", out)
	}
}

func TestCommentCmdListJSON(t *testing.T) {
	path := writeTestSlides(t)

	out, err := executeCmd("comment", path, "--json")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	var comments []struct {
		Slide   int `json:"slide"`
		Comment struct {
			Author string `json:"author"`
			Text   string `json:"text"`
		} `json:"comment"`
	}
	if err := json.Unmarshal([]byte(out), &comments); err != nil {
		t.Fatalf("JSON decode: %v", err)
	}
	if len(comments) != 1 {
		t.Errorf("got %d comments, want 1", len(comments))
	}
}

func TestCommentCmdFilterByAuthor(t *testing.T) {
	path := writeTestSlides(t)

	out, err := executeCmd("comment", path, "--author", "bob")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out, "No comments found") {
		t.Errorf("should find no comments for bob, got: %s", out)
	}
}

func TestCommentCmdFilterBySlide(t *testing.T) {
	path := writeTestSlides(t)

	out, err := executeCmd("comment", path, "--slide", "2")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out, "No comments found") {
		t.Errorf("slide 2 has no comments, got: %s", out)
	}
}

func TestCommentCmdNoFile(t *testing.T) {
	_, err := executeCmd("comment", "/nonexistent.slides")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestExportCmd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PDF export test in short mode")
	}

	path := writeTestSlides(t)
	dir := t.TempDir()
	output := filepath.Join(dir, "test.pdf")

	out, err := executeCmd("export", path, "-o", output)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.Contains(out, "Wrote") {
		t.Errorf("output = %q", out)
	}

	if _, err := os.Stat(output); err != nil {
		t.Error("PDF file not created")
	}
}

func TestExportCmdDefaultOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PDF export test in short mode")
	}

	path := writeTestSlides(t)
	dir := filepath.Dir(path)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	out, err := executeCmd("export", path)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.Contains(out, "test.pdf") {
		t.Errorf("should default to test.pdf, got: %s", out)
	}
}

func TestServeCmdNoArgs(t *testing.T) {
	_, err := executeCmd("serve")
	if err == nil {
		t.Error("expected error for missing file arg")
	}
}

func TestServeCmdBadFile(t *testing.T) {
	_, err := executeCmd("serve", "/nonexistent.slides")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestServeCmdHelpMentionsDirectoryMode(t *testing.T) {
	out, err := executeCmd("serve", "--help")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out, "directory") {
		t.Error("serve --help should mention directory mode")
	}
	if !strings.Contains(out, "Variants panel") {
		t.Error("serve --help should document the variants panel")
	}
	if !strings.Contains(out, "Decks panel") {
		t.Error("serve --help should document the decks panel")
	}
}

func TestExportCmdNoArgs(t *testing.T) {
	_, err := executeCmd("export")
	if err == nil {
		t.Error("expected error for missing file arg")
	}
}

func TestNewCmdNoArgs(t *testing.T) {
	_, err := executeCmd("new")
	if err == nil {
		t.Error("expected error for missing name arg")
	}
}

func TestAgentContextCmdNoArgs(t *testing.T) {
	_, err := executeCmd("agent-context")
	if err == nil {
		t.Error("expected error for missing file arg")
	}
}

func TestCommentCmdNoArgs(t *testing.T) {
	_, err := executeCmd("comment")
	if err == nil {
		t.Error("expected error for missing file arg")
	}
}

func TestCommentCmdAdd(t *testing.T) {
	// Write a slides file we can modify
	dir := t.TempDir()
	path := filepath.Join(dir, "test.slides")
	if err := os.WriteFile(path, []byte(testSlides), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := executeCmd("comment", path, "--add", "great slide", "--slide", "1", "--author", "bob")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "<!-- comment(@bob): great slide -->") {
		t.Errorf("comment not added to file, content:\n%s", content)
	}
}

func TestCommentCmdAddBadSlide(t *testing.T) {
	path := writeTestSlides(t)

	_, err := executeCmd("comment", path, "--add", "test", "--slide", "99", "--author", "bob")
	if err == nil {
		t.Error("expected error for out-of-range slide")
	}
}

func TestCommentCmdAddNoSlide(t *testing.T) {
	path := writeTestSlides(t)

	_, err := executeCmd("comment", path, "--add", "test", "--author", "bob")
	if err == nil {
		t.Error("expected error when --slide not set for --add")
	}
}

// Integration test: full roundtrip new -> parse -> agent-context
func TestRoundtripNewToAgentContext(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Create a new deck
	_, err := executeCmd("new", "roundtrip", "--theme", "terminal")
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	// Read it back via agent-context
	path := filepath.Join(dir, "roundtrip.slides")
	out, err := executeCmd("agent-context", path)
	if err != nil {
		t.Fatalf("agent-context: %v", err)
	}

	var result struct {
		File string `json:"file"`
		Deck struct {
			Meta struct {
				Title string `json:"title"`
				Theme string `json:"theme"`
			} `json:"meta"`
			Slides []any `json:"slides"`
		} `json:"deck"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("JSON decode: %v", err)
	}
	if result.Deck.Meta.Title != "roundtrip" {
		t.Errorf("title = %q", result.Deck.Meta.Title)
	}
	if result.Deck.Meta.Theme != "terminal" {
		t.Errorf("theme = %q", result.Deck.Meta.Theme)
	}
	if len(result.Deck.Slides) < 2 {
		t.Errorf("expected multiple slides, got %d", len(result.Deck.Slides))
	}
}
