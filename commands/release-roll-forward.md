# ReleaseRollForward

## Route

```
CurrentWorktreeRollTransition
+-- Owner: SingleCurrentReleaseBoundaryOwner
+-- Consumer: CanonicalPrefixRetirementConsumer
+-- Current: task-named clean MainWorktree or selected fork worktree
+-- Forward: clean Apply advances; source authoring stays on Current and selected tests answer actual source questions
+-- Fork: selected committed-history isolation follows GitCustody's Unit boundary
+-- Retained: the parent supplies its complete frontier for coverage comparison
+-- Converge: Current absorbs each fully covered parent after Git comparison and any required non-idempotent fixup-fold focused test
+-- Promote: reaching the retained MainWorktree frontier replaces Main immediately
+-- LaneGate: export a closed lane; full tests only at a user-selected frontier
+-- Successor: CanonicalGlobalRoleReuseSuccessor
`-- Terminal: RetiredPredecessorBeforeNextTagTerminal
```

```
PatchSelectionAuthorityOwner
+-- Consumer: TaskNamedPatchAndReadOnlyEvidenceConsumer
+-- Select: automatic Roll follows the sole task's complete declared Patch order; interactive Roll requires an explicit request
+-- Authority: the Roll request executes that topology but never changes its responsibilities or order
+-- Continue: Apply the next unchanged Patch across Lanes until the declared set is exhausted
+-- Rewrite: reconstruct a rejected Patch inside its declared Unit or Flat Patch after read-only purpose analysis
+-- Change: material contract or requirement-cost choices use PreparePatchForApproval
+-- Conflict: retain the successful prefix, publish the rejected Patch through Commit, and continue the selected suffix
+-- Forbidden: inferred source membership merge split redistribution or new Patch authority
+-- Successor: AuthorizedPatchTransportOrRewriteSuccessor
`-- Terminal: SelectedPatchAuthorityTerminal
```

- Root and lane note changes retain separate exact current-user approval; Roll execution never grants it.
- Product evidence may change direction only within the sole current release. TaskIndex admits exactly one unfinished release task and no release selector or ordering policy.
- TaskWorkspace publishes one release-roll-<Tag>.md as Active. SourcePrefixes declares the exact canonical tag directories available to that task; the first tag is its primary predecessor. Read every selected source's complete Unit set and preserve the responsibility contract across source changes.
- Entry and material returns select TaskWorkspace reconciliation and closure before continuing.
- A shorter source prefix selects the next declared source; missing input or a changed contract stops. Baseline stays through export.
- Interrupted baseline, replay, verification, or export reenters the same tag and retained evidence. A user-selected newer tag requires either a canonically exported clean closed Main frontier or an Established predecessor with zero private history whose Main equals its official parent. EditNamedTask then replaces the sole task and index while preserving unresolved responsibility, source-prefix, and lane-note custody. Failure recovery never creates a coexisting release, retained release queue, semantic-version selector, or compatibility state.
- The new ordinary Baseline validates that exact exported prefix or zero-patch parent and its one predecessor global topology, retires only those predecessor Git roles, and creates the target roles at the same paths. A selected full suite, BuildInstall, and explicit task retirement retain their own inputs and are not prerequisites for this next-tag transition.
- A request to clean the previous tag while this next-tag Baseline is selected names that existing predecessor-role retirement. Continue the same Baseline successor with its exact task and preserved prefix/Established inputs; do not reinterpret it as disk recovery, raw-output cleanup, Attached restoration, or completed-task release-cleanup.
- Lane children are Flat Patches or Units of at least two Child Patches. Their GitCustody boundary applies to insertion, withdrawal, append, modification and repair alike; a Child Patch is not a separate Lane child.
- Only a committed-Patch Product failure in the user's Established-relative full Finalize after declared Patch completion on clean, closed, converged Current Main starts temporary Main fixup. Pass and Helper terminals keep their normal successors; generic RunnerExec cannot start fixup.
- Prepare any generated tail, then use native Git on Main for contiguous temporary source commits with distinct canonical hyphenated Flat Patch metadata in the current Lane, including draft, not `fixup!` subjects.
- Reset, rebase and amend remain allowed; temporary Main does not use helper Commit/Revise.
- Each correction first consumes the purpose reconstruction below and cumulative PatchAcceptance for its actual owning Patch.
- Run corresponding focused tests on each corrected source.
- Before another correction or handoff, consume cumulative source acceptance; handoff also requires focused Pass.
- Failed checks return to read-only diagnosis; retain the selected full-suite failure as evidence.
- Publish the actual temporary source terminal and Patch-owner fold plan in the sole task. `git format-patch` only the contiguous focused-tested temporary source commits, not generated tail, into the exact `<ProjectScratchRoot>/fixup-handoff/<Tag>` child. Require a real scratch root and absent tag child; reentry verifies patch bytes without overwriting. This is not canonical export. Another complete Finalize occurs only on an explicit user request, never as handoff admission.
- Treat the handoff as read-only evidence and enter Editing Committed Patch History at the first affected Patch. Its ordinary Fork, helper Commit, retained-parent comparison, and Main convergence own the fold. Prepare any generated tail before comparing the final fork and tested temporary Main source trees. Exact equality needs no test; if an owner rewrite makes them unequal, run corresponding focused tests on the fork and repair any failure there before promotion. No extra full suite or topology follows. Only rewritten canonical Patches enter `export`.
- Clean unchanged Apply reaches the next declared Unit or Flat Patch without a focused current gate. During automatic Roll, reconstruction of a rejected Patch runs its focused tests before advancing; other post-baseline testing is selected by its diagnostic purpose. An ordinary nonfinal Lane exports its clean prefix and continues.
- Helper failure retains the actual test evidence and topology without phase or cursor; it does not turn a freely selected test into a full-suite prerequisite.
- PatchAcceptance resolves platform conditions against current user authority and version notes before dependent authoring, including resumed rewrites and conflict reconstruction. Normalization closure consumes its cumulative audit before selected full verification.

