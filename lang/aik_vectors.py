# SPDX-License-Identifier: MIT
# Written by aik test into the build tree of a metric (spec 08.7.5, rule X05). It runs each test vector through the
# score function of the implementation and prints one JSON line with the failures.
import importlib
import json
import math
import sys
import traceback

sys.path.insert(0, ".")


def close(got, want, rtol, atol, where):
    if isinstance(want, bool) or want is None or isinstance(want, str):
        return None if got == want else f"{where}: got {got!r}, want {want!r}"
    if isinstance(want, (int, float)):
        try:
            g = float(got)
        except (TypeError, ValueError):
            return f"{where}: got {got!r}, want the number {want!r}"
        if math.isnan(want):
            return None if math.isnan(g) else f"{where}: got {g!r}, want nan"
        if abs(g - want) <= atol + rtol * abs(want):
            return None
        return f"{where}: got {g!r}, want {want!r} (rtol {rtol}, atol {atol})"
    if isinstance(want, list):
        try:
            got = list(got)
        except TypeError:
            return f"{where}: got {got!r}, want a list of {len(want)}"
        if len(got) != len(want):
            return f"{where}: got {len(got)} items, want {len(want)}"
        for i, (g, w) in enumerate(zip(got, want)):
            why = close(g, w, rtol, atol, f"{where}[{i}]")
            if why:
                return why
        return None
    if isinstance(want, dict):
        if not isinstance(got, dict):
            return f"{where}: got {type(got).__name__}, want a dict with {', '.join(sorted(want))}"
        for k in sorted(want):
            if k not in got:
                return f"{where}: the output has no {k}"
            why = close(got[k], want[k], rtol, atol, f"{where}.{k}" if where else k)
            if why:
                return why
        return None
    return f"{where}: cannot compare with {want!r}"


def main():
    module, entry, rtol, atol, *files = sys.argv[1:]
    rtol, atol = float(rtol), float(atol)
    try:
        score = getattr(importlib.import_module(module), entry)
    except Exception:
        print(json.dumps({"error": traceback.format_exc(limit=3)}))
        return
    cases, failures = 0, []
    for path in files:
        with open(path, encoding="utf-8") as f:
            for line in f:
                if not line.strip():
                    continue
                v = json.loads(line)
                cases += 1
                case = path.rsplit("/", 1)[-1] + ":" + v["id"]
                try:
                    got = score(v["predictions"], v["references"], **v.get("options", {}))
                except Exception as e:
                    failures.append({"case": case, "reason": f"{type(e).__name__}: {e}"})
                    continue
                why = close(got, v["want"], rtol, atol, "")
                if why:
                    failures.append({"case": case, "reason": why.lstrip(": "), "want": json.dumps(v["want"])})
    print(json.dumps({"cases": cases, "failures": failures}))


main()
