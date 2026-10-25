# ProductRustSuite

## Admission

- Baseline uses release-baseline or selected coverage alignment. User-requested complete product tests use Finalize directly on registered Current, whether Main or fork; focused checks use RunnerExec.
- Before closing normalization, consume PatchAcceptance's platform-condition audit against approved version notes and resolve unsupported guards in their owning Patches; passing tests cannot substitute for it.
- No candidate tag, Prefix/Tagged seal, or historical acceptance chain exists. Tests report current findings and cannot gate editing, convergence, export, or installation.

## Canonical Coverage

```
PositiveBaselineCoverageOwner
+-- Consumer: CurrentWorkspaceExclusionDerivationConsumer
+-- Successor: UnitCoveragePromotionSuccessor
`-- Terminal: CoveredPackagesEstablishedTerminal
```

- Only rust-suite-baselines.json represents completed coverage, as one canonical map from release tag to its current-tag Established generation.
- Empty collection or missing selected entry is initial release input only. The selected producer establishes exact zero coverage or publishes inherited coverage after Pass; product consumers require this tag's established entry.
- Every update preserves all other tag entries byte-semantically under CargoLock and the complete collection preimage. Established stores sorted CoveredPackages plus distinct compile-only/helper roles.
- BugfixDiff is the exact native canonical source diff from the official parent through the standalone bugfix prefix, with Git index headers omitted by the existing patch producer.
- Nonempty BugfixDiff is retained in the existing selection and immutable Plan as actual source input, not a digest, revision-derived identifier or separate record.
- An empty diff is omitted and denotes the unpatched official parent.
- Baseline source admission validates canonical bugfix-only history beneath the empty marker; product comparison requires the same bugfix source before normalization.

```
CompletedBaselineFactOwner
+-- Consumer: PlanBoundPendingBaselineAttemptConsumer
+-- Successor: PassOnlyBaselinePublicationSuccessor
`-- Terminal: NoPendingGenerationInBaselineCollectionTerminal
```

- A pending proposal is invocation input fixed by the immutable Baseline Plan, not a baseline-collection generation. Initial inheritance leaves the current tag absent; incremental coverage leaves its current Established unchanged.
- A predecessor Established retains its recorded toolchain facts.
- Bind inherited proposal CargoVersion to the verified target toolchain before Plan publication.
- Only a validated and durably published Pass Result publishes the proposed current-tag selection and observed outcomes as Established. Interruption, Helper failure, cancellation, and replacement admission never retag or otherwise advance Established.
- Reentry consumes the exact retained Plan, Stage, and Result. A retained Pass may finish its still-missing Established publication and cleanup; once Established already matches the proposal, unchanged coverage validates the retained terminal without promoting it twice. No other terminal can promote, infer, or reconstruct a generation.

```
ZeroCoverageGeneration
+-- CoveredPackages: empty
+-- PackageRoles: no compile-only/helper package
+-- Outcomes: no expected failure or timeout
+-- Profile: fixed execution profile
`-- ExecutionEvidence: no Plan Stage Result compile or test
```

- Coverage begins at inherited alignment or exact zero. Cross-tag inheritance drops absent members and binds the proposal to the target's actual bugfix source rather than inheriting the predecessor's diff.
- With unchanged bugfix source, a selected Unit or Flat Patch automatically adds uncovered baseline packages before its Current focused test. Its complete sorted, disjoint package-role proposal retains all existing roles. Only new packages execute. Private-only packages have no baseline comparison. No product suite is implied.
- A shared bugfix addition, change or removal uses baseline-coverage with its owning Flat Patch and complete retained package-role proposal.
- Repeat affected-package for every established package whose behavior or test premises can change, including dependent packages and compile-only roles.
- A changed source without a proven affected subset executes the complete proposed coverage.
- Newly covered packages execute alongside affected established packages; required helper binaries retain their existing preparation ownership.
- Pass replaces only executed or removed package outcomes and carries proven unaffected outcomes forward onto the new source input.
- Source change with zero coverage updates only the source-bound zero selection and does not invent execution evidence.
- Supply required-test for each concrete regression test as its exact package-qualified expectation identity.
- Required tests must be selected, nonignored and observed passing; they cannot be recorded as expected failures or timeouts.
- Preserve the affected selection and required-test inputs in the existing unresolved task until publication closes.
- Unchanged-source reentry consumes retained evidence; source changes produce a different immutable Plan even when package coverage is identical.
- Neither stale Established outcomes nor an old Pass for the same packages establish comparison against changed bugfix source.
- Explicit user-selected `baseline-coverage --align` accepts a complete desired package-role selection at any registered Roll frontier, including an open Patch. Derive additions, removals, and role changes from Patch effects, upstream ownership, and affected dependents, not paths alone. A nonempty target remeasures every selected upstream package in a distinct marked Plan; repeated marked Plans are consumed. An empty target aligns without execution. Neither mode changes Current topology.
- Only validated Pass replaces executed or removed Established outcomes; failure preserves its preimage. Retained Plan, Stage, and Result resume exactly without new generation, retry, or topology effect.

## Product Plan

- The full-suite producer derives its Plan from Established coverage, helper roles, profile, known outcomes, discovery, and tools for its actual worktree. It neither selects a lane-only suite nor regenerates Baseline.
- Selected/ignored tests bind terminals exactly.

```
CompactRustSuiteStageOwner
+-- Consumer: ExactTemporaryExecutionEvidenceConsumer
+-- Successor: CountAndOutcomeSummaryPublicationSuccessor
`-- Terminal: InventoryFreeRetainedStageTerminal
```

