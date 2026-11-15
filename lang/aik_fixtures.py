# SPDX-License-Identifier: MIT
# Written by aik fixtures make into its work tree (spec 08.7.2, rule X15). It compares the .npz files of three
# fixtures/ directories by their arrays, not their bytes, since np.savez puts the time in the zip: the committed files
# with the first run of make.py, and the first run with the second. It prints one JSON line.
import json
import pathlib
import sys

import numpy as np


def arrays(path):
    with np.load(path, allow_pickle=False) as z:
        return {k: z[k] for k in z.files}


def diff(old, new):
    """The arrays of new that are not those of old: added, removed, or changed in dtype, shape or value."""
    out = []
    for k in sorted(set(old) | set(new)):
        if k not in old:
            out.append({"name": k, "change": "added", "shape": list(new[k].shape)})
        elif k not in new:
            out.append({"name": k, "change": "removed"})
        elif old[k].dtype != new[k].dtype:
            out.append({"name": k, "change": f"dtype {old[k].dtype} -> {new[k].dtype}"})
        elif old[k].shape != new[k].shape:
            out.append({"name": k, "change": f"shape {list(old[k].shape)} -> {list(new[k].shape)}"})
        elif not np.array_equal(old[k], new[k], equal_nan=old[k].dtype.kind in "fc"):
            a, b = old[k], new[k]
            ne = int(np.sum(a != b))
            d = {"name": k, "change": "values", "changed": ne, "size": int(a.size)}
            if a.dtype.kind in "fc":
                d["max_abs"] = float(np.max(np.abs(a.astype(np.complex128 if a.dtype.kind == "c" else np.float64) - b)))
            out.append(d)
    return out


def compare(a, b):
    res = {}
    for p in sorted(pathlib.Path(b).rglob("*.npz")):
        rel = p.relative_to(b).as_posix()
        q = pathlib.Path(a) / rel
        if not q.exists():
            res[rel] = [{"name": "*", "change": "new file"}]
            continue
        d = diff(arrays(q), arrays(p))
        if d:
            res[rel] = d
    return res


def main():
    committed, first, second = sys.argv[1:4]
    print(json.dumps({"numpy": np.__version__, "changed": compare(committed, first), "unstable": compare(first, second)}))


if __name__ == "__main__":
    main()
