---
name: codex-build
description: Maintain Codex Rust CLI patch releases, product verification, builds, installation, and repository synchronization in one session.
---

# CodexBuild

## Owners

- Invoke `$SKILLS_ROOT/codex-build/bin`. Public roots default to cwd for working input, PROJECT_SCRATCH_ROOT for scratch, PROJECT_RUNTIME_ROOT for disposable runtime content, and `<effective scratch>/records` for records. Supply root flags only for deliberate overrides. PROJECT_PERSISTENT_ROOT is for permanent settings/data when needed, never Cleanup. PROJECT_WORKING_ROOT, RUNTIME_TMP_ROOT and RUNTIME_STATE_ROOT are retired without fallback; SKILLS_SCRATCH_ROOT owns skill repair data, not product data.
- RouteEntry selects one effect set.
- [LoadGraph](/codex/skills/codex-build/contracts/load-graph.md) owns bootstrap and fixed-point loading.
- On entry, read LoadGraph to EOF, then Core, RouteEntry and selected command instructions before effects.
- [TaskWorkspace](/codex/skills/codex-build/reference/task-workspace.md) owns task placement and continuation; Records owns encoding and lifecycle joins.
- ReleaseRollForward's OriginalPatchPurposeOwner owns purpose and classification before replay. PatchFormat defines the representation, PatchAcceptance reviews actual behavior, and ProtectedCoreAreas preserves protected boundaries.
- RunnerEnv owns a private build-attempt guest `/tmp`, including `TMPDIR`; callers select only payloads.
- Existing helpers derive every Cargo home, compiler cache, build directory, target and bulk preparation temporary input beneath the effective working root's build directory; callers never select a separate host Cargo workspace or target.
- RepositoryFormatting owns repository-selected formatter execution and its writable source postimage.
- RepositoryMaintenance owns the fixed workspace Cargo lock update and the separate repository-selected Bazel lock recipe, each with its exact writable source postimage.
- FullSuiteAudit owns product verification.
- ReleaseRollForward owns release convergence.
- `bin` owns no policy.

## Release Terms

