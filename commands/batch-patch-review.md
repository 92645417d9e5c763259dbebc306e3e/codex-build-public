# BatchPatchReview

- Own artifact and package-lineage review with review-record-only LocalEdit.
- Freeze inputs, then return one path-ordered Flat Patch or Child Patch row per visible `.diff` artifact.
- Require reproducible correctness, replay, ownership, coupling, and observable contract evidence for each finding.
- Verify retained package bytes and references before boundary analysis.
- Sort boundary rows by PackageBuildTimestamp, semantic ReleaseVersion, then BuildSequence.
- Determine presence only from BehaviorIDs membership.
- Promotion, current selection, and directory presence prove nothing.
- Return Pass without findings or Blocked with bounded findings.
- Mutate no package, product, Git state, or validation state.

## Inputs

- Artifact input binds commit, tag, tree, and canonical Lane projection: Flat Patch uses one lane-level `.diff`; Unit uses at least two ordered Child Patch `.diff` artifacts. No `.patch` file is valid.
- Boundary input binds BehaviorID, complete retained-package inventory, metadata, bytes, timestamp evidence, and projection history.

## Return

- Artifact review returns ordered Lane, Unit or Flat Patch, Child Patch when present, observable, bridge, path, generated-output, verification, deletion, dependency, and gap rows.
- Package boundary evidence consists only of the inspected canonical package paths, exact metadata fields, build versions, build sequences, and the last-absent and first-present observations used by the decision.
- Incomplete, duplicate, ambiguous, mismatched, or nonmonotonic evidence returns Blocked without control.
