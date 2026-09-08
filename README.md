# curriculum-kit

**Revision:** 1
**Last modified:** 2026-09-07

A project-not-aware content model and assessment mechanism for structured
learning, in Go, with an empty dependency set.

```
Catalog
  └── Area                       a subject area
        ├── Lesson               ordered study units
        │     └── Material       illustration · diagram · scheme · graph
        │                        · video (chapter + time RANGE + transcript
        │                          anchor) · document
        └── Assessment           the end-of-area test
              ├── requiredLessons   the gate: complete these first
              ├── passPercent       the threshold
              └── Question          single · multi · short
```

The rule the model exists to enforce: **an area leads to lessons, and the
lessons lead to a test that is not available until they are complete.**

## What it owns, and what it does not

| Owned here | Owned by the consuming application |
|---|---|
| The types: area, lesson, material, video anchor, assessment, question | Authoring: where a catalog comes from |
| Validation: what makes a catalog incoherent | Storage: rows, JSON columns, files — anything |
| The gate: when a test becomes available | Transport: HTTP shape, auth, sessions |
| Grading and scoring | Rendering: every pixel |
| The progress value type | Persisting that value |

**Why this boundary and not the other one.** The alternative was a module that
owns only RENDERING, with each application supplying its own content. It was
rejected on the evidence: the two consuming applications have completely
different frontends and completely different chrome, so a shared renderer would
have had to embed one application's assumptions in order to be useful to it —
the exact thing a reusable module may not do. What they do NOT have is a shared
domain layer, and the defects that have actually shipped in this family were
domain-invariant defects, not drawing defects: an answer key that encoded to
"absent", a filter that silently fell back to a different value, a status
vocabulary that accepted a value nothing could tally. Every one of those is
preventable by a type and unreachable by a renderer.

Rendering stays with the consumer. This module publishes the JSON shape the
renderer consumes and refuses to draw anything.

## Using it

```go
import "github.com/vasic-digital/curriculum-kit/pkg/curriculum"

// 1. Validate the content before serving it.
rep := curriculum.ValidateWith(cat, curriculum.Options{KnownChapters: chapters})
switch rep.Verdict() {
case 0: // coherent
case 1: // real findings — rep.FindingRows()
case 2: // could not determine — rep.Undetermined(); NEVER treat as a pass
}

// 2. Gate the test.
av, err := curriculum.AvailabilityOf(area, progress)
// av.Available, av.MissingLessons, av.RequiredLessons  →  "3 of 5 lessons left"

// 3. Grade a submission. Submit refuses if the gate is shut.
res, err := curriculum.Submit(area, progress, responses, time.Now())
if errors.Is(err, curriculum.ErrAssessmentLocked) { /* 403, not a score of 0 */ }

// 4. Persist however you like — Progress is a plain JSON value.
blob, _ := json.Marshal(progress)
```

### Video materials carry a RANGE, not a position

`VideoAnchor` holds a chapter id, a start, an end and a transcript anchor, so a
consumer can seek a player into a chapter, highlight the passage, show a
duration, and scroll a transcript to the matching point. A single position
supports none of those, and the missing end cannot be recovered later.

Times are integer **milliseconds** on the wire. A `time.Duration` marshals as an
opaque nanosecond integer that no other language's client reads correctly;
`Start()`, `End()` and `Length()` give Go callers the typed form.

### Answers are named by ID, never by index

`Question.CorrectChoices` holds choice **ids**. An index is a number whose zero
value means both "the first choice" and "absent", so any encoder that elides
zero values turns "the answer is the first choice" into "there is no answer" —
silently, for exactly the items whose answer is first. Ids do not collide, and
reordering the choices does not move the answer.

### "Could not determine" is a first-class state

Three places, none of them collapsible:

- `Report.Verdict()` — 0 clean, 1 finding, 2 undetermined. A finding outranks
  an undetermined row, so a broken catalog cannot hide behind one.
- The validator binary's exit code, including **exit 2 for being given nothing
  to check**.
