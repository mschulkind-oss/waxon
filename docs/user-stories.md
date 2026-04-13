# User Stories: waxon

This document follows real people — humans and agents — through real
workflows with waxon. It's not a spec. It's a way to discover what
already works, what's still missing, and what's quietly broken by
telling the story of someone using the thing.

The lens: **the mind meld.** Every story is a check on whether a human
and an agent can collaborate on a deck as fluently as two humans on a
Google Doc.

---

## 1. Maya — Solo Founder Drafting an Investor Deck

**Context:** Maya is preparing a 12-slide pitch deck for a seed round.
She's a strong writer and a mediocre designer. She has Claude Code open
in one terminal and her notes in another. She has never used waxon
before.

**First 10 minutes:**

1. She runs `brew install mschulkind-oss/tap/waxon`. 11 seconds. Done.

2. She runs `waxon new pitch`.

   ```
   Created pitch.slides
   Edit it: pitch.slides
   Preview it: waxon serve pitch.slides
   ```

3. She opens `pitch.slides`. It's a starter template with frontmatter,
   one title slide, one bullet slide, and one closing slide. There's a
   `<!-- ai: -->` comment at the top explaining what each directive
   does.

4. She runs `waxon serve pitch.slides`. A browser opens to
   `localhost:8080`. She sees the title slide.

5. She drags `pitch.slides` into Claude Code and says: "I'm raising
   $2M for a developer-tools SaaS. Here are my notes." She pastes a
   wall of text from a Google Doc.

6. Claude writes 12 slides into `pitch.slides`. The browser refreshes
   every time it saves. Maya watches the deck materialize over about
   90 seconds.

   **Gap:** There's no visual indication in the browser of *which slide
   the agent just edited*. Maya has to scroll back through every slide
   to find the changes. A "recently edited" highlight or a sidebar of
   "agent activity in the last 60 seconds" would close the loop.

7. Maya scrolls through. Slide 5 has a placeholder: "**[INSERT REVENUE
   PROJECTIONS HERE]**". She edits the file directly, adds the numbers,
   saves. The browser refreshes.

**What would trip her up:**
- The starter template doesn't show off variants or comments. She
  doesn't discover them until she reads the README.
