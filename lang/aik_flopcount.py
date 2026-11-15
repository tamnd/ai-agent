# SPDX-License-Identifier: MIT
# Written by aik flopcount into the build tree of the NumPy reference of an architecture (spec 08.6, check 4). It runs
# the forward pass of each config that the test module's flop_case gives, on arrays of an instrumented type that counts
# the multiply-adds of the products (matmul and @, dot, vdot, inner, tensordot, einsum and multi_dot) as 2 FLOPs each.
# Softmax, normalization, activations and the other elementwise work are not counted, as in schema/flops.md. It prints
# one JSON line: the count of each config, and the calls to functions whose work it cannot count.
import importlib
import json
import math
import sys
import traceback

sys.path.insert(0, ".")

import numpy as np  # noqa: E402

try:
    from numpy._core.einsumfunc import _parse_einsum_input
except ImportError:  # NumPy 1
    from numpy.core.einsumfunc import _parse_einsum_input

COUNT = [0]
UNCOUNTED = {}

# The functions with multiply-adds that the count does not follow: their FLOPs are not a product of the shapes.
WATCHED = {"convolve", "correlate", "solve", "inv", "lstsq", "matrix_power", "cholesky", "qr", "svd", "eig", "eigh",
           "det", "pinv", "fft", "ifft", "rfft", "irfft", "fft2", "ifft2", "fftn", "ifftn"}


def unwrap(x):
    if isinstance(x, Counted):
        return x.view(np.ndarray)
    if isinstance(x, list):
        return [unwrap(v) for v in x]
    if isinstance(x, tuple):
        return type(x)(*map(unwrap, x)) if hasattr(x, "_fields") else tuple(unwrap(v) for v in x)
    if isinstance(x, dict):
        return {k: unwrap(v) for k, v in x.items()}
    return x


def wrap(x):
    if isinstance(x, np.ndarray) and not isinstance(x, Counted):
        return x.view(Counted)
    if isinstance(x, list):
        return [wrap(v) for v in x]
    if isinstance(x, tuple):
        return type(x)(*map(wrap, x)) if hasattr(x, "_fields") else tuple(wrap(v) for v in x)
    if isinstance(x, dict):
        return {k: wrap(v) for k, v in x.items()}
    return x


def product(res, a, b):
    """A product that sums over the last axis of a: two FLOPs for each output and each term of the sum."""
    if np.ndim(a) == 0 or np.ndim(b) == 0:
        return 0  # a product by a scalar has no sum
    return 2 * int(np.size(res)) * int(np.shape(a)[-1])


def dot(res, a, b, out=None):
    return product(res, a, b)


def inner(res, a, b):
    return product(res, a, b)


def vdot(res, a, b):
    return 2 * int(np.size(a))


def tensordot(res, a, b, axes=2):
    sa = np.shape(a)
    if isinstance(axes, int):
        summed = sa[len(sa) - axes:] if axes > 0 else ()
    else:
        ax = axes[0]
        ax = [ax] if isinstance(ax, int) else list(ax)
        summed = [sa[i] for i in ax]
    if len(summed) == 0:
        return 0
    return 2 * int(np.size(res)) * math.prod(int(d) for d in summed)


def einsum(res, *operands, **kw):
    """The pairwise contractions of the greedy order, each that sums an index as 2 FLOPs for each term."""
    ins, out, ops = _parse_einsum_input(operands)
    terms = ins.split(",")
    size = {}
    for t, o in zip(terms, ops):
        for ch, d in zip(t, np.shape(o)):
            size[ch] = max(size.get(ch, 1), int(d))
    if len(terms) == 1:
        return 0
    path = np.einsum_path(*operands, optimize="greedy")[0][1:]
    cur, total = list(terms), 0
    for step in path:
        picked = [cur[i] for i in step]
        for i in sorted(step, reverse=True):
            cur.pop(i)
        keep = set(out).union(*map(set, cur))
        letters = "".join(dict.fromkeys("".join(picked)))
        new = "".join(ch for ch in letters if ch in keep)
        if len(picked) >= 2 and len(new) < len(letters):
            total += 2 * math.prod(size[ch] for ch in letters)
        cur.append(new)
    return total


def multi_dot(res, arrays, out=None):
    """The cheapest order of the chain, the one that NumPy takes."""
    dims = []
    for i, a in enumerate(arrays):
        s = np.shape(a)
        if len(s) == 1:
            s = (1, s[0]) if i == 0 else (s[0], 1)
        if i == 0:
            dims.append(int(s[0]))
        dims.append(int(s[1]))
    n = len(dims) - 1
    cost = [[0] * n for _ in range(n)]
    for span in range(1, n):
        for i in range(n - span):
            j = i + span
            cost[i][j] = min(cost[i][k] + cost[k + 1][j] + dims[i] * dims[k + 1] * dims[j + 1] for k in range(i, j))
    return 2 * cost[0][n - 1]


RULES = {np.dot: dot, np.inner: inner, np.vdot: vdot, np.tensordot: tensordot, np.einsum: einsum,
         np.linalg.multi_dot: multi_dot}


class Counted(np.ndarray):
    def __array_ufunc__(self, ufunc, method, *inputs, **kw):
        ins = unwrap(inputs)
        if "out" in kw:
            kw["out"] = unwrap(kw["out"])
        res = getattr(ufunc, method)(*ins, **kw)
        if ufunc is np.matmul and method == "__call__":
            COUNT[0] += product(res, ins[0], ins[1])
        return wrap(res)

    def __array_function__(self, func, types, args, kwargs):
        a, kw = unwrap(args), unwrap(kwargs)
        res = func(*a, **kw)
        rule = RULES.get(func)
        if rule is not None:
            COUNT[0] += rule(res, *a, **kw)
        elif func.__name__ in WATCHED:
            name = func.__module__.replace("numpy", "np", 1) + "." + func.__name__
            UNCOUNTED[name] = UNCOUNTED.get(name, 0) + 1
        return wrap(res)

    def dot(self, b, out=None):
        return np.dot(self, b, out=out)


def main():
    module, configs = sys.argv[1], sys.argv[2]
    try:
        mod = importlib.import_module(module)
    except Exception:
        print(json.dumps({"error": "the test module does not load: " + traceback.format_exc(limit=3)}))
        return
    case = getattr(mod, "flop_case", None)
    if case is None:
        print(json.dumps({"error": f"{module}.py has no flop_case(c), which gives the forward call of a config of shapes.json"}))
        return
    with open(configs) as f:
        cs = json.load(f)["configs"]
    out = []
    for c in cs:
        if c["kind"] == "named":
            continue  # the arrays of the paper's models are too large
        COUNT[0] = 0
        UNCOUNTED.clear()
        try:
            got = case(c)
            fn, args = got[0], tuple(got[1]) if len(got) > 1 else ()
            kwargs = got[2] if len(got) > 2 else {}
            fn(*wrap(args), **wrap(kwargs))
        except Exception:
            out.append({"config": c["name"], "error": traceback.format_exc(limit=4)})
            continue
        out.append({"config": c["name"], "counted": COUNT[0], "uncounted": dict(UNCOUNTED)})
    print(json.dumps({"configs": out}))


if __name__ == "__main__":
    main()
