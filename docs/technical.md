# Technical design

zipit is a path-selection engine feeding a safe streaming archive writer. Its stable conceptual pipeline is:

```text
CLI and config adapters
        ↓
exclusion rule compilation
        ↓
filesystem traversal
        ↓
safe streaming ZIP writer
        ↓
atomic publication
```

The runtime stays filesystem-first. It does not infer frameworks, repositories, or workspace types.

## Package boundaries

`cmd/zipit` is the executable entry point. `internal/cli` defines command grammar, terminal output, interactive initialization, signal handling, and assembly of effective rules. `internal/config` locates, validates, and atomically persists global TOML configuration. `internal/rules` normalizes and compiles exclusions. `internal/archive` owns traversal, ZIP creation, statistics, and output publication.

`internal/presets` contains the explicit rule sets offered by interactive initialization. `internal/buildinfo` exposes release-injected version, commit, build date, Go version, and target platform. `internal/update` performs stable-release discovery and maintains cached discovery state; it has no dependency on the archive engine.

Release packaging and direct installers sit outside the runtime. GoReleaser builds six CGO-disabled release targets, packages the Linux builds as `.deb` and `.rpm`, and publishes checksums used by the installers. CI additionally verifies that the Linux binaries request no dynamic loader and declare no shared-library dependencies.

## Selection and traversal

Rules from global config, the source-root `.zipitignore`, and CLI flags are normalized and compiled before traversal. The matcher receives archive paths relative to the selected source root, using `/` separators. No host absolute path is written into an entry name.

Traversal streams entries to the ZIP writer instead of first accumulating a full-tree manifest or copying the tree into a staging directory. This bounds archive composition overhead and avoids duplicating the selected workspace on disk.

Directory exclusions are evaluated before descent. A match prunes the subtree rather than visiting every descendant. This is both a semantic property—the descendants are not individually counted—and the important performance invariant.

An observational benchmark on a generated dependency-heavy fixture measured:

```text
traversal without exclusion: 79.15 ms/op
with directory pruning:       0.44 ms/op
```

That is approximately 178 times lower traversal time for that generated fixture on that machine. It is not a universal performance guarantee; tree shape, storage, operating system, and rule set all affect results.

## Filesystem entry handling

Regular files are opened without following a final symlink and streamed through a context-aware copy loop into deflated ZIP entries. zipit checks that the opened object is still the regular file observed during traversal. Directory identity is also rechecked as traversal proceeds, so material filesystem replacement races fail the operation instead of silently changing its meaning.

Directories are represented as root-relative ZIP entries and traversed recursively unless excluded. Symlinks are preserved as symlink entries containing the link target and are never recursively followed. Unsupported special entries, such as FIFOs and device nodes, are skipped and reported in the summary rather than opened as ordinary files.

Filesystem paths use the host platform's path operations, while archive names and rules use portable `/` separators. Source symlinks are resolved to their canonical directory. Output parent symlinks are resolved where possible, and canonical filesystem identity is used where platform aliases or case behavior require it. Windows path comparisons account for case-insensitive aliases.

## Output safety and failure behavior

The default output is the absolute source path with `.zip` appended, placing it beside the source. A custom output may be inside the source tree; the resolved output and active staging file are both excluded from traversal so neither can enter the archive.

Archive data is written to a temporary file in the destination directory. zipit closes the ZIP writer and staging file, checks cancellation, and only then publishes the completed output. Publication uses platform-specific atomic replacement or no-replace operations.

Without `--force`, a pre-existing output is rejected, including an output created concurrently during generation. With `--force`, the old output remains in place while the new archive is generated; a failure or cancellation before publication leaves the old output intact. Failed operations remove their staging file and do not publish a partial final archive.

The archive command derives cancellation from the process interrupt signal. Traversal and file copying check the context, and cancellation follows the same no-partial-publication path as other failures.

## Dry-run

Dry-run uses the same source resolution, matcher, and traversal path as archive creation, but supplies no ZIP-entry sink. It therefore reports selected regular-file bytes and selection statistics without creating a ZIP or staging file. Since no compression runs, final archive size is intentionally unavailable.

## Configuration and workspace publication

Missing global configuration and missing `.zipitignore` files are valid empty states. Explicit config writes use a temporary file in the config directory followed by platform-specific publication. Interactive global and workspace initialization use no-replace publication and refuse to overwrite files that already exist, including files created concurrently during the interaction.

Configuration files are validated against schema version 1. Rule validation happens before persistence for command-driven changes and after an editor exits for `config edit`.

## Version and update discovery

`zipit version` is entirely local. `zipit update` and `zipit update --check` are equivalent explicit discovery operations: they query the latest stable GitHub release, compare semantic versions, and refresh cached discovery state.

Update is discovery-only. zipit never replaces its own binary. Normal archive commands do not contact GitHub. After a successful archive, an interactive terminal may display an update notice only when a still-fresh local cache already records a newer stable release; this implicit path performs no network request.

Network errors in explicit update checks do not affect local archive behavior. Development builds report discovery information but are not treated as comparable stable releases.

## Release and installation boundaries

Release archives contain a single executable and are produced for Linux, macOS, and Windows on amd64 and arm64. Linux binaries are built with CGO disabled and checked for dynamic-loader and shared-library dependencies before the same binary is exercised in representative distro containers.

The Unix and Windows direct installers select a matching release asset, download the published checksum manifest, verify the asset's SHA-256 digest, validate the archive payload, and stage replacement in the destination directory. A failed download, checksum, extraction, or staging step does not replace an existing installation. PATH integration is persisted by the installer when required for supported shells or the Windows user environment.

The update package and direct installers intentionally remain separate: update discovery reports availability, while installation is an explicit user action.
