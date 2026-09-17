# Security Policy

## Reporting a vulnerability

Please report security issues **privately** rather than opening a public issue.

Use GitHub's [private vulnerability reporting][report] on this repository. If
that is unavailable, contact the maintainer directly.

Please include:

- what the issue is and where it lives (file, endpoint, flag),
- how to reproduce it,
- the impact you believe it has,
- any suggested fix, if you have one.

You can expect an acknowledgement within a few days. This is a small,
volunteer-maintained project, so please allow reasonable time before
disclosing publicly.

[report]: https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability

## Supported versions

Only the latest release on the `main` branch receives fixes.

## Scope and threat model

`x-cyber-lrc-hub` is designed to run on a home network, typically behind a NAS
or router, and it ships **without authentication**. Understanding that is part
of evaluating any report.

In scope:

- Anything that lets a request escape the intended HTTP surface: path
  traversal, injection into the SQLite cache, request smuggling.
- Memory or goroutine exhaustion from a single request (unbounded reads,
  unbounded fan-out, missing timeouts).
- Leaking local files or environment through a response or a log line.

Known and accepted by design:

- **No authentication.** Anyone who can reach the port can use the service.
  Do not expose it to the public internet; put it behind a reverse proxy that
  authenticates, or keep it on your LAN.
- **`Access-Control-Allow-Origin: *`.** This is required for browser-based
  LRCLIB clients to work against a self-hosted instance. Be aware that it also
  means any web page you visit can issue requests to your instance, which will
  then originate from your IP. Restrict it with a proxy if that matters to you.
- **Third-party lyrics are fetched over the network.** The scraped content is
  returned as data and never executed, but it is not validated for accuracy or
  licensing.

## Handling of secrets

The service stores no credentials. The only persistent state is a SQLite file
at the path given by `-cache`, containing track metadata and lyrics.
