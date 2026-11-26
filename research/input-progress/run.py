#!/usr/bin/env python3
"""Run the real engine with temporary test-only hooks. No checkout files are modified."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]


def hook(name, anchor, code, scratch, overlay):
    source = ROOT / "internal/engine" / name
    text = source.read_text(encoding="utf-8")
    if text.count(anchor) != 1:
        raise SystemExit(f"Cannot locate experiment hook in {name}; recheck engine base")
    target = scratch / name
    target.write_text(text.replace(anchor, anchor + "\n" + code), encoding="utf-8")
    overlay[str(source)] = str(target)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("backend", choices=["gdb", "lldb"])
    parser.add_argument("output", type=Path)
    parser.add_argument("--repeat", type=int, default=3)
    parser.add_argument("--case", default="", help="run one named case")
    parser.add_argument("--course-source", type=Path)
    parser.add_argument("--course-input", type=Path)
    args = parser.parse_args()
    if bool(args.course_source) != bool(args.course_input):
        parser.error("--course-source and --course-input must be provided together")
    with tempfile.TemporaryDirectory() as tmp:
        scratch = Path(tmp)
        overlay = {}
        hook("engine.go", "\t\t\tc.result.Snapshots = append(c.result.Snapshots, snapshot)",
             "\t\t\tif err := researchDAP(c, frames, snapshot, input, crash); err != nil { return err }",
             scratch, overlay)
        hook("gdb.go", "\t\t\tc.result.Snapshots = append(c.result.Snapshots, snapshot)",
             "\t\t\tif err := researchMI(m, request, snapshot, input, crashed); err != nil { return err }",
             scratch, overlay)
        cfg = scratch / "overlay.json"
        cfg.write_text(json.dumps({"Replace": overlay}), encoding="utf-8")
        env = dict(os.environ, COBS_INPUT_OUTPUT=str(args.output.resolve()),
                   COBS_INPUT_BACKEND=args.backend, COBS_INPUT_REPEAT=str(args.repeat),
                   COBS_INPUT_CASE=args.case)
        if args.course_source:
            env.update(COBS_INPUT_COURSE_SOURCE=str(args.course_source.resolve()),
                       COBS_INPUT_COURSE_STDIN=str(args.course_input.resolve()))
        subprocess.run(["go", "test", "-count=1", "-run", "^TestInputProgressResearch$",
                        "-overlay", str(cfg), "-v", "./internal/engine"],
                       cwd=ROOT, env=env, check=True)


if __name__ == "__main__":
    main()
