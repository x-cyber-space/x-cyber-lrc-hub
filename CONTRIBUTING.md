# Contributing

Thanks for taking the time to improve `x-cyber-lrc-hub`. This document covers
the practical bits: how to build, what the checks are, and the couple of
project-specific rules that are easy to get wrong.

## Getting started

```bash
git clone https://github.com/x-cyber-space/x-cyber-lrc-hub.git
cd x-cyber-lrc-hub

make help     # list every target
make build    # -> bin/x-cyber-lrc-hub
make check    # everything CI runs: fmt-check, vet, test, lint
```

Requirements: Go 1.25+, and optionally `golangci-lint` v2
(`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`).
Docker is only needed for `make docker` and for building the release binaries.

## Tests

```bash
make test       # offline, with -race. This is what CI runs.
make test-all   # also runs the live-provider smoke test, which needs network.
```

The smoke test in `internal/provider` talks to four real music platforms, so it
is skipped under `-short` and must never be relied on in CI. Everything that
decides behaviour — matching, caching, protocol shape, the dispatcher — has
offline coverage driven by stub providers. **If you change behaviour, add an
offline test for it.**

## Project-specific rules

These exist because each one was a real bug at some point.

### `/api/get` must not guess

`/api/get` applies hard filters, never a weighted score. A wrong lyric is
indistinguishable from a right one once a player renders it, whereas a `404`
lets the client retry through `/api/search` with its own ranking and a user in
the loop. If you are tempted to "just return the best candidate", don't.

Scoring (`internal/scoring`) is a **ranking** function and belongs to
`/api/search`. Matching (`internal/matching`) is the **gate** and belongs to
`/api/get`. A full-marks title contributes 45 points on its own, which is
exactly why it must not be able to carry an unrelated artist past a threshold.

### The cache must not be able to answer a query the cold path would reject

Every cache hit is re-validated with the same predicate the cold path uses
(`cache.Matches`). If you add a code path that reads the cache, run that
predicate. A fuzzy search result must never become a confident `/api/get`
answer just by sitting in the database.

Identity is defined exactly once, in `matching.IdentityKey`, and is shared by
the storage key and result deduplication. Do not compute it in two places.

### Wire compatibility is verified, not assumed

Claims about `lrclib.net`'s behaviour in the README and code comments were
measured against the live service. If you change the response shape, error
bodies, or matching rules, probe the real endpoint first and say so in the
commit message. `docs/SPEC.md` covers the architecture; the README's
compatibility table is the contract.

### Errors are not "no lyrics"

If every provider fails, that is a `503`, not a `404`. Hiding an outage behind
"track not found" makes a broken upstream look like a missing song.

## Commits and pull requests

- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/):
  `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`.
- Explain **why** in the body. The diff already says what changed.
- Keep `go.mod`, `go.sum` and the CHANGELOG in step with code changes.
- Run `make check` before opening a pull request; the same checks run in CI.

## Code style

- `gofmt` is enforced. `golangci-lint` runs with a deliberately small linter
  set — the goal is catching mistakes, not accumulating opinions.
- Package names describe their domain. There is no `util`, `common` or
  `helpers` package, and new ones should not appear.
- Prefer a comment that explains a non-obvious constraint over one that
  restates the code.
