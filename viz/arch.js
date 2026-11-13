// SPDX-License-Identifier: MIT
// The architecture scene of spec 12.6: the reader changes a size, and every shape, parameter count and FLOP count
// of the page follows. The formulas are the ones of shapes.yaml, compiled by aik; this file only evaluates them.
// aik viz check runs evaluate in Node against the Go evaluator of shapes.yaml (rules V07 and V14), so the numbers
// that the page shows are the numbers that the tests of the implementations check.
(function (root) {
  "use strict";

  // Exact rationals as [numerator, denominator] over BigInt, the denominator positive and the pair reduced.
  const ONE = [1n, 1n], ZERO = [0n, 1n];
  const gcd = (a, b) => {
    a = a < 0n ? -a : a;
    b = b < 0n ? -b : b;
    while (b) [a, b] = [b, a % b];
    return a;
  };
  const rat = (n, d) => {
    if (d < 0n) { n = -n; d = -d; }
    const g = gcd(n, d) || 1n;
    return [n / g, d / g];
  };
  const add = (a, b) => rat(a[0] * b[1] + b[0] * a[1], a[1] * b[1]);
  const str = (x) => (x[1] === 1n ? x[0].toString() : x[0] + "/" + x[1]);

  function ev(e, env) {
    if (typeof e === "string") return [BigInt(e), 1n];
    if (e.r !== undefined) return env(e.r);
    if (e.n !== undefined) { const x = ev(e.n, env); return [-x[0], x[1]]; }
    const a = ev(e.a, env), b = ev(e.b, env);
    switch (e.o) {
      case "+": return add(a, b);
      case "-": return add(a, [-b[0], b[1]]);
      case "*": return rat(a[0] * b[0], a[1] * b[1]);
      case "/":
        if (b[0] === 0n) throw new Error("division by zero");
        return rat(a[0] * b[1], a[1] * b[0]);
      case "^": {
        if (b[1] !== 1n || b[0] < 0n || b[0] > 65535n) throw new Error("the exponent " + str(b) + " is not a small whole number");
        let z = ONE;
        for (let i = 0n; i < b[0]; i++) z = rat(z[0] * a[0], z[1] * a[1]);
        return z;
      }
    }
    throw new Error("unknown operator " + e.o);
  }

  // evaluate gives every number of the scene at one config, as decimal strings, or the first error.
  function evaluate(d, values) {
    const out = { sizes: {}, params: {}, flops: {}, tensors: {}, model: {} };
    const derived = new Map(d.derived.map((f) => [f.name, f.expr]));
    const axes = new Map(d.axes.map((a) => [a.name, a.var]));
    const blocks = new Map(d.blocks.map((b) => [b.id, b]));
    const model = new Map(d.model.map((m) => [m.name, m.expr]));
    const memo = new Map(), busy = new Set();
    const get = (name) => {
      if (memo.has(name)) return memo.get(name);
      if (busy.has(name)) throw new Error(name + " depends on itself");
      busy.add(name);
      try {
        const x = raw(name);
        memo.set(name, x);
        return x;
      } finally {
        busy.delete(name);
      }
    };
    const raw = (name) => {
      const dot = name.indexOf(".");
      if (dot >= 0) {
        const id = name.slice(0, dot), part = name.slice(dot + 1);
        const b = blocks.get(id);
        if (!b) throw new Error(name + " names no block");
        const key = part === "params" ? "params" : part === "flops_forward" ? "flops" : "";
        if (!key) throw new Error(name + ": a block has params and flops_forward");
        if (b[key] != null) return ev(b[key], get);
        if (!b.of || b.of.length === 0) {
          if (key === "flops") return ZERO;
          throw new Error("the block " + id + " has no " + part);
        }
        return b.of.reduce((s, c) => add(s, get(c + "." + part)), ZERO);
      }
      if (derived.has(name)) return ev(derived.get(name), get);
      if (axes.has(name)) return get(axes.get(name));
      if (model.has(name)) return ev(model.get(name), get);
      if (Object.prototype.hasOwnProperty.call(values, name)) return [BigInt(values[name]), 1n];
      throw new Error("the config does not set " + name);
    };
    const whole = (name) => {
      const x = get(name);
      if (x[1] !== 1n) throw new Error(name + " is " + str(x) + ", not a whole number");
      return x[0].toString();
    };
    try {
      for (const c of d.controls) out.sizes[c.name] = whole(c.name);
      for (const f of d.derived) out.sizes[f.name] = whole(f.name);
      for (const a of d.axes) out.sizes[a.name] = whole(a.name);
      for (const b of d.blocks) {
        out.params[b.id] = whole(b.id + ".params");
        out.flops[b.id] = whole(b.id + ".flops_forward");
        for (const t of b.tensors || []) {
          out.tensors[b.id + "." + t.role + "." + t.name] = t.dims.map((e, i) => {
            const x = ev(e, get);
            if (x[1] !== 1n || x[0] < 1n) throw new Error(t.name + " " + t.shape + ": axis " + (i + 1) + " is " + str(x) + ", not a whole number of at least 1");
            return x[0].toString();
          });
        }
      }
      for (const m of d.model) out.model[m.name] = whole(m.name);
    } catch (err) {
      return { error: err.message, sizes: {}, params: {}, flops: {}, tensors: {}, model: {} };
    }
    return out;
  }

  // group writes 1234567 as 1,234,567, as shapes.Group does.
  const group = (s) => s.replace(/\B(?=(\d{3})+(?!\d))/g, ",");

  // mount makes one figure of the page live. The figure already shows the default preset, so the page reads the
  // same with no script; the script only recomputes.
  function mount(fig) {
    const src = fig.querySelector('script[type="application/json"]');
    if (!src) return;
    const d = JSON.parse(src.textContent);
    const inputs = Array.from(fig.querySelectorAll("input[data-size]"));
    const preset = fig.querySelector("select[data-preset]");
    const msg = fig.querySelector(".arch-error");
    const live = fig.querySelector(".arch-live");
    const update = () => {
      const values = {};
      let bad = "";
      for (const el of inputs) {
        const v = Number(el.value);
        if (!Number.isInteger(v) || v < 1) bad = bad || el.dataset.size + " must be a whole number of at least 1";
        else values[el.dataset.size] = v;
      }
      const res = bad ? { error: bad } : evaluate(d, values);
      fig.classList.toggle("arch-invalid", !!res.error);
      if (msg) { msg.hidden = !res.error; msg.textContent = res.error || ""; }
      if (res.error) return;
      for (const el of fig.querySelectorAll("[data-v]")) {
        const [kind, name] = el.dataset.v.split(":");
        const x = res[kind] && res[kind][name];
        if (x !== undefined) el.textContent = group(x);
      }
      for (const el of fig.querySelectorAll("[data-t]")) {
        const x = res.tensors[el.dataset.t];
        if (x) el.textContent = "(" + x.map(group).join(", ") + ")";
      }
      if (live) live.textContent = d.model.map((m) => m.name + " " + group(res.model[m.name])).join("; ");
    };
    const match = () => {
      const p = d.presets.find((p) => inputs.every((el) => String(p.values[el.dataset.size]) === el.value));
      if (preset) preset.value = p ? p.name : "";
    };
    for (const el of inputs) el.addEventListener("input", () => { match(); update(); });
    if (preset) {
      preset.addEventListener("change", () => {
        const p = d.presets.find((p) => p.name === preset.value);
        if (!p) return;
        for (const el of inputs) el.value = String(p.values[el.dataset.size]);
        update();
      });
    }
    fig.classList.add("arch-live-on");
  }

  root.AikArch = { evaluate, mount, group };
  if (typeof document !== "undefined") {
    const go = () => document.querySelectorAll("figure.arch").forEach(mount);
    if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", go);
    else go();
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
