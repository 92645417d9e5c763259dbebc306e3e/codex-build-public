# RunnerEnv

## Projection

```
ProjectedRunnerExecutionOwner
+-- Consumer: FrozenWorkspaceExecutionConsumer
+-- Successor: CargoReuseAndTemporaryOutputSuccessor
`-- Terminal: CompactRunnerTerminalConsumer
```

- ProjectedRunner binds read-only source, private runtime/result, fixed Cargo/toolchain inputs, and no Git.
- FullSuiteAudit owns RustSuite rounds; RunnerEnv transports one native producer, process, attempt, deadline, and reap without inferring retries.
- BuildInstall package input lives at build/tmp/published-output/AttemptIdentity until consumption, separate from diagnostic raw output.
- Only installed RunnerExec, RustSuite, and Finalize execute Rust; RepositoryFormatting owns writable just fmt and PatchReplay compared rustfmt.
- Config-schema uses the same Runner projection, CargoLock, toolchain, deadline, custody, and cleanup. It transports one declared overlay fixture while real source stays read-only; it is neither a test nor generic writable mode.


```
CallerCodexHomeProjectionOwner
+-- Consumer: SelectedHomeInputConsumer
+-- Successor: PrivateSamePathMountSuccessor
`-- Terminal: ReapedHomeRemovalTerminal
```

- Read BuildInstall's required `--codex-home` or other callers' CODEX_HOME once per projection. Require a clean absolute path that does not overlap another projected mount. A descendant of the private HOME mount is admitted after that parent mount.
- Export that exact CODEX_HOME value and bind the empty attempt stage/codex directory at the same sandbox path. Never read, copy, or bind the caller home contents, and never replace a missing value with a hardcoded host path.
- CODEX_HARNESS retains its existing empty attempt mount. Tests may override their own environment values within the same filesystem namespace.
- Product startup and helper verification select private storage through this same-path mount boundary.
- Preserve the logical CODEX_HOME and CODEX_HARNESS values rather than creating short directories or substitute roots.
- A socket path-length failure must be demonstrated before selecting a namespace mount correction.
- That correction retains the same logical roots, process owner and attempt cleanup.
- [BuildInstall](/codex/skills/codex-build/commands/build-install.md) package identity queries share the logical home admission and native system namespace.
- Their bounded command owns private writable mounts until process exit, without a host temporary-home lifecycle or access to caller data.
- Build and test projections retain their existing attempt-backed mounts and cleanup.
- A projection or fixture discrepancy is evidence to compare with this execution contract and the product entrypoint's contract before selecting a repair.
- Projected paths and test-local environment values do not assign directory creation, fallback, or startup responsibilities to product code.
- Keep product findings in the existing release task; only an independently demonstrated defect in this skill's projection instructions or implementation belongs in a skill Topic.

## Admission

```
GenericTestConcurrencyOwner
+-- Consumer: BoundedNextestSelectionConsumer
+-- Successor: FixedConcurrencyExecutionSuccessor
`-- Terminal: UnmaskedThreadExhaustionTerminal
```

- Generic RunnerExec repository tests use the existing sixteen-test default also used when establishing a RustSuite profile, rather than Nextest's host-CPU-derived default inside the fixed sandbox.
- A frozen invocation may select one explicit positive concurrency from 1 through 16 with Nextest's --test-threads, --jobs, or -j spelling. Reject duplicate, unbounded, CPU-relative, missing, or larger values before projection instead of clamping them or rewriting argv.
- Publish the selected value through NEXTEST_TEST_THREADS for the existing preparation and final just invocation, preserving selectors, repository local groups, explicit profiles, timeout policy, and all test assertions. Cargo build jobs are a separate option and remain unchanged.
- Keep this selection only as execution scheduling input; it does not predict temporary bytes, inodes, child threads, memory, or another resource.
- Runner imposes no process-count limit; host and cgroup limits govern. Preserve child/thread exhaustion evidence without reclassification, retry, or product change.

```
RepositoryTestNetworkCapabilityOwner
+-- Consumer: CargoOfflineTestExecutionConsumer
+-- Successor: ComparableNetworkEnabledRustSuiteSuccessor
`-- Terminal: UnmaskedProductTestEvidenceTerminal
```