## Start Normalization

```
ReleaseBaselineOwner
+-- Consumer: NewReleaseRequestConsumer
+-- TaskIndex: sole selected tag task explicit source prefixes canonical global Git roles and origin
+-- Inherit: predecessor Established remains unchanged while the current tag stays absent until Pass
+-- First: exact zero coverage without baseline execution evidence
+-- Failure: remove only resources created by this invocation and retain the same tag continuation
+-- Fetch: one CargoLock before baseline or zero discovery
+-- Successor: OfflineBaselineAndOwnedCleanupSuccessor
`-- Terminal: CleanBaselineStopTerminal
```

```
OfficialOneParentTag->ParentMainWorktree->EmptyUpstreamMarker
```

- Before baseline, the product agent publishes the sole canonical task and TaskIndex through TaskWorkspace.
- The helper consumes that task unchanged rather than creating another task representation.
- Any second indexed task or any registered topology other than the selected tag or its primary predecessor blocks.
- Main, UpstreamBaseline, and fork worktrees use TaskWorkspace's canonical global paths. Native registration must match those exact paths and one admitted tag; tag-scoped, mixed, duplicate, or unrelated roles block before mutation.
- When private history exists, require the exact primary prefix directory and canonical Unit bytes without a release-completion receipt. When the prefix is absent, require an Established predecessor and Main exactly at that tag's official parent; an empty directory or any private Patch still blocks.
- New preparation validates that predecessor evidence, fetches and retains the official one-parent target tag, and converges the canonical global paths from the remaining exact predecessor roles to Main at the target parent and its empty upstream marker. Reentry consumes that same forward transition, retires remaining predecessor roles at their observed tips, and completes missing target roles without selecting the predecessor topology as current again.
- Once the target topology is complete, bounded Cargo fetch and the same tag's retained Plan, Stage, Result, and process chain remain its only baseline continuation. The predecessor Established stays immutable input evidence until zero bootstrap or a passing Baseline atomically publishes the target Established, retains that primary predecessor, and retires any older baseline generation; an existing target Established may only finish owned cleanup.
- Release-baseline emits startup guidance before work and bounded progress at phase changes and completed test rounds. Every line repeats the same-handle maximum-wait instruction. Continue that handle through cleanup and terminal JSON; progress is not completion or a request for caller commentary.

## Purpose And Causal Reconstruction

```
OriginalPatchPurposeOwner
+-- Consumer: SourceAndEnvironmentContractConsumer
+-- Successor: ContractGroundedUnitRewriteSuccessor
`-- Terminal: CurrentNativeRewriteReadyTerminal
```

