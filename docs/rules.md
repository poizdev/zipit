# Exclusion rules and configuration

zipit includes every traversed path unless an exclusion rule matches it. Version 0.1 uses an exclusion-only model: rule sources are combined into one set, and any matching rule excludes the path.

## Rule sources

For each archive, zipit unions rules from:

1. the global user config
2. `.zipitignore` in the selected source root
3. each `-x` or `--exclude` argument

The ordering does not create precedence. There is no negation or later unexclude operation.

For example:

```console
zipit ./project -x node_modules/ -x "*.log"
```

adds both CLI patterns to any global and workspace patterns already present.

## Matching semantics

Rules use `/` as the portable separator and are matched case-sensitively.

### Basename patterns

A pattern with no internal `/` matches a basename at any depth:

```text
node_modules/
*.log
.DS_Store
```

`node_modules/` excludes every directory with that name and prunes its subtree. `*.log` excludes matching regular files or symlinks at any depth. A non-directory rule does not exclude a directory of the same name.

### Root-relative patterns

A pattern containing `/` is matched against the path relative to the selected source root:

```text
build/generated/
docs/private/*.md
```

`build/generated/` matches that directory below the source root, but not `packages/build/generated/`. Recursive glob syntax can make a slash-containing pattern apply at multiple depths.

### Directory rules

A trailing `/` marks a directory-only rule:

```text
.git/
dist/
vendor/cache/
```

When a directory rule matches, traversal prunes the entire subtree. Descendants are not visited or counted individually.

### Recursive globs

`**` matches recursively across path segments. Examples:

```text
**/.cache/**
packages/**/generated/
**/*.tmp
```

The first excludes paths below `.cache` directories at any depth. The second excludes matching `generated` directories below `packages`. The third excludes `.tmp` files recursively.

### Normalization and validation

Leading `/` and repeated leading `./` are removed, so `/build/` and `./build/` are interpreted as source-root-relative `build/`. Backslashes are normalized to `/`, giving Windows-style input the same internal rule form.

Empty patterns, malformed globs, negation patterns beginning with `!`, and patterns containing a `..` path segment are rejected before traversal. zipit does not implement Gitignore semantics: in particular, it has no negation, no precedence-based unexclude behavior, and no nested `.zipitignore` discovery.

## `.zipitignore`

zipit reads at most one workspace rule file: `.zipitignore` directly inside the selected source root. Blank lines and lines whose first non-whitespace character is `#` are ignored. Every other trimmed line is one exclusion pattern.

Example:

```text
# Dependencies and repository metadata
node_modules/
.git/

# Generated output
dist/
*.log
```

Run `zipit init` from a workspace to create this file interactively. The selector offers common presets and accepts custom patterns. It refuses to overwrite an existing `.zipitignore`; edit the existing file directly instead. For non-interactive use, create the text file yourself.

## Global configuration

Global rules are optional. zipit resolves the config directory with Go's `os.UserConfigDir()` and stores `zipit/config.toml` beneath it. On Linux this is commonly:

```text
~/.config/zipit/config.toml
```

The actual location follows the platform and environment. A missing config is normal, and creating an archive does not create one automatically.

The current schema is:

```toml
schema = 1

[ignore]
patterns = [
  ".git/",
  "node_modules/",
]
```

Manage it with:

```console
zipit config init
zipit config add <pattern>
zipit config remove <pattern>
zipit config list
zipit config edit
```

`config init` interactively selects presets and custom rules and refuses to replace an existing config. `config add` creates the config if necessary, normalizes the rule, and avoids duplicates. `config remove` removes one configured rule. `config list` reports the active global rules and resolved config path. `config edit` opens the file using `$VISUAL` or `$EDITOR`, creating an empty valid config if needed, then validates the edited file when the editor exits.

Global, workspace, and CLI rules are always unioned for archive creation.

## Previewing selection

Use dry-run before creating an archive or refining a rule set:

```console
zipit . --dry-run
zipit ./project --dry-run -x node_modules/ -x "*.log"
```

Dry-run performs source resolution, rule loading and compilation, traversal, subtree pruning, and statistics collection. It shows the expected selected regular-file count and input size, exclusion counts where applicable, preserved symlink and skipped special-entry information, and the intended output path.

It creates no ZIP or temporary archive artifact. Because compression is not performed, dry-run cannot report the final compressed archive size. Its input size is the selected filesystem data, not an estimate of ZIP size.

If the intended output already exists, dry-run reports whether archive creation would require `--force`; it does not modify the file.