- Validation precedes effects; current Terminal binds Add and replaces noncurrent input.
- Admit clean role-bound worktrees; Input blocks, endpoint is Helper, payload is Product.
- Missing output rejects.
- Keep missing-package diagnostics; drift is Input, Cargo build Product, incomplete binding Helper.
- Tests use `/toolchain/bin/just test`.
- Ordinary focused gates pair one `--test-source` with one or more repeatable `--test-name` values, never package-only argv.
- Before freezing an invocation, group the selected exact names for the same source commit, Cargo target and owning Patch into one preparation and execution.
- Do not split that group into repeated or parallel RunnerExec invocations or combine different sources, targets or Patch responsibilities.
- An explicit user-selected package gate or a separately mandated whole-package gate uses `--package-gate` with frozen just argv.
- The complete suite alone uses `--complete-suite`.
- RunnerExec preserves selector bytes/order; it rejects other just paths/recipes, direct Cargo, cargo-nextest, shells, and mutable executables.
- Tests keep Cargo offline with shared network; frozen checks preserve declared networking.
- NativeToolchain matches manifest/projected just; absence/mismatch is Helper without Cargo fallback.

```
FocusedRunnerBinaryPreparationOwner
+-- Consumer: SelectedPackageBinaryContractConsumer
+-- Successor: ExactExecutableBindingsSuccessor
`-- Terminal: UnchangedFocusedTestInvocationTerminal
```

- Every admitted generic just test, focused or explicitly complete, prepares its required programs inside the existing projection before selected tests execute.
- Before process records or an attempt, one locked offline Cargo metadata query matches the supplied Source-relative regular, no-follow root file, never a module file, to exactly one enabled workspace library, binary, or integration-test target. A mismatch rejects without compilation or tests and reports up to eight sorted metadata target roots from the uniquely owning workspace package; these are suggestions, never substituted selectors.
- Metadata derives package, target, and required-feature flags for one exact just test argv. Go admits the source and fully qualified native names with codex-rs workdir and no argv or complete-suite; the private query checks file and manifest. Query failures keep bounded stderr in the existing pre-execution diagnostic.
- Cargo metadata does not expose harness configuration; read that target's owning Cargo.toml declaration and Cargo's default harness rule. Metadata identifies every library crate type as a library target. The private selection query and projected execution consume one remaining request window, with cancellation active before selection and no persistent timing field.
- Cargo metadata proves the target but not its native test names. The caller derives the qualified names from source modules.
- Focused preparation compiles and lists the selected target once through native JSON `nextest list`.
- Require every requested exact name in that listing before preparing binary dependencies or running any selected test; an absent name is Product no-tests (exit 4).
- Preserve input name bytes and order in the existing frozen argv and let native Nextest exact-name selection execute the group once.
- Package and complete preparation retain no-run. Both retain the selected local or explicit profile, package, features, target, and filters; strip only preparation output, no-tests, fail-fast, max-fail, debugger, and tracer controls.
- Cargo metadata owns workspace package and binary identities. Each selected package's literal codex_rust_crate extra_binaries and Linux-applicable extra_binaries_non_windows declarations in BUILD.bazel own additional binary dependencies. Its own enabled binary targets are also required. Missing, ambiguous, dynamic, or non-workspace declarations stop as Helper rather than being ignored or replaced by a helper-name registry.
- Reuse only executable artifacts reported by this current preparation, never guessed cache paths or host programs. One fixed Cargo build produces the remaining required targets with the selected build context and existing offline Cargo custody. Cargo JSON must account for every required package and binary, with unique executable paths and environment names.
- Bind exact executable paths under both original and underscore forms of CARGO_BIN_EXE and NEXTEST_BIN_EXE, then execute the original just argv once. No failed-test discovery, per-error binary fix, broader test run, baseline execution, or retry is added.
- The existing CargoLock, sccache server, private mounts, process group, total deadline, snapshot producer, and reap own the whole sequence. Complete metadata and compiler output remain private files under that attempt's build/tmp/runner-output root and follow its existing cleanup consumer.
- Native runner-test-preparation.json retains only the current Phase, FailureDomain, and bounded Diagnostic until terminal sealing. Preparation command construction, argument rejection, metadata, contract, artifact, or binding failures are Helper TestPreparation. Only a structured Cargo compiler error establishes a Product compilation failure; a nonzero preparation exit alone does not. Timeout, cancellation, and source-postimage findings retain precedence, and a preparation failure does not attest that selected tests ran.

```
RunnerSnapshotCaptureOwner
+-- Consumer: SemanticSnapshotPathConsumer
+-- Input: admitted test selection and immutable source commit
+-- Derive: native generated paths after selected tests execute
+-- Preimage: exact lower-tree external bytes or inline spans
+-- Execute: shared Complete capture inside the private overlay
+-- Reject: invalid generated paths spans or non-snapshot source changes
+-- Successor: ExactCurrentPreimageSuccessor
`-- Terminal: DeclaredSnapshotUpdatesTerminal
```

