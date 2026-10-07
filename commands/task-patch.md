# TaskPatch

## Boundary

- TaskPatch owns non-source task and artifact responsibilities only.
- It validates TaskName, ReleaseVersion, task record, target ledger, DraftPath, artifact path authority, and unrelated-byte preservation.
- It does not request, register, inspect, branch, reset, edit, author, replay, or validate a source worktree.
- Source mutation belongs to the task; PatchReplay names the exact Unit, Flat Patch, Child Patch, or Transform and preserves one commit per semantic Patch.

## Artifact Preparation

- Consume completed MainWorktree projection and typed patch-structure and validation evidence as immutable input.
- Freeze the task identity, owned targets, artifact destination, absence or replacement authority, and artifact validation plan before materialization.
- Under TaskLock, derive the target-limited artifact from that custody-provided projection without changing source Git state.
- Hold ArtifactLock only for NoReplace or authorized atomic replacement of the artifact destination and rollback of that destination.
- A failure removes only task-owned artifact staging and returns its typed result; it cannot restart source work, select a new source route, or create a replay checkpoint.

## Acceptance

- Require canonical task identity, injective ledger, target-limited artifact content, unrelated-byte preservation, artifact containment, and the consumed MainWorktree evidence.
- TaskPatch records artifact validation and task status but does not compile or test the skill.
- Product compilation, selected-scope test compilation, static checks, and author review belong to PatchAcceptance before publication.
- Pass returns the task and artifact result without a source branch, source worktree, source base, source replay root, or source-Git continuation.