- Complete discovery, binary bindings, and per-test events remain temporary native execution input and output, and the execution consumer validates exact identities, duplicate terminals, ignored tests, durations, and selected-package membership before sealing Stage.
- Stage retains preparation, roles, counts, bounded class-separated Failure and Timeout identities, and native validation, without test or binary inventories.
- Failure and timeout counts remain exact even when their combined diagnostic identity sample is limited to 64 entries.
- Pass requires complete validated execution counts and untruncated outcomes with no novel Failure or Timeout relative to Plan. Resolved Established outcomes cannot fail the selected full suite.
- Baseline measures its full admitted selection. Subsequent lane generation excludes every Established failure and timeout by exact package and test name before Nextest discovery and execution. Excluded identities acquire no synthetic test outcome. The same filter applies to every native remaining-set round.
- Lane Finalize alone runs the complete selection without fail-fast, then up to two recorded `--rerun latest` remaining-set rounds. Each round gives failing tests two native Nextest retries. A passing attempt passes. Passed identities remain skipped. Concurrency and repository groups remain unchanged. Non-test errors do not enter native reruns.
- Every round uses the Plan's admitted per-test limit with zero-grace termination.
- Round exhaustion kills the process group, seals incomplete evidence, and never advances to another round.
- Round notices and JSONL retain original deadlines. Full `--resume` continues only unresolved tests within that deadline. Later rounds receive their own allowances. Exhaustion seals failure. Cancellation retains the unfinished set.
- A fresh selected check replaces its same-purpose set. `--resume` is an invocation selector, not a task field or persistent status. It requires unchanged actual source and request. A sealed full result can finish interrupted generated publication but cannot acquire more test budget.
- Cancellation can leave store.zip unfinished. Continuation selects discovery minus JSONL Pass identities through the native profile, not that archive. Completed rounds use native reruns. Recheck shared binaries and discovery within the remaining budget; retain overlay and logs. Nextest exits other than 0/100 are Helper stops, not Product failures.
- Aggregate JSONL admits native retry suffixes and source-bound continuation boundaries. Each executable start requires an unresolved identity; interruption closes no test outcome. Only the final native terminal counts. Unknown names, missing starts, and duplicate terminals reject.
- Retained baseline validation checks its sealed summary against Plan and process evidence. Current logs remain available for explicit inspection, independently of result consumption.
- Discovery owns ignored status for both ordinary and interrupted summaries: an omitted ignored event completes as Skipped, never as a running or timed-out test. Repeated ignored lifecycle events across complementary native invocations collapse to the same Skipped terminal; executable duplicate starts and inconsistent terminals remain invalid outside the exact bounded retry lifecycle.
- Only a native per-test timeout terminal establishes a Timeout outcome. An executable test without a terminal remains incomplete on interruption and is rejected on ordinary completion; neither case manufactures a test result.
- Baseline allows at most 64 failures/timeouts.
- Result reports bounded class-separated novel and unobserved Established differences and validated incomplete counts; failed Finalize projects those same diagnostics into its PatchReplay Product result. Absence alone does not claim a pass for an excluded test.
- Cargo metadata fixes binaries.
- Compile/helper-only coverage skips discovery.
- Helpers precede tests.

