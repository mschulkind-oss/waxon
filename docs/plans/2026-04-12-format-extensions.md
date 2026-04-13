# RFC: Waxon Format Extensions — Colors, Layouts, Slide IDs

**Status:** Draft — awaiting open-question sign-off
**Date:** 2026-04-12
**Source:** `scratch/waxon-feature-requests.md` (first external user, ported 26 decks from a custom DSL)

## Goal

Make `.slides` files genuinely text-first for real-world presentations. Remove the need for raw HTML `<span>` / `<div>` soup for the three most common patterns: colored emphasis, full-line coloring, and side-by-side comparisons. Add slide IDs and clarify pause behavior.

## Non-Goals

- Not adding a full CSS-in-markdown language. Only a curated palette + two layout primitives.
- Not introducing a new template engine or component model. Everything emits plain HTML handled by goldmark or a tiny pre-processor.
- Not redesigning the pause system — only documenting it.
- Not changing existing directive syntax (`<!-- note: -->`, `<!-- ai: -->`, `<!-- comment(@x): -->`, `<!-- pause -->`, `<!-- slide: -->`).
- **Not preserving pure markdown compatibility.** Agents are the primary authors of `.slides` files; direct human authoring in a markdown preview is explicitly not a design constraint. The format must remain text-based, git-trackable, and diffable — custom tokens outside commonmark are fine when they simplify agent operation. (Recorded per user direction, 2026-04-12; see `project_philosophy.md`.)

## Problem Statement

The feature requests come from porting 26 decks off a Go-based DSL onto waxon. The pain points, ordered by incidence:

| Feature | Impact | Decks affected |
|---|---|---|
| Inline color | HIGH | 26/26 (100%) |
| Full-line color | MEDIUM | ~21/26 (80%) |
| Compare layouts | MEDIUM | 8/26 (31%) |
| HTML block reveals | LOW | 8/26 (31%) |
| Slide IDs | LOW | nice-to-have |
| Terminal theme color classes | CLARIFICATION | — |