- `Result.Determinate` — false when the submission held a free-text question.
  This package cannot judge prose, so `Percent` is then a LOWER BOUND and
  `Passed` is **always false**. A pass is never awarded over an answer nothing
  graded, and `Progress.BestAttempt` will not return such an attempt either.

## Validation rules

`CK001` empty catalog · `CK002` duplicate id · `CK003` area with no lessons ·
`CK004` lesson filed under a missing area · `CK005` assessment filed under a
missing area · `CK006` test gated on nothing · `CK007` gate names a foreign
lesson · `CK008` test with no questions · `CK009` threshold outside 1..100 ·
`CK010` video material with no usable time range · `CK011` unknown material
kind · `CK012` material with no uri · `CK013` video anchor on a non-video
material · `CK014` choice question with fewer than two choices · `CK015` answer
that is not one of the choices · `CK016` single-answer question without exactly
one answer · `CK017` multi question with no answer · `CK018` free text carrying
choices · `CK019` empty required text · `CK020` unknown question kind · `CK021`
area with no assessment · `CK022` question worth nothing · `CK023` visual
material with no alt text · `CK024` anchor into an unknown chapter ·
**`CK900`** chapter registry not supplied — *undetermined, not a finding*.

## Gates

```bash
bash scripts/verify-curriculum.sh                  # 0 / 1 / 2
bash scripts/verify-curriculum.sh --prove-failure  # §1.1 paired mutation proof
```

Measured on 2026-09-07 on this host, Go 1.26.0:

| Command | Exit | Observed |
|---|---|---|
| `go build ./...` | 0 | — |
| `go vet ./...` | 0 | — |
| `go test ./...` | 0 | 94.3% statement coverage |
| `go test -race ./...` | 0 | — |
| `scripts/verify-curriculum.sh` | 0 | `OK — 2 document(s), 0 finding(s), 0 undetermined` |
| `scripts/verify-curriculum.sh --prove-failure` | 0 | `32 passed / 0 failed / 32 mutations` |
| gate over `testdata/mutations/empty-catalog.json` | 1 | `CK001` |
| gate over `testdata/mutations/chapters-undeclared.json` | 2 | `CK900` |
| gate over a path that does not exist | 2 | `cannot read` |

The proof's own failure direction was exercised too, in a throwaway copy of the
tree: weakening one validator rule took it to `31 passed / 1 failed`, deleting
the mutation fixtures took it to exit 2 (`a proof that exercises nothing proves
nothing`), and deleting the good fixtures took the plain gate to exit 2.

## What is NOT done

Stated plainly, because an unstated gap reads as a finished feature.

- **No consumer is wired to this module.** Nothing imports it. It has not been
  integrated into any application, and no application's data has been migrated
  into these types. Whether the model fits a real catalog is **UNDETERMINED**;
  it was designed from a structural reading of an existing implementation, not
  from a migration.
- **No frontend.** The module publishes a JSON shape; no component renders it,
  and no TypeScript types are generated from it. A consumer writes its own.
- **No transcript rendering, no player.** `VideoAnchor` carries what a consumer
  needs to seek and to scroll a transcript. This module resolves neither, and
  cannot verify that a `chapterId` or a `transcriptAnchor` points at anything —
  which is exactly why an unsupplied chapter registry is reported as
  UNDETERMINED rather than accepted.
- **No i18n.** Every string is single-language. The existing implementation this
  was read from carries a per-field translation table; that is a real
  requirement and is **not modelled here**.
- **Free text is not graded.** Deliberately: see `Result.Determinate`. There is
  no hook for a consumer's own grader to write a mark back into a `Result` yet.
- **No spaced repetition.** Progress records lesson state and attempts. The
  scheduling idea in the implementation this was read from (streaks and
  doubling review intervals) is not ported.
- **No ordering or prerequisite graph between AREAS.** The gate is within an
  area only.
- **Not published, not tagged, not a submodule.** This is a plain directory. It
  has never been pushed to either upstream, and the upstream recipes name
  repositories that may not exist yet.

## License

MIT — see [LICENSE](LICENSE).
