# PatchReplay

## Task Anchor

```
PatchReplayTaskAnchor
+-- Owner: PatchReplayTaskAnchorOwner
+-- Consumer: ExactIndexedReleaseTaskConsumer
+-- Successor: AnchoredPatchReplayMutationSuccessor
`-- Terminal: CurrentTaskAnchoredPatchReplayTerminal
```

- Every public PatchReplay invocation requires `--active-task` with the exact absolute path named by the sole current TaskIndex ActiveTask; CLI admission writes that the flag is required, while execution reports when its value does not name the current indexed task and rejects an absent, unindexed, or non-regular task before native transport or topology effects.
- PatchReplay reads the task's exact original bytes before the selected operation and reports an unreadable or changed task distinctly after its native terminal, without initializing a missing TaskIndex or creating another task representation.
- Fork, Prepare, Apply, Revise, Finalize, Commit, List, and Export consume this same anchor; callers cannot omit, infer, reconstruct, migrate, or substitute it, and the anchor never becomes helper state or native transport input.
- Config-schema requires Target CurrentWorktree and identical open and terminal Unit-or-Flat-Patch discriminants; a Unit also supplies its committed ChildPatchSequence.
- Fork selects an available numbered branch and worktree path from Git's current inventory. Consumed forks do not reserve old numbers, and test-record retention is independent.
- After argument structure and shared public admission, Apply, Commit, and Revise derive rustfmt at --project-working-root/build/toolchains/native/toolchain/rust/bin/rustfmt. The public CLI accepts no formatter override. Validate the complete no-follow path and require an existing executable regular file before reading task evidence, invalidating schema results, creating a native attempt, or invoking Git. Failure returns the existing Input/Blocked result with the derived path. NativeToolchain alone prepares a missing toolchain; replay performs no acquisition, PATH search, or full-toolchain execution probe. General path validation still permits absent output paths.

```
PatchReplayNativeFormatterOwner
+-- Consumer: ProjectRootFormatterConsumer
+-- Successor: ValidatedNativeFormatterTransportSuccessor
`-- Terminal: FixedFormatterReplayTerminal
```
- EditNamedTask publishes the selected open discriminant and actual terminal. The open Patch may not yet exist in Git when inserted within earlier history; that admission does not authorize Fork for forward append.
- Fork consumes these fields with CurrentWorktree and RetainedParents. Its internal List validates the unchanged parent Patch sequence and terminal before topology effects; it cannot correct task bytes. A missing terminal or failed List stops before writing.
- Before constructing Fork, publish the TaskWorkspace no-parent form when Main has no retained parents: omit the whole RetainedParents line. The task reader rejects a present empty or malformed field with its name and canonical expectation. Fork identifies missing fork parents, placeholders, duplicate paths, and invalid real parent paths before its native List, without accepting another spelling or rewriting task bytes.
- After Fork adoption, one open Flat Patch or Unit and its ChildPatchSequence bound unrestricted native Git authoring. Ordinary Commit reads that unchanged selection for each final Patch; intermediate work selects no task transition.

## Schema Route

```
ConfigSchemaGenerationOwner
+-- Consumer: CurrentConfigUnitConsumer
+-- Generator: projected repository just write-config-schema
+-- Successor: BoundConfigSchemaPostimageSuccessor
`-- Terminal: SameUnitConfigSchemaTerminal
```

- The config-schema mode's allowed flags are in the parser-owned help grammar. It fixes the recipe and codex-rs/core/config.schema.json path; callers supply no generator argv, output path, lane, patch metadata, or test selector.
- Consume the formatted source Patch's clean tip. A partial Child Patch prefix leaves its Unit open. Native binds commit and fixture bytes before ProjectedRunner; configuration remains in its Unit or Flat Patch, never a generated transform.
- One fixed asset runs `/toolchain/bin/just write-config-schema` from codex-rs with the declared offline toolchain and CargoLock in the existing private overlay. The recipe owns generator package and binary selection. Only the fixed schema path is a declared temporary source output; every other overlay change retains normal rejection. Compiler and recipe output goes to stderr, while stdout carries one private binary-safe bundle with the source commit and exact before/after fixture bytes.
- A passing source-bound Runner terminal precedes publication. Recheck clean target, commit, semantic Patch identity, and fixture preimage; replace only that file while preserving Git state and all other bytes.
- Changed generation returns ConfigSchemaPostimage for config.schema.json. Review and Revise the owning Flat Patch or Child Patch. It creates no commit, patch scope, worktree, candidate, snapshot, or test result.
- Failure retains Runner evidence and the exact successor; success consumes its payload and attempt. No changed commit consumes another payload, fallback, or broader command surface.

```
ProjectedSchemaTransformOwner
+-- Owner: BinarySchemaPostimageTransport
+-- Consumer: CurrentSourceSchemaTerminalConsumer
+-- Successor: ReapedTerminalConsumption
`-- Terminal: BinarySafeSchemaTerminal
```

