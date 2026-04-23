package version

import (
	"strings"
	"testing"
)

func reset() {
	Version = ""
	GitCommit = ""
	Date = ""
	BuiltBy = ""
	Dirty = ""
}

func TestStringDev(t *testing.T) {
	reset()
	defer reset()
	// Build-info may be present in the test binary and populate the
	// commit automatically, so we accept any "dev[...]" output as long
	// as the headline version is "dev".
	got := String()
	if !strings.HasPrefix(got, "dev") {
		t.Fatalf("got %q, want prefix %q", got, "dev")
	}
}

func TestStringReleaseClean(t *testing.T) {
	reset()
	defer reset()
	Version = "0.1.0"
	GitCommit = "abc1234"
	Dirty = "false"
	if got := String(); got != "0.1.0 (abc1234)" {
		t.Fatalf("got %q", got)
	}
}

func TestStringDirty(t *testing.T) {
	reset()
	defer reset()
	Version = "0.1.0"
	GitCommit = "abc1234"
	Dirty = "true"
	if got := String(); got != "0.1.0 (abc1234, dirty)" {
		t.Fatalf("got %q", got)
	}
}

func TestFullRelease(t *testing.T) {
	reset()
	defer reset()
	Version = "0.3.2"
	GitCommit = "a15871c"
	Dirty = "false"
	Date = "2026-04-22T21:00:00Z"
	BuiltBy = "goreleaser"
	out := Full()
	for _, want := range []string{
		"waxon 0.3.2",
		"commit:",
		"a15871c",
		"source:",
		"goreleaser",
		"built:",
		"2026-04-22T21:00:00Z",
		"go:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Full() output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "dirty") {
		t.Errorf("clean build should not mention dirty:\n%s", out)
	}
}

func TestFullDirty(t *testing.T) {
	reset()
	defer reset()
	Version = "0.3.2"
	GitCommit = "a15871c"
	Dirty = "true"
	out := Full()
	if !strings.Contains(out, "dirty") {
		t.Errorf("dirty build should mention dirty:\n%s", out)
	}
}

func TestGetSourceRelease(t *testing.T) {
	reset()
	defer reset()
	Version = "0.3.2"
	GitCommit = "a15871c"
	if got := Get().Source; got != "release" {
		t.Errorf("Source = %q, want release", got)
	}
}

func TestGetSourceBuiltByWins(t *testing.T) {
	reset()
	defer reset()
	Version = "0.3.2"
	BuiltBy = "homebrew"
	if got := Get().Source; got != "homebrew" {
		t.Errorf("Source = %q, want homebrew", got)
	}
}

func TestShortSHATruncates(t *testing.T) {
	if got := shortSHA("a15871c054dc94eb22eb166b0ac9e07d27d40c21"); got != "a15871c" {
		t.Errorf("shortSHA = %q", got)
	}
	if got := shortSHA("abc"); got != "abc" {
		t.Errorf("shortSHA short input = %q", got)
	}
}
