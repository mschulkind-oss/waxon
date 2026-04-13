# Waxon Feature Requests

> From converting 26 slide decks from a custom DSL to waxon format. These are features the old system had that waxon doesn't natively support, requiring HTML workarounds.

## 1. Inline Color/Styling Syntax (HIGH — affects 100% of decks)

**The problem:** Every deck uses colored text extensively for emphasis, hierarchy, and visual storytelling. Currently requires raw HTML `<span>` tags which are verbose and hard to read/write.

**Current workaround:**
```markdown
# I eliminated <span class="red">18 weeks</span> of work
<span class="dim">One person. No team. No budget.</span>
```

**Desired syntax** (something like):
```markdown
# I eliminated {red|18 weeks} of work
{dim|One person. No team. No budget.}
```

Or even simpler, borrowing from the old DSL:
```markdown
# I eliminated .red{18 weeks} of work
.dim{One person. No team. No budget.}
```

**Colors needed:** red, green, yellow, aqua/cyan, dim (reduced opacity). These map naturally to terminal theme palettes.

**Why it matters:** Color is the primary visual hierarchy tool in terminal-themed slides. Without native syntax, every deck is 40-60% `<span>` tags by volume, which makes the files hard to read and painful for agents to reason about. The whole point of a text-first format is readability — HTML spans undermine that.

**Suggestion:** Since the terminal theme is built on WebTUI which already has these CSS classes, the syntax just needs to emit `<span class="color">` in the HTML output. The parser addition is small; the visual impact is massive.

---

## 2. Full-Line Color (MEDIUM — affects ~80% of decks)

**The problem:** Many slides have entire lines styled as a color, not just inline fragments. The old DSL had `.color Text goes here` (no braces) for this.

**Current workaround:**
```markdown
<span class="dim">— Matt</span>
```

**Desired syntax:**
```markdown
.dim — Matt
```

This is the "line-level" version of feature #1. If inline color syntax is added, line-level could be syntactic sugar — `.color text` is equivalent to `.color{text}` when the color is at the start of a line.

---

## 3. Side-by-Side Compare Layouts (MEDIUM — affects 31% of decks)

**The problem:** 8 of 26 decks use side-by-side comparison boxes (e.g., "Us Today" vs "Companies Lapping Us"). Currently requires raw HTML with `<div>` layout.

**Current workaround:**
```html
<div class="compare-row">
  <div class="compare-box" style="border-color: var(--gb-red);">
    <h3 class="red">Us today</h3>
    <p class="dim">Person + chatbot</p>
  </div>
  <div class="compare-box" style="border-color: var(--gb-green);">
    <h3 class="green">Companies lapping us</h3>
    <p class="dim">Person + agent team</p>
  </div>
</div>
```

**Desired syntax** (something like a two-column Markdown extension):
```markdown
<!-- layout: compare -->
| Us today {red} | Companies lapping us {green} |
| Person + chatbot | Person + agent team |
| One question at a time | Brief the team → deliverables back |
```

Or a simpler fence-based approach:
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

**Why it matters:** Compare layouts are the most common complex layout pattern in presentation decks. Having to drop to raw HTML breaks the text-first promise and makes agent collaboration much harder.

---

## 4. Slide IDs / Named Slides (LOW — nice to have)

**The problem:** Old DSL had `--- slide-id | layout` which gave each slide a named anchor. Useful for linking, for agents referencing specific slides, and for variant targeting.

**Current:** Slides are only identified by index position.

**Suggestion:** Allow an optional ID in the slide separator or via comment:
```markdown
---
<!-- slide: id=the-numbers, class=centered -->
```

Or:
```markdown
--- #the-numbers

<!-- slide: class=centered -->
```

---

## 5. Incremental Reveal of HTML Blocks (LOW — affects 31% of decks)

**The problem:** Some HTML blocks (compare boxes, data cards) need to appear on click, not all at once. The old DSL used a `.step` CSS class on the div. Waxon's `<!-- pause -->` works for Markdown content but it's unclear how it interacts with raw HTML blocks.

**Current workaround:** Adding `.step` class to divs and hoping the theme handles it. Unclear if waxon's pause system recognizes these.

**Suggestion:** Document how `<!-- pause -->` interacts with HTML blocks, or support a `data-pause` attribute on HTML elements that the reveal system picks up.

---

## 6. Terminal Theme Color Variables (CLARIFICATION NEEDED)

**The question:** The terminal theme is built on WebTUI. Do the CSS classes `red`, `green`, `yellow`, `aqua`, `dim` work out of the box when used inside `<span class="red">` etc.? The old system defined these via WebTUI's CSS variables (`--gb-red`, etc.).

If the terminal theme already supports these classes, then feature #1 is purely a syntax convenience request. If not, the terminal theme needs to expose standard color utility classes.

---

## Summary Priority

| # | Feature | Impact | Decks Affected | Difficulty Estimate |
|---|---------|--------|---------------|-------------------|
| 1 | Inline color syntax | HIGH | 26/26 (100%) | Parser addition, small |
| 2 | Full-line color | MEDIUM | ~21/26 (80%) | Syntactic sugar on #1 |
| 3 | Compare layouts | MEDIUM | 8/26 (31%) | New layout component |
| 4 | Slide IDs | LOW | All (nice-to-have) | Small parser change |
| 5 | HTML block reveals | LOW | 8/26 (31%) | Integration clarification |
| 6 | Theme color classes | CLARIFICATION | 26/26 (100%) | May already work |

Features 1-2 are the highest leverage — they affect every single file and are the difference between readable text files and HTML soup. Feature 3 is the most impactful layout addition. Features 4-6 are nice-to-haves.
