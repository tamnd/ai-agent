// SPDX-License-Identifier: MIT
// Runs arch.js on each config of runs.json, twice each, and prints the values for aik viz check (V02, V07, V14).
"use strict";
const fs = require("fs");
Math.random = () => { throw new Error("the scene calls Math.random (V01)"); };
new Function(fs.readFileSync("arch.js", "utf8"))();
const d = JSON.parse(fs.readFileSync("scene.json", "utf8"));
const runs = JSON.parse(fs.readFileSync("runs.json", "utf8"));
const out = runs.map((r) => {
  const a = globalThis.AikArch.evaluate(d, r.values), b = globalThis.AikArch.evaluate(d, r.values);
  return { values: a, again: JSON.stringify(a) === JSON.stringify(b) };
});
process.stdout.write(JSON.stringify(out));
