# PatchFormat

## Physical Inventory

- Canonical history is `Lane -> Flat Patch` or `Lane -> Unit -> Child Patch`. A Unit has at least two ordered distinct Child Patches. Each Flat Patch and Child Patch is one commit and GitDiff; no single-Patch Unit exists.
- Canonical ordinary Lane order is bugfix, normalization, backend, mixed, frontend, cross, and draft.
- Their directory names are `0-bugfix`, `1-normalization`, `2-backend`, `3-mixed`, `4-frontend`, `5-cross`, and `6-draft` in that order.
- Upstream remains the official baseline role, not a Patch Lane or exported directory.
- The initial normalization ExecutableTransform remains the first history row before all ordinary Lanes, including bugfix.
- Commit metadata is discriminated: GitDiff commits name either `FlatPatch` or both `Unit` and `ChildPatch`; ExecutableTransform commits name `Transform` and `TransformPatch`. Generic Unit-as-Patch metadata is invalid.
- Unit, Flat Patch and Child Patch names use three semantic parts in every Lane: action, object, and method or scope.
- Each semantic part may contain multiple hyphenated words.
- Names use the ordinary lowercase hyphenated grammar without export ordinals.
- For example, `report-usage-via-session-files` separates into `report`, `usage`, and `via-session-files`.
- Semantic acceptance identifies those parts from the complete responsibility rather than requiring exactly three words or a fixed preposition.
- Before transport or authoring, identify each name's action, concrete object and method or scope from its assigned effect.
- A noun-only label or an unsupported semantic part fails review even when native lexical admission passes.
- Native transport validates lexical form rather than claiming to recognize those meanings.
- Reserved Transform and CrossGroup names retain their existing naming contracts.
- An authorized rename preserves behavior, responsibility boundaries and child order while updating native metadata and direct references through existing Git custody.
- Flat Patch names equal their Patch field in every Lane, including draft.
- Exported semantic Patches end only in `.diff`. Flat Patch uses `<lane>/<ordinal>-<flat-patch>.diff`; each Child Patch uses `<lane>/<ordinal>-<unit>/<child-ordinal>-<child-patch>.diff`. No Patch artifact ends in `.patch`.
- Ordinary Lane-child ordinals have two digits and start at 01, except cross ordinals, which have three digits under Cross Groups below.
- Child Patch ordinals have three digits and start at 001 inside each Unit.
- Each exported GitDiff contains its native canonical commit message followed by its patch body, preserving the declared Lane, Patch identity, Form, and Segment without a sidecar.
- Export readers admit only that current form and require its metadata to match its semantic artifact path.
- A is immutable and B projects it; GitDiffs retain headers, modes, hunks, and bytes, while invalid forms stay unexported.
- Targets retain accepted prefixes; another source supplies only the selected Unit or Flat Patch.
- Commits, not records, order work; List, Apply, Commit, and Export recheck history.
- Exports regenerate GitDiffs plus root and lane notes and the cross relationship diagram into an absent staged tag. Lane-child ordinals derive from Git history; tag-prev is outside the projection.
- Each Patch's one commit and GitDiff contains semantic source only; editing an earlier committed child uses Fork.
- Every Patch retains name, commit, diff bytes, and position. No operation combines or splits Patches. Structure changes require explicit current-user approval.
- File moves, upstream adaptation, or matching metadata cannot transfer behavior between Units, Flat Patches, or Child Patches.