- This owner selects when a candidate must be reconstructed and the Unit or Flat Patch whose complete responsibility is affected. Rust owns construction and semantic review of its code.
- Recover the selected Patch's complete approved behavior from current authority, establishing rationale, complete retained Patches and relevant history, including approved dependents.
- Attribute every required effect and direct-entrypoint integration to its existing Unit or Flat Patch and unique current native implementation. Missing, overlapping or contradictory assignment is not resolved by editing more files.
- Agent-authored task text, candidate code, tests and historical structure cannot authorize broader behavior or replace approved obligations.
- Before broad discovery enters context, reduce it in the command to counts and at most sixteen sorted unique exact identifiers, then inspect the minimum causal ranges. Raw matches and truncation select no evidence.
- Follow introduction, repair, reversal, movement and partial replacement when a strategy recurs. Reconcile interacting strategies and their surviving consumers before choosing another correction.
- Compare actual entrypoints, construction, effective configuration, lifecycle, shared fixtures and RunnerEnv. Establish each test premise independently through Rust Diagnostic triage.
- Adequate existing configuration for every requested entrypoint blocks private code absent explicit code authority. An uncovered requirement permits source work; changing user configuration needs separate authority.
- Compare failed selected outcomes with Established before treating them as novel. Missing or incomparable evidence stays unresolved rather than implying a new Patch defect.
- A novel failure enters read-only causal reconstruction. When it exposes a missed producer or consumer, relocates the same mismatch, or requires compensation for a disproved choice, reject that candidate for continued adoption.
- This trigger applies equally to production, tests, fixtures and setting overrides, even with unchanged assertions or no new state types. Expand investigation, not the mutation boundary.
- A correct retained event becoming visible is not a defect merely because it breaks a downstream isolation assumption. Preserve its approved route and presentation while identifying the actual faulty premise or consumer.
- Rejecting an implementation does not withdraw any approved feature or constraint. Do not replace a failed candidate with a reduced, test-only or no-op Patch to obtain green verification.
- Before any further edit, select the complete owning Patch through actual Git and the established responsibility assignment, preserving every later accepted or unfinished Patch and retained source.
- Outside temporary Main fixup, a terminal Patch correction stays on Current; an earlier committed Lane child consumes the existing Fork, adoption and retained-suffix route. Failure alone creates no fork or reset authority.
- Reconstruct every surviving obligation in that selected boundary through Rust Semantic review, removing its disproved repairs together without retaining the candidate as a foundation for incremental fixes.
- A fixture finding belongs to its actual construction or integration owner; a dependent Patch cannot absorb missing predecessor seams or suppress predecessor behavior to make its test pass.
- PatchAcceptance owns cumulative source acceptance before publication and post-check continuation, including temporary Main corrections.
- PreparePatchForApproval owns unapproved substantive behavior, responsibility, API, failure-policy, runtime-premise or Patch-structure changes.
- Retired or compatibility paths and a nonunique native mapping also return there.
- Before construction, consume Rust's scope and cost decision; material requirement tradeoffs use that approval successor even without substantial upstream divergence.
- Pending a decision, preserve the last approved boundary and pause dependent edits.
- Otherwise complete authorized reconstruction and continue its successor. Needed unresolved work stays in the sole task, without a receipt, quality state or alternate repair route.
- Withdraw a declared Patch only through its separately authorized task and Git boundary. Reuse construction evidence only while its complete contract, consumers and execution premises remain supported.

## Forward Replay

