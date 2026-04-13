package themes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// external holds themes loaded from disk at startup. Nothing modifies it
// after LoadExternal returns, so All()/Get() can read it without locking;
// we still take the mutex during loading to keep LoadExternal reentrant.
var (
	externalMu sync.RWMutex
	external   []Theme
)

// LoadExternal scans the given directories for *.css files and registers
// each as an external theme. Non-existent directories are silently
// skipped so callers can pass optional paths (like ~/.config/waxon/themes)
// without pre-checking. Duplicate names shadow built-ins.
//
// A CSS file may start with a metadata header block of the form:
//
//	/*! waxon-theme
//	name: acme-corp
//	description: Acme Corporation brand theme
//	*/
//
// If no header is present, name = filename without .css and description
// is empty. The "!" in /*! marks the comment as preserved so CSS minifiers
// won't strip it.
func LoadExternal(dirs []string) error {
	var loaded []Theme
	seen := map[string]bool{}

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat %s: %w", dir, err)
		}
		if !info.IsDir() {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("read %s: %w", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".css") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			theme := parseExternalTheme(e.Name(), string(data))
			if seen[theme.Name] {
				// First hit wins within a single LoadExternal call so the
				// dir-order precedence stays predictable (earlier dirs win).
				continue
			}
			seen[theme.Name] = true
			loaded = append(loaded, theme)
		}
	}

	externalMu.Lock()
	external = loaded
	externalMu.Unlock()
	return nil
}

// parseExternalTheme extracts metadata from a /*! waxon-theme ... */ header
// block if present and returns a Theme. The CSS field contains the full
// file contents so the header (and any preserving comment) stays in place
// for the browser to ignore.
func parseExternalTheme(filename, css string) Theme {
	t := Theme{
		Name: strings.TrimSuffix(filename, ".css"),
		CSS:  css,
	}

	// Look for the metadata header at the top of the file. Whitespace is
	// allowed before the /*! marker.
	trimmed := strings.TrimLeft(css, " \t\r\n")
	const marker = "/*! waxon-theme"
	if !strings.HasPrefix(trimmed, marker) {
		return t
	}
	rest := trimmed[len(marker):]
	header, _, ok := strings.Cut(rest, "*/")
	if !ok {
		return t
	}
	for line := range strings.SplitSeq(header, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		switch key {
		case "name":
			if val != "" {
				t.Name = val
			}
		case "description":
			t.Description = val
		}
	}
	return t
}

// externalThemes returns a snapshot of the external themes registered via
// LoadExternal. Returns an empty slice when nothing has been loaded.
func externalThemes() []Theme {
	externalMu.RLock()
	defer externalMu.RUnlock()
	if len(external) == 0 {
		return nil
	}
	out := make([]Theme, len(external))
	copy(out, external)
	return out
}

// DefaultSearchPaths returns the directories searched by LoadExternal when
// the user doesn't provide an explicit --theme-dir: the deck-relative
// "themes" folder next to the current working directory, and the user-wide
// config dir. Missing directories are silently tolerated by LoadExternal.
func DefaultSearchPaths() []string {
	paths := []string{}
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, "themes"))
	}
	if cfg, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(cfg, "waxon", "themes"))
	}
	return paths
}
