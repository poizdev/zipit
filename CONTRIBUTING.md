# Contributing to zipit

zipit is a small cross-platform Go utility. Contributions should keep its command behavior predictable, its filesystem handling conservative, and its runtime portable.

## Development setup

The required Go version is declared by the `go` directive in [`go.mod`](go.mod). Clone the repository, install that Go toolchain, and run:

```sh
go build ./cmd/zipit
go test ./...
```

The build command writes a local `zipit` executable in the repository root; that root-only artifact is ignored by Git.

## Verification

Before submitting a behavioral change, run the same core checks used by the repository:

```sh
gofmt -w <changed-go-files>
go mod tidy
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/zipit
```

Inspect changes to `go.mod` and `go.sum` after `go mod tidy`. Do not introduce a dependency when the standard library or an existing dependency handles the requirement clearly.

Tests for a package normally live beside its implementation. End-to-end installer and release-package checks live in `tests/`. Behavioral changes should include focused tests that demonstrate both the intended result and relevant failure behavior.

## Filesystem and platform changes

Filesystem behavior is part of zipit's public contract. Changes involving traversal, rules, symlinks, output publication, cancellation, path comparison, or concurrent mutation need tests for safety properties as well as the success path.

Keep runtime code portable across Linux, macOS, and Windows. Isolate platform-specific behavior behind build-tagged files, follow the existing path-normalization boundaries, and avoid assuming Unix permissions, separators, case sensitivity, or rename semantics in shared code.

CI runs package tests on native Linux, macOS, and Windows runners, runs the race detector, cross-builds all six release targets, and executes the generic Linux amd64 and arm64 binaries in representative distro environments. A local change may still need targeted testing on an affected operating system before it is ready to merge.

## Repository structure

- `cmd/zipit` — executable entry point
- `internal/cli` — commands, terminal behavior, and rule assembly
- `internal/rules` — exclusion normalization and matching
- `internal/config` — global config location and persistence
- `internal/archive` — traversal, ZIP streaming, and publication
- `internal/presets` — interactive exclusion choices
- `internal/buildinfo` — release-injected build metadata
- `internal/update` — stable-release discovery and cache
- `tests` — installer and release-package integration checks
- `.github/workflows` and `.goreleaser.yaml` — CI and release definitions

## Pull requests

Keep each change focused and explain the user-visible behavior it affects. Add or update tests and public documentation with behavioral changes. Mention platform-specific implications and any verification that could not be performed locally.

Avoid mixing product changes with unrelated refactoring or generated release artifacts. Maintainers can then review the behavior, portability, and documentation as one coherent change.
