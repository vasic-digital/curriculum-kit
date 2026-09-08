# QWEN.md — curriculum-kit

This carrier is read by Qwen Code. It is one of the four governance carriers §11.4.157 requires to be maintained in lockstep; every line below this one is byte-identical across all four, once each carrier's own name is normalised.

## INHERITED FROM constitution/QWEN.md

**The inheritance below is conditional. Both cases are stated; neither is
assumed.**

When this module is consumed inside a project that includes the Helix
Constitution submodule, the rules in `constitution/QWEN.md` — and in the
`constitution/Constitution.md` it references — are authoritative for every
topic not covered here. The module-local rules below extend them; they never
weaken or override them.

When this module is consumed standalone — cloned on its own, with no
constitution reachable in any parent — there is nothing to inherit, and **only
the module-local rules below apply**.

### Locating the base file: a resolver, never a path

`constitution/QWEN.md` above is the **canonical name** of the base file, written
as the constitution's own examples write it. It is not a filesystem path
relative to this module and must not be rewritten into one: a consuming project
may mount the constitution at `constitution/` or at
`submodules/constitution/`, and this module cannot know which. Resolve it by
walking parents:

```bash
root=$(git rev-parse --show-toplevel)
bash "$root/submodules/constitution/find_constitution.sh" 2>/dev/null \
  || bash "$root/constitution/find_constitution.sh" 2>/dev/null \
  || echo "standalone — no constitution in scope"
```

Read the canonical text on demand, never eagerly: the corpus is large, and a
native import would load it into every session before any work begins.

## What this module is

A content model and assessment mechanism for structured learning:

```
area  ->  lessons  ->  materials        (illustration, diagram, scheme,
      \                                  graph, video-with-a-time-range,
       ->  end-of-area assessment        document)
```

An assessment becomes AVAILABLE only when the lessons it names are complete.
See [README.md](README.md).

**It owns the model and the mechanism; it owns no storage, no transport and no
rendering.** That boundary is enforced rather than promised: it is a separate
Go module with an empty require set, so it does not require any consumer and no
consumer-shaped symbol can appear here even by accident. `TestNoDependencies`
and `TestNoConsumerShapedVocabulary` assert both halves.

## Module-local rules

These extend the inherited rules; they never weaken them.

1. **No consumer-shaped symbol, ever.** No type, field, constant or fixture may
   name a concept belonging to a consuming application. If a consumer needs
   something expressed here, it is expressed in this module's own generic
   vocabulary or it stays in the consumer. `TestNoConsumerShapedVocabulary`
   scans every `.go` file in the package, including the file that holds the
   banned list — exempting that file would have created the blind spot the
   list exists to close.

2. **Fixtures are synthetic.** This repository is public. No fixture may
   contain material derived from any private corpus, any real recording, any
   real transcript, or any identifiable person's name or speech. Synthetic test
   data only — no exceptions, and no "it is only a test file". Every fixture
   under `testdata/` was invented for this repository.

3. **Dependencies are load-bearing or absent.** Every `require` line must
   justify itself in the `go.mod` comment block. Convenience is not a
   justification: every consumer inherits whatever this module requires.

4. **Gates ship with paired mutations (§1.1), and the mutations are DATA.**
   Every mutation is a fixture file under `testdata/mutations/` with a sibling
   `.expect` naming the exit code and the rule code it must produce. No source
   is edited to run the proof, so the proof cannot rot into a control that
   passes because somebody forgot to put the code back.

5. **The three-valued distinction is never collapsed.** "Could not determine"
   is neither a pass nor a failure. It appears in three places here and none of
   them may be folded: `Report.Verdict`, the validator's exit code, and
   `Result.Determinate`. A finding OUTRANKS an undetermined row, so a catalog
   that is measurably broken can never hide behind "could not determine".

6. **A check that iterates nothing has measured nothing.** An empty catalog is
   a finding, an empty input set is exit 2, an empty mutation set is exit 2, and
   an area with no lessons is a finding. Each has its own assertion, because
   the opposite defect — a validator reporting a pass over an empty collection —
   is the one that has actually shipped in sibling repositories.

7. **No CI workflow files.** Per §11.4.156 this repository ships no active
   server-side CI. Gates run locally and at the consuming project's seams.

8. **Never force-push** (§11.4.113). Integrate by merging onto the latest
   `main`; a fast-forward push then always succeeds and no commit is lost.

## Build and test

```bash
go build ./...
go vet ./...
go test ./...
go test -race ./...

bash scripts/verify-curriculum.sh                  # 0 clean / 1 finding / 2 undetermined
bash scripts/verify-curriculum.sh --prove-failure  # the §1.1 paired mutation proof
```

## Upstreams

Mirrored, and both are pushed on every publish (§2.1):

- `git@github.com:vasic-digital/curriculum-kit.git`
- `git@gitlab.com:vasic-digital/curriculum-kit.git`

Recipes are in `upstreams/`; run `install_upstreams` from the repository
root after cloning (§11.4.36).
