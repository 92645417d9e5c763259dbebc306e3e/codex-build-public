# RepositoryFormatting

## Route

```
CompleteOfflineRepositoryFormatting
+-- Owner: RepositoryFormattingSupportOwner
+-- Consumer: OfflineCanonicalFormatterClosureConsumer
+-- Successor: AllFormatterGroupsAndOwnedPostimageSuccessor
`-- Terminal: CompleteOfflineRepositoryFormattingTerminal
```

- The public `repository-format` route consumes one selected real product worktree, fixed project roots, release, attempt, and positive deadline. It accepts clean input, unstaged tracked edits, and the exact preexisting untracked regular files, so new owner files can be formatted before commit. Sequencers, staged changes including intent-to-add, deletions, and non-regular changed paths reject before support preparation.
- The single route graph above owns admission, support preparation, product execution, owned postimage classification, and cleanup; no earlier formatter route identity remains.
- Admission expands RepositoryFormattingSupport and requires manifest-selected `bin/just`, `bin/dotslash`, and `bin/uv` before creating an attempt or changing source bytes.
- RepositoryFormattingSupport uses the fixed DotSlash and uv executables before product execution to fetch the repository-selected buildifier and synchronize the two frozen Ruff project environments into the attempt; it consumes the current `tools/buildifier`, `sdk/python`, and `scripts` manifests and lockfiles, then requires the branch, HEAD, complete tracked binary diff, and untracked path set with exact bytes and modes to equal their admitted preimages before starting `just fmt`.
- One supervisor runs buildifier acquisition, SDK synchronization, scripts synchronization, and the formatter in that fixed order.
- Every command consumes the original invocation's remaining deadline.
- Each acquisition command additionally has a five-minute limit.
- Acquisition timeout, cancellation, and output truncation remain Helper before product execution and set the corresponding existing result flags.
- Support acquisition inherits only caller-set `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY`, `NO_PROXY`, and their lowercase spellings as temporary proxy inputs.
- Proxy variable spellings and values pass unchanged to the native download clients.
- PatchReplay Apply carries those variables to its nested formatting endpoint without adding them to native Git or transformation environments.
- The offline formatter receives no proxy variables.
- This route writes no persistent proxy or network configuration.
- The formatter sandbox binds the selected worktree writable, binds its Git metadata read-only for repository discovery, binds the native toolchain read-only, disables network, and projects the attempt-owned DotSlash cache and separate SDK and scripts uv environments at their repository-default paths.
- The formatter process sets `DOTSLASH_CACHE`, `UV_CACHE_DIR`, `UV_OFFLINE`, and disabled Python downloads to consume only the prepared closure; a missing cached artifact or frozen environment fails without a network, PATH, or host-tool fallback.
- The sole repository-level formatter payload is one fixed `just fmt fmt-check` process; just runs the canonical mutating recipe first and the repository-owned check recipe second, so every formatter group's native check mode must accept the retained postimage before Pass.
- Callers cannot select another recipe, argv, direct script, Cargo substitution, host executable, PATH fallback, source-independent formatter, or duplicate postimage verifier.
- One endpoint invocation owns the complete command sequence, bounded output, cancellation, and attempt cleanup.
- Actual stage entry emits acquisition, formatting, postimage-validation, and cleanup phases through the existing wait-notice formatter.
- Nested Apply delivers those notices to its original public handle before the child terminal.
- Progress notices do not enter error diagnostics.
- RepositoryFormattingSupport restores owner access only on directories inside its exact attempt root after the product process stops and never changes worktree permissions; it removes the entire prepared DotSlash and uv closure without following links and verifies that root is absent, while successful cleanup preserves the independently derived product process and tracked postimage terminal.
- The single just process exit proves successful ordered `fmt` and `fmt-check` recipes; Pass additionally requires unchanged HEAD and branch, no staged or deleted path, and exactly the admitted untracked files still present as regular files. FormattedPaths is the sorted union of changed tracked paths and admitted new paths, including unchanged new files. Undeclared new files reject. No intent-to-add workaround or post-commit formatter is used.
- Product failure in any repository-selected formatter group retains the formatter postimage and returns its bounded diagnostic to ReleaseRollForward; support preparation failure is Helper before product execution, changes no source, and returns the same owner successor.
- RepositoryFormatting creates no durable result record, retry state, alternate verifier, commit, branch, or Git-history effect.
- PatchReplay's file-local `rustfmt` after patch application is a distinct transport normalization and cannot satisfy or replace this repository formatter terminal.