```
RepositoryLocalRustSuiteProfileOwner
+-- Consumer: InheritedLocalIntegrationGroupConsumer
+-- Successor: ValidatedBuildTestThreadAndTimeoutOverlaySuccessor
`-- Terminal: RepositoryOwnedFourThreadIntegrationTerminal
```

- The one native producer generates a profile that inherits local while setting only global test threads and the current timeout override; repository Nextest parsing owns every integration group instead of a copied group assertion or second profile producer. Every Cargo compilation in the same RustSuite invocation receives that positive Plan concurrency as its native build-job limit, including the compilation performed by Nextest discovery and execution.
- Diagnose a per-test supervision defect only from a nonignored test's actual start, elapsed time against its effective limit, and native termination evidence. Unmatched started events and total invocation duration alone establish none of those facts.
- RunnerEnv bounds compilation and each test round, then terminal cleanup. Its reaped Stopped Helper terminal retains available outcomes without relabeling unfinished tests as Timeout.
- Before selected Finalize executes, emit one bounded notice from its admitted Established selection and Plan: covered package count, compile/helper roles, known failure/timeout counts, excluded package count, network, concurrency, and per-test/suite windows. This is output, not another selection or record.

- ExecutionProfile and Plan bind TestNetwork to Shared for selected full checks, independently of offline Cargo.

## Execution

- Finalize owns one current full-suite execution and its generated tail. Its actual source commit and exact generated preimages bind publication; this is execution input, not a new product identity. No second command accepts that result.
- Test failure returns findings for selected focused repair. Fresh selected tests replace their same-purpose evidence after reap; interruption continues the current unfinished execution within its original budget.
- Baseline holds CargoLock through bounded fetch, its selected execution or retained terminal, promotion, and owned cleanup. Native Git role registration selects this tag's actual baseline source path; another tag never replaces it.
- Label a nonempty bugfix comparison as patched upstream rather than claiming it is the pristine official source.
- Cargo stays offline; Baseline and product tests keep network with fixed tools/caches.
- Stage publication requires RunnerEnv's exact scratch absence after reap. Current test output remains available until same-purpose replacement or explicit drained Cleanup; only baseline comparison JSON survives that Cleanup.
- NextestConcurrency schedules only.
- Finalize derives B from Established: the first selection, including preparation, expires B after admission; later remaining sets expire B/2 and B/4 after their starts. Time never transfers. Runner supervises their 7B/4 sum and bounded terminal cleanup without preempting an eligible round. Snapshot publication checks exact generated bytes and preimages.
- Public elapsed output uses the captured invocation start directly, including route admission, rather than reconstructing it from a fixed deadline duration.
- Baseline calibration uses the existing eight-hour maximum from release-route entry through preflight, compilation, tests, publication, and cleanup. It never restarts a nested budget or changes the current deadline after measurement.
- After a successful fresh execution covers the complete established selection, the release route records its exact monotonic complete-call milliseconds in BaselineElapsedMilliseconds and rounds SuiteTimeoutMilliseconds upward to the next five-minute boundary. An exact boundary stays unchanged; 21 minutes becomes 25 minutes.
- Measure after all successful cleanup and immediately before the final benchmark publication under CargoLock. That publication and the final lock release are outside the stored interval. Native Stage elapsed retains its narrower process meaning.
- Automatic incremental coverage, unchanged or empty alignment, retained Results, and resumed executions preserve the previous benchmark and budget. Only actual new Runner startup identifies fresh execution.
- Zero coverage starts with unknown BaselineElapsedMilliseconds encoded as null and the existing eight-hour calibration ceiling. An explicit manually selected budget may accompany unknown historical timing; missing fields are rejected, with no automatic thirty-minute default or compatibility reader.
- Timing metadata is operational policy outside Plan comparison. Updating it cannot invalidate comparison policy or authorize another execution. A final benchmark-publication failure reports Helper while preserving the already sealed Result and prior timing facts.
