# Records

## Ownership

- JSON owns fields, producers values, commands behavior, and this file lifecycle/joins.

## Publication

```
CanonicalRead
+-- Owner: CanonicalRecordRead decodes only a successful read
+-- Consumer: MissingPendingStage preserves original absence
+-- Successor: FirstExecutionAdmission has no synthetic failure
`-- Terminal: MalformedPresentJSON alone is format failure
```

- Structured transport uses sorted object keys, two-space indentation, one final newline, and Go JSON string escaping, including ampersand, angle brackets, U+2028, and U+2029.
- Native producers and readers use that same encoding for embedded text without changing decoded values or accepting another representation.
- Publication validates and syncs through NoReplace or fixed replacement; terminal Result has no Active successor.

## Identity Lifecycles

```
RecordLifecycles
+-- LockGroupRecord: exact request through Add and observed Terminal
+-- LockRecord: semantic attempt and fixed Cargo lock
+-- RustSuitePlanRecord: release baseline plan or current full-suite input
+-- RustSuiteStageResultRecord: one product attempt
`-- RustSuiteResultRecord: terminal and diagnostics
```

## Joins

- PatchFormat owns Lane, Unit, Flat Patch, Child Patch, and Transform order. Export returns its publication effect without a durable completion record; consumers compare current Git and exact committed artifacts directly.
- Each PatchReplayRow has common `Kind`, `Lane`, `Form`, and `Segment`, then exactly one variant: `FlatPatch`, `Unit` plus at least two ordered `ChildPatches`, or `Transform` plus `TransformPatch`. Variant fields never coexist.
- Selected List rows additionally project Contract with Purpose, Requires, Provides, Boundary and Verification from the sole native commit owner.
- Cross contracts additionally include CrossGroup from that same owner; other Lanes omit it.
- Unselected List and other operation rows remain compact inventories; neither row form stores progress or duplicates the contract outside ordinary operation output.
- Baseline keeps its current Plan, Stage, and Result at `records/rust-suite/baseline/{plan,stage,result}.json`. Product full and focused checks keep only their current Runner result. The three process/output names are `baseline`, `full`, and `focused`, independent of release tag, branch, or source commit. Their records bind actual inputs; the names select storage, not product identity.
- Each new same-purpose execution replaces its prior reaped result, process records, and output. An explicit full continuation uses the same current records and original round deadlines. Full and focused replacement never touches baseline or Established. Native Nextest subrecords live only inside the current output set.
- Normal execution retains current diagnostic records and logs. Explicit Cleanup consumes all drained baseline, full and focused records and logs; only `tasks/rust-suite-baselines.json` remains as baseline comparison facts. Established readers do not require retired execution receipts or rerun tests to reconstruct them. Next-tag admission reads comparison facts and actual Git roles, not old test archives. No separate baseline directory, copy, archive or migration reader exists.
- RustSuite Phase is an optional bounded field of the existing process LockRecord, absent for generic consumers and replaced only by the CargoLock holder.
- The live handle reports bounded phase transitions. Finalize's temporary execution stream retains outer-round boundaries and deadlines alongside native test evidence; it adds no task or durable record format.
- Children reap before cleanup. expiry is Stopped Helper. Before Finalize terminal publication, same-source continuation may replace an absent process's Add. the sealed Add and Terminal pair describes the final process.
- Stage binds bounded preparation and execution summaries, process, package roles, materialization, sccache, exit, and timing without discovered-test, binary-binding, free-space, or per-test inventories. Actual preparation, payload, and output-publication failures retain their ordinary owners; immutable payload streams remain diagnostic evidence and cannot supply a helper terminal class.
- Temporary execution validation owns exact selected identities before publishing counts and a bounded outcome sample, and retained Pass requires validated complete counts with untruncated accepted outcomes.
- Result joins Plan, Stage, Add, Terminal, and bounded baseline-relative diagnostics. Complete class-compatible subsets pass; resolved or excluded Established outcomes cannot fail acceptance.
- RunnerEnv owns Cargo generations and materialization identity, process continuation, cleanup, and typed publication failures.
- RunnerExecResult stores bounded FailureSummary instead of raw Stdout or Stderr. Complete logs are temporary outputs, and their later consumption does not reopen sealed terminal or process evidence.
- RunnerRequest binds test selection, not a caller snapshot inventory. GeneratedSnapshotUpdates retains validated native postimages with exact lower-tree preimages and inline spans after execution.
- Generic repository tests consume the private RunnerTestPreparation output before sealing. It distinguishes Metadata, SelectedTestBuild, BinaryContract, BinaryBuild, BinaryBindings, and TestExecution using Phase, FailureDomain, and a bounded Diagnostic; it is not a durable record or baseline input. ExecutionStarted still describes the projected process, while Helper TestPreparation proves no accepted selected-test invocation.
- Cargo generation advances on workspace-owner changes, not repeated execution in a compatible workspace. Materialization retains only identity, action, invalidation cause, and reuse.
- Current test stdout, stderr, discovery, Nextest output, and native rerun records remain under `build/tmp/runner-output/<Purpose>` through Pass or failure until same-purpose replacement or explicit drained Cleanup. Ordinary scratch reap does not consume these logs; explicit Cleanup consumes every execution set and preserves only Established comparison facts.

- SourceSelection remains the exact physical source path relative to ProjectScratchRoot. New attempts use only the canonical global role paths; current records describe their selected execution and never authorize path fallback, relocation, or simultaneous release topology.
- Stage.ElapsedMilliseconds still equals native Observation.DurationMilliseconds. Plan excludes the operational suite budget and complete-call benchmark so calibration changes preserve the comparison policy.
