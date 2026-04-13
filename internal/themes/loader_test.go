package themes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExternal_NoHeader(t *testing.T) {
	dir := t.TempDir()
	css := `body { color: red; }`
	if err := os.WriteFile(filepath.Join(dir, "acme.css"), []byte(css), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = LoadExternal(nil) })
	if err := LoadExternal([]string{dir}); err != nil {
		t.Fatalf("LoadExternal: %v", err)
	}
	got := externalThemes()
	if len(got) != 1 {
		t.Fatalf("want 1 theme, got %d", len(got))
	}
	if got[0].Name != "acme" {
		t.Errorf("name = %q, want %q", got[0].Name, "acme")
	}
	if got[0].Description != "" {
		t.Errorf("description = %q, want empty", got[0].Description)
	}
	if got[0].CSS != css {
		t.Errorf("CSS did not round-trip")
	}
}

func TestLoadExternal_WithHeader(t *testing.T) {
	dir := t.TempDir()
	css := `/*! waxon-theme
name: acme-corp
description: Acme Corporation brand theme
*/
body { color: red; }`
	if err := os.WriteFile(filepath.Join(dir, "anything.css"), []byte(css), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = LoadExternal(nil) })
	if err := LoadExternal([]string{dir}); err != nil {
		t.Fatalf("LoadExternal: %v", err)
	}
	got := externalThemes()
	if len(got) != 1 {
		t.Fatalf("want 1 theme, got %d", len(got))
	}
	if got[0].Name != "acme-corp" {
		t.Errorf("name = %q, want %q (header should override filename)", got[0].Name, "acme-corp")
	}
	if got[0].Description != "Acme Corporation brand theme" {
		t.Errorf("description = %q", got[0].Description)
	}
}

func TestLoadExternal_MissingDirSilent(t *testing.T) {
	t.Cleanup(func() { _ = LoadExternal(nil) })
	err := LoadExternal([]string{"/this/path/does/not/exist/waxon-test"})
	if err != nil {
		t.Errorf("missing dir should be silent, got %v", err)
	}
	if len(externalThemes()) != 0 {
		t.Error("expected no external themes loaded")
	}
}

func TestLoadExternal_ShadowsBuiltin(t *testing.T) {
	dir := t.TempDir()
	// Use a name that collides with a built-in to verify shadow replacement.
	css := `/*! waxon-theme
name: default
description: overridden
*/
body { color: red; }`
	if err := os.WriteFile(filepath.Join(dir, "whatever.css"), []byte(css), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = LoadExternal(nil) })
	if err := LoadExternal([]string{dir}); err != nil {
		t.Fatalf("LoadExternal: %v", err)
	}
	all := All()
	var found *Theme
	for i := range all {
		if all[i].Name == "default" {
			found = &all[i]
			break
		}
	}
	if found == nil {
		t.Fatal("default theme not found in All()")
	}
	if found.Description != "overridden" {
		t.Errorf("external theme should shadow builtin, got description = %q", found.Description)
	}
	// And "default" should appear exactly once.
	n := 0
	for _, th := range all {
		if th.Name == "default" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("want exactly 1 'default' theme, got %d", n)
	}
}