- She doesn't know that Claude can read her file via `waxon
  agent-context`. She just pastes the whole file in chat.

**What makes this work:**
- The file *is* the source of truth. Whatever Claude does is visible
  in the browser within a second.
- She never had to leave her terminal. No GUI, no login, no project.

---

## 2. Atlas — Agent Building a Deck from Research

**Context:** Atlas is a Claude Code instance invoked from a CI job. The
job's input is a markdown file with research notes. The output should
be a `.slides` file ready for a human to review the next morning.

**What happens:**

1. Atlas reads the research file from disk.

2. Atlas runs `waxon new draft --theme corporate`. Reads the resulting
   `draft.slides` to understand the starter structure.

3. Atlas writes 14 slides into `draft.slides` directly with its file
   editor. Each slide gets a `<!-- ai: -->` block explaining its source
   and intent:

   ```
   <!-- ai: Slide 4 distills the "Risks" section of the research doc
   into three bullets. The full risk matrix is too dense for slides;
   Maya should add it as an appendix if reviewers ask. -->
   ```

4. Atlas runs `waxon agent-context draft.slides` to verify the file
   parses cleanly:

   ```json
   {
     "meta": {"title": "Q2 Findings", "theme": "corporate", ...},
     "slides": [{"index": 0, "content": "...", "ai_notes": [...]}, ...]
   }
   ```

   Exit code 0. Atlas knows the file is valid.

   **Gap:** There's no `waxon lint` or `waxon validate` command. Atlas
   has to use `agent-context` as a proxy validator, which works but
   isn't its purpose. A dedicated lint that returns structured errors
   ("slide 7 has unclosed code fence at line 142") would be cleaner.

5. Atlas adds two variants to slide 6 — the slide with the most
   contested claim — so Maya can pick a framing in the morning.

6. Atlas adds a top-of-deck comment:
   `<!-- comment(@atlas): drafted from research-2026-04-12.md. Slide 6
   has two variants — "cautious" and "bold" — please pick one. -->`

7. Atlas exits. The deck is on disk.

**What makes this work:**
- Atlas never needed an API key, a plugin, or a network call. It used
  a file editor and the CLI, like a human would.
- The `<!-- ai: -->` blocks mean the *next* agent that touches this
  deck (or Atlas itself in a later session) won't have to re-derive
  why each slide exists.

---

## 3. Lisa — Product Manager Who Doesn't Read Code

**Context:** Lisa is reviewing a deck Maya and Atlas built. She doesn't
use git, doesn't know what YAML is, and finds command-line tools
intimidating. Maya sent her a link.

**What happens:**

1. Maya runs `waxon serve pitch.slides --bind 0.0.0.0` on her laptop
   and Tailscales the URL to Lisa.

2. Lisa opens the link. She sees the deck. She uses arrow keys to
   advance — she figured that out without asking.

3. She gets to slide 7. She thinks the chart is wrong. She wants to
   leave a comment.

   **Gap:** There is no way for Lisa to leave a comment from the
   browser. She has to either (a) ask Maya to add it, (b) learn the
   `<!-- comment(@lisa): -->` syntax and ask Maya to paste it in, or
   (c) send a Slack message that lives outside the deck.

   The mind-meld test fails here. The collaboration channel is the
   file, but the file is reachable only through a text editor.

4. Lisa Slacks Maya: "slide 7, the chart is from Q2 not Q3."

5. Maya pastes `<!-- comment(@lisa): chart is from Q2 not Q3 -->` into
   the file manually.

**What would trip her up:**
- Everything that requires touching the file. The browser is her
  entire interface and it's read-only.

**What this story reveals:**
- waxon needs a "comment from the browser" affordance. Could be as
  simple as a small text box at the bottom of the presenter view that
  POSTs to the dev server, which appends a `<!-- comment(@author): -->`
  to the file. The author name comes from a `?author=` query param so
  no auth is needed for trusted-network use.

---

## 4. Derek — The Minimum Viable User

**Context:** Derek doesn't want themes, variants, or AI notes. He has
a single talk to give in three hours. He wants a deck. That's it.

**The minimal workflow:**

1. `waxon new talk`
2. He opens `talk.slides`, deletes everything, types his slides as
   plain Markdown separated by `---`.
3. `waxon serve talk.slides`
4. He presents from the browser, full screen.
5. After the talk, he runs `waxon export talk.slides -o talk.pdf` and
   emails the PDF to the organizer.

That's the entire interaction. Derek never sees a comment, never
touches a variant, never reads the README past the install command.

**What makes this work:**
- The default theme is good enough that Derek doesn't have to pick one.
- The starter template has zero advanced syntax in it — only what he
  needs.
- `waxon export` with no flags Just Works.

**What would trip him up:**
- If `waxon new` dumped a 60-line tutorial-template at him, he'd
  delete most of it and curse. The starter template has to be small.

**Why this story matters:** Derek is the floor. If waxon ever stops
working for Derek, it has lost the plot.

---

## 5. Kenji — Conference Speaker, Terminal Aesthetic

**Context:** Kenji is giving a 30-minute talk at GopherCon called
"Building CLIs in Go." He wants the slides to look like they were made
by someone who lives in the terminal — because he does.

**What happens:**

1. `waxon new gophercon --theme terminal`

2. He opens the file and edits the frontmatter:
   ```yaml
   ---
   title: "Building CLIs in Go"
   author: "Kenji"
   theme: terminal
   terminal-variant: gruvbox
   terminal-effects: true
   footer: "GopherCon 2026"
   ---
   ```

3. He writes his slides. Lots of code blocks. He uses
   `<!-- pause -->` between bullets so reveals are gradual.

4. `waxon serve gophercon.slides` — browser opens. The slides look
   like a terminal. Code blocks have proper highlighting. The footer
   reads `GopherCon 2026` in monospace.

5. He hits `t` to cycle themes — checks how it looks in `nord`,
   `catppuccin`, `everforest`. Picks `gruvbox`.

   **Gap:** Cycling themes is one keyboard shortcut, but there's no
   way to *preview the same slide in all themes side by side*. Kenji
   has to flip through one at a time and remember.

6. He hits `o` for the overview grid. Catches that slide 12 is empty
   (he forgot to fill it in).

7. The day of the talk: `waxon serve gophercon.slides --presenter`.
   Two windows. Audience sees the deck on the projector. Kenji sees
   speaker notes, timer, next-slide preview on his laptop.

**What makes this work:**
- Kenji never had to write CSS. The terminal theme was a one-line
  frontmatter field.
- Sub-themes (`gruvbox`, `nord`, etc.) gave him personality without
  complexity.
- Presenter mode is a single flag, not a separate app.

---

## 6. Priya — Engineering Manager Reviewing Maya's Deck

**Context:** Priya is Maya's manager. She got a `git pull` on Maya's
draft branch. She reads the deck the way she reads PRs — in her IDE,
with `git diff` open.

**What happens:**

1. `git diff main..maya/pitch-deck -- pitch.slides`

   Real diff. One slide changed: a heading reworded, two bullets added,
   an `<!-- ai: -->` note Atlas left explaining the wording choice.

2. She reads the diff. She agrees with the wording. She doesn't need
   to render the deck — the text told her everything.

3. She wants to push back on slide 4. She edits the file in nvim and
   adds:
   ```
   <!-- comment(@priya): the framing here implies we have product-
   market fit. We don't. Soften this. -->
   ```

4. She commits, pushes, asks Maya to take another pass.

5. Maya runs `waxon comment pitch.slides --author priya`:

   ```
   slide 4: the framing here implies we have product-market fit.
            We don't. Soften this.
            (priya, unresolved)
   ```

6. Maya rewrites slide 4. Resolves the comment with
   `waxon comment pitch.slides --resolve <id>`.

   **Gap:** Comments don't have stable IDs in the current format.
   `waxon comment` would need to assign one (a short hash of slide
   index + author + content) and the `--resolve` flag would need to
   either delete the comment or annotate it as resolved (e.g.,
   `<!-- comment(@priya, resolved): ... -->`).

**What makes this work:**
- The diff *is* the review. No "open the deck in a special viewer"
  step.
- Comments are part of the file, so they're part of the diff, so
  they're part of the PR.

---

## 7. Val — Designer Who Wants to Try All the Variants

**Context:** Val is helping Maya tighten the pitch. She thinks slide 6
("The Problem") is too soft. She wants to try three different versions
without committing to one.

**What happens:**

1. Val opens `pitch.slides`. After slide 6 she adds:

   ```markdown
   ---variant: problem-blunt
   # We're Bleeding Money

   Every onboarding takes 14 days. Every churned customer costs $4k.

   ---variant: problem-narrative
   # The Day Everything Broke

   Last March, our biggest customer asked us a question we couldn't
   answer in a week. They left.

   ---variant: problem-data
   # The Numbers

   - 14 days to onboard
   - 23% churn at month 6
   - $4k acquisition cost
   ```

2. `waxon serve pitch.slides`. In the browser, slide 6 has a small
   variant indicator at the top: `default | problem-blunt |
   problem-narrative | problem-data`. Val clicks each one to compare.

   **Gap:** The current rendering assumption is that the *active*
   variant is shown and others are hidden. Variant switching in the
   browser exists but the visual affordance for it is small. A
   side-by-side compare view (`waxon serve --compare slide=6`) would
   be a real upgrade.

3. Val takes a screen recording, sends it to Maya: "I like the
   narrative one for the warm investors, blunt for the cold ones."

4. Maya keeps both variants in the file. When she exports the deck for
   a specific investor, she runs:
   `waxon export pitch.slides --variant problem-narrative -o
   pitch-warm.pdf`

**What makes this work:**
- Variants are *cheap*. You don't have to commit to one. You can keep
  three forever.
- The file is the version control. Variants are not "branches" or
  "alternates" in some sidebar — they're just text in the same file.

---

## 8. Nova — Agent Generating Variants on Request

**Context:** Nova is an agent invoked by Maya from her editor. Maya
highlighted slide 9 and said "give me three other ways to say this."

**What happens:**

1. Nova reads `pitch.slides` via `waxon agent-context`. It pulls
   slide 9's content out of the JSON output.

2. Nova generates three rephrasings.

3. Nova edits `pitch.slides` directly: appends three
   `---variant: <name>` blocks after slide 9 with the new versions.

4. Nova adds an `<!-- ai: -->` block to each variant explaining the
   intent:
   ```
   ---variant: slide9-tighter
   ...
   <!-- ai: shortened version. cuts the qualifier in the second
   sentence — Maya can add it back if it matters to her audience. -->
   ```

5. Nova adds a comment for Maya:
   `<!-- comment(@nova): added 3 variants to slide 9. tighter,
   bolder, and one with a metaphor. pick whichever and delete the
   others. -->`

6. The browser auto-refreshes. Maya sees four versions of slide 9.
   She picks one, deletes the others, the comment goes away too.

**What makes this work:**
- Nova never *overwrote* Maya's slide. The original is still right
  there. Reverting is "delete the variants."
- Nova's reasoning lives next to the variants, so Maya doesn't have
  to ask "why this version?"

---

## 9. Marcus — Presenting Live, Things Go Wrong

**Context:** Marcus is at the podium. The projector is on. He's on
slide 8 of 22. The wifi just died.

**What happens:**

1. Marcus is running `waxon serve` locally. The deck doesn't depend on
   network. He keeps presenting.

2. He hits `p` to open presenter view. His laptop now shows the next
   slide, the speaker notes, and the timer. The audience sees only the
   current slide.

3. Slide 14 has an embedded image from a remote URL. It doesn't load.
   The slide shows a broken-image box.

   **Gap:** waxon should warn at parse time about non-local images
   (`waxon lint --network` could surface this). Or better, the parser
   could refuse to render a remote image and instead show a clear
   placeholder: `[Remote image: example.com/foo.png — not loaded]`.

4. Marcus glosses over it, finishes the talk.

5. After: he runs `waxon export deck.slides -o talk.pdf`. The PDF
   includes the broken image too. Same problem.

**What makes this work:**
- Local-first means a dead network is invisible for the 95% case.
- The CLI keeps running because there's no cloud component to break.

**What this story reveals:**
- waxon should aggressively favor *local* assets. A `waxon import-asset`
  command that downloads remote images and rewrites the path would
  solve this at draft time.

---

## 10. Aria — Browsing Themes Before Committing

**Context:** Aria is starting a new deck and doesn't know which theme
she wants. She wants to see what's available.

**What happens:**

1. `waxon themes`

   ```
   THEME              DESCRIPTION
   default            Clean, minimal dark theme
   light              Bright background, professional look
   corporate          Conservative palette for business
   minimal            Typography-focused, maximum whitespace
   vibrant            Bold colors and gradients
   terminal           Authentic TUI aesthetic (WebTUI)
   dracula            Dark with purple accents
   solarized-dark     Solarized palette, dark mode
   ...
   ```

2. She wants to *see* them, not just read names. She tries:
   `waxon themes --preview`.

   **Gap:** No `--preview` flag exists. She has to either run `waxon
   serve` with `--theme <name>` six times, or build a sample deck and
   cycle themes with `t`.

3. Workaround: `waxon new sample && waxon serve sample.slides`. Hits
   `t` to cycle through. After about 30 seconds she settles on
   `tokyo-night`.

**What this story reveals:**
- A gallery view at `localhost:8080/themes` would let Aria see all 20
  themes rendered against a sample slide in one screen. Cheap to build
  (one HTML page, one parsed sample deck, 20 iframes).
- Or: `waxon themes --preview` could open the gallery in the browser.

---

## 11. Quinn — The Git-Native Workflow

**Context:** Quinn maintains a deck of "engineering principles" that
the whole team contributes to. It lives in a git repo. PRs add or
edit slides. CI runs on every PR.

**What happens:**

1. A teammate opens a PR adding a new slide on "incident response."

2. CI runs `waxon export principles.slides -o /tmp/out.pdf` to verify
   the deck still parses and renders. If `waxon` exits non-zero, the
   PR fails.

   **Gap:** `waxon export` is overkill for "does this parse?" A real
   `waxon lint` would be faster (no Chromium spinup) and would give
   structured errors instead of "chromedp: navigate failed."

3. Quinn reviews the PR. She uses GitHub's diff view, then pulls the
   branch and runs `waxon serve principles.slides` locally to see the
   new slide rendered.

4. She approves. CI merges. The deck on `main` is the canonical one;
   anyone can clone the repo and run `waxon serve` to read it.

**What makes this work:**
- The deck is version-controlled like code. PRs work because the file
  diffs.
- waxon's exit code is meaningful — CI can rely on it.

---

## 12. Eli — Long-Running Service Install

**Context:** Eli wants `waxon serve` to run as a background service
on his laptop, always pointing at his "scratch" deck where he dumps
ideas. Quick to alt-tab to.

**What happens:**

1. `waxon service install ~/notes/scratch.slides`

   ```
   Installing waxon as a user service...
   Wrote ~/.config/systemd/user/waxon.service
   Reloading systemd...
   Started waxon.service
   Status: active (running)
   Visit: http://localhost:8080
   ```

2. He opens `http://localhost:8080`. The scratch deck is there.