- Before starting or changing a rewrite frontier, use List and Git history to inspect Main's complete discriminated rows, Current, retained parents, and proposed start. Compare actual Flat Patches, Units, Child Patches, and behavior. Resolve existing content as reused, rebuilt in its assigned structure with a retained suffix source, or explicitly withdrawn before mutation.
- Normal progression keeps Current and exports a closed lane before crossing it. Fork only when editing committed history; Prepare removes a clean generated tail before later source authoring.
- If Main already contains the requested structures, use that frontier. An earlier start cannot silently discard them; rebuilding requires explicit disposition of displaced behavior and source custody, followed by ordinary Fork, adoption, and convergence.
- Reconcile complete predecessor Flat Patches, Units, Child Patches, and relevant history with current decisions before consuming PatchSelectionAuthorityOwner.
- Publish only unresolved responsibilities and continuation details in the task.
- Preserve boundaries unless exact redistribution is approved.
- Prepare absent root and lane notes from the corresponding current publication, selected sources, or an empty file. Copy inherited entries exactly; preserve existing drafts, including empty files.
- Before every new or resumed Patch analysis, read root notes then the selected lane's notes and applicable source guards. Current user decisions govern; conflicting guards require resolution before dependent edits.
- Notes are not summaries. Put shared critical cross-Patch guards at the root and only lane-specific additions in lane files. Self-contained behavior stays with its native owners. Present exact additions, rewrites, moves, or removals for explicit current-user approval; task edits, Patch completion, tests, and agent importance grant none. Notes contain no Patch identity, topology, history, workflow, progress, or test transition.
- Before each declared Lane, reconcile its Unit and Flat Patch responsibilities, dependencies, and order. A changed membership, order, or Patch form needs approval. Producer seams and direct entrypoints stay in their assigned Unit.
- An interactive pause requested with exploration consumes OriginalPatchPurposeOwner for the next Patch on clean Current. Present its implementation, retained responsibilities, owners, dependents, focused checks and unresolved decisions before stopping; transport and task topology stay unchanged.
- Attribute each finding to its approved Unit and actual prefix before edits. An override selecting or propagating an earlier output returns to that Unit's committed-history edit and is removed during replay. Current stays Current and later stays later; ambiguity or redistribution repeats approval before dependent authoring or Commit.
- A proposed redistribution identifies the source and destination Units, exact moved behavior, dependencies, direct entrypoints, tests, and retained suffix consequences in the existing task. Apply existing user authority when it covers that concrete change; otherwise the existing approval successor owns the decision. General roll or repair authority and editing the task alone do not approve changing an established boundary. Resolve the assignment before Fork or reconstruction, then use the same Unit transport and verification route.
- Historical source is read-only purpose evidence during semantic rewrite. Author within the selected Flat Patch or Unit on Current, publish each final Patch by Commit or terminal Revise, and keep focused snapshots private until selected Finalize.
- A dependency Unit produces a stale workspace Cargo.lock with the fixed RepositoryMaintenance Cargo operation before its separate Bazel operation. Perform those bounded dependency edits before adding other source dirt, retain both lock postimages with their manifests, and publish them inside the same Patch.
- When Apply or an ExecutableTransform publishes the complete formatted source tip, consume it without a duplicate commit. A focused schema consumer uses transient Finalize on current cumulative source, its affected test, then Prepare; package identity alone does not require generation.
- When the current Flat Patch or Unit requires config.schema.json regeneration, use config-schema on its clean source tip, then fold the fixture into the same Flat Patch or Child Patch through Revise. The postimage is no new structure or lane terminal.
- A selected snapshot-producing focused check retains private generated bytes without publishing them. A selected complete codex-tui package test supplies private external and inline renderer evidence; the script's character projection alone does not establish snapshot geometry.
- For a selected failed test, compare exact outcomes with Established and diagnose any novel failure through lang's [Rust Diagnostic triage](/codex/skills/lang/references/rust/diagnostic-triage.md) and the purpose reconstruction above. That continuation is read-only until the unchanged approved boundary is confirmed or a changed boundary receives new approval. Preserve missing or incomparable evidence as such; do not infer a Pass or require a complete-suite rerun.
- Forward advancement and committed-history edits select no full suite. A user-selected full Finalize runs on actual Current without an extra tag or acceptance seal.
- A normalization platform-audit finding follows the existing Flat Patch or Unit boundary: correct the terminal structure in place, or Fork and rebuild an earlier one before replaying the retained suffix.
- Verification retains its RouteEntry handle. An interrupted full Finalize resumes through --resume on the same source and selection within its original round deadline; never omit that flag merely because the handle was lost. Baseline reentry uses release-baseline. Findings return to the owning Patch for freely selected focused diagnosis without changing topology or demanding a fresh full run.