- Every admitted RunnerExec repository test, including exact-test and package selections, and lane Finalize use the existing Complete collector. Tests determine generated output, not a caller path list or capture flag. After execution it discovers direct external `.snap`, adjacent `.snap.new`, and inline pending output under `codex-rs`, including first-time external snapshots. No pretest repository inventory, target manifest, or target-bound producer remains.
- Complete computes updates without rewriting overlay source. Inline normalization follows Insta; latest per-line observations span all same-source Nextest runs. Changed staging survives until reap. No-op staging is discarded. Go checks staging and exact lower spans. Unrelated source writes remain Product. Collector failure leaves no manifest and is Helper, preserving test exit, sccache and its diagnostic.
- Only captured snapshot mismatches use fixed Insta update controls. Ordinary assertions, compilation failures, test selectors, retries, timeouts, cancellation, and incomplete execution keep their existing meaning. Capture is an internal test-only opt-in; baseline, BuildInstall and other non-test projections leave it off. Finalize excludes Established non-pass tests and publishes its own result without another acceptance command.
- Snapshot files and inline source preimages retain exact UTF-8 bytes. Records owns the canonical JSON encoding of their structured descriptors and generated manifest; snapshot bodies are never parsed as JSON.
- Focused RunnerExec owns private evidence only. Finalize alone publishes the mechanical tail from its exact source-bound Attempt, with class-compatible Established outcome comparison and novel failures rejected. RunnerExec never changes product source or Git history.

```
BaselineSnapshotAssertionOwner
+-- Consumer: UpstreamBaselineExecutionConsumer
+-- Successor: OrdinarySnapshotMismatchTestFailureSuccessor
`-- Terminal: NoRustSuiteSnapshotInventoryTerminal
```

## Cargo And Temporary Storage

- Cold selection initializes materialization. A compatible unchanged SourceSelection uses ReusedWorkspace without advancing generation or invoking cargo clean, including revised bytes at the stable projected workspace path.
- Cargo owns file freshness and rebuilding within that workspace. A changed physical workspace selection still advances generation and invalidates workspace-member artifacts before use, preserving cross-worktree isolation. Incompatible toolchain or target state rejects, and no helper fingerprint, source copy, or automatic retry is introduced.
- EffectiveReuse CargoManaged denotes compatible workspace retention, not an invented cache-hit count. Other retained-cache claims still require the current sccache statistics.
- Prepared attempts reuse handoff while Cargo home, targets, sccache, and toolchain match.
- Materialization retains Cargo identity, action, invalidation cause, and reuse. Only absent state checks for unowned cache bytes; attempts persist no size, growth, timing, or free-space observations.

```
CargoInvalidationDiagnosticOwner
+-- Consumer: WorkspaceInvalidationProcessConsumer
+-- Successor: BoundedPreparationFailureSuccessor
`-- Terminal: PrivatePreparationOutputConsumer
```

- Workspace invalidation keeps its single fixed Cargo command, CargoLock, frozen source, and remaining deadline. Failed invalidation never publishes the new Cargo identity or starts the requested test.
- Write complete invalidation stdout and stderr to private cargo-invalidation.stdout.log and cargo-invalidation.stderr.log files under the same attempt runner-output root. Reuse the bounded stream-tail collector, retain output on failure, and let the existing resolved-output consumer remove the directory once.
- A prerequisite failure reports CargoPreparation/WorkspaceInvalidation, the process error, at most 2048 bytes of stderr-preferred diagnostic text, stream byte counts, command arguments, working directory, and temporary output location through the existing bounded Diagnostic field. Public EvidencePaths includes the existing temporary output directory even when no test process or durable Runner terminal exists.
- Output creation, writing, drain, or closing errors remain visible Helper failures. Missing output never becomes an empty successful stream, existing files are not overwritten, and source-postimage, timeout, and cancellation meaning is unchanged.
- A failed prerequisite retains the same focused-check successor without a retry, direct Cargo probe, new record format, inferred manifest diagnosis, or recovery command. The product consumer diagnoses the retained output before its exact cleanup.