```
PatchGitDiffLineAdmission
+-- AuthoringCommitAdmission: exact staged GitDiff at most 1500 lines
+-- PatchExportAdmission: every projected GitDiff remeasured before publication
+-- OneCallPrefixOverride: explicit current-user skip-line-limit only for one Export
+-- CommitLineLimitOverride: explicit current-user skip-line-limit only for one Commit
`-- ProductFinding: semantic Patch plus canonical diff body actual lines and fixed limit
```

- A snapshot forces no separate Patch, Unit, Child Patch, or line exemption. At most one `regenerate-snapshots` Transform ends the current history and remains unexported.
- Commit appends one declared Child Patch or creates one Flat Patch. Rebuilding a Unit preserves one commit per Child Patch.
- Within each ordinary Lane, all Units precede all Flat Patches; cross applies this rule within each consecutive CrossGroup instead of across unrelated groups.
- Draft follows the same layout rules and has no type-order exemption.
- Bugfix admits Flat Patches only.
- Live history, prospective changes and exported prefixes share Lane, CrossGroup, type, generation-impact and direct-predecessor admission.
- PatchReplay returns bounded Blocked/Input for invalid dependencies before product source or private export mutation.
- Complete assigned-behavior and Rust semantic acceptance precede the mechanical line check. Its ceiling authorizes neither compensating growth nor feature deletion, and splitting a rejected design does not resolve it.
- Patch authoring measures the staged GitDiff before commit. Commit measures its new Flat Patch or Child Patch, while Revise measures the amended Patch and each later Child Patch it regenerates. Empty or oversized output restores the exact pre-operation input and returns Product Required without publishing any Patch commit.
- Projection measures every canonical Git diff body against the 1500-line limit.
- Exported commit messages remain complete and are excluded from the Git diff line count.
- Export independently remeasures committed GitDiffs and returns the same owner-bound Product Required before private-tree mutation.
- Only --skip-line-limit on an explicitly authorized Commit suppresses that call's oversized finding; empty GitDiff and every other admission still apply. The authorization is not stored or inherited.
- An explicitly authorized export separately supplies --skip-line-limit for its own projection. It creates no persistent exemption and never applies to Revise, Apply, completed-release consumers, or a subsequent call.
- The source reader validates paths, metadata-derived sequences, patch syntax, and bytes. Apply does not reimpose authoring size policy on retained input or add a legacy import mode.
- Retained-directory comparison uses exact canonical bytes; a new export reapplies current publication policy.

## Responsibility Classification

- ReleaseRollForward's OriginalPatchPurposeOwner makes the single semantic classification decision before metadata or transport.
- Current user decisions fix observable behavior, explicit group members, exclusions, and maturity; metadata records the resulting assignment.
- Backend owns a complete non-presentation feature and its interfaces.
- Mixed owns a feature with one backend responsibility paired with one frontend responsibility.
- Frontend owns presentation using complete existing interfaces without adding a backend feature.
- Cross owns an approved non-one-to-one functional relationship among backend producers and frontend consumers that must change together to deliver the assigned behavior.
- Count independently assigned feature responsibilities, not functions, events, API calls, files, schema types, snapshots, or graph edges.
- Without that functional relationship, retain independent responsibilities in their applicable ordinary Lanes.
- An explicit user-defined group is a fixed scope boundary, not a seed for dependency closure or automatic member discovery.
- A member must contribute to that group's assigned producer-consumer behavior; sharing infrastructure with a member is insufficient.
- Recording an independent feature or calling its utility does not move that feature into the recorder's or caller's group.
- A shared widget qualifies only when the assigned features require coordinated changes to its observable behavior.
- Neither a real private dependency nor generation impact establishes group membership.
- Bugfix and normalization retain their existing purposes before formal feature Lanes.
- Backend, mixed, frontend and cross form the formal prefix; draft retains unfinished or unpromoted features.
- An approved future integration informs the functional boundary without implementing it or promoting its draft consumers.
- The complete formal prefix must build and preserve its approved behavior and tests without draft.
- Draft may consume complete formal interfaces; shared interfaces have one actual owner without merging otherwise independent features.
- Moving an interface or integration requires approved responsibility redistribution and preservation of every admitted consumer.

## Direct Prerequisites

- Requires names only private behavior that the consumer actually needs for its approved outcome.
- Establish the native upstream behavior first, then identify the prerequisite Patch's required behavioral addition or change and its actual consumer.
- An existing API call is not evidence that the private Patch touching its implementation supplies that API or the behavior this caller requires.
- Verify each asserted private dependency against the relevant native implementation and private delta before retaining or publishing it.
- Declare every proven direct prerequisite; omit merely transitive predecessors unless their behavior is also used directly.
- Upstream behavior and mandatory generated transforms are not private prerequisites.
- Each prerequisite uses an existing typed selector for an earlier complete Unit or Flat Patch, such as Unit:backend/serve-models-and-announce-changes.
- Earlier-only admission excludes self-dependencies, missing or later producers, cycles and formal-to-draft dependencies without a separate graph store.
- A missing required seam returns to its actual responsibility owner; it does not authorize speculative interfaces or a new group member.
- Dependency order and canonical layout must both hold.
- Preserve predecessor-relative order unless an approved change requires adjustment.
- Incompatibility returns to ReleaseRollForward's responsibility decision without automatic merging, splitting, reclassification, or weakened metadata.
- Native admission checks these declarations structurally; it cannot establish their semantic truth.

## Cross Groups

- The cross lane requires one `relationships.txt` ASCII graph describing every approved group under PatchExport's Cross Relationship Diagram contract.
- The diagram is a reviewed explanation of user-authorized functional relationships, not a Patch, Unit, group directory, metadata source or executable dependency graph.
- Each cross responsibility declares one semantic hyphenated CrossGroup name in its first-commit contract.
- A CrossGroup groups related complete responsibilities without becoming a Unit, Patch, commit, directory, dependency selector or separate stored object.
- All responsibilities of one CrossGroup are consecutive; a closed group cannot reappear.
- First appearance assigns group digits 1 through 9 in Git order.
- Within a group, complete Units and Flat Patches receive member numbers 01 through 99 in Git order.
- The exported three-digit Lane-child ordinal concatenates the group digit and member number, so the first group starts at 101 and the next at 201.
- Members retain their own names, contracts, direct dependencies and Child Patch boundaries.
- Each group independently applies Unit-before-Flat-Patch order and the generation-impact region below.
- CrossGroup encodes the already justified membership; its name and ordinal supply no admission evidence.
- Native transport derives ordinals from the sole commit metadata and rejects exhausted widths instead of widening or wrapping names.

## Continuous Generation Impact

- Segment describes the generation impact of the complete Unit or Flat Patch, never an individual child or generated payload.
- The admitted values are none, schema, snapshot and schema-snapshot.
- Schema impact means the responsibility changes app-server protocol source or another generator input that changes the app-server schema postimage.
- Snapshot impact means the responsibility changes a snapshot-producing behavior or its expectation such that authoritative snapshot regeneration is required.
- A responsibility affecting both declares schema-snapshot; every Child Patch retains the same declaration.
- Backend, mixed, cross and draft admit all four values; bugfix, normalization and frontend admit only none or snapshot.
- In each ordinary Lane, or independently inside each CrossGroup, affected responsibilities occupy one continuous region.
- Within that region, actual direct dependencies determine responsibility order without a generation-impact category rank.
- Segment none may appear before or after that region, never inside it.
- Unit-before-Flat-Patch order applies across the whole same scope, including the affected region and its surrounding none responsibilities.
- Review actual generators, output producers, assertions and callers to assign impact before publishing the responsibility contract.
- Helpers check declared continuity and reject protocol-source changes without schema impact; they do not infer semantic snapshot impact from paths or test results.
- The source layout makes generation responsibility observable but creates no per-region generator, transform commit or test gate.

## Responsibility Contract

- Every Flat Patch commit and the first Child Patch of every Unit appends exactly Purpose, Requires, Provides, Boundary and Verification to its ordinary metadata in that order.
- Cross appends CrossGroup after Verification at that same owner; every other Lane forbids that field.
- Subsequent Child Patches and generated transforms contain no responsibility fields.
- Each field is one nonempty printable ASCII-English line, separated by the same blank-line convention as existing metadata.
- Purpose states the approved observable outcome.
- Requires lists unique direct typed selectors separated by comma-space, or the exact value none when no private predecessor is required.
- Provides states the complete behavior and interfaces owned by this responsibility, including applicable lifecycle and failure semantics.
- Boundary states what remains with its producers, consumers or other owners.
- Verification names the relevant source or test entrypoints and observable assertions rather than a stored result or a mandatory rerun command.
- Keep these fields concise without dropping required behavior or packing unrelated obligations into shorthand.
- Native Git owns the sole contract copy; exported GitDiff metadata projects those exact fields.
- Contract changes follow the same Unit or Flat Patch authority and Git boundary as source changes.
- Apply and Unit-local suffix replay preserve each contract at its original first-commit owner.
- Revise retains the existing contract unless an authorized complete replacement is supplied for its owning commit.
- Bare ordinary histories without their required contracts are not admitted as a second format.
- Historical noncanonical patches remain read-only purpose evidence, never compatibility inputs.
- No sidecar, dependency database, contract version, reverse-dependency ledger or automatic reorder route is introduced.

## Metadata And Schema Terminals

```
GeneratedSchemaTerminalOwner
+-- CurrentOrdinarySourceFrontier
+-- CurrentTargetSchemaRegeneration
`-- UnexportedTransformCommitThenAncestorConvergence
```