## Editing Committed Patch History

- For multiple known rewrite locations, follow their declared Patch order from the earliest affected Unit and reuse unchanged suffixes on Current. Plan changes before Apply when known. A newly found issue inside the terminal Unit stays there, even in an earlier Child Patch; returning to an earlier Unit or Flat Patch uses Fork. Failures and repeated edits alone never select another fork.

- When changing a retained Patch, read the complete Unit from its task-named parent: every canonical Patch, cumulative production changes, callers, tests, and relevant semantic history. Compare those responsibilities with the actual current source and approved contract in both directions: nothing assigned here is lost, and nothing assigned elsewhere is absorbed. An inserted Patch uses its approved task responsibility and direct consumers. Retained responsibility remains in its assigned Patch, its explicitly unfinished sequence, or an authorized withdrawal or redistribution. Matching names, a test-only diff, or a passing narrow check cannot establish coverage.
- After a rejected Apply, use its restored Current tip and the task's same open Patch, not a skipped Patch or edited source diff. Native Git may materialize, split, reorder, or reconstruct it. Apply and Revise remain optional transports. Commit generates each final canonical Patch before the selected suffix resumes.
- Completeness is relative to that Patch contract, not the whole release. Do not publish only a diagnostic test or another Unit's implementation.
- Compare the complete Unit with retained obligations before TaskWorkspace reduction. Authorized withdrawal names affected behavior and consumers; missing evidence leaves that requirement unresolved.

```
RetainedFrontierCoverageOwner
+-- Consumer: GitUnitSequenceComparisonConsumer
+-- Successor: TaskNamedFrontierAbsorptionSuccessor
`-- Terminal: CoveredFrontierConvergenceTerminal
```

- List Current and each retained parent. Compare ordered Flat Patches and Units, every committed Child Patch through a partial terminal Unit, their production responsibilities and declared unfinished work. Approved rewrite, insertion or withdrawal explains differences; matching Unit names or commit ancestry alone cannot establish coverage.
- A parent with unconsumed content remains a replay source. A completely covered numbered parent goes directly to GitCustody removal, even while Main supplies later Patches. A later unrelated failure does not reopen that consumed frontier. Generated transforms keep their existing Prepare/Finalize ownership and do not create another retention gate.
- Publish actual Current, retained paths and terminal after the selected native effects. Future draft additions never delay coverage of already retained content. User-selected Finalize still runs on actual Current and owns its suite and generated tail, not topology completion.

## Convergence

- GitCustody owns the sole closure/reentry successor, native removal, Main promotion and interrupted continuation. A nested rewrite may temporarily need several unconsumed sources; after absorption only Current and genuinely needed parents remain. No numerical cap or test-result archive substitutes for that comparison. Until Main coverage, replay continues on the surviving Current fork.

## Publication And Retirement

- Export always publishes the current ordered source prefix; it creates no completion state and does not restrict later draft additions. Schema and snapshot remain the replaceable generated tail, never artifacts or permanent lane markers.
- Ordinary lane crossing consumes clean prefix export and the task's existing source selection. Removing a generated tail for later source preserves historical evidence but grants no acceptance to changed source.
- Same-tag export uses the existing one-generation rotation without automatic retry. Later source invalidates acceptance for the new tip, not historical immutable evidence.
