# RepositoryMaintenance

## Workspace Cargo Lock

```
RepositoryCargoLockPublicationOwner
+-- Consumer: DirtyWorkspaceManifestConsumer
+-- Successor: SameDependencyOwnerBazelSuccessor
`-- Terminal: ExactCargoLockPostimageTerminal
```

- Before the owning commit, consume the exact registered product worktree with at least one unstaged tracked Cargo.toml change inside codex-rs. Preserve every admitted manifest byte; reject staged, untracked, deleted, nondependency, or preexisting generated Cargo.lock changes. The existing regular codex-rs/Cargo.lock is the sole output, and an absent lock is not a request to resolve the whole workspace from scratch.
- Acquire the existing CargoLock before inspecting the selected source and retain it through execution and postimage inspection. Consume the declared native toolchain and existing project Cargo home; a missing cache or a Cargo-home config or credentials override blocks before source execution.
- Fixed Cargo runs `update --workspace --offline` from codex-rs in the maintenance sandbox, using only the existing project cache. The real source is writable with Git metadata read-only and network disabled. Cargo owns lock resolution and preserves unrelated locked packages; callers select no package, version, recipe, argv, executable, output, or network mode. Missing cached inputs return the Cargo failure without acquisition or host fallback.
- Pass requires the original HEAD and branch, unchanged index and manifest preimages, and either unchanged lock bytes or exactly the regular codex-rs/Cargo.lock postimage. Return the existing RepositoryMaintenanceResult with MaintainedPaths and CargoLockPostimage when changed; an unchanged lock has no product effect.
- Review the generated lock, then run the separate Bazel maintenance operation on the still-uncommitted dependency owner. Publish its Cargo manifests and required Cargo and Bazel locks together before clean locked verification. No provisional source commit, extra owner, generated task state, or passing test is produced here.

## Route

```
RepositoryBazelLockMaintenance
+-- Owner: RepositoryBazelLockMaintenanceOwner
+-- Consumer: DependencyOwnerPostimageConsumer
+-- Successor: SameOwnerLockfileCommitSuccessor
`-- Terminal: BazelLockMaintenanceTerminal
```

## Source Admission

```
DirtyDependencyMaintenanceAdmission
+-- Owner: RepositoryMaintenanceSourceAdmissionOwner
+-- Consumer: DirtyDependencyOwnerConsumer
+-- Successor: ExactMaintenancePreimageSuccessor
`-- Terminal: DirtyDependencyMaintenanceAdmissionTerminal
```

## Sandbox Environment

```
StableMaintenanceSandboxIdentity
+-- Owner: RepositoryMaintenanceSandboxEnvironmentOwner
+-- Consumer: BazelUserLookupConsumer
+-- Successor: BazelLockRecipeExecutionSuccessor
`-- Terminal: StableSandboxUserTerminal
```

```
BazelModuleAcquisitionOwner
+-- Consumer: SourcePinnedModuleCacheConsumer
+-- Successor: OfflineLockRecipeSuccessor
`-- Terminal: ClassifiedMaintenanceTerminal
```

- The Bazel `repository-maintenance` operation consumes one selected real product worktree, fixed project roots, release, attempt, and positive deadline after its dependency owner changes `Cargo.toml` or `Cargo.lock`.
- RepositoryMaintenance admits only the exact release MainWorktree or numbered fork branch while allowing its unstaged tracked dependency-owner postimage; RepositoryFormatting owns its separate tracked-input admission, while tests, replay, suites, and export retain clean-source admission.
- The maintenance source projection rejects active Git sequencing state, and its preimage disables rename folding before rejecting staged changes including intent-to-add index entries, untracked, deleted, nondependency, and preexisting `MODULE.bazel.lock` changes before product execution.
- Admission expands RepositoryMaintenanceSupport and requires manifest-selected `bin/just`, manifest-selected Bazelisk at `bin/bazel`, and the exact source `.bazelversion` before creating an attempt or changing source bytes.
- Require `.bazelversion` to declare one semantic release without comparing it with a skill-owned release number.
- The Bazel payload is only `/toolchain/bin/just bazel-lock-update`; callers cannot select another recipe, argv, shell, direct Bazel command, host executable, PATH fallback, or source-independent maintenance effect.
- A networked support phase binds the selected source read-only and runs fixed Bazel `--batch mod deps --lockfile_mode=off` to acquire its source-pinned release and module artifacts into the fresh attempt home.
- The product phase reuses only that attempt home, disables network, binds source writable and Git metadata read-only, and executes the sole repository recipe.
- The sandbox projects fixed `USER=codex` with private `HOME=/home`; it never inherits host user identity or falls back to a host environment.
- Bazel acquisition failure is Helper; only the prepared-cache offline recipe may classify a dependency or lock rejection as Product.
- Pass requires unchanged Git metadata, no untracked, deleted, or staged path, and either no tracked change or exactly the regular `MODULE.bazel.lock` postimage.
- A successful lock update returns that exact path with `BazelLockPostimage` so PatchAcceptance includes it in the same semantic dependency owner; an already aligned lock returns an empty postimage.
- Product failure retains any valid lock postimage and returns bounded diagnostics to the same dependency owner; timeout, cancellation, output truncation, invalid postimage, admission failure, or cleanup failure is Helper.
- The Bazel operation shares one deadline and cancellation across acquisition and recipe, then removes its fresh private cache and attempt without a durable result, retry, or fallback.
- RepositoryMaintenance neither formats source nor runs tests and cannot satisfy or replace RepositoryFormatting or RunnerExec.
