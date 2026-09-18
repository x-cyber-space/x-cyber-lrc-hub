# Changelog

All notable changes to this project are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `lyricsfile` in every response, matching the field `lrclib.net` serves
  alongside `plainLyrics` and `syncedLyrics`.
- `GET /api/get/{id}` for retrieving a cached record by ID.
- `-cache-ttl` to expire cached lyrics (default 30 days, `0` disables expiry).
- Server-side matching documentation and a `docs/SPEC.md` architecture spec.

### Changed

- **`/api/get` no longer accepts a candidate on a weighted score.** It now
  applies `lrclib.net`'s semantics: normalised equality on the supplied fields
  plus a ±3s duration window with nearest-record selection, then a single
  controlled relaxation stage (multi-artist credits, ±5s, platform title
  suffixes). The previous `>= 40` threshold let a full-marks title carry a
  completely unrelated artist.
- `/api/search` keeps the weighted score for **ranking** only, with a 40-point
  floor deliberately below the client-side 60 line.
- `-log-level` is now honoured; it was previously parsed, printed and ignored.
- Error responses, the response field set and JSON null semantics now match
  `lrclib.net` byte for byte. `plainLyrics` / `syncedLyrics` / `lyricsfile`
  are `null` rather than `""`.
- `album_name` participates in matching instead of being discarded. It is
  enforced in the exact stage only, because platform album strings routinely
  disagree with a listener's own tags.
- Cache rows are re-validated with the same predicate the cold path uses, so a
  fuzzy `/api/search` result can no longer resurface as a confident
  `/api/get` answer.
- The cache key includes a 2-second duration bucket, so different edits of one
  song no longer overwrite each other.
- Go 1.25+ is required; the module previously declared `go 1.26.5`, which made
  the toolchain refuse to build on older releases.

### Fixed

- A request for one artist's song could be answered with a different artist's
  lyrics when the titles collided.
- A missing `duration` acted as 20 free points, which pushed mismatches past
  the acceptance threshold.
- `/api/search` returned duplicate row IDs, and returned candidates far below
  its documented floor.
- `name` was present only on cache hits, making the response shape depend on
  cache state.
- The provider smoke test passed silently when it could not reach a platform.

### Engineering

- CI: gofmt, build, vet, `go test -short -race`, golangci-lint, and
  `CGO_ENABLED=0` cross-compilation for linux/amd64, linux/arm64,
  darwin/arm64 and windows/amd64.
- `Dockerfile` (multi-stage, static, non-root), `Makefile`, `.golangci.yml`,
  `.editorconfig`, Dependabot and release automation.
- CodeQL, dependency review and OpenSSF Scorecard workflows; Dependabot for Go
  modules, GitHub Actions and the Docker base image. Third-party actions are
  pinned to commit SHAs.
- Releases publish GoReleaser archives and a multi-arch container image to GHCR
  (`ghcr.io/x-cyber-space/x-cyber-lrc-hub`).
- SQLite runs in WAL mode with `synchronous=NORMAL`; server sets
  `ReadHeaderTimeout`; panics log a stack trace.

[Unreleased]: https://github.com/x-cyber-space/x-cyber-lrc-hub/commits/main
