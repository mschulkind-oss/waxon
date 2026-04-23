// Package version reports how this binary was built. It merges two
// sources:
//
//  1. Build-time ldflags set by the release pipeline (goreleaser/Homebrew)
//  2. runtime/debug.BuildInfo, which the Go toolchain populates for
//     every build — `go install` records the module version and tag,
//     and a plain `go build` in a git checkout records the commit and
//     dirty flag automatically.
//
// Combined, this gives an accurate picture regardless of install
// method: release artifacts, `go install`, or `go build` from a local
// checkout all produce a complete report.
package version

import (
	"fmt"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
)

// pseudoVersionRe matches Go's automatically-generated pseudo-versions
// like `v0.1.2-0.20260415143312-a15871c054dc` (optionally suffixed with
// `+dirty`). The Go toolchain emits these in debug.BuildInfo for builds
// that aren't from a clean tagged module, so treating them as a release
// version would be misleading.
var pseudoVersionRe = regexp.MustCompile(`[-.]\d{14}-[0-9a-f]{12}`)

// These are set via ldflags on release builds (see .goreleaser.yaml).
// Plain `go build` leaves them empty and we fall back to BuildInfo.
var (
	Version   = "" // semver without leading "v", e.g. "0.3.2"
	GitCommit = "" // short sha (7 chars)
	Date      = "" // build date, RFC3339
	BuiltBy   = "" // e.g. "goreleaser"
	Dirty     = "" // "true" / "false" — only reliable on release builds
)

// Info is a structured snapshot of the build metadata. All string
// fields are either populated or empty (never "unknown") so callers
// can format their own fallbacks.
type Info struct {
	Version    string
	Commit     string // short
	CommitFull string
	CommitDate string
	Dirty      bool
	Date       string
	BuiltBy    string
	GoVersion  string
	OS         string
	Arch       string
	Module     string
	Source     string // "release", "go install vX.Y.Z", "source (go build)", or "unknown"
}

// Get assembles an Info by preferring ldflags, falling back to
// debug.BuildInfo for anything not set at link time.
func Get() Info {
	info := Info{
		Version:   Version,
		Commit:    GitCommit,
		Date:      Date,
		BuiltBy:   BuiltBy,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
	if Dirty == "true" {
		info.Dirty = true
	}

	ldflagsReleased := Version != "" && Version != "dev"
	installedByGo := false

	if bi, ok := debug.ReadBuildInfo(); ok {
		info.Module = bi.Main.Path

		// If ldflags didn't set the version, a `go install example.com/x@v1.2.3`
		// leaves the tag in bi.Main.Version. A plain `go build` in a git
		// checkout leaves a pseudo-version like v0.1.2-0.<ts>-<sha>[+dirty] —
		// that's a source build, not an install, so we just report "dev".
		if info.Version == "" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			v := bi.Main.Version
			if pseudoVersionRe.MatchString(v) {
				if strings.HasSuffix(v, "+dirty") {
					info.Dirty = true
				}
				// Leave Version empty; we default to "dev" below.
			} else {
				info.Version = strings.TrimPrefix(v, "v")
				installedByGo = true
			}
		}

		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				info.CommitFull = s.Value
				if info.Commit == "" {
					info.Commit = shortSHA(s.Value)
				}
			case "vcs.time":
				info.CommitDate = s.Value
			case "vcs.modified":
				// Only trust BuildInfo's dirty flag when ldflags didn't
				// set one — release builds hardcode dirty=false.
				if Dirty == "" && s.Value == "true" {
					info.Dirty = true
				}
			}
		}
	}

	if info.Version == "" {
		info.Version = "dev"
	}

	switch {
	case info.BuiltBy != "":
		info.Source = info.BuiltBy
	case ldflagsReleased:
		info.Source = "release"
	case installedByGo:
		info.Source = "go install"
	case info.Commit != "":
		info.Source = "source (go build)"
	default:
		info.Source = "unknown"
	}

	return info
}

func shortSHA(s string) string {
	if len(s) >= 7 {
		return s[:7]
	}
	return s
}

// String returns a one-line version summary. Kept for callers that
// need a compact form (logs, user-agent strings).
func String() string {
	i := Get()
	if i.Commit == "" {
		return i.Version
	}
	if i.Dirty {
		return fmt.Sprintf("%s (%s, dirty)", i.Version, i.Commit)
	}
	return fmt.Sprintf("%s (%s)", i.Version, i.Commit)
}

// Full returns the multi-line report shown by `waxon --version`.
// It lists every available field so the user can see exactly how
// the binary was built.
func Full() string {
	i := Get()
	var b strings.Builder

	headline := "waxon " + i.Version
	if i.Dirty {
		headline += " (dirty)"
	}
	b.WriteString(headline)
	b.WriteByte('\n')

	writeField := func(label, value string) {
		if value == "" {
			return
		}
		fmt.Fprintf(&b, "  %-9s %s\n", label+":", value)
	}

	commitLine := i.Commit
	if commitLine == "" {
		commitLine = "unknown"
	} else if i.CommitFull != "" && i.CommitFull != i.Commit {
		commitLine = i.Commit + " (" + i.CommitFull + ")"
	}
	if i.Dirty {
		commitLine += " — working tree dirty"
	}
	writeField("commit", commitLine)
	writeField("authored", i.CommitDate)
	writeField("built", i.Date)
	writeField("source", i.Source)
	writeField("go", fmt.Sprintf("%s %s/%s", i.GoVersion, i.OS, i.Arch))
	writeField("module", i.Module)

	return strings.TrimRight(b.String(), "\n")
}
