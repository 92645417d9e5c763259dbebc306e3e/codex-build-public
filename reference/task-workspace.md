# TaskWorkspace

## Roots

- The effective working root defaults to cwd; its build directory owns builds and opaque Runner artifacts and output under build/tmp.
- `build/docs` owns retained user-authored project documents. A user-named Markdown basename resolves beneath this root, while an explicit path remains explicit; consumers open only the named document and do not inventory sibling contents.
- `$PROJECT_SCRATCH_ROOT` owns worktrees, tasks, locks, transactions, and durable attempt evidence, not Attached.
- `$SKILLS_SCRATCH_ROOT/<skill>/topics` is target-owned.
- `$CODEX_HOME` owns product state.
- Attached cleanup owns only tracked source restoration and the exact ordinary non-ignored untracked inventory at the supplied repository root. Root classification never makes ignored descendants or another supplied root disposable.
- Editable notes occupy one file at <ProjectScratchRoot>/lane-notes/<Tag>/notes.md.
- Platform, account, transport scope, and other approved cross-Patch guards belong there, not in skill defaults or Patch summaries.
- The file survives Current, Fork, reset, promotion, and export rotation.
- PatchExport owns exact entry approval and read-only publication.
- The cross diagram's editable owner is `<ProjectScratchRoot>/lane-notes/<Tag>/5-cross/relationships.txt`, separate from Markdown and task records.
- Its published copy is `<ProjectWorkingRoot>/build/patches/private/<Tag>/5-cross/relationships.txt`; inheritance and authorized relationship changes update the working file before export.
- PatchExport owns the diagram contract and exact publication. The same working-set lifetime preserves it across Fork, promotion and partial exports, and retires only an exactly published copy during selected cleanup.

## Worktrees

```
CanonicalGlobalReleaseRoleOwner
+-- Consumer: ExactRegisteredTopologyConsumer
+-- Successor: PredecessorRetirementAdmissionSuccessor
`-- Terminal: SingleGlobalWorktreeTopologyTerminal
```

- Attached enters Roll only with an empty regular root AGENTS.override.md; Baseline and Fork create the same ignored file in every canonical release worktree.
- The sole release uses one Git repository's exact registered Main, UpstreamBaseline, and fork roles at <ProjectScratchRoot>/main-worktree/source, <ProjectScratchRoot>/upstream-baseline/source, and <ProjectScratchRoot>/forked-worktrees/NNN for ordinals 001 through 999. Branches remain tag-bound but paths do not; role admission rejects alternate-tag, relocated, mixed, duplicate, unregistered, or unrelated roles.
- GitCustody owns topology, fork ordinals, Flat Patch and Unit transport, retained generations, and reader-bound cleanup.
- Fork numbers are available names from current Git branches and registered worktrees, not persistent identities. Consumed worktrees and refs reserve no number; no test record, counter or completed-fork list participates in allocation.
- The open task names Current and retained parent frontiers across context restoration; worktree names never select direction.
- Before Fork, OpenUnit, OpenFlatPatch or OpenTransform names the historical edit and one terminal field names Current's actual last row. An inserted Patch may be absent from Git; forward append still stays on Current. Every fork is Current or RetainedParents. Adoption publishes those paths, the open selection and actual pre-authoring terminal.
- When Current is Main with no retained parents, omit the complete RetainedParents line. A present blank value, none, or empty-list spelling is not the no-parent form. Every task-named Current fork retains at least one exact parent.
- GitCustody convergence precedes the next selection after closure and on reentry.
- A selected retained Apply range retains its source readers until the invocation returns, then consumes convergence before another range, rewrite or Fork.
- Remove covered intermediate parents without waiting for Main coverage.
- Full retained-frontier coverage promotes Current to Main.
- RetainedParents lists needed sources or readers, never failed-generation history.

```
SelectedForkConvergenceTaskBoundary
+-- CurrentWorktree: exact current Main or fork-worktree path
+-- RetainedParents: ordered task-named parent paths
`-- CurrentTerminal: exactly one of CurrentTerminalUnit CurrentTerminalFlatPatch or CurrentTerminalTransform
```

- A present RetainedParents field contains one or more unique existing absolute parent worktree paths joined only by `, `, without a conjunction or other prose. Its absence is permitted only for Main; publication before Fork resolves this actual topology fact without a placeholder or default.
- Export reads Main's closed Git history, independently of Current. Main remains until covered; numbered parents leave independently after coverage and reader drain. Future approved work comes from declared Patch topology and Git rows, not a RemainingSuffix field or release-completion marker.

