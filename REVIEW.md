# REVIEW

Before changing code in this directory, read the nearest `DESIGN.md` /
`REVIEW.md` from here up to the repo root (closer wins).

Global expectations for any change:

- `make build`, `make test` (`go test ./...`), `make lint` must pass; never
  weaken assertions or skip tests to go green.
- Do not weaken the hard invariants in the root `DESIGN.md` (auth-mode
  exclusivity, stdio log suppression, secret masking, read-only default)
  without an accepted ADR.
- Docs, code comments, and commit messages in English.

## Traps (paid for)

- None recorded yet. Entries are added only for real rework, incidents, or
  review rejections — never hypothetical risks.
