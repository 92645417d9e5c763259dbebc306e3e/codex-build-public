# RouteEntry

## Normal Form

```
CodexBuildNormalForm
+-- CodexBuildNormalFormOwner: selects one outcome command and effect set
+-- InstalledRouteConsumer: executes without creating authority
+-- SingleProductSuccessor: one continuation or typed terminal
`-- DrainedProductTerminal: closes only the selected effect
```

## Resume

- A Roll Apply conflict restores its target; ReleaseRollForward owns the retained prefix, scope decision and reconstruction successor.
- Every Roll Unit check selects VerifyRequestedProduct and reloads its fixed point; prior loading and process state are not inherited.

## Release Terms

```
TopLevelReleaseTermOwner
+-- Consumer: UserReleaseIntentConsumer
+-- Successor: IndependentRouteSelectionSuccessor
`-- Terminal: SelectedRouteStopTerminal
```

- Baseline maps to `release-baseline` and BaselineEstablishedAndCleanupComplete.
- Roll selects ReleaseRollForward for the sole task. Automatic Roll follows every declared Patch; an explicit interactive Unit, Flat Patch, or Lane stop consumes its requested read-only exploration before handoff. A newer user-selected tag uses next-tag Baseline after exact prefix export.
- Before BuildInstall, resolve the requested package-only or installation terminal from user intent; ask only when it is ambiguous.
- Install selects `build-install --install`, without a home-link, candidate or export predecessor.
- Package-only omits `--install` only when publication without activation fulfills the request.
- BuildInstall's handoff must fulfill that selected terminal.
- An explicit Sync request selects `sync` with the user-selected version and patch tag through [Public Sync](/codex/skills/codex-build/commands/sync.md).
- Sync publishes snapshots only and never selects Roll, export, build, installation or a test suite.
- Cleanup maps to exact Attached `main` source restoration followed by release-cleanup on the declared roots. Without a tag, that helper consumes only drained temporary records and output; with an explicitly selected tag, it additionally verifies registered Main and its exact export before retiring the closed task and published lane notes.
- Cleanup preserves tasks/rust-suite-baselines.json, exported patches, worktrees, build/docs, installed packages and reusable build inputs; all drained execution records and logs are disposable. Neither installation nor cleanup selects a suite.
- Within a selected next-tag Baseline, predecessor worktree cleanup stays in release-baseline: validate the exact predecessor prefix or Established zero-patch parent, retire only its registered roles, then reuse the global paths. Its internal baseline cleanup validates the new Established result. Neither operation invokes public release-cleanup or Attached restoration.
- Missing inputs return their route's typed stop.

## Routes

- ReviewArtifacts selects direct read-only artifact review by the product agent.
- InspectClosedPrefix selects PatchExport's `export` at registered MainWorktree's closed Git frontier, independently of a fork remaining Current.
- Roll invokes no subagent, delegation, parallel review, or reviewer wait. Source publication consumes the same agent's complete current-native design and cumulative Patch acceptance before process construction.
- PreparePatchForApproval consumes ReleaseRollForward's material contract and requirement-cost decisions or an explicitly requested interactive choice.
- Automatic Roll enters ReleaseRollForward directly; task text grants no scope change.
- ConvergeAcceptedWorktrees precedes next-Patch or Fork selection after Unit or Flat Patch closure and on reentry. GitCustody removes covered numbered parents independently of Main's suffix; full Main coverage then promotes. Return to the selected successor without a helper topology command or test gate.
- DraftIssue selects claim-reviewed UpstreamBugReport.
- EditNamedTask selects TaskWorkspace's reconciliation, publication and requested recap.
- After migrating the declared Patch set, consume TaskWorkspace closure before export and handoff; retain the task for later draft additions. RewritePatchArtifacts is live transport.
- FormatRepositoryRust runs one `repository-format` on the real worktree before the owning commit and Rust verification, consuming clean or tracked unstaged input under RepositoryFormatting admission.
- UpdateRepositoryBazelLock acquires source-pinned Bazel artifacts as Helper support, then runs one offline `repository-maintenance` recipe before acceptance.
- UpdateWorkspaceCargoLock selects `repository-maintenance --cargo-lock` on the uncommitted workspace manifest postimage, then returns to that dependency Unit's separate Bazel maintenance and source commit.
- RepositorySelectedVerification sends every Roll Rust test from repository or user instructions through VerifyRequestedProduct or RustSuite; selection supplies no host executor.
- An open Patch may select transient schema-only Finalize when its chosen test needs generated schema, then Prepare; package identity alone selects no generation.
- VerifyRebuiltReleasePrefix and VerifyFinalRustProduct select the same user-requested full Finalize on closed actual Current, with no Open Patch, separate acceptance command, source tag, or convergence prerequisite.
- Its committed-Patch Product failure enters temporary Main fixup only after declared Patch completion and Main convergence; other findings return to the owning Patch. No automatic second complete run follows.
- User-authorized internal bypass records the missing public route with SILK before Roll resumes; its topic grants no authority.
- RepairFailedReleaseVerification consumes Rust's read-only diagnostic evidence through the release's original-purpose selection. A refuted candidate returns to complete owning-Patch reconstruction, not another local test or fixture fix. ReleaseRollForward owns its scope decision and continuation.
- PrepareNativeToolchain publishes fixed Rust, Cargo, Bazel, build, repository, V8, and voice tools before consumers. If a projected consumer's exact fixed-root admission fails before its product process starts, route through that same producer and shared admission, then resume the unchanged complete invocation once; failed preparation stops without host fallback or a product attempt.
- EnterRollWorkspace requires empty root AGENTS.override.md at Attached before Roll; Baseline and Fork create it in each canonical worktree, and Cleanup preserves it.
- A user request to read a Markdown basename without a directory, such as `1.md`, resolves to `build/docs/1.md` under cwd; an explicit path remains explicit. Read only the named document rather than enumerating or opening its siblings.
- Long commands use RunnerEnv's self-contained startup and progress notices. A nonterminal yield continues the same handle at its longest permitted wait up to twenty-five minutes, without commentary or polling. After an interrupted full Finalize, reenter with --resume on unchanged source and selection; omitting it selects a fresh full run and is not recovery. Baseline reenters release-baseline. Only a separately selected fresh check replaces current diagnostics.

