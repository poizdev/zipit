# zipit

zipit creates intentional ZIP archives from directory trees by excluding dependencies, caches, build output, and other files you do not want to hand off.

Any directory tree → intentional archive.

## Installation

### Linux and macOS

The direct installer verifies the release checksum, installs `zipit` to `~/.local/bin/zipit`, and configures `PATH` for future Bash, Zsh, or Fish shells when needed:

```sh
curl -fsSL https://raw.githubusercontent.com/poizdev/zipit/main/install.sh | sh
```

### Windows

Run the direct installer from PowerShell:

```powershell
irm https://raw.githubusercontent.com/poizdev/zipit/main/install.ps1 | iex
```

It verifies the release checksum, installs to `%LOCALAPPDATA%\Programs\Zipit\bin\zipit.exe`, and adds that directory to the user `PATH` when needed. Open a new terminal after installation.

### Go

With the Go version declared in [`go.mod`](go.mod) installed:

```sh
go install github.com/poizdev/zipit/cmd/zipit@latest
```

The Go binary directory must already be in `PATH`.

## Quick start

Archive the current directory to a ZIP beside it:

```console
zipit .
```

Archive another directory, preview a selection, or exclude dependencies for one run:

```console
zipit ./project
zipit . --dry-run
zipit . -x node_modules/
```

## Why zipit?

Modern workspaces accumulate material that rarely belongs in a handoff: dependencies, repository metadata, virtual environments, caches, build output, logs, editor state, and generated files. General-purpose ZIP tools archive the paths they are given. zipit adds an intentional selection policy before archive creation.

zipit is not a different compression algorithm. It is a path-selection engine feeding a safe streaming ZIP writer. It does not decide what your project is; it helps you decide what your archive is.

## Exclusion rules

zipit unions exclusions from three sources:

1. the global user config
2. `.zipitignore` at the source root
3. repeated `-x` or `--exclude` flags

A basename pattern such as `node_modules/` or `*.log` matches at any depth. A trailing `/` marks a directory rule and prunes that entire subtree. Patterns containing `/` are normally relative to the selected source root, and `**` provides recursive glob matching.

Rules are case-sensitive and exclusion-only. There is no `!` negation or precedence-based unexclude behavior. See [Exclusion rules and configuration](docs/rules.md) for exact semantics, global configuration, workspace initialization, and dry-run behavior.

## Common usage

```console
# Choose the output path
zipit ./project -o project.zip

# Combine exclusions
zipit ./workspace -x node_modules/ -x .git/ -x "*.log"

# Safely replace an existing output
zipit ./project -o project.zip --force

# Create a workspace .zipitignore interactively
zipit init

# Inspect global exclusions
zipit config list

# Check for a newer stable release
zipit update --check
```

Existing output is never replaced unless `--force` (or `-f`) is given. `zipit version` prints build and platform details; `zipit version --short` prints only the version. Shell completion scripts are available through `zipit completion --help`.

Update is discovery-only: zipit reports available releases but never replaces its own binary. Normal archive commands do not contact GitHub; any implicit update notice uses previously cached discovery data.

## Real-world example

In one real multi-project workspace, configured exclusions reduced an archive from approximately 35,614 entries and 325 MB to 1,291 entries and 21.8 MB. The selected archive contained 1,286 regular files and 5 preserved symlinks, representing 60.8 MB of selected input.

That is about 96.4% fewer archive entries and a 93.3% smaller final archive for this workspace and configuration. The reduction came primarily from excluding dependency, cache, build, and editor artifacts—not from compressing the same input more efficiently. Results vary by directory and rules.

## Platform support

Release archives are built for:

- Linux amd64 and arm64
- macOS amd64 and arm64
- Windows amd64 and arm64

CI executes the same generic Linux binary in representative distro environments. The amd64 smoke matrix covers Ubuntu, Debian, Fedora, Arch Linux, Alpine, and Nix; the arm64 matrix covers Ubuntu, Debian, Fedora, Alpine, and Nix. This is runtime compatibility coverage, not native AUR or Nix packaging.

## Documentation

- [Exclusion rules and configuration](docs/rules.md)
- [Technical design and safety properties](docs/technical.md)

## Contributing

Bug reports and focused contributions are welcome. Read [CONTRIBUTING.md](CONTRIBUTING.md) for development commands, repository structure, and portability expectations.

## License

Zipit is licensed under the [Apache License 2.0](LICENSE).

Copyright © 2026 poizdev.
