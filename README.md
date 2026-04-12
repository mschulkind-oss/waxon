# waxon

A slide deck toolkit built for the mind meld between human and agent.

You write slides in a plain-text format. Your agent writes slides in the same format. You review in the browser. The agent reads your comments and proposes variants. You pick the best one and export to PDF. The entire loop runs through a single CLI that any agent harness can drive, and a human-readable `.slides` format that both sides can read, write, and reason about without special tooling.

```
waxon serve deck.slides      # live preview at localhost:8080
waxon export deck.slides     # PDF export
waxon new my-talk             # scaffold a new deck
```

Single binary. No runtime dependencies. Install and go.

## Install

```bash
# npm/npx — run without installing
npx @waxon/waxon serve deck.slides

# Python/uvx — run without installing
uvx waxon serve deck.slides

# Homebrew
brew install mschulkind-oss/tap/waxon

# Go
go install github.com/mschulkind-oss/waxon/cmd/waxon@latest

# Binary
# Download from https://github.com/mschulkind-oss/waxon/releases
```

## Quick Start

**1. Create a deck**

```bash
waxon new my-talk
```

Creates `my-talk.slides` with a starter template.

**2. Edit it** — you or your agent, same format either way

```
---
title: "My Talk"
author: "Your Name"
theme: terminal
aspect: "16:9"
---

# Hello, World

Welcome to my presentation.

<!-- note: Open with a question to the audience -->
<!-- ai: This is the hook slide — keep it punchy -->

---

# The Problem

- Users are frustrated
- Agents can't help
- The format is locked in a binary blob

<!-- comment(@agent): Consider adding a data point here -->
```

**3. Present it**

```bash
waxon serve my-talk.slides
```

Live preview at `http://localhost:8080`. Edit the file — the browser updates instantly via WebSocket.

**4. Export**

```bash
waxon export my-talk.slides -o my-talk.pdf
```

---

## Why waxon?

Existing slide tools assume a single author using a GUI. waxon assumes a different workflow:

1. **An agent drafts a deck** from a prompt, research, or meeting notes
2. **A human reviews it** in the browser with live reload
3. **Both iterate** — the human leaves comments, the agent proposes variants, the human picks
4. **The result ships** as PDF, or presented live from the CLI

This works because:

- **The format is text.** Any agent can read and write `.slides` files. No API, no SDK, no plugins — just text.
- **The CLI is the interface.** `waxon serve`, `waxon export`, `waxon agent-context` — composable commands any harness can call.
- **Comments, variants, and AI notes are first-class.** They live in the file, not in a sidebar or database. They survive version control, diffs, and agent handoffs.
- **Speaker notes and AI notes are separate.** Humans get `<!-- note: -->` for what to say. Agents get `<!-- ai: -->` for how to help. Neither pollutes the other.

---

## The `.slides` Format

A `.slides` file is a plain-text document with YAML frontmatter and Markdown content. Slides are separated by `---` on its own line.

### Frontmatter

```yaml
---
title: "Presentation Title"
author: "Author Name"
date: "2026-04-11"
theme: terminal
aspect: "16:9"           # "16:9" (default), "4:3", or "16:10"
footer: "Company Name"   # optional persistent footer
---
```

| Field    | Required | Default    | Description                             |
|----------|----------|------------|-----------------------------------------|
| `title`  | yes      | —          | Deck title, shown on the title slide    |
| `author` | no       | —          | Author name                             |
| `date`   | no       | —          | Presentation date                       |
| `theme`  | no       | `default`  | Built-in theme name or path to CSS file |
| `aspect` | no       | `16:9`     | Slide aspect ratio                      |
| `footer` | no       | —          | Text shown at the bottom of every slide |

### Slides

Separate slides with a line containing only `---`:

```markdown
# First Slide

Content here.

---

# Second Slide

More content.
```

### Markdown Support

Each slide body supports standard Markdown:

