# NativeToolchain

- `<effective working root>/build/toolchains/native/toolchain` is the single published Rust, Cargo, Bazel launcher, just, DotSlash, uv, sccache, cargo-nextest, V8, and native voice root.
- `native-toolchain.json` records the prepared semantic component versions without a source revision, release suffix, or second selection ledger.
- `bin native-toolchain` consumes the selected release's registered source and builds the complete composite root from empty project state.
- Skill instructions and implementation contain no fixed tool release numbers or default version selections.
- Resolve Rust and Cargo from the source's `codex-rs/rust-toolchain.toml` and retain its declared components together with the required clippy, rustfmt, and rust-src components.
- The published rustfmt executable is NativeToolchainRoot/rust/bin/rustfmt. PatchReplay derives this fixed child from its project working root for per-file transport normalization; callers never select another formatter path.
- Resolve V8 from the unique `v8` package version in `codex-rs/Cargo.lock`.
- Resolve matching GStreamer core, base and good plugins and the host Opus requirement from `third_party/voice/sources.json`.
- Bazelisk at `bin/bazel` consumes the source's exact `.bazelversion` during bounded RepositoryMaintenance acquisition and the offline lock recipe in one private attempt.
- At each selected release or changed requirement, the product agent reads the source's build declarations and relevant tool capability contracts before invoking preparation or consuming an existing root.
- For the six auxiliary tools, reuse the published version only when the actual executable and required capabilities remain verified for that source.
- When an auxiliary tool is missing or incompatible, select an official stable release satisfying the source and consumer requirements without a user question or a skill-version edit.
- CI examples are evidence rather than mandatory pins when the source imposes no corresponding requirement.
- Supply the resulting exact auxiliary versions through the public native-toolchain arguments for cargo-nextest, sccache, Bazelisk, just, DotSlash and uv.
- Bind source, release and all auxiliary versions directly to the toolchain command's options, independently of release commands' record-root selection.
- Parser-owned help exposes those exact options without executing preparation.
- The producer accepts no caller override of the source-owned Rust, V8, GStreamer or Opus requirements.
- Unavailable or conflicting requirements stop before publication instead of selecting a guessed version, weakening a capability, or falling back to host tools.
- Rust bootstrap uses the official target rustup initializer only inside the preparation attempt, never reads or writes host Rust state, and copies the selected installed toolchain into the staged fixed root.
- The producer downloads the selected official release payloads directly into its attempt, validates semantic versions and required regular postimages, writes one semantic-only `native-toolchain.json` after the complete payload exists, and then publishes once by first-root rename or replacement-root directory exchange.
- A standalone executable download is copied into an exclusively created pending file under the publication stage, validated at that staged postimage, and renamed only within the publication filesystem.
- cargo-nextest admission parses the semantic release from its leading version line and ignores only additional release metadata lines; the parsed release, fixed target archive, executable postimage, and manifest field must still match exactly.
- Staged sccache admission executes the pending payload with canonical `sccache` argument-zero, reports a bounded execution diagnostic separately from semantic release mismatch, and requires the selected semantic release before the same-filesystem rename.
- Every staged manifest write replaces a private staged file, so hard-linked unchanged payloads cannot expose a component selection in the published root before the directory exchange.
- The single immutable toolchain root contains `voice/sdk` and `voice/runtime`; one native build supplies the matching target, relocatable GStreamer development inputs, private GStreamer runtime dependency closure, required plugins, and matching plugin scanner.
- GStreamer source construction disables host ccache and sccache in its isolated environment, so the fixed system compiler uses no ambient compiler-cache daemon, socket, or cache state.
- Shared native admission compares the published Rust, V8 and GStreamer versions with the selected source requirements before every consumer.
- Read Rust's native component inventory as newline-delimited names, retaining its preview aliases and target suffixes.
- Native voice admission compares the SDK modules with that GStreamer requirement and invokes the SDK inspector with the packaged scanner, private plugin directory, and nonpersistent registry to prove every required plugin is discoverable.
- The producer writes auxiliary versions to `native-toolchain.json` only with their matching regular executables in `bin`.
- Shared admission compares every auxiliary version with its executable before RepositoryFormatting, RunnerExec, RustSuite, or another projected consumer starts.
- Shared admission checks the Bazelisk launcher without launching Bazel outside the maintenance sandbox.
- FinalProductRustSuite requires Cargo build-directory capability, cargo-nextest build-directory metadata capability, and the complete target-compatible voice input before it creates a plan.
- RustSuitePlanRecord uses the project-relative `build/toolchains/native/toolchain` locator, Cargo version, and Rust target directly.
- ProjectedRunner consumes the complete verified fixed toolchain through read-only in-sandbox paths and never selects, updates, replaces, or falls back from it.
- Voice compilation uses host pkg-config with only the projected SDK followed by host ABI metadata required by GLib, ALSA, and Opus, while voice execution uses only the projected private GStreamer runtime and plugin directory plus those unchanged host ABI libraries.
- Host GStreamer, package exclusion, a second toolchain root, and an optional voice route are not admitted.
- ALSA and other unchanged host ABI dependencies retain their native build and link requirements rather than a skill-authored version pin.
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

- The producer alone may acquire the selected component payloads; both projected repository command consumers execute offline and have no host-PATH fallback, host installation, Cargo substitution, second root, or consumer-owned provisioning.
- The public endpoint returns `NativeToolchainResult` Pass only from a clean Prepared producer result whose exact fixed root passes the shared admission used by its repository command consumers; a producer diagnostic, shared-admission failure, or producer-output mismatch is a distinct Helper diagnostic and starts no product command.
