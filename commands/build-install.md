# BuildInstall

## Admission

- Invoke installed `build-install` alone using parser-owned `--help` after TaskWorkspace reconciles pending obligations and RouteEntry selects the effect on clean registered MainWorktree.
- An indexed Roll consumes the same native source-coverage admission before tool acquisition, completed-package reuse, compilation or selection.
- During explicit draft quick iteration, coverage consumes the task's canonical terminal reachable from clean Main while the existing draft owner verifies the temporary suffix.
- Coverage neither parses temporary fixup subjects nor changes the actual Main source selected for building.
- An independent Install without an active Roll retains its existing package and launch-entry contract.
- Unrelated probes and skill maintenance follow the product terminal.
- Caller-home release links are user-owned; the helper's dist selection does not depend on them.
- Dist selection neither reads nor writes caller-home release links.
- Package-only leaves `build/dist/current` unchanged.

## Build And Package

```
BuildInstallPublicationOwner
+-- Consumer: RemoteToolReleaseConsumer
+-- Successor: OfflineLocalToolPackageSuccessor
`-- Terminal: AtomicCurrentInspectionToolTerminal
```

- Under CargoLock, prepare the latest semantic shuck and tokei releases in retained bare remote clones at `PROJECT_SCRATCH_ROOT/tool-sources/{shuck,tokei}.git`.
- Archive each selected commit.
- Validate the source version and Cargo lock before fetching its locked dependencies with the fixed toolchain.
- The existing tool preparation owner derives its private source extraction, HOME and temporary files beneath the effective working root at build/tmp/build-tools and removes that exact attempt after consumption or failure.
- Caller-supplied inspection-tool binaries are not build inputs.
- Before package admission, match Main's locked V8 version to the GNU toolchain. Authenticate its remote two-file checksum manifest against Main's pinned release-manifest digest, then verify the existing archive and bindings. Keep no checksum state and never substitute a target or download unverified build inputs.
- Build only in ProjectedRunner, keeping Main at 0.0.0 and stamping the full BuildVersion once in the private overlay's workspace.package version, inherited by all workspace crates including CLI.
- Package directory, manifest, compiled `codex --version`, TUI status line and other runtime version reports use that same complete version, including its sequence suffix.
- Restore the workspace manifest and lock from immutable lower bytes on exit.
- Build `codex`, `codex-code-mode-host`, independent `codex-voice-host` and the two prepared tools offline for `x86_64-unknown-linux-gnu`, with `-C target-cpu=native`, release LTO off and Cargo jobs equal to half the available logical CPUs, rounded down and at least one.
- Voice uses the fixed SDK and package-relative runtime.
- These build-only settings do not alter tests.
- Tool source extraction uses fixed `build/cache/build/runner/tools/<Tool>/source` paths, replacing only the selected source tree while preserving archive mtimes and that tool's native Cargo build directory.
- Each tool retains its target directory at `build/cache/target/runner/tools/<Tool>`; Cargo owns freshness across exact prepared source revisions without a helper cache ledger or content identity.
- Every build invokes Cargo with verbose output and retains `build.stdout.log`, `build.stderr.log` and native sccache statistics under `build/tmp/published-output/build-install-<BuildVersion>` for the caller's reuse verification.
- Remove consumed projected executable copies without deleting that evidence; explicit drained Cleanup owns its eventual removal.
- A new build of the same unpublished version replaces only its prior temporary evidence under CargoLock.
- An ordinary completed-package retry leaves that evidence unchanged.
- Explicit draft replacement rebuilds the selected version and replaces its temporary build evidence under the same lock.
- A successful package or CargoManaged materialization alone does not prove reuse; consume current Cargo Fresh/Compiling lines and native statistics, separating first-use cache population from a subsequent warm build.
- The package contains `bin/codex`, `bin/codex-code-mode-host`, `codex-path/rg`, `codex-path/shuck`, `codex-path/tokei`, `codex-resources/bwrap`, `codex-resources/voice/bin/codex-voice-host`, the matching private voice runtime, and canonical `codex-package.json`.
- Both inspection-tool commands share Codex's package PATH directory.
- Publication and retry share exact manifest bytes, inventory, modes, private voice-runtime bytes and GNU ELF checks.
- CLI, voice and tool version checks run in the existing native system namespace with networking disabled and the complete package mounted read-only.
- Reuse RunnerEnv's logical home-path admission and system mounts, with private writable HOME, CODEX_HOME, CODEX_HARNESS and temporary mounts removed by process exit.
- Clear the caller environment and never expose its configuration, credentials, runtime directories or loader overrides to package execution.
- Preserve bounded stdout, stderr, execution errors and the existing version comparisons through the bounded command owner.
- Metadata exposes lowercase `version`, `target` set to `x86_64-unknown-linux-gnu`, and `entrypoint` set to `bin/codex`, alongside tool names and semantic versions.
- A passing CLI version check does not replace the daemon-required manifest fields.
- Successful rename records `PackagePublished`; later fsync or response failure retains that effect.
- Ordinary retry prepares inputs and revalidates the immutable BuildVersion package without rebuilding or republishing it.
- Input blocks, product build fails Product, and helper execution, publication, or cleanup fails Helper; remove only incomplete stage and retain reusable materialization and completed packages.
- Sequence zero starts a release; each later build uses its greatest canonical sequence plus one and appends that decimal.
- Sequence selection reads only typed SchemaVersion, RecordType, ReleaseVersion and explicit BuildSequence from prior metadata.
- Prior tool inventories and executable admission do not govern sequence ownership.

## Selected Draft Replacement

- `--replace-draft` requires `--install` and explicit user selection of same-version draft iteration.
- Product work establishes draft-only source authority under ReleaseRollForward before invoking this option.
- Require the selected `current` link to name the supplied existing BuildVersion.
- Validate that package with its retained semantic tool versions before building.
- Build and validate a fresh package in the ordinary stage with the same complete semantic version.
- This option bypasses new-version sequence allocation only for that selected existing package.
- The native selection asset exchanges the stage and selected package atomically.
- The current link remains unchanged throughout replacement.
- Observe the exchanged directory through the original stage's native file identity before reporting PackagePublished.
- A failure before exchange preserves the old package.
- A failure after exchange retains the published effect in the result.
- An unreadable publication postimage reports `Unverified` rather than claiming no effect.
- Cleanup removes only the invocation's stage, which contains the old package after exchange.
- Other package versions remain untouched.
- Repeating this explicit option requests another build rather than an immutable-package retry.
- No source revision, alternate version suffix or replacement receipt enters the package or runtime identity.

## Install

- Ordinary install retains each immutable `build/dist/BuildVersion` package.
- The existing CargoLock serializes package publication and selection.
- The native `build-select.zsh` asset atomically selects relative `build/dist/current -> BuildVersion`.
- Selection records `CurrentLinkChanged` and returns `BuildVersion` as `CurrentTarget`.
- Selection fsyncs the package and dist directories without invoking a product binary.
- Existing package versions remain untouched unless the selected draft replacement above was explicitly requested.
- Pre-publication failure preserves the prior target; later failure retains the published effect.
- After successful dist selection, check for the caller-home `CODEX_HOME/release` entry without following it.
- If absent, tell the user to create `CODEX_HOME/release -> <effective working root>/build/dist/current` themselves.
- Pause until the user confirms creation before validating the new entry.
- Do not request caller-home write permission or create or repair the link.
- Once present, verify that the entry is a symlink to `<effective working root>/build/dist/current`.
- Verify that `CODEX_HOME/release/bin/codex` resolves to the selected package's admitted executable.
- This host-side check resolves paths only; consume package admission's isolated startup proof without executing the launch entry again in the caller environment.
- Existing entries go directly to verification without repeating setup.
- A mismatch waits for user correction and subsequent verification of the changed entry.
- Stop only after the launch-entry path verifies; helper Pass alone proves only dist selection.