```
TopLevelReleaseTerms
+-- Baseline: establish or align baseline then clean and stop
+-- Roll: migrate the complete declared Patch set within user-selected scope; ReleaseRollForward owns material decisions
+-- Install: commands/build-install.md selects the package and verifies its launch entry
+-- Sync: commands/sync.md publishes manually requested public snapshots
`-- Cleanup: restore Attached main, retire drained helper temporaries, and optionally retire one explicitly selected task
```

- During a selected next-tag Baseline, a request to clean the predecessor means that Baseline's existing exact predecessor-worktree retirement before target preparation. It preserves exported patches, reusable cache/target, toolchains, and Established evidence; it is not standalone Cleanup or a disk-space investigation.
- Explicit Cleanup restores its supplied Attached `main` worktree to the current `HEAD`, consumes all drained execution records and logs, and preserves only tasks/rust-suite-baselines.json as baseline comparison facts. It may additionally retire one separately named completely published tag.
- Select one requested term.
- Only an explicit current-user sync request selects public snapshot publication.
- Roll, export, build, installation and skill maintenance never trigger public sync automatically.

## Invariants

- Durable patches are product, upstream is fetch-only, and MainWorktree is the release line.
- A Lane contains Units and Flat Patches. A Unit contains at least two ordered distinct Child Patches. Each Flat Patch and Child Patch is one named commit and GitDiff; Flat Patch is semantic, never a file.
- Native Git retains PatchFormat's canonical representation; contracts and Requires declarations remain evidence rather than authority for purpose or group membership.
- The complete formal prefix remains independent of draft, whose consumers may use complete shared formal interfaces.
- One `export` publishes registered MainWorktree's ordered closed Git prefix, never a fork, isolated lane, or release-completion decision. Semantic artifacts are `.diff`: one per Flat Patch or Unit Child Patch, accompanied by PatchExport's release documents. `owner` is effect authority, not Patch structure.
- Apply advances an unchanged Patch only after ReleaseRollForward establishes its authorized purpose and placement. Conflict retains the successful prefix; reconstruct within that boundary and publish through Commit or terminal Revise. Exports remain read-only purpose evidence.
- ReleaseRollForward requires measured consumer benefit before retaining a performance-motivated addition; functional correctness and fewer apparent operations do not establish that benefit.
- Continue declared Unit and Lane transitions within the user's Roll scope without repeated approval; ReleaseRollForward owns explicit pauses and missing-authority decisions.
- Establish Baseline on the official parent plus the approved standalone bugfix prefix shared with the product. During Roll, uncovered upstream packages join automatically and explicit user-requested coverage alignment may add or remove them at any registered frontier.
- Adding, changing or removing a shared bugfix remeasures affected Baseline coverage through FullSuiteAudit before that comparison is reused, even when discovered after private integration.
- Tests remain freely selected diagnostics; only the user requests a complete product suite. No product suite gates Patch closure, Fork, convergence, export, or install. A failed selected suite remains evidence for focused repair, not a rerun demand. An automatic Roll rewrite or non-idempotent fixup fold runs corresponding focused tests before advancing.
- Finalize and Prepare own generated tail only when selected. Full-suite results come directly from Finalize; focused snapshots stay private.
- One version-root notes.md holds the approved product guards consumed before each selected Unit or Flat Patch.
- Every cross lane has a maintained `relationships.txt` ASCII graph under PatchExport's document lifecycle. OriginalPatchPurposeOwner and PatchAcceptance consume its functional relationships without treating the diagram as authority or a dependency database.
- The helper owns baseline preparation, TaskIndex admission, fork ordinals, declared patch transport, mechanical retained-range and generated-tail task synchronization, and task retirement. Product work owns responsibilities, source selection, topology, and judgment.
- Baseline interruption resumes the same tag, task, global topology and Plan/Stage/Result chain.
- Every Roll entry reads TaskIndex and the sole ActiveTask through its trailing Additional Instructions, then consumes TaskWorkspace's reconciliation and handoff route before a response. LoadGraph expands this anchor, never replaces it.
- Forward work stays on Current; ordinary editing of an earlier committed Lane child forks and retains its parent.
- The terminal Unit may revise a middle Child Patch on Current; failure or committed status alone selects no Fork.
- Only a user-selected full Finalize Product failure after declared Patch completion and Main convergence starts temporary Main fixup, which later folds by Fork.
- An explicit draft quick-iteration request instead selects ReleaseRollForward's native draft-only fixup and fold on converged Main.
- Apply copies an absent Flat Patch, a whole Unit, an approved unchanged retained range, or a declared Child Patch suffix.
- After a rewrite, ReleaseRollForward selects one continuous retained Apply range within user scope up to the next planned rewrite or selected stop instead of repeated per-Unit calls.
- Commit records one Patch without collapsing a Unit.
- A Unit or Flat Patch includes its approved behavior, exact structure, and complete producer seams required by approved dependents. Its first commit owns the responsibility contract; other Child Patches do not repeat it.
- ReleaseRollForward selects rewrite triggers and complete Patch responsibility boundaries. The rust skill's [construction owner](/codex/skills/rust/references/rust.md) supplies `PurposeBoundRustChange` to construct and review code inside them before dependent edits and source publication.
- Feature additions, removals and upstream fulfillment consume ReleaseRollForward's behavior-change decision independently of textual conflicts.
- Roll implementation stays with one product agent.
- PatchAcceptance owns cumulative source acceptance and the user-selected subagent permission for reviewing multiple Units or Flat Patches.
- Repository-wide discovery for purpose reconstruction is a reduction step, never a model-context payload. The command performing a broad search emits only aggregate counts and at most sixteen sorted unique exact path or commit identifiers; inspect only the minimum exact ranges needed for the current causal decision. If that set is insufficient, narrow the predicate outside model context before another bounded result. Raw broad matches and transport truncation never select evidence.
- Verification refuting a candidate's completeness returns through ReleaseRollForward before any correction, including fixtures or settings. Rejection grants no feature withdrawal or Git effect; substantive contract or structure changes need new authority.
- Before a rewrite, reconcile Main's complete Lane children and retained frontiers with the proposed start, deciding reuse or authorized withdrawal in the task.
- A closed nonfinal Lane exports Main's ordered prefix without a test gate, then continues through ReleaseRollForward's scope decision.
- Authored or reconstructed source selects focused tests from actual risk and prior findings after helper Commit. Results cannot establish classification or authorize changed contracts.
- A helper Pass or below-limit diff cannot supply semantic acceptance. The existing line ceiling is not a design budget and splitting compensation does not resolve its cause.
- Commit as `nobody <nobody@nobody>` unless the user authorizes another identity for that exact commit.
- Derive Lane-child order from source history and canonical metadata, never worktree names, helper state, export, or inferred lineage.
- Admit only current canonical formats.
- Skill maintenance has no test facility, fixture, or test-only schema.
- Binary preparation required by admitted repository tests is production Runner behavior and remains required when skill self-test facilities are removed.
- A selected full suite compares Established upstream outcomes with the actual registered Main or fork source. RunnerExec runs a selected frozen check. Neither requires a candidate tag, historical acceptance chain, or convergence prerequisite.
- RepositoryFormatting owns writable `just fmt`, VerifyRequestedProduct sends exact `just test` to RunnerExec, RustSuite owns suites, and PatchReplay owns per-file `rustfmt`.
- Dependency Units or Flat Patches update stale Cargo and Bazel locks through RepositoryMaintenance and include both postimages in the same Patch commit before locked verification.
- Long public helpers emit immediate startup guidance and bounded event progress, each carrying the same-handle wait instruction. Request the longest permitted native wait capped at twenty-five minutes; every nonterminal yield continues that handle without commentary. Only terminal JSON completes. Explicit inspection follows RunnerEnv, then resumes that handle.