3. He edits the file in his editor whenever an idea hits. The browser
   tab — pinned in his sidebar — auto-refreshes.

4. Reboot the laptop. Service comes back. Same URL.

5. After a week: `waxon service logs` shows the request log and any
   parser errors.

   **Gap:** If `scratch.slides` has a parse error, the service stays
   up but the browser shows a stack trace. Better: an error overlay
   in the browser saying "line 42: unclosed code fence" with the
   previous successful render still visible underneath.

**What makes this work:**
- Cross-platform: same command works on Linux (systemd) and macOS
  (launchd). No "if you're on Mac, do this instead" in the docs.
- It's a *user* service, not a system one. No sudo. No root.

---

## 13. Tess — Exporting a Specific Variant for an Audience

**Context:** Tess has a deck with three variants per slide — one for
investors, one for engineers, one for marketing. She needs three PDFs.

**What happens:**

1. `waxon export deck.slides --variant investors -o investors.pdf`
2. `waxon export deck.slides --variant engineers -o engineers.pdf`
3. `waxon export deck.slides --variant marketing -o marketing.pdf`

   **Gap:** Today, `--variant` accepts one variant name and applies it
   wherever it appears. But what does "wherever it appears" mean if a
   slide has variants `investors-v1` and `investors-v2`? The matching
   rule is unclear. A naming convention (`investors:v1`, `investors:v2`,
   matched on prefix) or an explicit `--variant-set` flag would help.

