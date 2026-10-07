#!/usr/bin/env zsh
emulate -LR zsh
setopt NO_ALIASES ERR_EXIT NO_UNSET PIPE_FAIL

stats=/result/sccache-stats.json
stats_pending=/result/.sccache-stats.json.stage
sccache=/toolchain/bin/sccache

"$sccache" --start-server >&2
trap '"$sccache" --stop-server >&2' EXIT
"$sccache" --zero-stats >&2

if [[ ${1-} == /toolchain/bin/just && ${2-} == test ]]; then
  set -- /usr/bin/python3 -s /runner/runner-binaries.py "$@"
fi
if "$@"; then
  command_status=0
else
  command_status=$?
fi
if [[ -f /runner/complete-snapshots ]]; then
  /usr/bin/python3 -s /runner/release-roll.py runner-complete-snapshot-manifest /workspace-lower /workspace /result/snapshot-updates.json || true
fi
"$sccache" --show-stats --stats-format json >"$stats_pending"
command mv -- "$stats_pending" "$stats"
exit "$command_status"