```
GuestTemporaryStorageOwner
+-- Consumer: TemporaryPathExecutionConsumer
+-- Successor: IsolatedBuildOverlayTemporarySuccessor
`-- Terminal: ExactAttemptTemporaryRemovalTerminal
```

- Bubblewrap overlays guest `/tmp` from empty lower, upper, and work directories under one build/tmp attempt. This keeps a distinct guest mount while all temporary bytes remain build-backed. `TMPDIR` names `/tmp` directly; the exact attempt reaper removes its backing directories, while failed raw output retains its separate owner.
- RuntimeTemporaryRoot retains home, codex, harness, run, and /dev/shm. No caller host `TMPDIR` or external mount is selected.
- Before a new complete Rust test process, require build/tmp available bytes strictly greater than 100 GiB. A failed read or shortfall returns Helper with available bytes when known, without sealing a new test terminal; a retained Plan stays retryable. No forecast, record, post-failure classification, or extra retry is added.
- Failures stay with their operation: payload is Product, Runner write/drain/sync/close is OutputPublication, and errno creates no reclassification, retry, or snapshot.
- Cancellation, timeout, source-postimage, preparation, and output-publication owners retain their existing typed precedence. No alternative runner, compatibility reader, migration, or passing replacement is inferred from old records or logs.
- One server publishes private `sccache-stats.json`.
- Hits prove reuse.

## Process And Terminal

```
ImmediateLongRunNoticeOwner
+-- Consumer: AdmittedSynchronousCommandConsumer
+-- Successor: SameHandleNativeWaitSuccessor
`-- Terminal: TerminalJSONOnlyCompletionTerminal
```

- One formatter emits Starting after admission and before work, then phase, bounded native test-start progress, and completed-round events. Each line contains operation, observed progress, elapsed time, and its own continuation instruction: wait on the same running handle using the longest permitted native interval capped at twenty-five minutes, without commentary; only terminal JSON completes.
- These event-bound lines are the progress report, not a request for another status message. No timer, heartbeat, raw-log stream, or automatic inspection is introduced. A long-running test may remain silent; existing explicit inspection retains its owner.
- A shorter host limit uses that maximum repeatedly on the same handle. The cap bounds caller waiting, not helper execution, and fits the requested thirty-minute cache window. The formatter cannot override higher-priority caller instructions.

```
SingleRustSuiteTotalDeadlineOwner
+-- Consumer: PreflightAndRunnerPhaseConsumer
+-- Successor: RemainingDurationExecutionSuccessor
`-- Terminal: TypedNonProductTotalTimeoutTerminal
```

```
OwnedAttemptRemovalOwner
+-- Consumer: ReapedPrivateAttemptCleanupConsumer
+-- Successor: NoFollowDirectoryPermissionPreparationSuccessor
`-- Terminal: ExactOwnedAttemptAbsenceTerminal
```

- Explicit Cleanup shares this private-directory removal boundary after CargoLock acquisition, exact root inventory and process/reader drain. It removes only admitted children of RuntimeTemporaryRoot, build/tmp and ephemeral RecordRoot; it leaves those roots empty and preserves tasks/rust-suite-baselines.json. Native directory access repair never follows a link or crosses a nested mount. No additional cleanup identity or lifecycle record exists.

- RunnerEnv supervises the native producer's original round deadline and terminal cleanup. Unsealed Finalize reentry retains that deadline, native output, and overlay under the same request and source; time never resets. Expiry seals Stopped Helper.
- Release-baseline retains that same process through cleanup.

