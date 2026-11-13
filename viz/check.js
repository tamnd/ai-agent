// SPDX-License-Identifier: MIT
// Runs a bundled scene on its presets, twice each, and prints what aik viz check needs.
// The scene must be pure (rule V01): Math.random, timers, fetch and the DOM throw here.
"use strict";
const fs = require("fs");
const slug = process.argv[2];
const deny = (name) => () => { throw new Error(`the scene calls ${name} (V01)`); };
Math.random = deny("Math.random");
globalThis.setTimeout = deny("setTimeout");
globalThis.setInterval = deny("setInterval");
globalThis.fetch = deny("fetch");
globalThis.window = globalThis;
new Function(fs.readFileSync("scene.js", "utf8"))();
const scene = globalThis.AlgoScenes && globalThis.AlgoScenes[slug];
if (!scene) {
  process.stderr.write(`the bundle registers no scene with the slug ${slug}\n`);
  process.exit(1);
}
const presets = JSON.parse(fs.readFileSync("presets.json", "utf8"));
const STATES = new Set(["default", "active", "compare", "swap", "pivot", "frontier", "visited", "path", "rejected", "invariant"]);
// The place of the state in the arguments of each event that has one.
const STATE_AT = { "array.mark": 3, "graph.mark": 3, "graph.edge": 4, "tree.mark": 3, "tree.edge": 3, "table.mark": 3, "heap.mark": 3 };
// validate walks the events of a trace. Each event of a view names a view of its kind that exists, and each state is in the vocabulary (V09). It gives the first problems.
function validate(t) {
  const views = new Map(), bad = [];
  t.steps.forEach((s, i) => {
    for (const e of s.ev) {
      const [op, name] = e;
      const kind = op.split(".")[0];
      if (op === "destroy") { views.delete(name); continue; }
      if (op.endsWith(".create")) { views.set(name, kind); continue; }
      if (!["array", "graph", "tree", "table", "heap"].includes(kind)) { bad.push(`step ${i + 1}: the event ${op} is not in @aik/viz`); continue; }
      if (op === "array.match" && views.get(e[3]) !== "array") bad.push(`step ${i + 1}: array.match names ${JSON.stringify(e[3])}, which is not a string view that exists`);
      if (views.get(name) !== kind) bad.push(`step ${i + 1}: ${op} names ${JSON.stringify(name)}, which is not a ${kind} view that exists`);
      const at = STATE_AT[op];
      if (at !== undefined && e[at] !== null && !STATES.has(e[at])) bad.push(`step ${i + 1}: the state ${JSON.stringify(e[at])} is not in the vocabulary (V09)`);
    }
  });
  return bad.slice(0, 5);
}
// The TypeScript reference with its markers turned into probes (rule V06), when the node has one.
let ref = null, refError = "";
if (fs.existsSync("ref.js")) {
  try {
    const mod = { exports: {} };
    new Function("module", "exports", "require", fs.readFileSync("ref.js", "utf8"))(mod, mod.exports, () => { throw new Error("the reference imports a module"); });
    const fns = Object.values(mod.exports).filter((f) => typeof f === "function");
    if (fns.length !== 1) throw new Error(`the reference exports ${fns.length} functions, and V06 needs exactly one`);
    ref = fns[0];
  } catch (e) {
    refError = String(e && e.message ? e.message : e);
  }
}
let probes = [];
globalThis.__probe = (id) => {
  if (probes.length >= 1000000) throw new Error("the reference runs more than 1000000 marked lines");
  probes.push(id);
  return true;
};
// merged removes the repeats of the same id next to each other.
const merged = (ids) => ids.filter((id, i) => i === 0 || id !== ids[i - 1]);
// argsOf maps the input of a preset to the parameters of the reference, by name. A reference with one parameter that is not a key of the input gets the whole input.
function argsOf(fn, input) {
  const m = /^[^(]*\(([^)]*)\)/.exec(fn.toString());
  const names = m ? m[1].split(",").map((s) => s.trim().split(/[\s=]/)[0]).filter(Boolean) : [];
  if (input !== null && typeof input === "object" && !Array.isArray(input) && names.every((n) => n in input)) return names.map((n) => input[n]);
  if (names.length === 1) return [input];
  throw new Error(`the input has no key for each parameter of the reference (${names.join(", ")})`);
}
// lineTrace runs the reference on the input and compares its marked lines with the lines of the scene (V06). It gives a message, or "" when the two agree.
function lineTrace(t, input) {
  if (refError) return refError;
  probes = [];
  ref(...argsOf(ref, JSON.parse(JSON.stringify(input))));
  const a = merged(t.steps.map((s) => s.line)), b = merged(probes);
  let i = 0;
  while (i < a.length && i < b.length && a[i] === b[i]) i++;
  if (i === a.length && i === b.length) return "";
  const show = (xs) => {
    const w = xs.slice(Math.max(0, i - 3), i + 3);
    if (i >= xs.length) w.push("(end)");
    return (i > 3 ? "... " : "") + w.join(" ") + (i + 3 < xs.length ? " ..." : "");
  };
  return `the lines differ from the TypeScript reference at line ${i + 1} of ${Math.max(a.length, b.length)}: the scene runs ${show(a)}, the reference runs ${show(b)} (V06)`;
}
const out = [];
for (const p of presets) {
  try {
    const a = JSON.stringify(globalThis.AlgoViz.trace(scene, p.input));
    const b = JSON.stringify(globalThis.AlgoViz.trace(scene, p.input));
    const t = JSON.parse(a);
    const invalid = validate(t);
    if (ref || refError) {
      const msg = lineTrace(t, p.input);
      if (msg) invalid.push(msg);
    }
    out.push({ name: p.name, same: a === b, steps: t.steps.length, lines: [...new Set(t.steps.map((s) => s.line))], invalid });
  } catch (e) {
    out.push({ name: p.name, error: String(e && e.message ? e.message : e) });
  }
}
process.stdout.write(JSON.stringify({ presets: out }));
