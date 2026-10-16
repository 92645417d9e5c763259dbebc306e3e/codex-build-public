# ProtectedCoreAreas

- Owner: protected code and runtime namespaces use owner-local proof or RouteEntry StopLoss in `commands/route-entry.md`.

## Current List

- `codex-rs/core/src/tools/handlers/multi_agents_v2/**`: multi-agent customizations are `6-draft`-only; a V2 change that crosses shared code uses an explicit resolved-version gate and the paired V1 preservation proof owned by `commands/patch-acceptance.md`.
- `$CODEX_HOME/model-catalogs/model-catalogs.json`: outside the current patch chain. Official-model overlay work must not read, seed, or alter it.
- `$CODEX_HOME/models_overlay.json`: OpenAI metadata overlay. It may patch or hide existing official slugs after cache/remote merge; it cannot add models, providers, routes, credentials, or instructions.
