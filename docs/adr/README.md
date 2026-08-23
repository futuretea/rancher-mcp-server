# Architecture Decision Records

An ADR records a decision, the alternatives rejected, and why — at the time it
was made.

## When to write one

Only when either is true:

- A plausible alternative was rejected for a non-obvious reason.
- Something was deferred with a specific condition for revisiting it.

Otherwise skip the ADR and update the relevant `DESIGN.md`. Most changes do not
need one.

## Rules

- **Immutable once built against.** Never edit an accepted ADR after
  implementation has landed against it — supersede with a new one that links
  back, and mark the old one `Superseded by NNNN`. Before anything ships
  against it, an accepted ADR may still be amended (workflow step 5).
- **Outside the drill-down hierarchy.** ADRs live here, never next to code.
  `DESIGN.md`/`REVIEW.md` are read root-down on every task; ADRs are not,
  because they are history rather than current state.
- **Not a substitute for `DESIGN.md`.** When the work lands, update the live
  design docs to describe what now exists. The ADR keeps the "why we didn't";
  `DESIGN.md` keeps the "what is".

## Status lifecycle

`Proposed` → `Accepted` → (`Superseded by NNNN`)

Use `Rejected` for decisions considered and declined; keep the file.

## Format

Files are named `NNNN-<kebab-case-title>.md` (four digits, monotonically
increasing). Every new ADR is added to the index table below in the same
change. Nygard-style sections:

```markdown
# <title>

- Status: Proposed | Accepted | Rejected | Superseded by NNNN
- Context: the constraints and problem at decision time
- Decision: what was chosen
- Consequences: what follows, including costs and rejected alternatives
```

## Workflow

1. **Decide first.** Draft the ADR as `Proposed` and land it on its own before
   implementation. Flipping it to `Accepted` is the decision gate.
2. **The accepted ADR is the spec** while implementation is in flight.
   `DESIGN.md` never describes in-progress or planned work — only what exists.
3. **`DESIGN.md` rides the code.** Each change that alters the architecture
   updates the affected `DESIGN.md` files in the same change, not as a
   follow-up pass.
4. **Plans are not documents.** Sequencing, checklists, and rollout order live
   in the task or branch driving the work; they are meant to go stale.
5. **Wrong decisions:** an `Accepted` ADR may be amended while nothing has
   shipped against it. Once implementation has landed, supersede instead.

## Index

| ADR | Title | Status |
| --- | --- | --- |
