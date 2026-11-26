#!/usr/bin/env python3
"""Extract variable IDs and size statistics from a c-observer RunResult.

The input is either {"run": RunResult, ...} or a bare RunResult. Outputs a
gzip text corpus (one ID per line) and a JSON summary. The script never stores
values, source text or environment paths in the summary.
"""
import argparse, gzip, hashlib, json, os, re, sys


def walk(obj, var_ids, all_ids, in_var=False):
    if isinstance(obj, dict):
        if isinstance(obj.get("id"), str):
            all_ids.add(obj["id"])
            if in_var:
                var_ids.append(obj["id"])
        for k, v in obj.items():
            if k in ("variables", "globals", "children"):
                walk(v, var_ids, all_ids, True)
            elif k == "frames":
                walk(v, var_ids, all_ids, False)
            else:
                walk(v, var_ids, all_ids, in_var)
    elif isinstance(obj, list):
        for v in obj:
            walk(v, var_ids, all_ids, in_var)


def rewrite(obj, order, mode):
    if isinstance(obj, dict):
        for k, v in list(obj.items()):
            if k == "id" and isinstance(v, str):
                if mode == "int":
                    obj[k] = order[v]
                elif mode == "strint":
                    obj[k] = str(order[v])
                elif mode == "hash":
                    obj[k] = hashlib.blake2b(v.encode(), digest_size=8).hexdigest()
                elif mode == "none":
                    obj.pop(k)
            else:
                rewrite(v, order, mode)
    elif isinstance(obj, list):
        for v in obj:
            rewrite(v, order, mode)


def gz(data):
    return len(gzip.compress(data, 9))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("record")
    ap.add_argument("ids_out")
    ap.add_argument("--stats", required=True)
    ap.add_argument("--label", default="")
    ap.add_argument("--sanitize-root", default="")
    ap.add_argument("--sanitized-record", default="")
    args = ap.parse_args()

    raw = open(args.record, "rb").read()
    doc = json.loads(raw)
    run = doc.get("run", doc)

    root = args.sanitize_root or doc.get("workdir", "")
    if root:
        root = os.path.abspath(root)

        def clean(x):
            if isinstance(x, str):
                return re.sub(r"/tmp/c-observer-[0-9]+", "<temp>", x.replace(root, "<workdir>"))
            if isinstance(x, list):
                return [clean(v) for v in x]
            if isinstance(x, dict):
                return {k: clean(v) for k, v in x.items() if k != "workdir"}
            return x

        doc = clean(doc)
        if args.sanitized_record:
            data = json.dumps(doc, separators=(",", ":"), ensure_ascii=False).encode()
            with gzip.open(args.sanitized_record, "wb", compresslevel=9) as f:
                f.write(data)

    var_ids, all_ids = [], set()
    walk(run, var_ids, all_ids)
    unique = set(var_ids)
    with gzip.open(args.ids_out, "wt", encoding="utf-8") as f:
        for i in var_ids:
            f.write(i + "\n")

    order = {i: n + 1 for n, i in enumerate(sorted(all_ids))}
    base = json.dumps(doc, separators=(",", ":"), ensure_ascii=False).encode()
    summary = {
        "label": args.label or os.path.basename(args.record),
        "snapshots": len(run.get("snapshots") or []),
        "reason": run.get("reason"),
        "id_occurrences": len(var_ids),
        "unique_variable_ids": len(unique),
        "unique_all_ids": len(all_ids),
        "avg_id_len": round(sum(map(len, var_ids)) / max(1, len(var_ids)), 2),
        "max_id_len": max(map(len, var_ids), default=0),
        "json_compact_bytes": len(base),
        "json_gzip_bytes": gz(base),
        "id_value_bytes": sum(len(json.dumps(i, ensure_ascii=False)) for i in var_ids),
        "id_key_bytes": 5 * len(var_ids),
        "dictionary_bytes": sum(len(json.dumps(i, ensure_ascii=False)) for i in all_ids),
    }
    for mode in ("int", "strint", "hash", "none"):
        d = json.loads(base)
        rewrite(d, order, mode)
        if mode == "int":
            d["id_dictionary"] = {str(n): i for i, n in order.items()}
        data = json.dumps(d, separators=(",", ":"), ensure_ascii=False).encode()
        summary[mode + "_bytes"] = len(data)
        summary[mode + "_gzip_bytes"] = gz(data)
    with open(args.stats, "w", encoding="utf-8") as f:
        json.dump(summary, f, ensure_ascii=False, indent=2, sort_keys=True)
        f.write("\n")
    print(json.dumps(summary, ensure_ascii=False))


if __name__ == "__main__":
    main()
