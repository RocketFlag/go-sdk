# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0] - 2026-10-05

### Changed (breaking)
- The module path is now `github.com/rocketflag/go-sdk/v2`.
- `GetFlag` takes a `context.Context` as its first argument and binds the request to it, so callers can cancel a request or give it a deadline.
- `UserContext` is now a `map[string]string`. v1 accepted any value and formatted it with `%v`, so a nil, struct or nested map was sent as text that never matched. Convert numbers and booleans to strings yourself.

### Added
- `WithCacheMaxEntries` client option and `DefaultCacheMaxEntries`. The cache now holds at most 10,000 entries by default and evicts the least recently used entry when full, so a high-cardinality context (a `targetingKey` per user) no longer grows memory without bound.
- README sections on sticky rollouts (`targetingKey`), audiences, group flags, and migrating from v1.
- A runnable example for pkg.go.dev.

### Fixed
- The release workflow reads the module path from `go.mod`, so it warms the Go module proxy for the `/v2` path.
