# PatchExport

## Closed Prefix

```
ClosedPrefixExportOwner
+-- Consumer: CurrentClosedPrefixConsumer
+-- Successor: CompleteProjectionRotationSuccessor
`-- Terminal: RetainedPreviousProjectionTerminal
```

- `export` resolves registered MainWorktree and projects its entire canonical history from the release base through its actual closed Git frontier. It selects neither one lane nor a subset of Patches. An open Unit cannot be cut, and unknown later commits are not silently trimmed.
- Task Current may remain a fork. Its open Patch and terminal do not describe Main and cannot select export's source or boundary. Publication neither changes Current nor promotes, retires, or limits cross-worktree transport.
- When Current is Main, a task-open Patch already in its history returns Blocked/Input before projection. TaskWorkspace owns closure, even when every Child Patch is committed.
- An absent future selection does not block the preceding prefix.
- Export proves artifact publication, not full-suite acceptance or release completion. It selects no tests. A closed prefix may advance after export without a suite. Complete verification occurs only when selected by the current user; Export never fabricates a result.
- Before staging a projection, native export consumes ReleaseRollForward's source-coverage admission against Main and the active task's approved source changes.
- A short closed prefix remains publishable only when every source responsibility through its frontier is accounted for.
- Current Git-to-export equality cannot excuse a source responsibility missing from both.
- Resolve this single argv from the current task and clean Git frontier.

```text
"$SKILLS_ROOT/codex-build/bin" patch-replay
  --active-task <ExactIndexedActiveTask>
  --mode export
  --tag <CurrentTag>
```

- Export accepts no source, lane selector, endpoint range, generic runner flags, or payload. It never runs tests or generation.
- A closed-Unit prefix may be published before its lane closes, including to seed a newer release. Such an intermediate export alone does not cross an open lane or retire a release.
- Only explicit current-user authority for one export selects --skip-line-limit. It suppresses only that call's 1500-line publication limit, not empty diffs, Unit shape, Patch sequence, task custody, or source immutability. The next export and completed-release consumers use the default.
- Export projects only GitDiff commits. Schema and snapshot stay in the replaceable generated tail and never enter `.diff` artifacts. Source structure, responsibility, and exact publication checks still apply; flags accept no snapshot payload or source repair.
- Native Git produces each exported commit message and patch body as one GitDiff artifact, retaining canonical identity and first-commit responsibility metadata and omitting index lines through the existing byte policy.
- The declared Segment survives export and replay without path-based responsibility inference or an external metadata file.
- CrossGroup survives in the same first-commit contract; the shared native projection derives each cross artifact's three-digit group/member ordinal.
- Export readers compare every artifact path with that same projection after validating the complete source layout, rather than maintaining a separate ordinal interpretation.
- Pass replaces canonical <ProjectWorkingRoot>/build/patches/private/<Tag> with every included Patch projected from its distinct Git commit and the selected version documents below. Every invocation commits only that tag in private main; the artifact commit is not a release-completion claim. Another task may explicitly select that prefix while unfinished responsibility remains in the sole task.
- Artifact inspection lists only immediate canonical tag directories and the selected tag, never private `.git` or recursive storage. Requested projection uses this same export route.

## Version And Lane Notes

```
LaneNotesOwner
+-- Consumer: CurrentLaneNotesConsumer
+-- Input: version-root notes and selected lane notes from the independent working set
+-- Successor: InheritedLaneNotesProjectionSuccessor
`-- Terminal: ReviewedLaneNotesPublicationTerminal
```

