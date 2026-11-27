# Loop identity and identifier encoding research

This directory accompanies `docs/loop-identity-encoding-research.md`. It contains
the reproducible audit, the real ID corpora and the encoding benchmark.

## Layout

- `run.sh BACKEND FIXTURE LABEL [baseline]` — run one real audit, extract IDs,
  write a sanitized record and run the benchmark.
- `extract_ids.py` — build an ID corpus and size statistics from a RunResult.
- `analyze.py` — print reliable loop-variable IDs and values from an audit.
- `bench/` — standard-library benchmark for string IDs, interning, 64-bit hashes,
  structured keys and ID construction.
- `corpus/*.ids.txt.gz` — one real variable ID per line, in observation order.
- `records/*.json.gz` — sanitized small audit records (loop/shadow/goto).
- `stats/*.json` — compact JSON, gzip, ID share and encoding variants.
- `bench-results/*.txt` — raw benchmark output; `*-3x.txt` uses three repeats.

`raw/` is ignored: a 1000-snapshot stress record is about 21 MiB as indented
JSON and is regenerated, never committed.

## Reproduce

```sh
cd <repo>
./research/loop-identity-encoding/run.sh lldb loop_identity.c loop-lldb
./research/loop-identity-encoding/run.sh gdb  loop_identity.c loop-gdb
./research/loop-identity-encoding/run.sh lldb loop_identity_shadow.c shadow-lldb
./research/loop-identity-encoding/run.sh lldb loop_identity_goto.c goto-lldb
./research/loop-identity-encoding/run.sh lldb loop_identity_stress.c stress-lldb
./research/loop-identity-encoding/run.sh gdb  loop_identity_stress.c stress-gdb
```

The audit test is opt-in (`COBS_LOOP_AUDIT`) and uses the actual Linux
Clang/LLDB-DAP and GCC/GDB. `COBS_LOOP_DISABLE=1` reproduces the pre-change
LLDB behavior. The script sets `DEBUGINFOD_URLS=` because this machine's
debuginfod network path can stall LLDB launch; that is an environment issue, not
part of the engine change.

## Environment

Ubuntu 26.04 x86_64, Go 1.26.8, Clang 21.1.8, LLDB-DAP 21.1.8, GCC 15.2.0,
GDB 17.1 (Python 3.14). The project CI baseline is Go 1.23/LLVM 19; the loop
index uses Clang AST declarations and should be re-run there before release.