Without these, every file is 40–60% HTML tags by volume, which defeats the purpose of a text-first format and makes agent collaboration harder (the user's core project philosophy per `project_philosophy.md`).

## Current State

- **Parser** (`internal/format/parser.go`, 248 lines) — splits frontmatter, slides on `---`, variants on `---variant:`, extracts HTML-comment directives. Goldmark renders the body.
- **Render** (`internal/render/render.go`, 2063 lines) — wraps parsed slide HTML, supplies shared CSS, wires WS/keybinds.
- **Themes** (`internal/themes/themes.go`, 543 lines) — 20 themes via CSS variables. **Verified:** no theme currently defines `.red` / `.green` / `.yellow` / `.aqua` / `.dim` utility classes, so feature #6 is "add them."
- **Pauses** — parser replaces `<!-- pause -->` with `<div class="waxon-pause"></div>` sentinels; render JS walks `container.children` and hides everything past the sentinel at the current reveal step. **Verified:** this works for *any* block children, including raw HTML, because it iterates `children` not just markdown nodes.

## Proposed Solution

### Feature 1 + 2 — Inline and line-level color

**Chosen syntax:** `.color{text}` (inline) and `.color text` (line-level).

Why this shape:
- Mirrors the user's old DSL so they can port 26 decks with minimal rewriting.
- Line-level version is a natural extension of inline (`.red{text}` → `.red text at start of line`).
- Single leading dot is rare in real prose — low false-positive risk.

Rejected alternatives:
- `{red|text}` — cleaner but has no natural line-level extension.
- `[text]{.red}` (pandoc) — verbose, backwards-feeling, pandoc-pedigree isn't a goal here.

**Palette (closed set):** `red`, `green`, `yellow`, `blue`, `aqua`, `dim`. These map to WebTUI/ANSI conventions and every terminal theme already has matching hues. Unknown class names are left as plain text (safe fail, no surprise).

**Parser pass:** Run as a pre-goldmark text transform. For each line not inside a fenced code block:
1. If the line starts with `.red ` / `.green ` / etc. (space after palette class), wrap the remainder of the line in `<span class="red">…</span>` (the whole line, trailing content only — no prefix).
2. Replace all `.color{…}` occurrences in the line with `<span class="color">…</span>`. Balanced-brace matching (one level; nested `{}` allowed via counter). Falls back to literal if unbalanced.

**Code-block guard:** Track fence state (` ``` ` or `~~~`). Skip transform inside fences. Inline `` `code` `` is left alone (the transform runs on whole lines, not inside backtick spans — handled via a simple segment walker that skips spans between backticks).

**Why a pre-goldmark pass rather than a goldmark extension:** goldmark extensions require registering AST node types + renderers — much more code. A pre-pass is ~50 LOC, easy to test, easy to remove if we regret it. Goldmark still gets valid markdown with raw HTML, which it passes through unchanged (we already run `WithUnsafe()`).

### Feature 3 — Compare layouts

**Chosen syntax:** `:::` fence blocks, matching the most common markdown extension pattern (admonitions, Pandoc fenced divs).

```markdown
:::compare
::left red
### Us today
- Person + chatbot
- One question at a time

::right green
### Companies lapping us
- Person + agent team
- Brief the team → deliverables back
:::
```

**Transform:** Pre-goldmark. Recognize `:::compare` … `:::` blocks; inside, `::left <color>` / `::right <color>` delimit two panes. Emit:

```html
<div class="waxon-compare">
  <div class="waxon-compare-pane waxon-compare-left red">
    <!-- markdown inside the left pane, rendered by goldmark recursively -->
  </div>
  <div class="waxon-compare-pane waxon-compare-right green">
    <!-- right pane -->
  </div>
</div>
```

CSS in base stylesheet: flex row, equal split, border-color tinted by palette class on the pane. Stacks vertically below ~700px.

**Why fence-based not table-based:** Tables can't carry arbitrary block content (lists, headings). Fences can. User's own rejected-alternative table example was already limited to single-line cells.

**Nesting:** Not supported in v1. A compare pane cannot contain another compare fence. Parser errors if it finds one. (We can relax later.)

### Feature 4 — Slide IDs

**Chosen syntax:** `--- #slide-id` on the slide separator line.

```markdown
--- #the-numbers
# The numbers
...
```

Parser picks up the ID from the separator, stores it on `Slide.ID`, renders `id="the-numbers"` on the slide `<section>`. URL fragment support (`#the-numbers` in the URL) is a follow-up — not required for v1 since the ID is already in the DOM.

No collision with front-matter delimiter `---` because the ID token requires a `#` prefix.

### Feature 5 — HTML block reveals (docs only)

**Finding:** the existing pause system already works for HTML blocks. The pause sentinel div sits between siblings at the DOM level; the JS reveal walks `container.children`, which includes raw HTML divs, sections, anything at block level.

**Action:** document this in README/format reference. Show an example:

```markdown
:::compare
::left red
### A
:::

<!-- pause -->

:::compare
::left green
### B
:::
```

No code change needed. This resolves the user's question ("unclear if waxon's pause system recognizes these").

### Feature 6 — Terminal theme color classes

**Finding:** no theme currently defines `.red` / `.green` / `.yellow` / `.aqua` / `.dim` utility classes.

**Action:** add these as utility classes in the *base CSS* (shared across all themes, in `internal/render/render.go` where the existing base styles live), using CSS custom properties so each theme can override per-theme colors if it wants. Default mapping:

```css
.slide .red    { color: var(--color-red,    #ef4444); }
.slide .green  { color: var(--color-green,  #22c55e); }
.slide .yellow { color: var(--color-yellow, #eab308); }
.slide .blue   { color: var(--color-blue,   #3b82f6); }
.slide .aqua   { color: var(--color-aqua,   #06b6d4); }
.slide .dim    { opacity: 0.6; }
```

**Per-theme tuning (v1):** every built-in theme gets its `--color-*` variables set to match its own palette (dracula's pink-red, monokai's lime-green, nord's frost-blue, etc.). The fallback above is only a safety net for themes that forget to tune. This was confirmed as a v1 requirement, not a follow-up.

## Alternatives Considered

1. **Full markdown-it-style attributes (`{.class #id}`)** — more general, but larger surface and colors are 90% of the use case. Rejected as overkill.
2. **Templated components (`<Compare left="…" right="…" />`)** — breaks text-first philosophy, requires component plumbing. Rejected.
3. **Parser fork (goldmark extension)** — more canonical but ~5× the code for this scope. Pre-pass is reversible; goldmark extension is not. Rejected for v1; can migrate later if pre-pass proves brittle.
4. **Generalized fenced divs (`::: any-name`)** — cleaner but opens the door to every layout being a one-off. Keeping the set closed (`compare` only) forces thoughtful expansion.

## Impact on existing behavior

- Authors who don't use `.red{…}` / `:::compare` / `--- #id` see zero change.
- Existing `<span class="red">…</span>` raw HTML continues to work (goldmark still passes it through with `WithUnsafe()`), so the old decks don't need migration.
- `---` (plain, no ID) continues to work as the slide separator.
- `<!-- pause -->` behavior is unchanged; only documented.

## Risks

| Risk | Mitigation |
|---|---|
| Pre-pass regex eats something it shouldn't | Skip inside fenced code blocks; skip inside inline backticks; require exact palette match. |
| Unbalanced `.color{` corrupts output | Balanced-brace parser; fall back to literal on failure. |
| Compare pane width on small screens | CSS media query stacks to column ≤700px. |
| Slide ID collision with front-matter `---` | ID requires `#` prefix; front-matter is only the first `---`…`---` block. |
| Goldmark interprets transformed `<span>` oddly | Already running `WithUnsafe()` — raw HTML is passthrough. Tested indirectly by existing `<span>` usage in `scratch/`. |

## Open Questions

See `OPEN_QUESTIONS.md` at repo root.

## Success Criteria

1. All 26 of the user's ported decks (hypothetical) can be expressed without any raw `<span>` / `<div>` tags for color and compare layouts.
2. `parser_test.go` has tests for each new syntax form, including edge cases (nested braces, code-block guards, unbalanced, unknown palette).
3. `README.md` and format reference document all new syntax with examples.
4. No existing tests break. `go test ./...` green.
5. Visual QA: render the example deck with every new feature and verify in-browser.
