# Driving Motivations

This document captures the *why* behind waxon. Code and tickets describe
what we're building. This is the only place that records why those choices
matter and what we're trying to make true about the world.

When a decision feels arbitrary, look here first. When the right answer
isn't obvious, the answer is whichever choice makes more of these things
true.

---

## 1. The mind meld

The reason waxon exists in one sentence:

> A human and an AI agent should be able to collaborate on the same
> presentation as fluently as two humans collaborating on a Google Doc —
> with the agent reading, writing, suggesting, and reasoning about the
> deck as a first-class participant, not as a chat assistant operating at
> arm's length.

Most "AI in your slide tool" experiences today are bolted on. There's a
sidebar. You ask it for a "summary" or "make this prettier" and it
returns text you have to manually paste somewhere. The agent doesn't
*own* the artifact — the human does, and the agent is a guest.

waxon flips that. The artifact is a plain text file. Both parties read
and write it directly. There is no privileged actor. The agent doesn't
need an API key or a plugin or an iframe — it needs `cat`, `grep`, and
the ability to call `waxon agent-context`. That's it.

**How to apply this:** if a feature requires a human to be in the loop
to function (drag this here, click that to confirm), it has failed the
mind-meld test. The escape hatch always exists — humans should be able
to drive the GUI when they want to — but the agent must be able to do
everything the human can do, and do it in the same place.

---

## 2. Text first, always

Slides are not pictures. Slides are *outlines that have been styled.*
Confusing the two is what makes traditional slide tools hostile to
agents and to version control.

waxon's source format is a Markdown file with YAML frontmatter. That
means:

- **`git diff` works.** A single-character change shows as a single-line
  diff, not a binary blob. PR review is real.
- **Agents can read it.** No OCR, no XML schema, no proprietary parser.
- **Agents can write it.** The format is small enough to keep in
  attention. Common edits don't require special tools.
- **Humans can read it.** A `.slides` file is meaningful even if you've
  never seen waxon before. It looks like notes.
- **It outlives the tool.** If waxon disappears tomorrow, your decks
  are still Markdown.

The cost is that visual fidelity is mediated by themes rather than
hand-placed. We accept this cost. The kind of person who hand-places
text boxes for a living is not our user.

**How to apply this:** when adding a feature, the question is "how does
this round-trip through the file?" If a feature only exists in memory or
in the rendered output, it doesn't exist as far as the agent is
concerned.

---

## 3. CLI as universal interface

Every capability waxon adds must be reachable from the command line
without an interactive prompt. `waxon serve`, `waxon export`,
`waxon comment --add`, `waxon agent-context`. No "hit any key to
continue." No menu trees. No required GUI.

The reason isn't ideology. It's that the CLI is the *only* interface
both humans and agents already know. A human can pipe `waxon
agent-context deck.slides | jq '.deck.slides[].content'` and read it.
An agent can run the exact same command and get the exact same output.
The terminal is the shared substrate.

**How to apply this:** if a feature is only reachable from the browser
preview, it's incomplete. Every feature ships with a CLI verb. The
browser preview is allowed to be richer, but never the only path.

---

## 4. Comments are messages, not metadata

When two people work on a doc, they leave comments for each other:
"this needs a chart," "I think slide 4 is too long," "done — moved to
slide 7." Those comments are how collaboration actually happens.

waxon treats comments as the primary collaboration channel between
human and agent. They live inline in the file as
`<!-- comment(@author): text -->` directives. They survive `git diff`,
they show up in `waxon comment --json`, they can be filtered by author,
and they explicitly tell you who said what.

A human leaves a comment that says "the data on slide 3 feels wrong."
The agent reads it via `waxon comment deck.slides --json`, fixes the
slide, leaves its own comment that says "updated to Q4 numbers from the
attached PDF — please verify," and the human reads that on the next
pass. No DM. No issue tracker. No status meeting.

**How to apply this:** when designing a feature that involves any back
and forth (review, approval, revision), the answer is almost always
"add a comment directive." Resist building separate review systems.

---

## 5. AI notes are a thinking trail

Distinct from comments, `<!-- ai: ... -->` directives are how the agent
records its *reasoning*. Not what it changed — `git diff` shows that —
but *why* it made the choice it did.

> `<!-- ai: reordered slides 3 and 4 because the chart on slide 4 is
>   the payoff for the question on slide 3. moved them adjacent so the
>   audience doesn't lose context. -->`

This is for two audiences: future agents (who can see how previous
agents thought about the same deck), and humans (who get a real
explanation instead of "the AI did it"). Without this, every agent edit
is opaque.

**How to apply this:** train every agent that touches a deck to leave
ai notes. Treat unexplained edits as a smell.

---

## 6. Variants are how alternatives stay close to the work

When a human and an agent disagree about a slide, the bad answer is
"pick one and overwrite." The right answer is "keep both, side by side,
in the same file." waxon supports `---variant: name` blocks within a
slide so an agent can offer "here's another way to say this" without
destroying the human's version.

This matters because iteration is how decks get good. The human writes
a draft. The agent offers three takes on slide 5. The human reads them,
picks one, deletes the others. The variants live for as long as they're
useful and disappear when the choice is made.

**How to apply this:** never overwrite a slide silently. Either edit in
place (and leave an `<!-- ai: -->` note) or add a variant. Both are
visible to the human on the next read.

---

## 7. Make the obvious thing the easy thing

waxon is opinionated. There are six built-in themes and we don't ship
a theme builder. Slides are 16:9 by default. Comments use a single
syntax. There's one command to start a server and one to export a PDF.

Every "should we make this configurable?" decision should default to
"no, pick a sensible default." Configurability is debt: every option is
a thing the agent has to know about, the human has to remember, and we
have to test. We will add knobs *only* when we have evidence that the
default is wrong for a real workflow.

**How to apply this:** when in doubt, ship the most opinionated version
that handles the 80% case. A user with weird needs can always edit the
file directly. The format is just text.

---

## 8. README-driven development

For anything more than a one-line fix, the spec or doc gets written
first. We figure out what we want before we figure out how to build it.
This is partly because waxon is small and we can afford it, but mostly
because the alternative is building features that don't fit the mental
model anyone (human or agent) has of the tool.

The driving motivations doc you're reading right now is the most
extreme version of this principle: write down why the project exists
*before* writing the code that justifies its existence.

**How to apply this:** before opening an editor on a new feature, write
the README section, the help text, or the user story for it. If it
doesn't read well, it won't build well.

---

## 9. The format is the spec

There is no separate specification document for the `.slides` format.
The parser is the spec. The example deck is the documentation. Adding
a new directive means adding a regex to the parser, an example to a
test deck, and a paragraph to the README — and that's it.

This works because the format is small enough to fit in your head, and
we plan to keep it that way. Adding a directive should feel slightly
expensive — it should be a real conversation, not a one-line PR.

**How to apply this:** when proposing a new directive, the burden is on
proving the existing directives can't already do it. If `<!-- note: -->`
plus `<!-- ai: -->` plus a comment can express something, we don't need
a fourth directive for it.

---

## 10. Decks are working artifacts, not deliverables

The presentations people give from waxon decks are the *least*
interesting thing about them. The interesting part is the file: the
notes, the comments, the variants, the ai trail, the back and forth.

A waxon deck should be the most useful artifact in the project for
explaining what the project is, what's been decided, and what's still
open. The slides themselves are a side effect of the conversation
between the human and the agent that produced them.

**How to apply this:** when evaluating a feature, ask "does this make
the file more useful as a working artifact?" If it only makes the
*output* prettier, it's lower priority than something that makes the
file better at being a thing two minds collaborate on.
