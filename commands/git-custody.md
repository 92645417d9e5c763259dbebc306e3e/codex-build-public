# GitCustody

## Boundary

- Product owns topology, direction, parent, retention, removal, promotion, and publication.
- Fork, Prepare, Apply, Commit, and Revise require the caller-supplied exact indexed release task before GitCustody changes topology or history.
- PatchReplay owns its declared exact-preimage mechanical task updates and preserves every unrelated task byte.
- A Unit or Flat Patch, not a Child Patch, is the ordinary history-edit boundary. Forward work never forks. Repeated edits within the terminal Unit stay on Current, including resetting to its middle Child Patch and rebuilding its suffix. Committed status alone requires no Fork; crossing back to an earlier Lane child forks first.
- Adopt the fork before ordinary native reset, rebase or authoring; helper Commit publishes final Patches. Automatic Roll rewrites and non-idempotent fixup folds run corresponding focused tests. For a temporary Main fixup fold, compare its source with focused-tested Main using native Git diff: identical source needs no retest, changed source requires focused tests on rewritten Patches. No topology operation selects a full suite.
- ReleaseRollForward's explicitly selected draft quick iteration owns native fixup and autosquash on Main without Fork or helper Commit.

```
AvailableForkOrdinalOwner
+-- Consumer: RetainedForkSourceIdentityConsumer
+-- Successor: NextUnusedThreeDigitOrdinalSuccessor
`-- Terminal: CanonicalForkCreationTerminal
```

```
SelectedPatchForkAdmissionOwner
+-- Consumer: TaskSelectedPatchConsumer
+-- Successor: FreeUnitAuthoringThenCanonicalCommitSuccessor
`-- Terminal: BoundedTaskForkTerminal
```

- Fork requires Current as parent, the selected Open Patch, actual last terminal and every registered fork named by the task. An inserted Patch may be absent from Git, but insertion within earlier history is not forward append. These transport inputs never select Fork instead of the Unit-boundary rule.
- Before Fork from Main with no retained parents, publish TaskWorkspace's no-parent form by omitting the complete RetainedParents line. A Current fork retains its exact existing parent paths when another historical edit needs isolation.
- Fork selects an available name in `001..999` from current Git branch occupancy and registered worktrees. Numbers name worktrees. they neither encode ancestry nor establish permanent identities. After a covered fork and its branch are removed, its number may be reused. No candidate, counter, ordinal tag, historical-record reader, or consumed branch is retained for numbering. Fork creates one unmentioned worktree. another Fork waits for task adoption.
- Fork retains its parent and makes the child Current. Adopt it once. Native Git may reset or rebase inside that fork for the selected historical edit. Helper Commit publishes final Patches without an automatic reset or intermediate task replacement. Convergence promotes the accepted tip without rebasing Main.
- ReleaseRollForward owns closed-lane export and selected full verification; GitCustody adds no suite gate.
- Other UnitTransport uses explicit paths and owns no topology.
- Commits use `nobody <nobody@nobody>` unless the user supplies an identity for that commit.

## Flat Patch Unit And Child Patch Transport

- List validates history without topology inference; Commit consumes target bytes and metadata.
- Focused Runner snapshot output remains private evidence and never selects Commit or Revise.
- Apply commits each selected complete Flat Patch or Unit from its admitted source.
- An approved unchanged retained range uses one Apply invocation.
- Only the first selected Unit can admit a strict later Child Patch suffix after its exact task-declared rewritten prefix.
- Each semantic Patch remains one commit.
- Revise requires matching open and terminal discriminants. It amends one Flat Patch, or one Child Patch and then separately replays only later Child Patches of that Unit.
- Revise preserves earlier Lane children and a Unit's declared ChildPatchSequence, restoring the original dirty tip and ORIG_HEAD on conflict or interruption. Structural change uses native Git inside the isolated boundary and Commit for final Patches.
- Reset, rebase, split, reorder, and source edits inside the selected Flat Patch or Unit create no helper or task progress. Crossing back to an earlier Lane child requires Fork; a retained parent supplies later structures when present.
- UnitTransport never decides that a Git-clean, affected-pass, prefix-pass, or final-pass worktree is acceptable.

## Attached Main Restoration

