# NativeToolchain

- `<effective working root>/build/toolchains/native/toolchain` is the single published Rust, Cargo, Bazel launcher, just, DotSlash, uv, sccache, cargo-nextest, V8, and native voice root; `native-toolchain.json` selects its component versions.
- `bin native-toolchain` builds the complete composite root from an empty project.
- The fixed base contains isolated Rust and Cargo 1.95.0 with clippy, rustfmt, and rust-src.
- The published rustfmt executable is NativeToolchainRoot/rust/bin/rustfmt. PatchReplay derives this fixed child from its project working root for per-file transport normalization; callers never select another formatter path.
- The fixed base contains cargo-nextest 0.9.131, sccache 0.15.0, and rusty_v8 150.4.0.
- The fixed base contains Bazelisk 1.29.0 at `bin/bazel`; the repository `.bazelversion` selects Bazel 9.0.0 during bounded RepositoryMaintenance acquisition and the offline lock recipe in one private attempt.
- The fixed base contains the declared just, DotSlash, uv, and locally built GStreamer releases.
- Rust bootstrap uses the official target rustup initializer only inside the preparation attempt, never reads or writes host Rust state, and copies the selected installed toolchain into the staged fixed root.
- The producer downloads official fixed release payloads directly into its attempt, validates semantic versions and required regular postimages, writes one semantic-only `native-toolchain.json` after the complete payload exists, and then publishes once by first-root rename or replacement-root directory exchange.
- A standalone executable download is copied into an exclusively created pending file under the publication stage, validated at that staged postimage, and renamed only within the publication filesystem.
- cargo-nextest admission parses the semantic release from its leading version line and ignores only additional release metadata lines; the parsed release, fixed target archive, executable postimage, and manifest field must still match exactly.
- Staged sccache admission executes the pending payload with canonical `sccache` argument-zero, reports a bounded execution diagnostic separately from semantic release mismatch, and requires exact release 0.15.0 before the same-filesystem rename.
- Every staged manifest write replaces a private staged file, so hard-linked unchanged payloads cannot expose a component selection in the published root before the directory exchange.
- The single immutable toolchain root contains `voice/sdk` and `voice/runtime`; one native build supplies the matching target, relocatable GStreamer development inputs, private GStreamer runtime dependency closure, required plugins, and matching plugin scanner.
- GStreamer source construction disables host ccache and sccache in its isolated environment, so the fixed system compiler uses no ambient compiler-cache daemon, socket, or cache state.
- Native admission requires the fixed GStreamer version from `native-toolchain.json` and invokes the SDK inspector with the packaged scanner, private plugin directory, and nonpersistent registry to prove every required plugin is discoverable.
- The producer writes fixed just, DotSlash, and uv versions to `native-toolchain.json` only with their matching regular executables in `bin`; shared NativeToolchain admission compares every field with the executable before RepositoryFormatting, RunnerExec, RustSuite, or another projected consumer starts.
- The producer writes the fixed Bazelisk version only with its matching regular `bin/bazel`; shared admission compares the launcher before RepositoryMaintenance checks the exact repository Bazel selection.
- FinalProductRustSuite requires Cargo build-directory capability, cargo-nextest build-directory metadata capability, and the complete target-compatible voice input before it creates a plan.
- RustSuitePlanRecord uses the project-relative `build/toolchains/native/toolchain` locator, Cargo version, and Rust target directly.
- ProjectedRunner consumes the complete verified fixed toolchain through read-only in-sandbox paths and never selects, updates, replaces, or falls back from it.
- Voice compilation uses host pkg-config with only the projected SDK followed by host ABI metadata required by GLib, ALSA, and Opus, while voice execution uses only the projected private GStreamer runtime and plugin directory plus those unchanged host ABI libraries.
- Host GStreamer, package exclusion, a second toolchain root, and an optional voice route are not admitted.
- No CI-version mirror, host-PATH substitute, host installation, generated setup script, second toolchain lane, or tool-specific runner route is admitted.

```
CanonicalRepositoryCommandToolchain
+-- Owner: NativeToolchainPublicationOwner
+-- Consumer: RepositoryCommandToolchainConsumer
+-- Successor: RepositoryFormatAndTestSuccessor
`-- Terminal: CanonicalRepositoryCommandToolchainTerminal
```

```
NativeToolchainFromScratch
+-- Owner: NativeToolchainBootstrapOwner
+-- Consumer: EmptyNativeToolchainRootConsumer
+-- Successor: CompleteCompositeToolchainPublicationSuccessor
`-- Terminal: FromScratchNativeToolchainTerminal
```

```
NativeToolchainRecoveryOwner
+-- Consumer: FailedConsumerAdmission
+-- Successor: PrepareNativeToolchainSuccessor
`-- Terminal: OriginalFrozenInvocationTerminal
```

- Before any projected product command starts, missing or invalid fixed-root admission selects the existing from-scratch producer as that command's sole support predecessor.
- One passing producer result is consumed by the shared admission, then the caller starts the unchanged frozen product invocation once. The failed admission did not consume the product attempt and does not authorize a second product execution.
- Producer failure, producer-output mismatch, or a second shared-admission failure returns the named Helper stop. It never selects host tools, another root, consumer-owned acquisition, a partial refresh, or a reconstructed invocation.

```
DeclaredNextestRelease
+-- Owner: NativeToolchainReleaseReportOwner
+-- Consumer: DeclaredNextestReleaseConsumer
+-- Successor: ExactSemanticVersionAdmissionSuccessor
`-- Terminal: DeclaredNextestReleaseTerminal
```

```
SccacheExecutableIdentity
+-- Owner: SccacheExecutableIdentityOwner
+-- Consumer: StagedSccacheVersionConsumer
+-- Successor: CanonicalArgumentZeroAdmissionSuccessor
`-- Terminal: SemanticSccacheReleaseTerminal
```

```
NativeVoiceBuildEnvironment
+-- Owner: NativeVoiceBuildEnvironmentOwner
+-- Consumer: GStreamerCompilerConsumer
+-- Successor: HostCompilerCacheDisabledSuccessor
`-- Terminal: IsolatedVoiceBuildEnvironmentTerminal
```

```
StandalonePublicationFilesystem
+-- Owner: StandalonePublicationFilesystemOwner
+-- Consumer: ValidatedStandalonePayloadConsumer
+-- Successor: SameFilesystemStageRenameSuccessor
`-- Terminal: AtomicStandalonePublicationTerminal
```

- The producer alone may acquire the fixed component payloads; both projected repository command consumers execute offline and have no host-PATH fallback, host installation, Cargo substitution, second root, or consumer-owned provisioning.
- The public endpoint returns `NativeToolchainResult` Pass only from a clean Prepared producer result whose exact fixed root passes the shared admission used by its repository command consumers; a producer diagnostic, shared-admission failure, or producer-output mismatch is a distinct Helper diagnostic and starts no product command.