```
RustSuiteLiveEvidenceOwner
+-- Consumer: CurrentProcessPhaseAndTemporaryOutputConsumer
+-- Successor: PublishedPassOrDiagnosedReplacementCleanupSuccessor
`-- Terminal: BoundedPublicEvidenceAndCanonicalRecordsTerminal
```

- While holding CargoLock, RustSuite replaces only the current Phase in its existing process LockRecord, including Cargo preparation, runner startup, compilation, test execution, drain, and terminal publication.
- Native compilation, test entry, and completed test rounds send phase events through the existing attempt FIFO. During each round, the Nextest JSONL stream is copied unchanged to its canonical output while at most twenty observed test-start milestones enter the FIFO, spaced by the discovered non-ignored test count. These counts include retry starts and do not claim completion or Pass.
- The parent formats each milestone's observed starts and round ordinal, then each completed round's ordinal and exit status with elapsed time. Its durable Phase stays TestExecution; neither notice ends the helper or transfers scheduling to the agent.
- The FIFO is transient IPC removed on reap, with no polling or separate progress record.
- Explicit inspection selects the current `baseline`, `full`, or `focused` process and output set. Stage or Runner result names that same process. Read only its LockRecord Phase, output sizes and mtimes, aggregate `nextest.jsonl` outcomes, latest eight test identities, and last 32 stderr lines capped at 8192 bytes; never scan directories or infer another execution from a branch name.
- Never follow, watch, sleep, poll, dump, mutate, cancel, clean, duplicate, change deadlines, or infer completion. It creates no record; resume the same silent handle. Each later explicit request admits one new snapshot.
- Every projection writes complete stdout, stderr and native rerun records directly under build/tmp/runner-output for its attempt, without raw-log caps or archive copies. Non-log declared outputs retain their existing consumers.
- RunnerExecResult retains only a structured FailureSummary with byte counts, reason, and at most 2048 diagnostic bytes for non-Pass terminals. Public output includes this bounded summary and existing evidence paths, never complete logs. Missing output is explicit rather than counted as an empty successful stream.
- Terminal publication validates available outputs before sealing. Retained terminal acceptance then uses its sealed process and summary evidence, so consuming temporary logs cannot invalidate the terminal or authorize another execution.
- RunnerExec replaces its current focused set only before another selected check, under CargoLock after proving the prior process absent. Success and failure both retain current logs. Schema Finalize keeps its separate transient bundle until native consumption.
- The same temporary output root retains raw Nextest and preparation output through Stage sealing and Result publication.
- Published Pass and failure retain current test logs equally. A fresh full request replaces only `full`; `--resume` instead consumes that existing request and source without resetting any round budget. A fresh focused request replaces only `focused`. Baseline replacement consumes only `baseline` and never advances Established before Pass.
- Public RustSuite results contain bounded diagnostics, phase, elapsed duration, terminal, counts, and validated record locators rather than raw logs or test inventories.
- Baseline fixes `INSTA_UPDATE=no`; repository tests and Finalize share automatic capture, while only Finalize publishes generated source for acceptance.
- Timeout/cancellation terminate directly; missing statistics remain typed.
- Post-exit drain never waits on stream-retaining descendants.
- Live Add blocks another process. After proven absence, unsealed Finalize with native round evidence resumes the same AttemptID; its current process receipt is replaced under CargoLock. Other incomplete attempts seal and clean. Sealed receipts never change.
- Interrupted Finalize retains its private overlay and native records until continuation or exhausted-budget sealing. Timeout and cancellation publish no partial snapshot. The exact no-follow reaper removes scratch after terminal publication; raw output keeps its separate consumer.
- Preparation failure is Helper within the attempt.
- Finalize retains current output for continuation and publication. A new selected check replaces its same-purpose set after reap; an old terminal never grants or denies editing permission.
- Records owns three current diagnostic sets during normal operation: baseline Plan/Stage/Result and full/focused Runner results, each with current process receipts and logs. Same-purpose replacement consumes its old set. Explicit drained Cleanup consumes all three sets, preserving only tasks/rust-suite-baselines.json as baseline comparison facts.

```
RetainedRustSuiteChainAdmissionOwner
+-- Consumer: ExistingOrMissingFixedResultConsumer
+-- Successor: AdmittedFixedStageCompletionSuccessor
`-- Terminal: ValidatedRetainedResultTerminal
```

- Existing and missing Result share fixed source, source-role, toolchain, Plan, Stage, applicable Established-selection, and attempt-absence admission before Result consumption or effects.
- An existing Result must equal the Result derived from the admitted Plan and Stage.
- Missing Result invokes the existing Stage finisher only after admission, preserving Baseline promotion and deadline gates before unchanged coverage without product rerun, suffix workflow state, or another budget.
- RunnerExec public output omits preimages, generated bytes, process streams, and the complete durable Terminal.