- Every commit uses exactly one metadata shape above plus Lane, Form and Segment, with responsibility fields only at their declared owner; malformed or mixed discriminants are Blocked.
- Helper commits use nobody, epoch, exact subject, and fields.
- Generated commits follow all ordinary Patches: schema when required, then snapshot. Their Lane identifies the current source frontier, not a permanent place among that lane's Patches.
- Transform names are reserved; unexported `ExecutableTransform` uses Segment `schema` or `snapshot`.
- Ordinary Segment values and continuous placement follow Continuous Generation Impact.
- A GitDiff touching protocol source must belong to backend, mixed, cross or draft with Segment schema or schema-snapshot.
- Cumulative schema generation is required exactly when an ordinary responsibility declares schema or schema-snapshot.
- Snapshot-impact metadata changes source placement only; authoritative snapshot generation remains selected Finalize's responsibility.
- A temporary schema tip is no Unit, Flat Patch, exported artifact, suffix member, or accepted lane terminal. A failed transform, test, or restoration blocks replay.
- Generated tails are not pending semantic work or proof that a patch set can never grow. Retained-parent coverage compares ordinary Lane children; generated tail does not require another product identity or historical acceptance record.
- Initial terminal-chrome consumes only current target source and covers `.rs` and `.snap`.
- Each generation consumes RepositoryFormatting before comparison, and the two complete formatted postimages must agree before the single transform commit.

