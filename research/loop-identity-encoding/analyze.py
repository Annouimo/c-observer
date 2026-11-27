#!/usr/bin/env python3
"""Summarize reliable loop-variable identities in an audit RunResult."""
import argparse, json, collections


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("record")
    args = ap.parse_args()
    run = json.load(open(args.record)).get("run", {})
    reliable = collections.defaultdict(lambda: collections.defaultdict(set))
    counts = collections.defaultdict(lambda: [0, 0])
    for s in run.get("snapshots") or []:
        for fr in s.get("frames") or []:
            if (fr.get("location") or {}).get("function") != "main":
                continue
            for v in fr.get("variables") or []:
                if v.get("identity_reliable"):
                    reliable[v["name"]][v.get("id", "")].add(v.get("value"))
                    counts[v["name"]][0] += 1
                else:
                    counts[v["name"]][1] += 1
    print(f"snapshots={len(run.get('snapshots') or [])} reason={run.get('reason')}")
    for name in sorted(reliable):
        print(f"{name}: reliable={counts[name][0]} unreliable={counts[name][1]}")
        for ident, values in sorted(reliable[name].items()):
            shown = sorted(values, key=lambda x: (x is None, str(x)))
            print(f"  {ident} -> {shown}")


if __name__ == "__main__":
    main()
