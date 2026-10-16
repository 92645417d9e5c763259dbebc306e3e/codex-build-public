# Public Sync

## Manual Selection

- Only an explicit current-user request selects the installed `sync` command.
- Ordinary release work and skill maintenance do not invoke it automatically.
- Obtain syntax from its parser-owned `--help`.
- Supply the public snapshot version with `--version`.
- Supply one canonical exported patch set with `--patch-tag`.
- Version is a caller-selected release value, never a Git revision or derived suffix.
- The version is the entire public commit message.

## Inputs

- The installed skill must be a clean `main` checkout.
- Snapshot its committed tracked source files, preserving original bytes and modes.
- Ignore installed binaries and other ignored local data.
- The patch source is `<effective working root>/build/patches/private/<PatchTag>`.
- Require the selected tag to be a committed, unchanged directory in private `main`.
- Other tags remain untouched.
- Consume an existing exported patch set without running export or changing product source.
- Resolve the public target as the authenticated GitHub user's `codex-build-public` repository through `gh`.
- Require that target to exist and be public.
- Use `~/projects/codex-build-public` as its dedicated local checkout.
- Require an existing checkout to be clean, including ignored and untracked content, before snapshot replacement.

## Snapshot Layout

```
codex-build-public
+-- SKILL.md
+-- commands/
+-- contracts/
+-- docs/
+-- go/
+-- reference/
`-- patches/
    `-- <PatchTag>/
```

- The latest skill occupies the repository root.
- The latest selected patch set occupies `patches/<PatchTag>`.
- The public tracked tree is the replaceable current snapshot.
- Earlier snapshots remain in normal public Git history.
- Native Git archives copy source content without importing private branches, commits or Git directories.
- Never publish the installed binary, credentials, task records, caches or runtime output.

## Publication

- Clone or fetch only the public repository through its HTTPS GitHub destination.
- An empty repository starts on `main`.
- An existing repository retains `main` through fast-forward-only remote alignment.
- Divergence stops without reset, force push or automatic retry.
- Native Git owns snapshot staging and one normal child commit.
- Every explicit invocation records its selected version, including an explicitly republished identical snapshot.
- Commit as `nobody <nobody@nobody>` for both author and committer.
- Use `gh` authentication for the ordinary native Git push.
- Verify the server's `main` message and snapshot content before returning Pass.
- Retain local commits and any published effect when a later step fails.
- Never rewrite earlier public history.
- The existing process-group deadline, cancellation and same-handle wait own the operation.
- Remove only its private runtime attempt after reap.
- Preserve the public checkout and both source repositories.