```
TerminalChromeNormalizationOwner
+-- Consumer: EveryOwnerApplyChromeConsumer
+-- Successor: KnownGlyphOrBoundedFindingSuccessor
`-- Terminal: StableKnownTerminalChromeTerminal
```

- The Python ExecutableTransform admits rendered snapshot symbols through the installed Cascadia Mono PL Regular character set, not a handwritten glyph allowlist.
- Scan `.rs` and `.snap` under `codex-rs/tui/src` and `codex-rs/mermaid/src`, plus `.snap` under `codex-rs/tui/tests/suite/snapshots`.
- Mermaid producers and their output expectations use the same ordinary glyph mapping as TUI consumers without a separate renderer rewrite or projection table.
- Mermaid family-test labels retain U+1F680 through the existing source-bound Unicode payload classifier, not the global terminal font repertoire.
- Font resolution requires the exact family and Regular style; an unavailable font is an explicit transform failure rather than a fallback to another face.
- Supported U+2039/U+203A and box and line glyphs, including U+2502, U+253C, and U+256A, remain literal.
- Only the four rounded border corners become U+250C/U+2510/U+2514/U+2518.
- Supported block bars and partial block cells remain literal; U+2610 becomes U+25A1.
- Mermaid's U+25C7 white diamond remains literal when present in the selected font.
- The shared table retains approved check, close, warning, information, shape, emoji, and unsupported-font substitutions.
- Every replacement target character is named by a constant at the start of the script; replacement sources use numeric codepoints.
- The unsupported-font substitutions map U+22EE to U+2026, U+21B3/U+21B5/U+21E5 to U+203A, U+2303 to U+2191, U+2304 to U+2193, U+2610 to U+25A1, and U+2301/U+2325 to U+007E.
- The Shift modifier U+21E7 becomes font-supported U+21E1, retaining one display cell and remaining distinct from Control's U+2191.
- Logo and animation remain image payload rather than chrome.
- Preserve legacy TXT assets and complete TUI empty-state and welcome Braille across module paths through source ownership.
- The composer sparkle producer and its tests retain the eight sparse dots when present in the selected font, without a source-only exception.
- Unicode input and grapheme fixtures remain unchanged.
- Rendered-selection U+1F469 and U+1F4BB remain source-owned hyperlink and clipboard payload.
- Hook-browser status-title U+1F980 remains source-owned payload.
- Only Markdown math owners, the two streaming math tests, and snapshots attributed to those sources bypass script normalization and unknown-symbol checking.
- Other Markdown and Mermaid sources and snapshots receive ordinary replacement, rounded-corner conversion, and unknown-symbol checking.
- Product code owns any decision to disable math rendering; the transform does not rewrite math renderer logic or test selectors.
- Analytics and every other admitted TUI source and snapshot retain approved replacement, rounded-corner conversion, and unknown-symbol checking without a cell-payload exception.
- Markdown dash runs, keymap en dashes, and U+2212 remain unchanged.
- Inequality remains unchanged when present in the selected font; wave and macOS option glyphs become U+007E.
- History-search U+21B5 and U+21E5 become U+203A as display markers for raw newline and tab query characters.
- Raw query characters, matching, draft restoration, cursor computation, and their assertions remain unchanged.
- These marker producers need no padding or two-cell arrow; their normalized display markers are visually identical.
- Actual renderer output owns snapshot geometry.
- Normalization checks all prospective source and snapshot changes before writing and preserves Unicode semantics boundaries. Rust comments and snapshot metadata remain byte-exact. Rust literal syntax distinguishes Unicode escapes from raw or escaped text. Equivalent display literals consume the same mapping. Assertions retain their sense, counts, ordering, width and style; no predicate repair or synthetic snapshot padding compensates for a changed renderer.
- Key prefixes, platform declarations, general match structure, and layout behavior remain outside normalization. No separate display-contract Patch or blanket test-literal exclusion is introduced.
- Existing product gates own acceptance; skill maintenance alone authorizes no product edits or commands.
- The transform invokes the existing public `repository-format` on its tracked unstaged output using the admitted product roots, release, installed endpoint, and bounded deadline. Both generations begin at the same clean parent; each completes the canonical formatter and rendered-symbol check before staging or comparison. The combined changed paths remain inside `codex-rs/tui` or `codex-rs/mermaid/src`.
- Only identical formatted generations select the one transform commit. A failed formatter, invalid postimage, unequal generation, or interruption restores the generation's clean parent; nested formatting must stop and clean up before rollback, with its diagnostics and Helper or Product distinction preserved by Apply. No provisional commit or later formatting-only Patch closes this route.
- Generation stability proves only mechanical projection. The complete codex-tui gate owns private renderer evidence; lane snapshot Finalize later regenerates the authoritative postimage from the complete current source.
- Its read-only check inspects admitted rendered `.snap` symbols against the resolved font character set after Apply and before Commit or Revise closes. Metadata selects source-owned exclusions and payload exceptions; unsupported new symbols are never learned silently.
- Unknown symbols return a bounded count and representative path, line, codepoint, and Unicode name through the existing transport Conflict output after rollback.
- For upstream terminal-chrome changes during Roll, the user's standing authorization permits direct SILK custody to maintain this existing mapping and its owning instructions without another approval question.
- The product owner first distinguishes display chrome from source-owned Unicode payload and carries only the demonstrated mapping defect into SILK.
- Select font-supported replacements that preserve the displayed meaning, distinguishable controls and cell width within the existing transform boundary.
- Preserve semantic inputs, payload exclusions, key-prefix construction, platform declarations and renderer-owned geometry rather than adding a fallback, global exemption or compensating product rewrite.
- Complete the ordinary SILK history, scoped correction, validation, installation and cleanup, then report the chosen mapping and resume the retained product successor.
- This authorization covers only upstream adaptation of the existing terminal-chrome owner and grants no unrelated skill, font, product contract or transform-scope change.
- Only the normalization transform writes chrome replacements. No later Apply invokes another scanner or records Topics.

## Product Coverage

- Causal production follows approved Unit and Flat Patch dependency order; earlier work does not absorb later behavior to complete a prefix.
- Publication retains Unit, Flat Patch, and Child Patch boundaries. Across releases, consume complete predecessor Patches and history rather than final trees or labels. Repartition requires prior approval.
- Accepted Units, Flat Patches, and Child Patches contain no fixup, empty behavior, tail repair, compatibility artifact, replacement prefix, or alternate replay.
