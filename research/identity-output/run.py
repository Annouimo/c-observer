#!/usr/bin/env python3
"""Audit real debugger runs using temporary, test-only engine overlays."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("backend", choices=["gdb", "lldb"])
    parser.add_argument("output", type=Path)
    parser.add_argument("--strip-declarations", action="store_true",
                        help="simulate absent LLDB extension on real Linux DAP traffic")
    parser.add_argument("--preset-header", type=Path, help="optional expanded preset for the birthday fixture")
    args = parser.parse_args()
    with tempfile.TemporaryDirectory() as tmp:
        scratch = Path(tmp)
        overlay = {}
        for name, anchor, injection in [
            ("dap.go", "return json.Unmarshal(message.Body, body)",
             "return json.Unmarshal(auditIdentityDAP(message.Body), body)"),
        ]:
            source = ROOT / "internal/engine" / name
            text = source.read_text(encoding="utf-8")
            if text.count(anchor) != 1:
                raise SystemExit(f"engine audit anchor changed: {name}")
            target = scratch / name
            target.write_text(text.replace(anchor, injection), encoding="utf-8")
            overlay[str(source)] = str(target)
        config = scratch / "overlay.json"
        config.write_text(json.dumps({"Replace": overlay}), encoding="utf-8")
        env = dict(os.environ, COBS_IDENTITY_AUDIT=str(args.output.resolve()),
                   C_OBSERVER_REAL_GDB="1" if args.backend == "gdb" else "0",
                   C_OBSERVER_REAL_DEBUGGER="1" if args.backend == "lldb" else "0",
                   COBS_STRIP_DECLARATIONS="1" if args.strip_declarations else "0")
        if args.preset_header:
            env["COBS_IDENTITY_PRESET"] = str(args.preset_header.resolve())
        subprocess.run(["go", "test", "-count=1", "-run", "^TestIdentityOutputAudit$",
                        "-overlay", str(config), "-v", "./internal/engine"],
                       cwd=ROOT, env=env, check=True)


if __name__ == "__main__":
    main()
