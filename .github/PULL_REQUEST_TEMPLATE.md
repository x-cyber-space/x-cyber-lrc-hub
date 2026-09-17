## What and why

<!-- The diff says what changed; explain why it needed to. -->

## How it was verified

<!-- e.g. "make check", plus any manual curl against a running server. -->

- [ ] `make check` passes (`fmt-check`, `vet`, offline tests with `-race`, lint)

## Checklist

- [ ] Behaviour changes have an offline test (a stub provider, not the network).
- [ ] Wire-format changes were checked against the live `lrclib.net` endpoint.
- [ ] `/api/get` still applies hard filters rather than returning a best-guess.
- [ ] `CHANGELOG.md` updated for anything user-visible.
- [ ] `README.md` updated if flags, endpoints or compatibility changed.

## Related issues

<!-- Closes #... -->
