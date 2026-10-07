# BuildInstall

## Admission

- Invoke installed `build-install` alone using parser-owned `--help` for RouteEntry's selected effect on clean registered MainWorktree.
- Unrelated probes and skill maintenance follow the product terminal.
- Caller-home release links are user-owned; the helper's dist selection does not depend on them.
- Dist selection neither reads nor writes caller-home release links.
- Package-only leaves `build/dist/current` unchanged.

## Build And Package

```
BuildInstallInspectionToolOwner
+-- Consumer: RemoteToolRevisionConsumer
+-- Successor: OfflineLocalToolPackageSuccessor
`-- Terminal: AtomicCurrentInspectionToolTerminal
```

- Under CargoLock, prepare three bare remote clones at `PROJECT_SCRATCH_ROOT/tool-sources/{shuck,tokei,ccusage}.git`: latest semantic shuck/tokei releases and default-branch HEAD of `https://github.com/92645417d9e5c763259dbebc306e3e/ccusage.git`. Archive exact commits, validate versions and locks, and fetch locked dependencies with the fixed toolchain. ccusage `package.json`, `rust/Cargo.lock` and `flake.lock` select its version, workspace and pinned LiteLLM pricing JSON. Retain clones, never consume local `build/ccusage` edits or caller-supplied tool binaries.
- Before package admission, match Main's locked V8 version to the GNU toolchain. Authenticate its remote two-file checksum manifest against Main's pinned release-manifest digest, then verify the existing archive and bindings. Keep no checksum state and never substitute a target or download unverified build inputs.
- Build only in ProjectedRunner, keeping Main at 0.0.0 and stamping the full BuildVersion once in the private overlay's workspace.package version, inherited by all workspace crates including CLI.
- Package directory, manifest, compiled `codex --version`, TUI status line and other runtime version reports use that same complete version, including its sequence suffix.
- Pass Main HEAD as STABLE_GIT_COMMIT and restore the workspace manifest and lock from immutable lower bytes on exit.
- Build `codex`, `codex-code-mode-host`, independent `codex-voice-host` and the three prepared tools offline for `x86_64-unknown-linux-gnu`, with `-C target-cpu=native`, release LTO off and Cargo jobs equal to half the available logical CPUs, rounded down and at least one.
- Voice uses the fixed SDK and package-relative runtime; ccusage uses prepared pricing/version, and these build-only settings do not alter tests.
- Tool source extraction uses fixed `build/cache/build/runner/tools/<Tool>/source` paths, replacing only the selected source tree while preserving archive mtimes and that tool's native Cargo build directory.
- Each tool retains its target directory at `build/cache/target/runner/tools/<Tool>`; Cargo owns freshness across exact prepared source revisions without a helper cache ledger or content identity.
- Unchanged ccusage pricing retains its existing file bytes and mtime at the fixed tool build path.
- Every build invokes Cargo with verbose output and retains `build.stdout.log`, `build.stderr.log` and native sccache statistics under `build/tmp/published-output/build-install-<BuildVersion>` for the caller's reuse verification.
- Remove consumed projected executable copies without deleting that evidence; explicit drained Cleanup owns its eventual removal.
- A new build of the same unpublished version replaces only its prior temporary evidence under CargoLock; an identical completed-package retry leaves that evidence unchanged.
- A successful package or CargoManaged materialization alone does not prove reuse; consume current Cargo Fresh/Compiling lines and native statistics, separating first-use cache population from a subsequent warm build.
- The package contains `bin/codex`, `bin/codex-code-mode-host`, `codex-path/rg`, `codex-path/shuck`, `codex-path/tokei`, `codex-path/ccusage`, `codex-resources/bwrap`, `codex-resources/voice/bin/codex-voice-host`, the matching private voice runtime, and canonical `codex-package.json`. All three tool commands share Codex's package PATH directory.
- Publication and retry share exact manifest bytes, inventory, modes, private voice-runtime bytes and GNU ELF checks.
- CLI, voice and tool version checks run in the existing native system namespace with networking disabled and the complete package mounted read-only.
- Reuse RunnerEnv's logical home-path admission and system mounts, with private writable HOME, CODEX_HOME, CODEX_HARNESS and temporary mounts removed by process exit.
- Clear the caller environment and never expose its configuration, credentials, runtime directories or loader overrides to package execution.
- Preserve bounded stdout, stderr, execution errors and the existing version comparisons through the bounded command owner.
- Metadata exposes lowercase `version`, `target` set to `x86_64-unknown-linux-gnu`, and `entrypoint` set to `bin/codex`, alongside source/tool provenance.
- A passing CLI version check does not replace the daemon-required manifest fields.
- Successful rename records `PackagePublished`; later fsync or response failure retains that effect.
- Retry prepares inputs and accepts only an identical existing package without rebuilding or republishing it.
- Input blocks, product build fails Product, and helper execution, publication, or cleanup fails Helper; remove only incomplete stage and retain reusable materialization and completed packages.
- Sequence zero starts a release; each later build uses its greatest canonical sequence plus one and appends that decimal.
- Sequence selection reads prior typed metadata without requiring those immutable packages to pass current executable admission.

## Install

- Install retains each immutable `build/dist/BuildVersion` package.
- The existing CargoLock serializes package publication and selection.
- The native `build-select.zsh` asset atomically selects relative `build/dist/current -> BuildVersion`.
- Selection records `CurrentLinkChanged` and returns `BuildVersion` as `CurrentTarget`.
- Selection fsyncs the package and dist directories without invoking a product binary.
- Existing package versions remain untouched.
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
