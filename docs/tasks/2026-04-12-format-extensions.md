# Tasks — Format Extensions

Tracks execution of `docs/plans/2026-04-12-format-extensions.md`. Each item is sized to <1 hour.

## Parser

- [ ] **P1.** Add `preprocessLine` pass that handles inline `.color{text}` across the full palette (red/green/yellow/blue/aqua/dim). Balanced-brace matching; unbalanced falls back to literal. Skip inside fenced code blocks and inside inline backtick spans.
- [ ] **P2.** Add line-level `.color text` handling (palette class at start of line followed by space → wrap the rest of the line). Must not fire inside code blocks.
- [ ] **P3.** Add `:::compare` / `::left <color>` / `::right <color>` / `:::` block parser. Emit the div layout documented in the RFC. Error loudly on nested `:::compare`.
- [ ] **P4.** Add `--- #slide-id` separator parser. Capture ID into `Slide.ID`. Preserve existing bare-`---` behavior.
- [ ] **P5.** Wire the pre-pass into `Parse` so it runs before slide splitting (so slide-id separators split on the right lines) — but color and compare passes run *after* slide splitting, per slide, so they don't cross slide boundaries.
- [ ] **P6.** Ensure variants pick up the same pre-pass (`buildSlide`/`splitVariants`).

## Types

- [ ] **T1.** Add `ID string` to `Slide` struct in `internal/format/types.go`.

## Render

- [ ] **R1.** Emit `id="…"` on the slide `<section>` when `Slide.ID` is set.
- [ ] **R2.** Add palette utility classes (`.red .green .yellow .blue .aqua .dim`) to the base CSS in `render.go`, using `--color-*` custom properties with a sensible fallback.
- [ ] **R3.** Add `.waxon-compare` + `.waxon-compare-pane` base CSS (flex row, responsive stack, border-color honors palette class on the pane).

## Themes

- [ ] **Th1.** Add `--color-red/green/yellow/blue/aqua/dim` to every theme's `:root` block, tuned to each palette. 20 themes × 6 variables.

## Tests

- [ ] **Te1.** `parser_test.go`: inline color — simple, adjacent, nested-braces, unbalanced, unknown palette (leave as literal), inside fenced code (no-op), inside backtick code span (no-op).
- [ ] **Te2.** `parser_test.go`: line-level color — start of line, not mid-line, not inside fence.
- [ ] **Te3.** `parser_test.go`: `:::compare` — happy path, missing `::right`, nested error, palette on panes.
- [ ] **Te4.** `parser_test.go`: slide ID — `--- #id`, plain `---`, ID with hyphens and digits.
- [ ] **Te5.** `render_test.go`: slide `<section id=…>` emission, base CSS contains palette classes, compare CSS present.
- [ ] **Te6.** `themes_test.go`: each theme defines all six color variables.

## Docs

- [ ] **D1.** README section: new format features with examples (inline color, line color, compare, slide IDs, pause + HTML).
- [ ] **D2.** `example.slides` or a new `docs/format-reference.slides`: one slide per feature, serves as a runnable demo.
- [ ] **D3.** Remove `scratch/waxon-feature-requests.md` (or move to `docs/research/` as a source artifact).

## QA

- [ ] **Q1.** `go build ./...`, `go vet ./...`, `gofmt -l cmd internal`, `go test ./...` — all green.
- [ ] **Q2.** Chrome-devtools MCP: serve directory, load the example/reference deck, visually verify each feature in at least two themes (default + terminal).
- [ ] **Q3.** Regression spot-check: load cats.slides and dogs.slides, confirm nothing visually broke.
