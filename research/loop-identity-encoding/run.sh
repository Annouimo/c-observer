#!/usr/bin/env bash
# Regenerate one real debugger audit, ID corpus, stats and benchmark output.
# Usage: ./run.sh lldb loop_identity.c loop-lldb [baseline]
set -euo pipefail
backend=${1:-lldb}
fixture=${2:-loop_identity.c}
label=${3:-loop-$backend}
mode=${4:-}
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
cd "$root"
export DEBUGINFOD_URLS=
mkdir -p "$here/raw"
if [ "$backend" = gdb ]; then
  export C_OBSERVER_REAL_GDB=1
  unset C_OBSERVER_REAL_DEBUGGER
else
  export C_OBSERVER_REAL_DEBUGGER=1
  unset C_OBSERVER_REAL_GDB
fi
if [ "$mode" = baseline ]; then
  export COBS_LOOP_DISABLE=1
fi
COBS_LOOP_FIXTURE="$fixture" COBS_LOOP_AUDIT="$here/raw/$label.json" \
  go test -count=1 -run '^TestLoopIdentityAudit$' ./internal/engine
python3 "$here/extract_ids.py" "$here/raw/$label.json" "$here/corpus/$label.ids.txt.gz" \
  --stats "$here/stats/$label.json" --label "$label" \
  --sanitized-record "$here/records/$label.json.gz"
go run ./research/loop-identity-encoding/bench -repeats 3 \
  -ids "$here/corpus/$label.ids.txt.gz" > "$here/bench-results/$label-3x.txt"
python3 "$here/analyze.py" "$here/raw/$label.json" > "$here/stats/$label-identities.txt"
