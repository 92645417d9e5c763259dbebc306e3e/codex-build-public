# LoadGraph

```
LoadGraph
+-- Core
|   +-- SKILL.md
|   +-- contracts/
|   |   +-- json/records.json
|   |   +-- records.md
|   |   `-- load-graph.md
|   +-- commands/
|   |   +-- route-entry.md
|   |   +-- locks.md
|   |   +-- git-custody.md
|   |   `-- runner-env.md
|   `-- reference/task-workspace.md
+-- CargoSupport
|   +-- Core
|   +-- commands/
|   |   `-- native-toolchain.md
+-- NativeToolchainSupport
|   `-- RunnerSupport
+-- RunnerSupport
|   +-- CargoSupport
|   +-- go/go.mod
|   +-- go/main.go
|   +-- go/internal/core/
|   |   +-- constants.go
|   |   +-- records.go
|   |   `-- versions.go
|   +-- go/internal/assets/
|   |   `-- embed.go
|   +-- go/internal/assets/files/
|   |   +-- build-install.zsh
|   |   +-- build-select.zsh
|   |   +-- sync.zsh
|   |   +-- native-toolchain.py
|   |   +-- runner.env
|   |   +-- runner-bwrap.args
|   |   +-- runner-binaries.py
|   |   +-- release-roll.py
|   |   `-- runner-stage.zsh
|   +-- go/internal/runtime/
|   |   +-- audit.go
|   |   +-- build.go
|   |   +-- cli.go
|   |   +-- custody.go
|   |   +-- materialization.go
|   |   +-- records.go
|   |   +-- release.go
|   |   +-- worktrees.go
|   |   +-- format.go
|   |   +-- maintenance.go
|   |   +-- runner.go
|   |   +-- sync.go
|   |   +-- toolchain.go
|   |   `-- publish.go
|   `-- bin
+-- RepositoryFormattingSupport
|   +-- RunnerSupport
|   +-- commands/
|   |   `-- repository-format.md
+-- RepositoryMaintenanceSupport
|   +-- RunnerSupport
|   +-- commands/
|   |   `-- repository-maintenance.md
+-- RustSuiteSupport
|   +-- RunnerSupport
|   `-- commands/full-suite-audit.md
+-- ReleaseBaselineSupport
|   +-- RustSuiteSupport
|   +-- commands/cleanup.md
|   `-- commands/release-roll-forward.md
+-- TaskSupport
|   +-- Core
|   +-- commands/task-patch.md
|   `-- reference/patch-format.md
+-- PatchSupport
|   +-- RustSuiteSupport
|   +-- commands/
|   |   +-- patch-replay.md
|   |   +-- patch-acceptance.md
|   |   +-- patch-export.md
|   |   +-- repository-format.md
|   |   `-- release-roll-forward.md
|   +-- reference/
|   |   +-- patch-format.md
|   |   `-- protected-core-areas.md
|   +-- go/internal/assets/files/
|   |   +-- normalize-tui-terminal-chrome.py
|   |   +-- regenerate-app-server-schema.py
|   |   `-- config-schema.py
+-- BuildSupport
|   +-- RunnerSupport
|   `-- commands/build-install.md
`-- SyncSupport
    +-- RunnerSupport
    `-- commands/sync.md
```

- Named public helpers admit their support natively.
- Other routes expand Core, their command and literal calls to a fixed point without reference edges.
- Every ReleaseRollForward owner check, including one reached after resume or context restoration, expands VerifyRequestedProduct through RunnerSupport before process construction.
- Native admission owns the ordered readable implementation closure and fixed endpoint.
- The agent loads its route instructions.
- Every row is bootstrap-reachable and owned; missing, dynamic, orphaned, duplicate, unreadable, or outside entries block.
- A transition closes RouteState and starts an uninherited RouteRequest.
- Agent implementation inspection requires an execution need or explicit investigation.
- RustSuiteSupport admits no test asset, maintenance test route, child runner, or external maintenance record.

## Shell Script Rewrite

- Every retained embedded shell asset has a `.zsh` name and exact zsh shebang, then runs `emulate -LR zsh` and enables `NO_ALIASES`, `ERR_EXIT`, `NO_UNSET`, and `PIPE_FAIL` before its body performs an effect.
- `build-install.zsh`, `build-select.zsh`, `runner-stage.zsh` and `sync.zsh` are the only embedded shell asset names. `.sh` aliases and inherited shell state are not admitted.
- Compilation and static inspection consume RunnerSupport before effects.
- Cargo materialization and release-load checks remain route-owned.