## Public Invocation Construction

```
PublicInvocationConstruction
+-- Owner: owns pre-process argv
+-- Consumer: consumes RouteEntry and LoadGraph
+-- ReleaseRoots: skill project scratch runtime record
+-- runner-exec: parser-owned help and RunnerEnv's exact tests or frozen argv selection
+-- repository-maintenance: skill project scratch runtime roots source release attempt deadline and optional cargo-lock for the fixed workspace Cargo operation
+-- release-baseline: ReleaseRoots and tag
+-- release-cleanup: ReleaseRoots and optional explicitly retired tag
+-- Cleanup Attached restoration: exact supplied Attached repository root
+-- baseline-coverage: ReleaseRoots Unit or Flat Patch for automatic addition, or user-selected align at any Roll frontier; complete covered compile-only and helper selections
+-- build-install: parser-owned help and direct BuildInstall entry
+-- sync: parser-owned help and explicitly selected public snapshot publication
+-- native-toolchain: project scratch runtime roots
+-- patch-replay: skill scratch runtime roots mode tag active task and record root; PatchReplay owns exact exported Apply argv
+-- Fork: optional parent NNN and a free numbered Git worktree name
+-- Internal: runner-child-exec belongs to ProjectedRunner
+-- Missing: required semantic inputs stop; roots use SKILL.md defaults
+-- Gate: route-specific closed source frontier and exact selected evidence
+-- Successor: CompleteInstalledInvocation contains endpoint command roots inputs and payload
`-- Terminal: NoArgumentFreeProbe reaches admission
```

- Construct `runner-exec` from the installed endpoint and its parser-owned `--help`. RouteEntry owns intent and roots; RunnerEnv owns same-target exact-name grouping before freezing the selection or separator argv.
- RunnerEnv owns guest temporary paths and fixed capacity admission; callers select no host temporary root or storage policy. Shell-only tools build endpoint arguments internally.
- Preserve each frozen test selection without splitting its invocation, moving tests between Units, or invoking an internal producer. Launch failure reports the executable, error, and argument count without echoing the expanded command.
- Generic repository tests receive RunnerEnv's bounded sixteen-test default. Existing native --test-threads, --jobs, or -j arguments may select one positive value no greater than 16; they do not alter the frozen test selection or authorize a broader run.
- `runner-exec --help`, `repository-format --help`, and `patch-replay --help` are successful read-only parser views before admission. This route selects purpose; help owns syntax without a second option inventory.
- Construct `repository-maintenance` only from the installed endpoint and its documented roots, source, release, attempt, and deadline; it fixes the repository-selected `just bazel-lock-update` process.
- Fork derives its ordinal from existing current-release numbered branch refs. It accepts no record root, caller-supplied ordinal, or maximum.
- List uses the effective root defaults or overrides; actual source identity selects live Current, a retained parent, or a canonical prefix declared in SourcePrefixes, never root-flag presence.
- RunnerExec owns its fixed current slot; callers choose no AttemptID. After reap, a new selected check replaces that slot regardless of the old outcome. Full interruption instead follows Finalize `--resume` on unchanged input. Findings return to the owning Patch without a mandatory broader test or topology change.

## StopLoss

- Product failure retains its named successor without repair endpoint. A pre-process fixed-root admission failure uses only NativeToolchain's declared preparation predecessor; failure of that predecessor retains its Helper stop and never enters product recovery.
- StopLoss preserves state, closes handles and locks, and terminates Stopped or Blocked without a successor.

## Exit

- Each command owns one typed terminal; its route owner defines the Pass or Required postimage.