- Schema generation runs two ProjectedRunner generations and requires identical binary postimages.
- Runner JSON-encodes the schema bundle; native Finalize decodes exact bytes in producer order.
- Command failure and unequal postimages are distinct Product findings.
- Packaging, transport, decode, and missing output are Helper; missing schema stdout is `OutputPublication`.
- Finalize resumes a reaped schema terminal only at its exact current source commit. It retires stale reaped evidence and regenerates; native checks that commit again before applying the bundle.
- Same-source failure evidence stops without re-execution.
- Full Finalize consumes closed Current. Each native generated boundary synchronizes the task from actual Git rows before its successor, including reentry after cancellation. Schema precedes the selected suite; validated snapshots follow it. Empty transforms remain commits. Task publication uses exact preimages, independently of test outcome.

- An open semantic Patch selects transient schema-only Finalize with its last ordinary lane as --lane, then its affected test and Prepare. It preserves the test finding and task terminal. Package identity alone selects no generation.

## Source Preparation

- The prepare mode's allowed flags are in the parser-owned help grammar. Its target is CurrentWorktree; it accepts no revision, reset count, payload, or generation input.
- Prepare on clean registered Current derives only its canonical schema/snapshot tail and ordinary parent. It removes that tail, synchronizes the generated task terminal to the ordinary row, or returns the unchanged ordinary tip; interruption reconciles those exact postimages. It preserves ordinary commits, retained worktrees, records, and normalization.
- Invoke Prepare before source edits or Apply only when a clean generated tail exists, including after a failed temporary Main Finalize. Dirty source or interleaved transforms reject. Prepare runs no generator or tests; Finalize later regenerates the one tail without saved state or a second task.

## Unit Flat Patch And Child Patch Transport

- Transport checks custody and rollback. Before reconstruction or constructing Commit or Revise, consume ReleaseRollForward's selected complete Patch boundary and PatchAcceptance's two-way source acceptance. Native Pass proves no behavioral completeness or code quality. A Git failure retains bounded stderr, rejected identity and the accepted prefix.

```
CurrentPatchRevision
+-- Consumer: CurrentPatchCorrectionConsumer
+-- Input: task CurrentWorktree exact open discriminant sequence and exact terminal discriminant
+-- Boundary: open and actual terminal rows match for Revise
+-- Correction: source edits amend one existing Patch without changing structure
+-- Replay: only later Child Patches of the same Unit
+-- Failure: original dirty tip restored exactly
+-- Successor: CurrentPatchAffectedGateSuccessor
`-- Terminal: SamePatchStructurePreservedTerminal
```

```
PatchReplayLiveList
+-- Owner: PatchReplayLiveListOwner
+-- Consumer: IntermediatePatchPrefixConsumer
+-- Successor: CanonicalPresentRowsSuccessor
`-- Terminal: IncompleteRollFrontierListTerminal
```

- Live List validates clean history, discriminated metadata, path classification, Lane order, and transitions. Earlier Lane children are complete; only the last Unit may be a committed Child Patch prefix.
- Backend and mixed admission consumes the complete responsibility Segment and PatchFormat's ordered categories; mixed rejects Flat Patches before effects.
- Live and exported List/Apply, Commit, Revise, and Export consume PatchFormat's Lane-child order before product mutation.
- List selects source kind by identity using effective roots. Live sources equal task CurrentWorktree or one exact RetainedParents path. A canonical prefix requires an exact tag declared in SourcePrefixes. Root-flag presence never selects exported custody; unrelated sources reject.
- List accepts an earlier lane frontier without future lanes or transforms and returns canonical discriminated rows without effects.
- Fork copies Current into one isolated worktree. It requires the selected open Patch and actual last terminal, not an earlier open row; retained work needs coverage comparison, with focused testing for a non-idempotent fixup fold.
- Exported List, Apply, and Export require complete selected structures. Commit and Revise retain partial-Unit rules; an unfinished Unit cannot publish, infer children, or create a cursor.

```
DeclaredPrefixPatchSetOwner
+-- Input: task SourcePrefixes and exact canonical source directory
+-- Consumer: ExplicitSourcePatchSetConsumer
+-- Source: complete canonical selected structure and GitDiff bytes
+-- Successor: ExactSelectedPatchTransportSuccessor
`-- Terminal: UnchangedSourcePrefixTerminal
```

- Exported-prefix Apply selects one task-open row from the task's declared SourcePrefixes directory. For a Unit:

```text
"$SKILLS_ROOT/codex-build/bin" patch-replay \
  --active-task <ExactIndexedActiveTask> \
  --mode apply \
  --tag <CurrentTag> \
  --source <ProjectWorkingRoot>/build/patches/private/<SourceTag> \
  --target <CurrentWorktree> \
  --start-unit <Lane>/<Unit>