- **Headings** (`#`, `##`, `###`) — slide titles and subtitles
- **Lists** — ordered and unordered, with nesting
- **Code blocks** — fenced with syntax highlighting (` ```go `)
- **Images** — `![alt](path)` with automatic scaling to fit the slide
- **Bold**, *italic*, `inline code`, [links](https://example.com)
- **Tables** — GitHub-flavored Markdown tables
- **Math** — LaTeX math expressions with `$inline$` and `$$block$$`

### Speaker Notes

Notes for the human presenter. Visible only in presenter mode:

```markdown
# My Slide

Slide content here.

<!-- note: This is a speaker note. Only visible in presenter view. -->
<!-- note: You can have multiple note blocks per slide. -->
```

### AI Notes

Context for agents working on the deck. Never rendered, never shown to audiences:

```markdown
# Architecture Overview

The system has three layers...

<!-- ai: This slide should use the architecture diagram from docs/arch.png -->
<!-- ai: Keep this high-level; detailed breakdown is on the next slide -->
```

AI notes are the agent's scratchpad — rationale, instructions, constraints, context that helps the next agent (or the same agent in a later session) understand *why* a slide exists and *how* to improve it.

### Comments

Inline review comments with author attribution. The collaboration channel:

```markdown
# Revenue Projections

Growth rate is 15% YoY.

<!-- comment(@alice): Should we use the updated Q3 numbers? -->
<!-- comment(@bob): Yes, and add a footnote about the methodology change -->
<!-- comment(@agent): I can update this with Q3 data — want me to proceed? -->
```

Comments stay in the file. They're visible in the presenter UI, manageable via `waxon comment`, and parseable by any agent reading the file.

### Variants

Alternate versions of a slide, linked to the same position. The mechanism for proposing and choosing between options:

```markdown
# Welcome
This is the standard opening.

---variant: welcome-bold
# Welcome to the Future
A more dramatic opening for keynotes.

---variant: welcome-minimal
# Welcome
Just the title, no subtitle.

---

# Second Slide
...
```

Variants are how agents propose alternatives without overwriting your work. The presenter UI lets you toggle between variants live. When exporting, choose which to include:

```bash
waxon export deck.slides --variant welcome-bold
```

### Incremental Reveal

Use `<!-- pause -->` to reveal content step-by-step:

```markdown
# Three Things

- First point
<!-- pause -->
- Second point (appears on next click)
<!-- pause -->
- Third point (appears on next click)
```

### Slide-Level Overrides

Override theme settings for a single slide:

```markdown
<!-- slide: bg=#1a1a2e, class=centered -->
# Special Slide

This slide has a custom background and centered layout.
```

---

## Themes

### Built-in Themes

| Theme        | Description                                                   |
|--------------|---------------------------------------------------------------|
| `default`    | Clean, minimal dark theme with good contrast                  |
| `light`      | Bright background, dark text, professional look               |
| `corporate`  | Conservative palette, suitable for business presentations     |
| `minimal`    | Maximum whitespace, typography-focused, no distractions       |
| `vibrant`    | Bold colors and gradients for creative talks                  |
| `terminal`   | Authentic TUI aesthetic powered by [WebTUI](https://webtui.ironclad.sh/) |

### The Terminal Theme

The `terminal` theme renders your slides as if they were running inside a terminal emulator. Built on the [WebTUI](https://webtui.ironclad.sh/) CSS library, it provides an authentic TUI experience in the browser:

- Monospace typography throughout
- Box-drawing characters for borders and separators
- Terminal-style color palettes with sub-themes: **Nord**, **Catppuccin**, **Gruvbox**, **Everforest**, **Vitesse**
- ASCII-art title treatment
- Optional retro scanline and CRT effects
- Cursor-blink animations
- Code blocks that look native — because the whole slide *is* a terminal

```yaml
---
theme: terminal
terminal-variant: catppuccin   # nord, gruvbox, everforest, vitesse
terminal-effects: true         # scanlines, CRT glow (default: false)
---

# $ whoami

> Building the future, one commit at a time.
```

The terminal theme is especially well-suited for developer conferences, internal engineering talks, CLI demos, and anyone who wants their slides to look like they were made by someone who lives in the terminal.

### Selecting a Theme

Set in frontmatter:

```yaml
---
theme: vibrant
---
```

Override at serve time:

```bash
waxon serve deck.slides --theme terminal
```

### Custom CSS

Point `theme` to a local CSS file:

```yaml
---
theme: ./my-theme.css
---
```

Or extend a built-in theme:

```css
/* my-theme.css */
@import "builtin:default";

:root {
  --slide-bg: #0d1117;
  --slide-fg: #c9d1d9;
  --accent: #58a6ff;
  --font-heading: "JetBrains Mono", monospace;
}
```

### List Themes

```bash
waxon themes
```

```
THEME          DESCRIPTION
default        Clean, minimal dark theme
light          Bright background, professional look
corporate      Conservative palette for business
minimal        Typography-focused, maximum whitespace
vibrant        Bold colors and gradients
terminal       Authentic TUI aesthetic (WebTUI)
```

---

## CLI Reference

### `waxon serve <file>`

Start a live-preview server with hot reload.

```bash
waxon serve deck.slides [flags]
```

| Flag         | Default      | Description                          |
|--------------|--------------|--------------------------------------|
| `--port`     | `8080`       | HTTP server port                     |
| `--bind`     | `0.0.0.0`    | Bind address                         |
| `--theme`    | (frontmatter)| Override the theme                   |
| `--no-open`  | `false`      | Don't auto-open browser              |
| `--presenter`| `false`      | Start in presenter mode              |

The server watches the `.slides` file for changes and pushes updates via WebSocket. No manual refresh needed.

**Keyboard controls:**

| Key             | Action                    |
|-----------------|---------------------------|
| `Right` / `Space` / `Enter` | Next slide / reveal |
| `Left` / `Backspace`        | Previous slide      |
| `f`                         | Toggle fullscreen   |
| `p`                         | Toggle presenter mode |
| `t`                         | Cycle theme         |
| `o`                         | Slide overview grid  |
| `Escape`                    | Exit fullscreen / overview |
| `1`..`9`                    | Jump to slide N     |
| `g`                         | Go to slide (enter number) |

### `waxon export <file>`

Export to PDF.

```bash
waxon export deck.slides [flags]
```

| Flag        | Default        | Description                            |
|-------------|----------------|----------------------------------------|
| `-o`        | `<name>.pdf`   | Output file path                       |
| `--theme`   | (frontmatter)  | Override the theme                     |
| `--variant` | (all default)  | Choose specific variants by name       |
| `--pages`   | all            | Page range (e.g., `1-5`, `3,7,9`)      |

Uses headless Chromium for pixel-perfect rendering.

### `waxon new <name>`

Create a new `.slides` file from a starter template.

```bash
waxon new my-talk [flags]
```

| Flag      | Default   | Description                      |
|-----------|-----------|----------------------------------|
| `--theme` | `default` | Set the initial theme            |
| `--ratio` | `16:9`    | Set the aspect ratio             |

### `waxon themes`

List available themes.

```bash
waxon themes [--json]
```

### `waxon comment <file>`

View and manage comments.

```bash
waxon comment deck.slides [flags]
```

| Flag        | Default | Description                        |
|-------------|---------|------------------------------------|
| `--add`     | —       | Add a comment to slide N           |
| `--slide`   | all     | Filter by slide number             |
| `--author`  | all     | Filter by author                   |
| `--resolve` | —       | Mark a comment as resolved         |
| `--json`    | `false` | Output as JSON                     |

### `waxon agent-context <file>`

Emit structured context for agents.

```bash
waxon agent-context deck.slides
```

Outputs JSON with the full parsed structure: slides, notes, AI notes, comments, variants, and metadata. Designed for piping into agent workflows:

```bash
waxon agent-context deck.slides | claude "improve the narrative flow"
```

---

## Presenter Mode

Press `p` during a presentation or start with `--presenter`:

- **Current slide** — what the audience sees
- **Next slide** — preview of what's coming
- **Speaker notes** — your `<!-- note: -->` content
- **Timer** — elapsed time and optional countdown
- **Slide count** — current position (e.g., "7 / 23")

Presenter mode opens in a separate window. The audience view stays clean.

---

## Agent Collaboration

The `.slides` format is designed so that any agent — Claude Code, Copilot, Gemini, or a custom harness — can participate in the slide-making process as a first-class collaborator.

### The Loop

```
Human: "Make me a deck about X"
  Agent: creates deck.slides
Human: waxon serve deck.slides → reviews in browser
Human: adds <!-- comment(@me): this slide needs data -->
  Agent: reads file, updates slide, adds <!-- ai: sourced from Q3 report -->
  Agent: creates ---variant: data-heavy with an alternative
Human: picks variant, exports
  Agent: cleans up, archives
```

### For Agent Developers

**Read a deck programmatically:**

```bash
waxon agent-context deck.slides
```

Returns JSON:

```json
{
  "meta": {"title": "...", "theme": "...", "aspect": "16:9"},
  "slides": [
    {
      "index": 0,
      "content": "# Hello\n\nWelcome.",
      "notes": ["Open with a question"],
      "ai_notes": ["This is the hook slide"],
      "comments": [{"author": "alice", "text": "Add a graphic?"}],
      "variants": ["welcome-bold", "welcome-minimal"]
    }
  ]
}
```

**Create a deck:** Just write a `.slides` file. The format is trivially producible — Markdown with HTML comments.

**Add a variant:** Append `---variant: name` followed by alternate content. The human picks.

**Leave a comment:** Add `<!-- comment(@agent): ... -->` to any slide.

**Add context for the next agent:** Add `<!-- ai: ... -->` with rationale, constraints, or instructions.

---

## Examples

### Minimal deck

```
---
title: "Hello"
---

# Hello, World
```

### Conference talk with terminal theme

```
---
title: "Building CLIs in Go"
author: "Jane Developer"
date: "2026-06-15"
theme: terminal
terminal-variant: nord
aspect: "16:9"
footer: "GopherCon 2026"
---

# Building CLIs in Go

Jane Developer
GopherCon 2026

<!-- note: Wait for mic check before starting -->
<!-- ai: Audience is experienced Go developers, keep it advanced -->

---

## Why Go for CLIs?

- Single binary distribution
- Fast startup time
- Cross-compilation built in
- Rich standard library

<!-- pause -->

**It just works.**

<!-- note: Emphasize the "just works" — this is the thesis -->

---

## Demo Time

```go
func main() {
    fmt.Println("Hello, GopherCon!")
}
```

<!-- ai: Run the demo from ~/talks/demo/ -->
<!-- note: Switch to terminal for live demo -->

---

# Thank You

github.com/jane/cli-tools

<!-- comment(@reviewer): Add QR code? -->
```

### Collaborative review deck with variants

```
---
title: "Q4 Strategy"
author: "Strategy Team"
theme: corporate
---

# Q4 Strategy Overview

Three key initiatives for the quarter.

<!-- ai: This deck was generated from the Q4 planning doc. Source: gdocs/q4-plan -->

---

# Initiative 1: Platform Expansion

Expand into two new markets.

---variant: initiative-1-conservative
# Initiative 1: Platform Stabilization

Focus on existing markets, improve retention.

<!-- comment(@cto): Let's discuss both approaches Tuesday -->
<!-- ai: CTO prefers conservative framing for board decks -->

---

# Initiative 2: Developer Experience

Reduce onboarding time from 2 weeks to 2 days.

<!-- ai: Include metrics from the developer survey in appendix -->
<!-- comment(@agent): I can pull the survey data and add a chart — want me to? -->
```

---

## Architecture

Single Go binary with all assets embedded via `go:embed`.

```
cmd/waxon/            CLI entry point (cobra)
internal/
  format/            .slides parser and writer
  render/            HTML + CSS rendering engine
  server/            HTTP server + WebSocket hot reload
  pdf/               Headless Chromium PDF export
  themes/            Theme registry and loader
  comments/          Comment system
themes/              Built-in theme CSS files (embedded)
static/              JS, base CSS, fonts (embedded)
```

### Design Decisions

- **Single binary** — no runtime dependencies, no Node.js, no Python
- **Embedded assets** — themes, CSS, JS compiled into the binary via `go:embed`
- **Headless Chromium for PDF** — pixel-perfect rendering matching the browser preview
- **WebSocket hot reload** — sub-100ms from file save to browser update
- **Text-first format** — no databases, no binary formats, git-friendly, agent-friendly
- **HTML comments for metadata** — notes, AI notes, and comments use `<!-- -->` so they're invisible in any Markdown renderer but parseable by waxon

---

## Development

```bash
git clone https://github.com/mschulkind-oss/waxon.git
cd waxon

just run -- serve example.slides   # run from source
just build                          # compile binary
just test                           # run tests
just check                          # lint + format + test
```

---

## License

MIT