3. She emails the three PDFs to the three audiences.

**What makes this work:**
- Tess never had to maintain three separate files. One source of
  truth, three outputs.
- Variants are first-class citizens of the export pipeline.

---

## 14. Jin — Debugging a Deck That Won't Parse

**Context:** Jin pulled a teammate's deck. `waxon serve` errors out.
She has no idea why.

**What happens:**

1. `waxon serve broken.slides`

   ```
   Error: parse broken.slides: line 87: unexpected variant outside
   slide block
   ```

2. She opens line 87. There's a `---variant: foo` that isn't preceded
   by a slide. The teammate forgot the leading `---`.

3. She fixes it. Server starts.

   **Gap:** The error message tells her *what* but not *what to do*.
   "Did you mean to start a new slide here? Insert `---` on its own
   line above this line." would close the loop.

**What makes this work:**
- The error is *in the file*, with a real line number. She doesn't
  need to read source code to find it.

---

## 15. Ravi — Multi-Author Comment Threads

**Context:** Ravi, Maya, and Atlas (the agent) are all collaborating
on the pitch deck. Each leaves comments. Each reads the others'.

**What happens:**

1. Maya writes slide 10. Adds a question:
   `<!-- comment(@maya): is this number current? -->`

2. Atlas reads the comments via `waxon agent-context | jq '.slides[].
   comments[]'`. Sees Maya's question. Looks at the source data. Adds:
   `<!-- comment(@atlas): yes — pulled from Q4 dashboard 2026-04-10.
   confidence: high. -->`