```
AttachedMainSourceRestoration
+-- Owner: Cleanup's exact registered Attached main worktree
+-- Consumer: fixed HEAD tracked and ordinary non-ignored inventory
+-- Successor: ExactNonIgnoredSourceRemovalSuccessor
`-- Terminal: unchanged main tip and preserved ignored inventory
```

- Before mutation, resolve the supplied Attached path to its Git top level, require that exact path to be the registered worktree for `refs/heads/main`, bind its current `HEAD`, reject unmerged entries and sequencer state, and inventory tracked changes, ordinary non-ignored untracked paths, and ignored paths as three distinct native Git results.
- Restore the authorized tracked source with `git -C <AttachedPath> restore --source=HEAD --staged --worktree -- .`. Then invoke `git -C <AttachedPath> clean -f -- <ExactPath>` once for each path in the captured ordinary non-ignored untracked file inventory. Re-read native state between changed inputs; never add `-d`, `-x`, `-X`, an inferred directory, or a repository-wide clean selector.
- These effects own only the explicit Attached source postimage. They do not target ignored paths, supplied roots, nested repositories, worktrees, branches, refs, release state, or files outside that exact inventory.
- Verify the same `refs/heads/main` and `HEAD`, no tracked or ordinary non-ignored change, and the identical ignored-path inventory. Failure preserves the observed remaining state for the same Unit; it does not broaden removal or select a recovery reset.

## Publication

```
ProductOwnedConvergenceOwner
+-- Consumer: TaskBoundCoveredFrontierConsumer
+-- Decision: product compares retained content and dependent readers at each closed Unit or Flat Patch
+-- Successor: NativeGitPromotionAndRemovalSuccessor
`-- Terminal: VerifiedConvergenceBeforeNextPatchTerminal
```

- After Unit or Flat Patch closure, and on reentry before selecting another Patch or Fork, consume this route. It does not run between Child Patch commits or native edits inside the selected Unit. The product agent owns coverage and native effects; helper inventory admission proves membership, not that every retained worktree remains necessary.
- One already selected retained Apply range keeps its source readers until that invocation stops; consume convergence at that return before selecting another range or rewrite.
- Read the indexed task, List Current and retained parents, and bind registered paths, branches, tips and cleanliness. Compare complete retained content through ReleaseRollForward, including Child Patches and approved rewrites. Names, ancestry after a rewrite, or a passing test alone cannot prove coverage. Drain each consumed worktree's native readers and preserve uncertain coverage, dirty or unique content.
- Remove each covered numbered parent independently of Main's remaining suffix. Later unrelated findings or tests do not renew its custody. Publish the selected action in the existing task, recheck identity and cleanliness, then run `git -C <Repository> worktree remove <ExactParentPath>` without force. Once registration is absent, run `git -C <Repository> update-ref -d <ExactBranchRef> <ObservedCommit>` and remove that parent from the task.
- This step never removes Current, Main or UpstreamBaseline. Retain only actual unconsumed sources or readers, not failed generations or numbering history. Other-tag roles remain invalid inputs, not deletion targets. Neither a fixed fork-count cap nor a future full-suite Pass decides retention.
- If Main still supplies unconsumed content, keep the fork Current after intermediate removal.
- Select its next retained Patch only through ReleaseRollForward's Unit and Lane boundary-approval decision.
- Once Current covers every retained frontier through Main, recheck both clean tips and run `git -C <MainPath> reset --hard <AcceptedCurrentCommit>`. Preserve the fork until Main's exact commit, cleanliness and canonical rows are verified; no merge, rebase or replay publishes Main.
- After verified promotion, remove superseded numbered worktrees and refs by the same sequence and publish CurrentWorktree as Main, omitting RetainedParents. Preserve the actual terminal and next-open discriminants. This closes existing frontiers, not future draft work.
- Recheck Git and the task before the next Patch. Interruption resumes these effects from actual remaining paths and refs, correcting only the observed task postimage. Never force deletion, invent recovery refs or states, or rerun tests to conceal partial effects. Convergence creates no Fork, test or export.
- Explicit Cleanup consumes the converged Main task; exact task-to-Main closure and committed export permit retirement without a test-result prerequisite.
- Export always uses atomic same-tag directory rotation, retains one tag-prev sibling, and commits that tag in private main without a release-completion receipt.
- Export never creates, removes, or repairs a product worktree. Baseline prepares only its selected tag's missing roles; Git registration owns existing paths, and a failed creation never authorizes removing a pre-existing role.