- Every emitted tag root and lane contains one regular 0644 notes.md, empty when no guard applies. Shared constraints appear only at the root; lane files contain only their specific additions. Nonempty bytes are exact current-user-approved critical cross-Patch guards, never progress, history, results, reviewer data, or Patch inventory.
- Nonempty notes are UTF-8 text with a final newline and consecutive unindented `- ` lines, one concise reminder per line. Blank lines, headings, numbered or nested lists, paragraphs, tables, fences, and front matter are invalid.
- TaskWorkspace owns the editable two-level set independently of worktrees and exports. ReleaseRollForward owns copying and approved mutation; export only reads it.
- Preserve exact inherited entries. Adding, rewriting, moving, or removing one requires presenting the exact flat line and explicit current-user approval. Task edits, Patch completion, tests, and agent importance grant none. Keep initialized empty files. Notes never move Patch responsibility; resolve contradictory guards with the user before dependent edits.
- The helper requires root notes and every emitted lane's notes, validates their format, captures exact bytes, and rechecks them immediately before atomic publication. It copies them without export-time inheritance or fallback. Missing or changed drafts reject publication. Export never modifies the working set.
- Export always reads root notes and only emitted lanes, preserving other drafts for later work. No notes-only lane is emitted. Cleanup requires all remaining nonempty notes at either level to match publication before retirement.
- The next Established Baseline retires its predecessor's published working copy, not its exported notes. Roll inherits from the canonical source prefix; it never needs retired working copies as a second source of truth.
- Historical prefixes missing either notes level remain readable. Present notes share syntax validation and exact byte-and-mode source custody. Unit enumeration and application consume only GitDiffs. Private Git includes both levels; product-source comparison validates notes without treating them as source or Patch rows.

## Cross Relationship Diagram

- Every emitted cross lane contains one nonempty regular 0644 `relationships.txt` file with an ASCII graph and a final newline.
- Use one compact plain-text graph of the current cross lane, without Markdown fences.
- Include only current cross responsibilities and relationships between them; no other lane, including draft, appears anywhere in the file.
- Use short functional labels that identify the corresponding cross Patches.
- Connect nodes with concise labels explaining how their functions work together.
- Show each member once within its CrossGroup instead of repeating membership, dependency and exclusion inventories.
- The diagram records neither execution progress nor test results and supplies no authority to add members, move responsibilities or reorder Patches.
- ReleaseRollForward inherits an existing diagram into TaskWorkspace's working file and updates that file when an authorized relationship changes.
- An absent inherited diagram requires source-grounded authoring before cross acceptance; an empty placeholder does not satisfy this contract.
- Diagram maintenance follows the authorized feature scope rather than notes.md's separate exact-line approval rule; it cannot change the approved scope itself.
- PatchAcceptance checks cross-only scope, readability and source-grounded relationships before publication.
- Native export requires the working diagram whenever cross is included, freezes its exact bytes with the notes, and rechecks that same file before the existing atomic publication.
- Retained-prefix reading and product-source comparison require the diagram for cross and exclude it from semantic Patch enumeration.
- Missing diagrams return an input finding; export never invents a graph, inherits at publication time, or substitutes a rendered image.
- Existing source custody, whole-tag rotation, private Git publication and exact published-working-document cleanup include the diagram without a second store or lifecycle.

## Shared Rotation

- Existing private storage uses ProjectWorkingRoot only as a locator. Initial cloning freezes committed main; unrelated project checkout bytes are neither admission inputs nor mutation targets.
- Export replaces the complete tag directory in one staging transaction. Remove staged Tag-prev, move staged Tag to Tag-prev when present, then generate the entire diff and selected-document projection into absent Tag. Preserve the selected document bytes through this regeneration without merging stale destination diffs or keeping retired lanes.
- One sibling private.pending preserves the repository and other tag bytes. Linux atomic directory exchange publishes the complete pair and prepared Git commit together. Initial publication renames the absent root. Unsupported exchange and unexplained residue block.
- Private main must have no staged changes or sequencer. Uncommitted changes under canonical tag directories are normal prefix data. Bind exact tag bytes and modes, private HEAD, and the selected pair before staging and immediately before exchange; reject unrelated dirty paths. Never commit, reset, or remove another tag's prefix.
- Tag-prev is the sole previous generation, excluded from private Git and replay. It belongs to user cleanup; the next export replaces only that generation.
- Failure before exchange removes only this transaction's stage. Failure after publication reports the completed effect and retains the new pair without automatic re-execution.
- The internal repository copy is publication machinery, not an inspection route. No migration, receipt, recovery ref, or replacement product worktree is introduced.

- One nobody commit stages only this tag and advances private main. Publication validation uses this tag's own last changing commit and exact tracked directory bytes; unrelated later commits or another tag's uncommitted prefix do not invalidate it.