## Records

- Commands admit current records only.

```
TaskIndexListContractOwner
+-- CurrentTaskIndexAdmission: three exact lists
+-- SingleActiveTaskPathConsumer: the sole unfinished release is the one ActiveTask
`-- CanonicalThreeListTaskIndex: empty role is []
```

```
ClosedPatchTaskReductionOwner
+-- Consumer: SemanticPatchClosureConsumer
+-- Successor: NextAuthorizedPatchSuccessor
`-- Terminal: BoundedReleaseTaskTerminal
```

- Apply conflict retains the successful prefix and selected open Patch; conflict alone selects no approval stop.
- Consume ReleaseRollForward's scope and measured-benefit decisions before dependent work.
- Missing evidence and an explicit pause still stop their dependent successor.
- Every release anchor ends with an empty or populated `## Additional Instructions` section reserved for user-added bullets.
- Keep this heading last when creating or updating the anchor, including when its bullets have been consumed.
- At each reconciliation boundary below, read the anchor through EOF and consume those bullets before selecting the next action.
- Integrate each observed bullet's complete meaning into the appropriate existing pending instruction, constraint, or unresolved decision in the anchor before removing that exact original bullet.
- Removing an input bullet means its instruction is incorporated, not that its requested work is complete.
- Apply the governing scope and approval rules before effects; retain an unresolved instruction and its required question rather than silently dropping it or inferring new authority from file content.
- Reread the current file before the contextual edit and preserve additions or changes not included in the consumed preimage.
- A concurrent edit returns to the current file for reconciliation instead of replacing it with an earlier whole-file copy.
- An active operation retains its process and completes before these additions select a successor; no watcher, polling loop, interrupt, or separate instruction queue is added.
- If task-byte admission reports an edit during an operation, reconcile its actual effect with the current anchor instead of repeating the operation to restore old task bytes.
- Reconcile on every entry, whole Apply return, completed Unit or Flat Patch authoring, selected test or review return, and before export, BuildInstall, task replacement, retirement, handoff or next selection. Read the entire anchor, actual Git rows and relevant source and existing results before deriving work. If interruption separated Git effects from task publication, reconcile those effects first; never repeat transport merely to repair stale prose.
- Before BuildInstall, reconcile the current request and any indexed task against the selected Main implementation and required verification.
- An indexed Roll also consumes ReleaseRollForward's complete inventory comparison before BuildInstall or task retirement.
- Unresolved current requirements keep their existing product successor instead of selecting installation.
- Only explicit user approval naming the unresolved remainder permits deferral, withdrawal, or a partial build or installation.
- Preserve approved deferrals and their conditions in the task; generic build, cleanup or newer-tag requests do not imply that approval.
- Explicit draft quick iteration may build its verified temporary suffix before the required fold; unrelated pending requirements remain unresolved.
- Task retirement requires every obligation fulfilled or explicitly withdrawn; deferred work still needs its task.
- During selected draft quick iteration, native Git supplies the temporary fixup suffix and PendingWork retains its draft owners and fold obligation.
- Canonical terminal fields remain at the last canonical row until that suffix is folded under ReleaseRollForward.
- Resume this native suffix before invoking canonical-history consumers.
- For each affected pending instruction, compare its requested behavior with Current's implementation and selected verification outcomes. Remove satisfied actions, duplicate reminders and obsolete successors wherever they occur. Partial fulfillment replaces an action with only its unresolved remainder and relevant evidence.
- Preserve the original user decision and its source for every boundary still governing pending work, including explicit group members, exclusions and deferred integration.
- Keep that decision once beside the affected scope; distinguish it from candidate rationale, current metadata and the next execution action.
- Completing a member or its tests does not discharge a boundary still governing other members or deferred consumers.
- Reentry consumes OriginalPatchPurposeOwner before selecting work; recover missing user decisions from available history rather than infer them from task prose or ask for repetition.
- When that owner rejects a premise, replace all dependent pending decisions together without changing the user's controlling boundary.
- Remove resolved cause, design, detail and result locators only when no remaining admitted action depends on them; preserve no completion history or separate grouping ledger.
- Matching Patch names, clean Apply, or a passing narrow test cannot discharge a different outstanding rewrite or stronger failure. Conflict preserves the accepted prefix and rejected Patch; missing children, unmet behavior, failed checks and interrupted checks retain their exact remaining action. Reconcile each completed selected check before choosing another action without creating a full-suite gate.
- Keep the anchor limited to release identity, necessary task topology, and ongoing, deferred, or pending-verification work.
- Keep unresolved decisions and their controlling user boundaries; native Git retains committed per-responsibility contracts.
- Retain implementation detail only when it is necessary to resume deferred work.
- Recover completed behavior from its source, current decisions, and relevant history rather than retaining its implementation contract in the anchor.
- A compact Git inventory and selected contracts narrow source navigation after authority recovery; a short-context return does not change that order.
- Derive the first unresolved action from ReleaseRollForward's complete typed inventory comparison rather than the lane label or last successful operation.
- SourcePatchChanges optionally names unique typed Unit:Lane/name or FlatPatch:Lane/name selectors joined by comma-space.
- Omit SourcePatchChanges when no approved source-structure difference exists.
- A selector belongs there only after current-user authority establishes its withdrawal, replacement, rename, relocation or changed Child Patch boundaries.
- Each selector must exist in a declared canonical source inventory.
- The field exempts only that historical structure from native source coverage, never its surviving behavioral obligations or the approval requirement.
- Keep the establishing user decision and any unresolved replacement work beside its affected pending scope.
- Do not copy the complete source inventory, completion state or helper findings into this field.
- Native Git and canonical sources remain the only Patch inventories.
- Preserve the complete declared future order and unresolved work on other Patches.
- Git owns completed order.
- Native results own test evidence.
- Atomically publish actual Current and retained paths, the actual terminal, and the first unresolved action in declared order as open selection and successor.
- With no selected open Patch, omit its fields.
- Reread the postimage to ensure prose and fields choose the same remaining action before continuing.
- Closure consumes direct cumulative semantic acceptance and GitCustody convergence before publishing the task and invoking its successor.
- Continue declared work within the user's Roll scope without repeated approval or an approval field.
- Retain any missing user decision or explicit pause with its unresolved obligation instead of treating task publication as approval.
- Before closing a lane, compare every admitted source Unit, Flat Patch and ordered Child Patch in that lane with its accepted current Git responsibility and the authorized disposition of every difference.
- Any missing or unexplained entry forbids lane crossing, a completion claim or removal of its pending work.
- A shorter closed prefix may still be exported without closing its unfinished lane.
- Clean Git, matching Patch names, passing checks and artifact publication cannot discharge a different pending requirement or select a completion handoff.
- A question cannot reorder topology or establish an accident. Only actual topology mismatch requires renumbering and approval; otherwise answer and continue the reconciled successor. Task reduction grants no changed responsibility and does not edit separately approved version-note guards.
- A requested recap consumes the task, Git, available Apply diagnostics and selected check terminals; it creates no recap file or copied progress state.
- Explain rejected Apply inputs and the reason for reconstruction, followed by the committed Child Patch rewrite order and observed outcomes.
- Distinguish verified results, missing historical evidence and remaining work with its next action; final Git alone cannot prove failed attempts or test outcomes.
- An explanation-only request keeps reconciliation read-only and reports stale task text as such; a pause or stop suspends the active Roll successor.
- An open Unit names unique ordered ChildPatchSequence. An open Flat Patch names itself, and an open Transform names TransformPatch. Each also names CurrentWorktree and retained Sources.
- The open selection binds Apply's first responsibility; an explicitly bounded unchanged retained range consumes later complete source members through PatchReplay.
- Retained Apply mechanically publishes the actual terminal and next selected open responsibility at each completed range boundary while preserving pending prose and Additional Instructions.
- Its completed endpoint has no open selection until the product owner reconciles the remaining work and selects the next authorized action.
- Publish the selected Patch, complete Unit sequence, adopted topology, pre-authoring terminal, cause and successor before authoring. Keep that selection through Unit-local reset, rebase and final Child Commits. At the completed source boundary publish its actual terminal before a selected test; retain unresolved verification or behavior. No per-Child transition or premature Unit closure follows a partial prefix. A handoff during authoring preserves the unfinished selection and truthful continuation.
- Candidates before topology resolution are unordered; declared order survives reduction.
- Roll executes the reconciled order within the user's authorized scope; an explicit pause selects read-only exploration on clean Current.
- Preserve the complete approved purpose and its establishing source or user decision. Candidate rejection does not cancel requirements; task wording, repair hypotheses and tests cannot replace or narrow them.
- Record findings and any selected complete reconstruction against their assigned Unit or Flat Patch. Retain every unmet obligation and its original authority until implemented or explicitly withdrawn; a shorter or passing replacement cannot clear it. Tree-wide plans retain unresolved requirements only.
- The rewrite start decision records actual Main, proposed start, and disposition of existing Lane children and suffix sources before Fork or reset. Desired state never replaces observed facts.
- Derive later-release behavior from current decisions, predecessor Patches, and history, but bind it only to its current upstream implementation owner. Task reduction cannot withdraw the obligation or retain a retired path; a nonunique map returns to ReleaseRollForward.
- Every release anchor records SubagentReview as yes or no.
- Initialize the setting to no unless the user explicitly selects yes.
- Change an existing setting only at the user's direction.
- Preserve the choice through task reconciliation, Fork, promotion and tag replacement.
- The field grants only PatchAcceptance's review permission and stores no review progress or acceptance state.
- Roll implementation remains linear in the product agent.
- A direct material finding uses actual Git and the existing Unit or Flat Patch correction boundary. Preserve later work and never restore saved task bytes or falsify its frontier.
- The task has no completed list, archive, event log, checkpoint, cursor, helper progress, test verdict, reviewer state, Roll mode or second representation.
- Explicit Cleanup removes drained execution records and logs, preserving tasks/rust-suite-baselines.json as baseline comparison facts. Selected release retirement also removes the closed task and published working release documents while retaining Main and UpstreamBaseline. Next-tag Baseline cleanup retires its primary predecessor's published working notes. Source restoration and temporary cleanup leave final patches, binaries and reusable build inputs untouched.
- EditNamedTask publishes the sole release-roll-<Tag>.md before baseline, at Fork adoption and through reconciliation above.
- Temporary Main fixup retains its focused-tested fold plan until absorbed.
- PatchReplay synchronizes retained-range fields and generated-tail terminals through their declared exact-preimage owners.
- Semantic pending instructions remain the product agent's responsibility.
- ActiveTask contains zero or one release task. OtherTasks is always empty and OverallTask keeps its independent existing role. The task survives each selected roll stop until explicit retirement. No semantic-version ordering or next-task selector exists.
- Each task declares SourcePrefixes as one or more unique release tags joined by comma-space. The first is the primary predecessor and differs from the task tag. Further entries may name older, newer completed, or same-tag retained canonical prefixes. They provide read-only purpose evidence and exact unchanged transport input for the declared next Patch; they do not authorize changing its responsibility.
- Before replacing the sole task for a user-selected newer tag, report pending work and carry every unresolved obligation, approved deferral and controlling user boundary into the replacement.
- Preserve source-prefix and version-note custody across that replacement.
- Reread the replacement through EOF before Baseline retires predecessor roles.
- ReleaseBaseline consumes the sole selected task and index unchanged. It admits either that tag's canonical global roles for same-tag continuation or its primary predecessor's canonical global roles after product work replaced the sole task without losing unresolved custody. A predecessor with private history requires the exact exported primary prefix; an absent prefix is admitted only when Established exists and clean Main equals the official tag parent.
- The predecessor roles are retired and replaced at the same paths before selected-tag preparation returns; no completion receipt or invented empty export is required. Failure after preparation resumes the same tag and chain. Tag-scoped, mixed, duplicate, or unvalidated predecessor topology blocks.
- The single rust-suite-baselines.json contains at most the selected release and its primary predecessor, each mapped to exactly one completed matching Established generation. Inheritance reads the primary predecessor without changing the existing pair and leaves the new tag absent; validated cross-tag Pass publication atomically replaces an older generation with the selected entry while retaining that predecessor. Same-tag publication preserves the pair.
- Baseline begins from predecessor Established or exact zero coverage. Empty collection and missing selected entry are initial producer inputs, never accepted product coverage, pending state, or permission to promote without Pass.
- Canonical publishers create a missing three-empty-list index or empty-object baseline collection, then re-enter.
- Existing files are not normalized.
- The selected action must provide every owned value.
- PatchReplay receives the sole indexed ActiveTask as an exact absolute path and neither discovers nor initializes a replacement task during a product operation.
- No initializer, legacy parser, converter, migration state, compatibility input, fallback, or alternate writer exists.

## Attempts

- Helper defects stop Product until installation restores it, records bind terminals, and installation changes no worktree.
