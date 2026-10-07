# PatchFormat

## Physical Inventory

- Canonical history is `Lane -> Flat Patch` or `Lane -> Unit -> Child Patch`. A Unit has at least two ordered distinct Child Patches. Each Flat Patch and Child Patch is one commit and GitDiff; no single-Patch Unit exists.
- Canonical ordinary Lane order is bugfix, normalization, backend, mixed, frontend, and draft.
- Their directory names are `0-bugfix`, `1-normalization`, `2-backend`, `3-mixed`, `4-frontend`, and `5-draft` in that order.
- Upstream remains the official baseline role, not a Patch Lane or exported directory.
- The initial normalization ExecutableTransform remains the first history row before all ordinary Lanes, including bugfix.
- Commit metadata is discriminated: GitDiff commits name either `FlatPatch` or both `Unit` and `ChildPatch`; ExecutableTransform commits name `Transform` and `TransformPatch`. Generic Unit-as-Patch metadata is invalid.
- Draft Unit names are single words; Flat Patch names stay hyphenated and equal their Patch field in every Lane, including draft.
- Exported files are diff artifacts ending only in `.diff`. Flat Patch uses `<lane>/<ordinal>-<flat-patch>.diff`; each Child Patch uses `<lane>/<ordinal>-<unit>/<child-ordinal>-<child-patch>.diff`. No artifact ends in `.patch`.
- Each exported GitDiff contains its native canonical commit message followed by its patch body, preserving the declared Lane, Patch identity, Form, and Segment without a sidecar.
- Export readers admit only that current form and require its metadata to match its semantic artifact path.
- A is immutable and B projects it; GitDiffs retain headers, modes, hunks, and bytes, while invalid forms stay unexported.
- Targets retain accepted prefixes; another source supplies only the selected Unit or Flat Patch.
- Commits, not records, order work; List, Apply, Commit, and Export recheck history.
- Exports regenerate GitDiffs plus root and lane notes into an absent staged tag. Lane-child ordinals derive from Git history; tag-prev is outside the projection.
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
- Lanes other than backend, mixed, bugfix, and draft place Units before Flat Patches.
- Backend and mixed use the responsibility order below.
- Bugfix admits Flat Patches only.
- Draft permits any Unit and Flat Patch order.
- Live history, prospective source changes, and exported prefixes share this Lane-child ordering admission.
- PatchReplay returns bounded Blocked/Input for invalid Lane-child order before product source or private export mutation.
- Complete assigned-behavior and Rust semantic acceptance precede the mechanical line check. Its ceiling authorizes neither compensating growth nor feature deletion, and splitting a rejected design does not resolve it.
- Patch authoring measures the staged GitDiff before commit. Commit measures its new Flat Patch or Child Patch, while Revise measures the amended Patch and each later Child Patch it regenerates. Empty or oversized output restores the exact pre-operation input and returns Product Required without publishing any Patch commit.
- Projection measures every canonical Git diff body against the 1500-line limit.
- Exported commit messages remain complete and are excluded from the Git diff line count.
- Export independently remeasures committed GitDiffs and returns the same owner-bound Product Required before private-tree mutation.
- Only --skip-line-limit on an explicitly authorized Commit suppresses that call's oversized finding; empty GitDiff and every other admission still apply. The authorization is not stored or inherited.
- An explicitly authorized export separately supplies --skip-line-limit for its own projection. It creates no persistent exemption and never applies to Revise, Apply, completed-release consumers, or a subsequent call.
- The source reader validates paths, metadata-derived sequences, patch syntax, and bytes. Apply does not reimpose authoring size policy on retained input or add a legacy import mode.
- Retained-directory comparison uses exact canonical bytes; a new export reapplies current publication policy.

## Responsibility Order

```
Backend
+-- 1: non-app-server Unit
+-- 2: non-schema app-server Unit
+-- 3: schema app-server Unit
+-- 4: schema app-server Flat Patch
+-- 5: non-schema app-server Flat Patch
`-- 6: non-app-server Flat Patch
```

```
Mixed
+-- 1: non-app-server Unit
+-- 2: non-schema app-server Unit
`-- 3: schema app-server Unit
```

- Mixed admits Units only, keeping each cross-frontend/backend responsibility in its complete Child Patch sequence.
- A complete Unit or Flat Patch declares its responsibility through Segment rather than inferring app-server ownership from touched files.
- Segment `none` denotes a non-app-server responsibility in backend and mixed.
- Segment `non-schema` denotes an app-server responsibility without an assigned protocol-source change.
- Segment `schema` denotes an app-server responsibility with an assigned protocol-source change.
- Every Child Patch in a Unit keeps that Unit's declared Segment, including children that only implement or present its protocol.
- Categories never reorder Child Patches within a Unit or merge distinct Lane children.

## Metadata And Schema Terminals

```
GeneratedSchemaTerminalOwner
+-- CurrentOrdinarySourceFrontier
+-- CurrentTargetSchemaRegeneration
`-- UnexportedTransformCommitThenAncestorConvergence
```

- Every commit uses exactly one metadata shape above plus `Lane`, `Form`, and `Segment`; malformed or mixed discriminants are Blocked.
- Helper commits use nobody, epoch, exact subject, and fields.
- Generated commits follow all ordinary Patches: schema when required, then snapshot. Their Lane identifies the current source frontier, not a permanent place among that lane's Patches.
- Transform names are reserved; unexported `ExecutableTransform` uses Segment `schema` or `snapshot`.
- Backend and mixed permit `none`, `non-schema`, and `schema` for ordinary GitDiffs.
- Draft permits `none` or `schema`; all other source lanes use `none`.
- A GitDiff touching protocol source must belong to backend, mixed, or draft with Segment `schema`.
- Cumulative schema generation is required only when an ordinary Patch declares Segment `schema`.
- A temporary schema tip is no Unit, Flat Patch, exported artifact, suffix member, or accepted lane terminal. A failed transform, test, or restoration blocks replay.
- Generated tails are not pending semantic work or proof that a patch set can never grow. Retained-parent coverage compares ordinary Lane children; generated tail does not require another product identity or historical acceptance record.
- Initial terminal-chrome covers `.rs` and `.snap`; each generation consumes RepositoryFormatting before comparison, and the two complete formatted postimages must agree before the single transform commit.

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