```

- For a Flat Patch, use `--start-flat-patch <Lane>/<FlatPatch>`. For initial normalization, use `--start-transform normalization/normalize-terminal-chrome-for-powerline` matching OpenTransform. It regenerates on the target before GitDiffs, without an exported Transform diff.
- Exported-prefix Apply forbids `--lane` and every `--end-*`.
- Retained-worktree Apply may select its first boundary with exactly one `--start-unit`, `--start-flat-patch`, or `--start-transform` and an optional matching `--end-*`.
- Commit and Revise accept only `--flat-patch` or `--unit` plus `--child-patch`; no Transform or generic selector.
- Live Apply uses task Current target and exact retained Source. Before effects, match the selected source Unit children or unique strict suffix to ChildPatchSequence. A GitDiff conflict retains its accepted prefix.
- Native Input refusals return bounded Blocked/Input with the rejected field, preserving task and dirty source without diagnostic-text classification.
- Prefix Apply binds source directory and complete selected structure, freezes its bytes, and rechecks them at terminal without requiring final release evidence.
- Whole Apply requires an absent target Lane child and complete source structure; a retained source need not contain earlier reconstructed children. Unknown, reordered, duplicate, overlapping, empty, or nonunique suffix rejects before mutation.
- Export resolves registered Main, validates its own complete canonical history, and projects that ordered prefix without reading a fork Current terminal or deciding whether future Patches exist.
- Prefix List and Apply validate `.diff` artifact syntax, modes, Lane order, and structure; a shorter source is never final-release evidence.
- Exported GitDiff metadata uses the same parser as native commit metadata and must match the artifact's Lane, structure, and Patch identity.
- Prefix root and lane notes.md files follow PatchExport's format and exact source preimage. Both constrain Roll and review, never Patch rows, sequence, or source to apply. Historical absence remains readable; new exports require both levels.
- Revise selects the terminal Flat Patch or a committed Child Patch in the terminal Unit.
- Revise measures the complete formatted staged Patch against its parent before amendment; later replayed children use their current parent.
- Earlier Units and Flat Patches require Fork; children in the terminal Unit do not.
- A source-only assertion exposed by Transform output belongs to its semantic Patch, not unrelated Unit. A wrong deterministic projection belongs to the Transform script.
- An open Unit admits Revise against its exact committed Child Patch prefix and complete declared ChildPatchSequence. A schema postimage may amend the current terminal Child Patch without inventing later children or requiring Unit completion.
- Commit binds CurrentWorktree, the open Flat Patch or Unit, and its ChildPatchSequence. Commit and Revise reject Lane-incompatible Segment as Blocked/Input before native execution. After formatting, Commit measures the exact staged GitDiff before native commit. An empty or above-limit diff restores the input and returns its typed Product finding. Explicit current-user authority may add --skip-line-limit to this invocation, suppressing only the above-limit finding without storing or inheriting a waiver. Commit otherwise creates the Flat Patch or next declared Child Patch without a terminal match or intermediate task update. Revise remains a source-only correction. Neither accepts a snapshot attempt or generated bytes.
- Focused Runner terminals retain automatically captured snapshots as private evidence. They never amend semantic history, and manual reconstruction never copies their postimages.
- The selected suite excludes Established non-pass identities. Complete inventory and exact preimages precede native snapshot publication; failure publishes no accepted snapshot Transform and blocks no ordinary source edit.
- Retain the current Runner result and diagnostics. The generation producer checks its source-bound postimage and publishes the generated tail directly, without a later acceptance chain.
- A fresh user-selected full check runs Prepare first if a generated tail exists, then Finalize without `--resume`. it replaces only the prior full set after reap. To continue an interrupted full execution or finish its unconsumed generated publication, repeat Finalize with `--resume` on the same source and selection. This invocation choice adds no stored task state. Exhausted rounds remain failed. changed source selects a fresh check, usually focused, rather than continuation.
- Apply, Revise and Commit restore incomplete work on conflict or interruption, preserving ignored files. Normalization and schema Finalize keep their existing order.
- Apply and Commit retain file-local `rustfmt` only to normalize each transported Rust Patch commit; this does not replace the repository-level `just fmt` gate owned by RepositoryFormatting.
- Terminal-chrome Apply instead consumes the public RepositoryFormatting terminal inside each generation before its stable comparison and single commit. The formatter receives the exact admitted endpoint and roots; ordinary GitDiff transport and schema Finalize retain their existing consumers. Apply reserves nested formatter cleanup before restoring a failed generation.
- Apply, Commit, and Revise consume the rendered-symbol check before closing. Unknown symbols return bounded Conflict and restore exact input.

## Generated Tail

- Generated transforms are not suffix work, Flat Patches, Units, or exported artifacts. Export admits their canonical identity, then projection omits them. Apply skips selected source generated groups as Pass and regenerates the target tail through Finalize.
- The snapshot commit binds the current verification frontier, not permanent patch-set completion. The producer checks both changed and empty postimages. Adding source requires its selected checks, not reuse of an older source's Pass.

## Publication

- Export consumes PatchExport's current-prefix rotation without selecting verification or retirement. A history edit does not select a full suite; every complete check requires current-user selection.
- Product owns topology, gates, review, promotion, and cleanup.