3. Ravi reads slide 10's comments via:
   `waxon comment deck.slides --slide 10`

   ```
   slide 10:
     [maya] is this number current?
     [atlas] yes — pulled from Q4 dashboard 2026-04-10. confidence: high.
   ```

4. Ravi adds his own:
   `waxon comment deck.slides --add 10 --author ravi --text "trust
   atlas, ship it"`

   **Gap:** `waxon comment --add` exists in the help text but it's
   unclear whether it appends to the file or just emits text. The
   semantics need to be unambiguous: it edits the file, in place,
   below the existing comments on that slide.

5. The thread lives in the file. No issue tracker. No DM. Anyone
   pulling the deck sees the conversation.

**What makes this work:**
- Author attribution is per-comment, so threads make sense.
- Both humans and agents use the same syntax.

---

## 16. Wren — Incremental Reveals on a Technical Slide

**Context:** Wren is teaching a class on transformers. She wants to
build up a diagram one piece at a time.

**What happens:**

1. She writes:
   ```markdown
   # The Architecture

   ```
   [Embeddings]
   ```
   <!-- pause -->
       ↓
   ```
   [Self-Attention]
   ```
   <!-- pause -->
       ↓
   ```
   [Feed-Forward]
   ```
   <!-- pause -->
       ↓
   ```
   [Output]
   ```
   ```

2. In the browser she presses Space four times to reveal each piece.

   **Gap:** Multiple `<!-- pause -->` between code blocks works, but
   the visual transition is abrupt — content snaps in. There's no
   built-in fade. Wren wants subtle, not dramatic. A `<!-- pause:
   fade -->` variant or a CSS hook would solve it without bloating
   the format.

3. She also wants to *replace* a piece (e.g., "Embeddings" → "Token
   + Position Embeddings"). That's not what `<!-- pause -->` does;
   pause only adds.

   **Gap:** No "replace" reveal. Today she'd have to use two separate
   slides. Could `<!-- pause: replace -->` work? Or is this a feature
   creep we should resist?

**What makes this work:**
- Reveals are in the file, not in some animation panel. They survive
  git diff and re-rendering.

---

## 17. Morgan — README-Driven Workflow with AI Notes

**Context:** Morgan is the kind of person who writes the README before
the code. She's using waxon to build a deck about a project that
doesn't exist yet.

**What happens:**

1. She creates a deck called `vision.slides`. Each slide is one bold
   claim about the future product.

2. After every slide she leaves an `<!-- ai: -->` block describing
   what would have to be true for the claim to land:

   ```
   <!-- ai: this slide claims "10x faster than the alternatives." for
   that to be defensible we need a benchmark suite. when the project
   exists, regenerate this slide with real numbers from
   benchmarks/results.json. -->
   ```

3. Six months later, the project exists. Morgan asks an agent: "look
   at vision.slides, find every `<!-- ai: -->` that says 'when X
   exists,' and update the slide with real data."

4. The agent reads the file via `waxon agent-context`, parses out the
   ai notes, finds the ones with conditional language, fetches real
   data, edits the slides, leaves new ai notes documenting the
   changes.

**What makes this work:**
- AI notes are not just for current context — they're a *future-self*
  channel. Morgan can leave instructions for an agent that doesn't
  exist yet.
- The format encourages this without enforcing it.

---

## 18. Drew — One-Off Demo Deck for a Slack Share

**Context:** Drew wants to make a 4-slide explainer about a regression
he found. He'll share it in Slack and then delete it.

**What happens:**

1. `waxon new regression && nvim regression.slides`

2. Four slides, plain Markdown. No theme tweaks. No comments. No ai
   notes. No variants.

3. `waxon export regression.slides -o regression.pdf`

4. He uploads `regression.pdf` to Slack. Done.

5. `rm regression.slides regression.pdf`

**What makes this work:**
- The friction is so low that an ephemeral deck is a reasonable tool
  for a Slack message.
- He never had to think about "where does this live" or "do I need to
  delete it from the cloud."

**What would trip him up:**
- If `waxon export` opened a Chromium UI or required a long-running
  server, he wouldn't bother. The fact that it's one command, blocking,
  exits cleanly is what makes it Slack-worthy.

---

## 19. Sasha — Custom Theme via CSS

**Context:** Sasha works at a company with strict brand guidelines.
None of the built-in themes match. She has the brand CSS variables.

**What happens:**

1. She creates `acme-theme.css`:
   ```css
   @import "builtin:minimal";

   :root {
     --slide-bg: #ffffff;
     --slide-fg: #1a1a1a;
     --accent: #cc0000;
     --font-heading: "Acme Sans", sans-serif;
   }
   ```

2. Sets it in her deck:
   ```yaml
   theme: ./acme-theme.css
   ```

3. `waxon serve deck.slides`. Brand colors. Done.

   **Gap:** The `@import "builtin:minimal";` syntax is non-standard
   CSS — it's a waxon convention. There's no error if Sasha typos it
   (`@import "builtin:miminal"`); the @import silently fails and she
   gets the unstyled fallback. A clear "unknown builtin theme: miminal"
   warning at parse time would help.

4. She commits the CSS to the company repo. Other teams import it.

**What makes this work:**
- "Custom theme" is one CSS file with three variables. Not a plugin,
  not a build step.
- She extended a built-in instead of starting from scratch.

---

## 20. Bex — Agent-Driven Deck Iteration Loop

**Context:** Bex is using waxon as part of an agent harness she's
building. The harness lets a human give a prompt, an agent drafts a
deck, the human leaves comments in the browser, the agent revises.
Tonight she's testing the loop end-to-end.

**What happens:**

1. Bex types into her harness: "make me a 6-slide deck about
   PostgreSQL indexes."

2. Her harness calls Claude with the prompt + a system message
   instructing it to write to `/tmp/deck.slides` using the waxon
   format. Claude writes the file.

3. Her harness runs `waxon serve /tmp/deck.slides --no-open` and
   tells Bex to open `localhost:8080`.

4. Bex reads the deck. Slide 3 is wrong about B-tree internals. She
   wants to leave a comment.

   **Gap:** (Same as Lisa's gap.) She can't leave the comment from
   the browser. She has to either edit the file directly or tell her
   harness "tell the agent slide 3 is wrong about B-tree internals,"
   which adds a layer.

5. Workaround: Bex uses the harness UI to send a follow-up message:
   "slide 3 — the bit about B-tree leaf pointers is wrong, B-trees
   use linked leaves." The harness appends a comment to the file
   programmatically:
   `<!-- comment(@bex): slide 3 — leaf pointer claim is wrong, B-trees
   use linked leaves -->`

6. The harness re-prompts Claude. Claude reads the file via `waxon
   agent-context`, sees the comment, edits the slide, adds an ai note
   explaining the fix.

7. Bex sees the slide refresh in the browser. Approves with another
   harness command.

8. The loop exits when Bex says "ship it." Harness runs
   `waxon export /tmp/deck.slides -o ~/Desktop/postgres-indexes.pdf`.

**What makes this work:**
- Every loop participant — Bex, Claude, the harness — talks through
  the file. No shared state in some database.
- `waxon agent-context` and `waxon comment --add` are the only two
  primitives the harness needs.

**What this story reveals:**
- A `waxon comment --add` that's idempotent (same author + slide +
  text won't duplicate) would let harnesses safely retry.
- A WebSocket message *from* the browser saying "user clicked
  approve" would let harnesses skip the polling step. Could be a
  small JSON-over-WS protocol on the existing dev server.

---

## Open Questions

1. **Browser-based comment authoring.** Lisa, Bex, and several others
   need to leave comments without touching the file. The current
   architecture is read-only-from-browser. Adding write capability
   raises auth questions on shared servers.

   _Leaning:_ Add a `?author=<name>` query param and a small comment
   box in the presenter view. POSTs to the dev server, which appends
   `<!-- comment(@<name>): ... -->` to the file. No auth — assumes
   the server is on a trusted network (localhost or Tailscale). For
   public-network use, document that you should use a reverse proxy
   with auth in front.

   **Answer:**
   > _(empty — fill in when decided)_

2. **`waxon lint` vs reusing `waxon export` for CI.** Several stories
   want a fast parse-only validator that doesn't require Chromium.

   _Leaning:_ Add `waxon lint <file>` that runs the parser only. Exit
   non-zero on parse errors, with structured output (`--json` for
   tooling). Don't make it pluggable — only the parser's own errors
   count. Style nags belong in a separate `waxon style` later, if at
   all.

   **Answer:**
   > _(empty — fill in when decided)_

3. **Comment IDs and resolution semantics.** `waxon comment --resolve`
   needs to identify a specific comment. Stable IDs based on (slide
   index, author, content hash) would work but break if the slide
   moves.

   _Leaning:_ Generate a short ID at first read and write it back into
   the comment: `<!-- comment(@maya, id=a3f9): ... -->`. The ID is
   stable across moves. `--resolve a3f9` then either deletes the
   comment or marks it: `<!-- comment(@maya, id=a3f9, resolved): ...
   -->`. Default to delete (the git history is the audit trail).

   **Answer:**
   > _(empty — fill in when decided)_

4. **Variant selection grammar.** `--variant investors` is ambiguous
   when slides have variants like `investors-bold` and
   `investors-cautious`. We need a clear matching rule.

   _Leaning:_ Treat variant names as paths separated by `:`. So
   `investors:bold` and `investors:cautious`. `--variant investors`
   matches the prefix and picks the first one (or errors if multiple
   match). `--variant investors:bold` is exact. Document this as the
   contract.

   **Answer:**
   > _(empty — fill in when decided)_

5. **Remote image policy.** Marcus's broken image showed up in his
   live presentation. Remote images are a footgun.

   _Leaning:_ Default to *warn* at parse time on any non-local image
   URL. Add `waxon assets fetch` that downloads remote images into a
   local `assets/` directory and rewrites the paths. Document that
   `waxon export` should always be run after `waxon assets fetch` for
   reliability.

   **Answer:**
   > _(empty — fill in when decided)_

6. **Reveal animations beyond `<!-- pause -->`.** Wren wants subtle
   transitions and "replace" semantics. Format creep is a real risk.

   _Leaning:_ Add a single `<!-- pause: <style> -->` syntax with two
   styles initially: `add` (default, current behavior) and `replace`
   (clears the previous reveal). No fade/transition options — those
   live in CSS via theme. Theme authors can opt into fades for
   `[data-reveal]` elements.

   **Answer:**
   > _(empty — fill in when decided)_

7. **Theme gallery / preview UX.** Aria couldn't see all themes at
   once. Cycling with `t` is slow and unfair to a 20-theme catalog.

   _Leaning:_ Build a `/themes` route on the dev server that renders
   one sample slide in every theme as a grid. `waxon themes --preview`
   opens the browser to that route using a built-in sample deck. Cheap
   to build, big UX win.

   **Answer:**
   > _(empty — fill in when decided)_

8. **Live-reload error overlay.** Today, if the file becomes invalid
   while the server is running, the browser shows whatever the server
   serves (often a stack trace or blank page).

   _Leaning:_ The dev server should keep the *last successfully
   parsed* version in memory and serve that, with an overlay banner
   on top showing the parse error and the line number. The overlay
   clears when the file becomes valid again. This makes editing safe
   — you're never staring at a broken page while you fix a typo.

   **Answer:**
   > _(empty — fill in when decided)_
