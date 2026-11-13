// SPDX-License-Identifier: Apache-2.0
// The player of the scenes (spec 10.6). A scene bundle sets window.AlgoScenes[slug] and window.AlgoViz. The player runs the scene on an input, computes the state after each step and draws it. It needs no server and sends nothing.
(function () {
  "use strict";
  var d = document;
  var MAX_STEPS = 20000;
  var STATES = ["active", "compare", "swap", "pivot", "frontier", "visited", "path", "rejected", "invariant"];
  var NAMES = { active: "read", compare: "compare", swap: "write or swap", pivot: "pivot", frontier: "frontier", visited: "visited", path: "result", rejected: "out of range", invariant: "invariant holds" };

  function clone(o) { return JSON.parse(JSON.stringify(o)); }
  function el(tag, cls, text) { var e = d.createElement(tag); if (cls) e.className = cls; if (text != null) e.textContent = text; return e; }
  function fmt(v) {
    if (v === null || v === undefined) return "nil";
    if (typeof v === "number") return Number.isInteger(v) ? String(v) : String(Math.round(v * 1000) / 1000);
    if (typeof v === "string") return v;
    return JSON.stringify(v);
  }
  // A range is an index or [lo, hi], with both ends included.
  function each(range, f) {
    if (typeof range === "number") { f(range); return; }
    if (Array.isArray(range)) for (var i = range[0]; i <= range[1]; i++) f(i);
  }

  // frames turns a trace into one state for each step.
  function frames(trace) {
    var st = { arrays: {}, graphs: {}, trees: {}, tables: {}, order: [], vars: {}, counters: {}, seen: {} };
    Object.keys(trace.counters || {}).forEach(function (k) { st.counters[k] = 0; });
    var all = {}, out = [];
    trace.steps.forEach(function (step) {
      var hot = {}, hot2 = {};
      step.ev.forEach(function (e) {
        var a = st.arrays[e[1]];
        if (/^(graph|tree)\./.test(e[0])) { structEvent(st, e, hot2, all); return; }
        if (/^table\./.test(e[0])) { tableEvent(st, e, hot2); return; }
        // A heap keeps the state of an array. The player draws it twice.
        var op = e[0].indexOf("heap.") === 0 ? "array." + e[0].slice(5) : e[0];
        switch (op) {
          case "array.create":
            st.arrays[e[1]] = { values: e[2].slice(), view: (e[3] && e[3].view) || "bars", label: (e[3] && e[3].label) || e[1], marks: {}, ptrs: {}, br: null, win: null, shift: 0 };
            if (st.order.indexOf(e[1]) < 0) st.order.push(e[1]);
            break;
          case "destroy":
            delete st.arrays[e[1]]; delete st.graphs[e[1]]; delete st.trees[e[1]]; delete st.tables[e[1]];
            st.order = st.order.filter(function (n) { return n !== e[1]; });
            break;
          case "array.get":
            if (a) (hot[e[1]] = hot[e[1]] || {})[e[2]] = "active";
            break;
          case "array.set":
            if (a) { a.values[e[2]] = e[3]; (hot[e[1]] = hot[e[1]] || {})[e[2]] = "swap"; }
            inc(st, "write");
            break;
          case "array.swap":
            if (a) {
              var t = a.values[e[2]]; a.values[e[2]] = a.values[e[3]]; a.values[e[3]] = t;
              var h = (hot[e[1]] = hot[e[1]] || {}); h[e[2]] = h[e[3]] = "swap";
            }
            inc(st, "swap");
            break;
          case "array.compare":
            if (a) { var c = (hot[e[1]] = hot[e[1]] || {}); c[e[2]] = "compare"; if (e[3] >= 0) c[e[3]] = "compare"; }
            inc(st, "compare");
            break;
          case "array.mark":
            if (a) each(e[2], function (i) { a.marks[i] = e[3]; });
            st.seen[e[3]] = true;
            break;
          case "array.unmark":
            if (a) each(e[2], function (i) { delete a.marks[i]; });
            break;
          case "array.ptr":
            if (a) { if (e[3] === null || e[3] === undefined) delete a.ptrs[e[2]]; else a.ptrs[e[2]] = e[3]; }
            break;
          case "array.bracket":
            if (a) a.br = e[2] === null ? null : [e[2], e[3], e[4]];
            break;
          case "array.label":
            if (a) a.label = e[2];
            break;
          case "array.push":
            if (a) { a.values.push(e[2]); (hot[e[1]] = hot[e[1]] || {})[a.values.length - 1] = "swap"; }
            break;
          case "array.pop":
            if (a) { delete a.marks[a.values.length - 1]; a.values.pop(); }
            break;
          case "array.window":
            if (a) a.win = e[2] === null ? null : [e[2], e[3]];
            break;
          case "array.shift":
            if (a) a.shift = e[2];
            break;
          case "array.match":
            var o = st.arrays[e[3]];
            if (a) (hot[e[1]] = hot[e[1]] || {})[e[2]] = "compare";
            if (o) (hot[e[3]] = hot[e[3]] || {})[e[4]] = "compare";
            inc(st, "compare");
            break;
        }
      });
      Object.keys(hot).forEach(function (n) { Object.keys(hot[n]).forEach(function (i) { st.seen[hot[n][i]] = true; }); });
      if (step.vars) Object.keys(step.vars).forEach(function (k) { st.vars[k] = step.vars[k]; });
      var f = clone(st);
      f.hot = hot; f.hot2 = hot2; f.line = step.line; f.note = step.note || ""; f.phase = step.phase || "";
      delete f.seen;
      out.push(f);
    });
    return { frames: out, seen: Object.keys(st.seen), all: all };
  }

  // tableEvent applies one event of a table to the state. A fill lights the cells that the value comes from.
  function tableEvent(st, e, hot2) {
    var t = st.tables[e[1]], h;
    if (e[0] === "table.create") {
      st.tables[e[1]] = { label: e[2].label || e[1], rows: e[2].rows, cols: e[2].cols, rh: e[2].rowHeads, ch: e[2].colHeads, values: clone(e[2].values), marks: {}, from: {} };
      if (st.order.indexOf(e[1]) < 0) st.order.push(e[1]);
      return;
    }
    if (!t) return;
    var k = e[2] + "," + e[3];
    switch (e[0]) {
      case "table.get": hotOf(hot2, e[1]).n[k] = "active"; st.seen.active = true; break;
      case "table.fill":
        t.values[e[2]][e[3]] = e[4];
        t.from[k] = e[5].length ? e[5][0] : null;
        h = hotOf(hot2, e[1]); h.n[k] = "swap"; st.seen.swap = true;
        e[5].forEach(function (x) { h.n[x[0] + "," + x[1]] = "compare"; st.seen.compare = true; });
        inc(st, "write");
        break;
      case "table.mark": t.marks[e[2][0] + "," + e[2][1]] = e[3]; st.seen[e[3]] = true; break;
      case "table.unmark": delete t.marks[e[2][0] + "," + e[2][1]]; break;
      case "table.label": t.label = e[2]; break;
    }
  }

  function inc(st, name) { st.counters[name] = (st.counters[name] || 0) + 1; }
  function idKey(id) { return JSON.stringify(id); }
  function edgeKey(g, u, w) {
    var a = idKey(u), b = idKey(w);
    return g.directed || a <= b ? a + ">" + b : b + ">" + a;
  }
  function hotOf(hot2, name) { return hot2[name] || (hot2[name] = { n: {}, e: {} }); }

  // structEvent applies one event of a graph or a tree to the state. all keeps, for each graph, each node and edge that the trace ever has, for the layout.
  function structEvent(st, e, hot2, all) {
    var name = e[1], g = st.graphs[name], t = st.trees[name], h, k;
    function seen(s) { if (s) st.seen[s] = true; }
    switch (e[0]) {
      case "graph.create":
        g = st.graphs[name] = { label: e[2].label || name, directed: e[2].directed, weighted: e[2].weighted, nodes: {}, keys: [], edges: {}, frontier: null };
        all[name] = all[name] || { layout: e[2].layout, directed: e[2].directed, keys: [], nodes: {}, edges: {}, frontier: false };
        e[2].nodes.forEach(function (n) { addNode(g, all[name], n); });
        e[2].edges.forEach(function (x) { addEdge(g, all[name], x[0], x[1], x.length > 2 ? x[2] : null); });
        if (st.order.indexOf(name) < 0) st.order.push(name);
        return;
      case "tree.create":
        t = st.trees[name] = { label: e[2].label || name, binary: e[2].binary, nodes: {}, keys: [] };
        e[2].nodes.forEach(function (n) { treePut(t, n.parent === undefined ? null : n.parent, n.id, n.side || null, n.value === undefined ? n.id : n.value); });
        if (st.order.indexOf(name) < 0) st.order.push(name);
        return;
    }
    if (g) {
      switch (e[0]) {
        case "graph.addNode": addNode(g, all[name], e[2]); break;
        case "graph.addEdge": addEdge(g, all[name], e[2], e[3], e[4]); break;
        case "graph.removeEdge": delete g.edges[edgeKey(g, e[2], e[3])]; break;
        case "graph.visit": hotOf(hot2, name).n[idKey(e[2])] = "active"; seen("active"); inc(st, "visit"); break;
        case "graph.edge":
          k = edgeKey(g, e[2], e[3]);
          if (e[4] === null) { hotOf(hot2, name).e[k] = "compare"; seen("compare"); inc(st, "edge"); }
          else if (g.edges[k]) { g.edges[k].mark = e[4]; seen(e[4]); }
          break;
        case "graph.relax":
          h = hotOf(hot2, name); h.e[edgeKey(g, e[2], e[3])] = "compare"; seen("compare");
          if (e[6]) { g.nodes[idKey(e[3])].tag = e[5]; h.n[idKey(e[3])] = "swap"; seen("swap"); }
          inc(st, "relax");
          break;
        case "graph.nodeLabel": g.nodes[idKey(e[2])].tag = e[3]; break;
        case "graph.mark": g.nodes[idKey(e[2])].mark = e[3]; seen(e[3]); break;
        case "graph.unmark": delete g.nodes[idKey(e[2])].mark; break;
        case "graph.path":
          e[2].forEach(function (u, i) {
            g.nodes[idKey(u)].mark = "path";
            if (i > 0 && g.edges[edgeKey(g, e[2][i - 1], u)]) g.edges[edgeKey(g, e[2][i - 1], u)].mark = "path";
          });
          seen("path");
          break;
        case "graph.frontier": g.frontier = { ids: e[2], label: e[3] }; all[name].frontier = true; break;
        case "graph.label": g.label = e[2]; break;
      }
      return;
    }
    if (t) {
      switch (e[0]) {
        case "tree.insert":
          if (e[6]) treePut(t, e[2], e[3], e[4], e[5] === null ? e[3] : e[5]);
          else { k = idKey(e[3]); t.nodes[k].parent = e[2] === null ? null : idKey(e[2]); t.nodes[k].side = e[2] === null ? null : e[4]; }
          break;
        case "tree.remove":
          k = idKey(e[2]);
          t.keys.forEach(function (c) { if (t.nodes[c].parent === k) { t.nodes[c].parent = t.nodes[k].parent; t.nodes[c].side = t.nodes[k].parent === null ? null : t.nodes[k].side; } });
          delete t.nodes[k];
          t.keys = t.keys.filter(function (x) { return x !== k; });
          break;
        case "tree.rotate": treeRotate(t, idKey(e[2]), e[3]); inc(st, "rotate"); break;
        case "tree.replace": t.nodes[idKey(e[2])].value = e[3]; hotOf(hot2, name).n[idKey(e[2])] = "swap"; seen("swap"); break;
        case "tree.visit": hotOf(hot2, name).n[idKey(e[2])] = "active"; seen("active"); inc(st, "visit"); break;
        case "tree.nodeLabel": t.nodes[idKey(e[2])].tag = e[3]; break;
        case "tree.mark": t.nodes[idKey(e[2])].mark = e[3]; seen(e[3]); break;
        case "tree.unmark": delete t.nodes[idKey(e[2])].mark; break;
        case "tree.edge": t.nodes[idKey(e[2])].edge = e[3]; seen(e[3]); break;
        case "tree.label": t.label = e[2]; break;
      }
    }
  }

  function addNode(g, a, n) {
    var k = idKey(n.id);
    g.nodes[k] = { id: n.id, text: n.label != null ? String(n.label) : fmt(n.id) };
    g.keys.push(k);
    if (!a.nodes[k]) { a.nodes[k] = n; a.keys.push(k); }
  }
  function addEdge(g, a, u, w, weight) {
    var k = edgeKey(g, u, w);
    g.edges[k] = { u: idKey(u), w: idKey(w), weight: weight };
    a.edges[k] = { u: idKey(u), w: idKey(w) };
  }
  function treePut(t, parent, id, side, value) {
    var k = idKey(id);
    t.nodes[k] = { id: id, value: value, parent: parent === null ? null : idKey(parent), side: parent === null ? null : side };
    t.keys.push(k);
  }
  function treeChild(t, k, side) {
    for (var i = 0; i < t.keys.length; i++) { var c = t.nodes[t.keys[i]]; if (c.parent === k && c.side === side) return t.keys[i]; }
    return null;
  }
  // treeRotate is the rotation of @aik/viz: "left" lifts the right child of the node.
  function treeRotate(t, u, dir) {
    var up = dir === "left" ? "right" : "left", x = treeChild(t, u, up);
    if (x === null) return;
    var inner = treeChild(t, x, dir), n = t.nodes[u];
    t.nodes[x].parent = n.parent; t.nodes[x].side = n.side;
    n.parent = x; n.side = dir;
    if (inner !== null) { t.nodes[inner].parent = u; t.nodes[inner].side = up; }
  }

  // graphLayout places each node of the trace once, so a node never moves. The box is 320 units wide, the width of a small phone.
  function graphLayout(a) {
    var W = 320, pos = {}, n = a.keys.length, H = 250;
    if (a.layout === "fixed") {
      a.keys.forEach(function (k) { var o = a.nodes[k]; pos[k] = { x: 26 + (o.x || 0) * (W - 72), y: 30 + (o.y || 0) * (H - 56) }; });
    } else if (a.layout === "layered" && n) {
      var level = {}, q = [a.keys[0]], out = {};
      Object.keys(a.edges).forEach(function (k) {
        var x = a.edges[k];
        (out[x.u] = out[x.u] || []).push(x.w);
        if (!a.directed) (out[x.w] = out[x.w] || []).push(x.u);
      });
      level[a.keys[0]] = 0;
      for (var i = 0; i < q.length; i++) (out[q[i]] || []).forEach(function (w) { if (level[w] === undefined) { level[w] = level[q[i]] + 1; q.push(w); } });
      var top = 0;
      a.keys.forEach(function (k) { if (level[k] === undefined) level[k] = 0; top = Math.max(top, level[k]); });
      // A node that the search does not reach goes to the first level.
      H = Math.max(120, (top + 1) * 64 + 20);
      var rows = {};
      a.keys.forEach(function (k) { (rows[level[k]] = rows[level[k]] || []).push(k); });
      Object.keys(rows).forEach(function (l) {
        rows[l].forEach(function (k, j) { pos[k] = { x: W * (j + 1) / (rows[l].length + 1), y: 34 + l * 64 }; });
      });
    } else {
      a.keys.forEach(function (k, i) {
        var ang = -Math.PI / 2 + 2 * Math.PI * i / Math.max(1, n);
        pos[k] = { x: W / 2 + 120 * Math.cos(ang), y: H / 2 + 4 + 96 * Math.sin(ang) };
      });
    }
    return { pos: pos, w: W, h: H };
  }

  // treeLayout is a tidy layout: a leaf takes the next column, and a parent is over the middle of its children. A binary tree uses the in-order rank as the column, so a left child is always to the left of its parent.
  function treeLayout(t) {
    var kids = {}, roots = [], pos = {}, next = 0, depth = 0;
    t.keys.forEach(function (k) { var p = t.nodes[k].parent; if (p === null) roots.push(k); else (kids[p] = kids[p] || []).push(k); });
    function side(k, s) { return (kids[k] || []).filter(function (c) { return t.nodes[c].side === s; })[0]; }
    function bin(k, dep) {
      if (k === undefined) return;
      depth = Math.max(depth, dep);
      bin(side(k, "left"), dep + 1);
      pos[k] = { col: next++, row: dep };
      bin(side(k, "right"), dep + 1);
    }
    function tidy(k, dep) {
      depth = Math.max(depth, dep);
      var cs = kids[k] || [];
      if (!cs.length) { pos[k] = { col: next++, row: dep }; return; }
      cs.forEach(function (c) { tidy(c, dep + 1); });
      pos[k] = { col: (pos[cs[0]].col + pos[cs[cs.length - 1]].col) / 2, row: dep };
    }
    roots.forEach(function (r) { if (t.binary) bin(r, 0); else tidy(r, 0); });
    return { pos: pos, cols: next, rows: t.keys.length ? depth + 1 : 0 };
  }

  function Player(root) {
    this.root = root;
    this.slug = root.dataset.scene;
    this.scene = (window.AlgoScenes || {})[this.slug];
    this.stage = root.querySelector(".pl-stage");
    this.noteEl = root.querySelector(".pl-note");
    this.varsEl = root.querySelector(".pl-vars");
    this.countEl = root.querySelector(".pl-counters");
    this.scrub = root.querySelector(".pl-scrub");
    this.pos = root.querySelector(".pl-pos");
    this.playBtn = root.querySelector('[data-act="play"]');
    this.speed = root.querySelector(".pl-speed");
    this.presetSel = root.querySelector(".pl-preset");
    this.text = root.querySelector(".pl-custom textarea");
    this.err = root.querySelector(".pl-err");
    this.code = root.querySelector(".player-code");
    this.lines = {};
    var self = this;
    if (this.code) this.code.querySelectorAll("li[data-line]").forEach(function (li) { self.lines[li.dataset.line] = li; });
    try { this.presets = JSON.parse(root.dataset.presets || "[]") || []; } catch (e) { this.presets = []; }
    if (!this.scene || !window.AlgoViz) { this.stage.textContent = "The scene did not load."; return; }
    this.presets.forEach(function (p, i) { var o = el("option", null, p.name); o.value = String(i); self.presetSel.appendChild(o); });
    this.bind();
    this.load(this.presets.length ? this.presets[0].input : {});
  }

  Player.prototype.bind = function () {
    var self = this, r = this.root;
    r.addEventListener("click", function (e) {
      var b = e.target.closest("[data-act]");
      if (!b) return;
      var act = b.dataset.act;
      if (act === "start") self.go(0);
      else if (act === "prev") self.go(self.k - 1);
      else if (act === "next") self.go(self.k + 1);
      else if (act === "end") self.go(self.f.length - 1);
      else if (act === "play") self.toggle();
      else if (act === "run") self.custom();
    });
    this.scrub.addEventListener("input", function () { self.stop(); self.go(+self.scrub.value); });
    this.presetSel.addEventListener("change", function () { self.stop(); self.load(self.presets[+self.presetSel.value].input); });
    r.tabIndex = -1;
    var keyTarget = d.querySelectorAll(".player").length === 1 ? d : r;
    keyTarget.addEventListener("keydown", function (e) {
      var t = e.target;
      if (t && (t.tagName === "INPUT" && t.type !== "range" || t.tagName === "TEXTAREA" || t.tagName === "SELECT")) return;
      if (keyTarget === d && !self.visible()) return;
      var k = e.key, done = true;
      if (k === " ") self.toggle();
      else if (k === "ArrowRight") { self.stop(); self.go(self.k + 1); }
      else if (k === "ArrowLeft") { self.stop(); self.go(self.k - 1); }
      else if (k === "Home") { self.stop(); self.go(0); }
      else if (k === "End") { self.stop(); self.go(self.f.length - 1); }
      else if (k === "+" || k === "=") self.faster(1);
      else if (k === "-" || k === "_") self.faster(-1);
      else done = false;
      if (done) e.preventDefault();
    });
  };

  Player.prototype.visible = function () {
    var b = this.root.getBoundingClientRect();
    return b.bottom > 0 && b.top < innerHeight;
  };

  Player.prototype.faster = function (dir) {
    var i = this.speed.selectedIndex + dir;
    if (i >= 0 && i < this.speed.options.length) this.speed.selectedIndex = i;
  };

  Player.prototype.load = function (input) {
    var trace;
    this.err.textContent = "";
    try {
      trace = window.AlgoViz.trace(this.scene, input);
    } catch (e) {
      this.err.textContent = "The scene cannot run on this input: " + (e && e.message ? e.message : e);
      return false;
    }
    if (!trace.steps.length) { this.err.textContent = "The scene made no steps for this input."; return false; }
    if (trace.steps.length > MAX_STEPS) { this.err.textContent = "This input makes too many steps for the player. Use a smaller input."; return false; }
    var r = frames(trace);
    this.trace = trace; this.f = r.frames; this.seen = r.seen; this.all = r.all;
    this.text.value = JSON.stringify(input);
    this.scrub.max = String(this.f.length - 1);
    this.max = {};
    var self = this;
    this.f.forEach(function (fr) {
      fr.order.forEach(function (n) {
        if (fr.arrays[n]) fr.arrays[n].values.forEach(function (v) { if (typeof v === "number") self.max[n] = Math.max(self.max[n] || 0, Math.abs(v)); });
      });
    });
    // All the strings use one column grid, wide enough for the longest string and its shift, so a pattern stays under its text.
    this.strCols = 1;
    this.f.forEach(function (fr) {
      Object.keys(fr.arrays).forEach(function (n) { var a = fr.arrays[n]; if (a.view === "str") self.strCols = Math.max(self.strCols, a.shift + a.values.length); });
    });
    // All arrays use one scale, so equal values have equal bars.
    this.top = Object.keys(this.max).reduce(function (t, n) { return Math.max(t, self.max[n]); }, 0);
    this.build();
    this.go(0);
    return true;
  };

  Player.prototype.custom = function () {
    var v;
    try { v = JSON.parse(this.text.value); } catch (e) { this.err.textContent = "This is not valid JSON: " + e.message; return; }
    var spec = this.scene.input || {};
    var msg = check(spec, v);
    if (msg) { this.err.textContent = msg; return; }
    this.stop();
    this.load(v);
  };

  // check tests a custom input against the input spec of the scene, for the common case of an array of numbers.
  function check(spec, v) {
    if (typeof v !== "object" || v === null || Array.isArray(v)) return "The input must be a JSON object, in the same form as a preset.";
    var arr = null;
    Object.keys(v).forEach(function (k) { if (Array.isArray(v[k]) && !arr) arr = v[k]; });
    if (arr && spec.length) {
      if (arr.length < spec.length[0] || arr.length > spec.length[1]) return "The array must have from " + spec.length[0] + " to " + spec.length[1] + " items.";
    }
    if (arr && spec.values) {
      for (var i = 0; i < arr.length; i++) {
        if (typeof arr[i] !== "number" || arr[i] < spec.values[0] || arr[i] > spec.values[1]) return "Each item must be a number from " + spec.values[0] + " to " + spec.values[1] + ".";
      }
    }
    return "";
  }

  // build makes the elements of the stage for the arrays of the trace. go then only changes classes and sizes.
  Player.prototype.build = function () {
    var self = this;
    this.stage.textContent = "";
    this.views = {};
    var names = [];
    this.f.forEach(function (fr) { fr.order.forEach(function (n) { if (names.indexOf(n) < 0) names.push(n); }); });
    names.forEach(function (n) {
      var kind = kindOf(self.f, n);
      if (kind === "graph") { self.views[n] = buildGraph(self, n); return; }
      if (kind === "tree") { self.views[n] = buildTree(self, n); return; }
      if (kind === "table") { self.views[n] = buildTable(self, n); return; }
      if (kind === "heap") { self.views[n] = buildHeap(self, n); return; }
      self.views[n] = buildArray(self.stage);
    });
    if (this.seen.length) {
      var lg = el("div", "legend");
      STATES.forEach(function (s) {
        if (self.seen.indexOf(s) < 0) return;
        var x = el("span", null, NAMES[s]);
        x.style.setProperty("--c", "var(--st-" + s + ")");
        lg.appendChild(x);
      });
      this.stage.appendChild(lg);
    }
  };

  Player.prototype.go = function (k) {
    if (!this.f) return;
    k = Math.max(0, Math.min(this.f.length - 1, k));
    this.k = k;
    var fr = this.f[k], self = this;
    Object.keys(this.views).forEach(function (n) {
      var v = self.views[n], a = fr.arrays[n];
      if (v.kind === "graph") { drawGraph(v, fr.graphs[n], fr.hot2[n]); return; }
      if (v.kind === "tree") { drawTree(v, fr.trees[n], fr.hot2[n]); return; }
      if (v.kind === "table") { drawTable(v, fr.tables[n], fr.hot2[n]); return; }
      if (v.kind === "heap") { drawHeap(self, v, a, fr.hot[n] || {}); return; }
      drawArray(self, v, a, fr.hot[n] || {});
    });
    var note = fr.note;
    this.noteEl.textContent = note || fr.phase || "";
    this.vars(fr);
    Object.keys(this.lines).forEach(function (id) { self.lines[id].classList.toggle("cur", id === fr.line); });
    var cur = this.lines[fr.line];
    if (cur && this.code) {
      var box = this.code.parentElement, top = cur.offsetTop - box.offsetTop;
      if (box.scrollHeight > box.clientHeight && (top < box.scrollTop || top > box.scrollTop + box.clientHeight - 30)) box.scrollTop = top - 40;
    }
    if (window.AlgoMark) window.AlgoMark(fr.line);
    this.scrub.value = String(k);
    this.pos.textContent = (k + 1) + " / " + this.f.length;
  };

  function buildArray(parent) {
    var w = el("div", "arr"), lab = el("div", "arr-label"), cells = el("div", "arr-cells");
    w.appendChild(lab); w.appendChild(cells);
    parent.appendChild(w);
    return { kind: "array", w: w, lab: lab, cells: cells, list: [] };
  }

  // drawArray draws an array as bars or cells, or a string as cells on the column grid of the strings. view replaces the view of the array, for the array half of a heap.
  function drawArray(p, v, a, hot, view) {
    v.w.hidden = !a;
    if (!a) return;
    view = view || a.view;
    var str = view === "str", cellsLike = view === "cells" || str;
    v.w.className = "arr " + (cellsLike ? "cells" : view) + (str ? " str" : "") + (!str && a.values.length > 14 ? " dense" : "");
    v.lab.textContent = a.label;
    if (str) v.cells.style.setProperty("--n", String(p.strCols));
    while (v.list.length < a.values.length) {
      var c = el("div", "cell"), pt = el("div", "ptrs"), bar = el("div", "blk"), val = el("div", "v"), idx = el("div", "i", String(v.list.length));
      if (cellsLike) { bar.appendChild(val); c.appendChild(pt); c.appendChild(bar); c.appendChild(idx); }
      else { c.appendChild(pt); c.appendChild(bar); c.appendChild(val); c.appendChild(idx); }
      v.cells.appendChild(c);
      v.list.push({ c: c, p: pt, bar: bar, val: val });
    }
    var ptrAt = {};
    Object.keys(a.ptrs).forEach(function (name) { var i = a.ptrs[name]; (ptrAt[i] = ptrAt[i] || []).push(name); });
    var m = p.top || 1;
    v.list.forEach(function (x, i) {
      x.c.hidden = i >= a.values.length;
      if (i >= a.values.length) return;
      var val = a.values[i];
      x.val.textContent = str && val === " " ? "\u2423" : fmt(val);
      if (view === "bars") x.bar.style.height = (typeof val === "number" ? Math.max(0.2, Math.abs(val) / m * 6.4) : 3.2).toFixed(2) + "rem";
      var s = hot[i] || a.marks[i] || "";
      if (s) x.c.dataset.st = s; else delete x.c.dataset.st;
      var out = a.br && (i < a.br[0] || (a.br[2] !== null && i > a.br[2]));
      x.c.classList.toggle("out", !!out);
      x.c.classList.toggle("win", !!(a.win && i >= a.win[0] && i <= a.win[1]));
      x.c.style.gridColumnStart = str && i === 0 ? String(a.shift + 1) : "";
      x.p.textContent = (ptrAt[i] || []).join(" ");
    });
  }

  // buildTable makes an HTML table with the size and the headers of the create event. The size does not change, so the stage keeps its height.
  function buildTable(p, n) {
    var t0 = null;
    p.f.some(function (fr) { t0 = fr.tables[n]; return !!t0; });
    var w = el("div", "tbl"), lab = el("div", "arr-label"), sc = el("div", "tbl-scroll"), tb = el("table", "tbl-grid"), cells = [];
    if (t0.ch) {
      var hr = el("tr");
      if (t0.rh) hr.appendChild(el("th", "tbl-corner"));
      t0.ch.forEach(function (h) { var th = el("th", null, fmt(h)); th.scope = "col"; hr.appendChild(th); });
      var thead = el("thead"); thead.appendChild(hr); tb.appendChild(thead);
    }
    var body = el("tbody");
    for (var r = 0; r < t0.rows; r++) {
      var tr = el("tr"), row = [];
      if (t0.rh) { var th = el("th", null, fmt(t0.rh[r])); th.scope = "row"; tr.appendChild(th); }
      for (var c = 0; c < t0.cols; c++) {
        var td = el("td"), arrow = el("span", "tbl-from"), val = el("span", "tbl-v");
        arrow.setAttribute("aria-hidden", "true");
        td.appendChild(arrow); td.appendChild(val); tr.appendChild(td);
        row.push({ td: td, arrow: arrow, val: val });
      }
      body.appendChild(tr); cells.push(row);
    }
    tb.appendChild(body);
    sc.appendChild(tb); w.appendChild(lab); w.appendChild(sc);
    p.stage.appendChild(w);
    return { kind: "table", w: w, lab: lab, cells: cells };
  }

  var ARROWS = { "-1,-1": "\u2196", "-1,0": "\u2191", "-1,1": "\u2197", "0,-1": "\u2190", "0,1": "\u2192", "1,-1": "\u2199", "1,0": "\u2193", "1,1": "\u2198" };
  function drawTable(v, t, hot) {
    v.w.hidden = !t;
    if (!t) return;
    hot = hot || { n: {} };
    v.lab.textContent = t.label;
    v.cells.forEach(function (row, r) {
      row.forEach(function (x, c) {
        var k = r + "," + c, val = t.values[r][c], s = hot.n[k] || t.marks[k] || "", f = t.from[k];
        x.val.textContent = val === null || val === undefined ? "" : fmt(val);
        x.arrow.textContent = f ? ARROWS[Math.sign(f[0] - r) + "," + Math.sign(f[1] - c)] || "" : "";
        paint(x.td, s);
        if (s) x.td.setAttribute("aria-label", x.val.textContent + " " + (NAMES[s] || s)); else x.td.removeAttribute("aria-label");
      });
    });
  }

  // buildHeap makes the tree of the heap over its array. The tree has room for the largest heap of the trace, so the stage keeps its height.
  var HROW = 50;
  function buildHeap(p, n) {
    var most = 1;
    p.f.forEach(function (fr) { if (fr.arrays[n]) most = Math.max(most, fr.arrays[n].values.length); });
    var depth = Math.floor(Math.log2(most)) + 1, W = Math.max(320, Math.pow(2, depth - 1) * 34), H = depth * HROW + 8;
    var w = el("div", "gv hv"), lab = el("div", "arr-label");
    var box = svg("svg", { viewBox: "0 0 " + W + " " + H, role: "img", "class": "gv-svg" });
    box.style.maxWidth = Math.max(32, W / 10) + "rem";
    var eg = svg("g", { "class": "gv-edges" }, box), ng = svg("g", { "class": "gv-nodes" }, box), nodes = [];
    for (var i = 0; i < most; i++) {
      var l = Math.floor(Math.log2(i + 1)), x = (i + 1 - Math.pow(2, l) + 0.5) * W / Math.pow(2, l), y = HROW / 2 + 4 + l * HROW, e = null;
      if (i > 0) {
        var q = (i - 1) >> 1, ql = Math.floor(Math.log2(q + 1)), qx = (q + 1 - Math.pow(2, ql) + 0.5) * W / Math.pow(2, ql), qy = y - HROW;
        var dx = x - qx, dy = y - qy, len = Math.sqrt(dx * dx + dy * dy);
        e = svg("line", { "class": "gv-edge", x1: (qx + dx / len * R).toFixed(1), y1: (qy + dy / len * R).toFixed(1), x2: (x - dx / len * R).toFixed(1), y2: (y - dy / len * R).toFixed(1) }, eg);
      }
      var g = svg("g", { "class": "gv-node", transform: "translate(" + x.toFixed(1) + " " + y.toFixed(1) + ")" }, ng);
      svg("circle", { r: R }, g);
      var t = svg("text", { y: 4, "class": "gv-id" }, g), ix = svg("text", { y: R + 11, "class": "gv-ix" }, g);
      ix.textContent = String(i);
      nodes.push({ g: g, t: t, e: e });
    }
    w.appendChild(lab); w.appendChild(box);
    p.stage.appendChild(w);
    var arr = buildArray(w);
    arr.lab.hidden = true;
    return { kind: "heap", w: w, lab: lab, box: box, nodes: nodes, arr: arr };
  }

  function drawHeap(p, v, a, hot) {
    v.w.hidden = !a;
    if (!a) return;
    v.lab.textContent = a.label;
    drawArray(p, v.arr, a, hot, "cells");
    v.arr.w.hidden = false;
    var words = [];
    v.nodes.forEach(function (x, i) {
      var on = i < a.values.length;
      x.g.style.display = on ? "" : "none";
      if (x.e) x.e.style.display = on ? "" : "none";
      if (!on) return;
      var s = hot[i] || a.marks[i] || "";
      paint(x.g, s);
      x.t.textContent = fmt(a.values[i]);
      words.push(say(fmt(a.values[i]), s));
    });
    v.box.setAttribute("aria-label", a.label + " as a tree, level by level: " + (words.length ? words.join(", ") : "empty"));
  }

  var NS = "http://www.w3.org/2000/svg", uid = 0;
  function svg(tag, attrs, parent) {
    var e = d.createElementNS(NS, tag);
    Object.keys(attrs || {}).forEach(function (k) { e.setAttribute(k, attrs[k]); });
    if (parent) parent.appendChild(e);
    return e;
  }
  function kindOf(fs, n) {
    for (var i = 0; i < fs.length; i++) {
      if (fs[i].graphs[n]) return "graph";
      if (fs[i].trees[n]) return "tree";
      if (fs[i].tables[n]) return "table";
      if (fs[i].arrays[n]) return fs[i].arrays[n].view === "heap" ? "heap" : "array";
    }
    return "array";
  }
  function paint(e, s) {
    if (s) { e.dataset.st = s; e.style.setProperty("--c", "var(--st-" + s + ")"); }
    else { delete e.dataset.st; e.style.removeProperty("--c"); }
  }
  // say gives the state of a node in words, for the label that a screen reader reads.
  function say(text, s, tag) {
    return text + (s ? " " + (NAMES[s] || s) : "") + (tag != null && tag !== "" ? " (" + fmt(tag) + ")" : "");
  }

  var R = 14;
  function buildGraph(p, n) {
    var a = p.all[n], lay = graphLayout(a), id = "ar" + (++uid);
    var w = el("div", "gv"), lab = el("div", "arr-label");
    var box = svg("svg", { viewBox: "0 0 " + lay.w + " " + lay.h, role: "img", "class": "gv-svg" });
    var defs = svg("defs", {}, box), m = svg("marker", { id: id, viewBox: "0 0 10 10", refX: "9", refY: "5", markerWidth: "7", markerHeight: "7", orient: "auto-start-reverse" }, defs);
    svg("path", { d: "M0,0 L10,5 L0,10 z", "class": "gv-head" }, m);
    var eg = svg("g", { "class": "gv-edges" }, box), ng = svg("g", { "class": "gv-nodes" }, box);
    var edges = {}, nodes = {};
    Object.keys(a.edges).forEach(function (k) {
      var x = a.edges[k], P = lay.pos[x.u], Q = lay.pos[x.w];
      var dx = Q.x - P.x, dy = Q.y - P.y, len = Math.sqrt(dx * dx + dy * dy) || 1, ux = dx / len, uy = dy / len;
      // Two directed edges in opposite directions are drawn side by side.
      var off = a.directed && a.edges[x.w + ">" + x.u] ? 4 : 0, ox = -uy * off, oy = ux * off;
      var g = svg("g", { "class": "gv-edge" }, eg);
      var line = svg("line", { x1: P.x + ux * R + ox, y1: P.y + uy * R + oy, x2: Q.x - ux * (R + (a.directed ? 2 : 0)) + ox, y2: Q.y - uy * (R + (a.directed ? 2 : 0)) + oy }, g);
      if (a.directed) line.setAttribute("marker-end", "url(#" + id + ")");
      var t = svg("text", { x: (P.x + Q.x) / 2 - uy * 9 + ox, y: (P.y + Q.y) / 2 + ux * 9 + oy + 4, "class": "gv-w" }, g);
      edges[k] = { g: g, t: t };
    });
    a.keys.forEach(function (k) {
      var P = lay.pos[k], g = svg("g", { "class": "gv-node", transform: "translate(" + P.x.toFixed(1) + " " + P.y.toFixed(1) + ")" }, ng);
      svg("circle", { r: R }, g);
      var t = svg("text", { y: 4, "class": "gv-id" }, g), tag = svg("text", { "class": "gv-tag" }, g), dirs = [];
      Object.keys(a.edges).forEach(function (e) {
        var x = a.edges[e], o = x.u === k ? x.w : x.w === k ? x.u : null;
        if (o !== null && o !== k) dirs.push(Math.atan2(lay.pos[o].y - P.y, lay.pos[o].x - P.x));
      });
      placeTag(tag, dirs, true);
      nodes[k] = { g: g, t: t, tag: tag };
    });
    w.appendChild(lab); w.appendChild(box);
    var fr = null;
    if (a.frontier) { fr = el("div", "gv-frontier"); w.appendChild(fr); }
    p.stage.appendChild(w);
    return { kind: "graph", w: w, lab: lab, box: box, edges: edges, nodes: nodes, fr: fr };
  }

  function drawGraph(v, g, hot) {
    v.w.hidden = !g;
    if (!g) return;
    hot = hot || { n: {}, e: {} };
    v.lab.textContent = g.label;
    var words = [];
    Object.keys(v.edges).forEach(function (k) {
      var x = v.edges[k], e = g.edges[k];
      x.g.style.display = e ? "" : "none";
      if (!e) return;
      paint(x.g, hot.e[k] || e.mark || "");
      x.t.textContent = e.weight === null || e.weight === undefined ? "" : fmt(e.weight);
    });
    Object.keys(v.nodes).forEach(function (k) {
      var x = v.nodes[k], o = g.nodes[k];
      x.g.style.display = o ? "" : "none";
      if (!o) return;
      var s = hot.n[k] || o.mark || "";
      paint(x.g, s);
      x.t.textContent = o.text;
      x.tag.textContent = o.tag === undefined || o.tag === null ? "" : fmt(o.tag);
      words.push(say(o.text, s, o.tag));
    });
    v.box.setAttribute("aria-label", g.label + ": " + words.join(", "));
    if (v.fr) {
      var f = g.frontier;
      v.fr.textContent = "";
      v.fr.appendChild(el("span", "arr-label", (f ? f.label : "frontier") + ":"));
      if (!f || !f.ids.length) v.fr.appendChild(el("span", "gv-empty", "empty"));
      else f.ids.forEach(function (u) { v.fr.appendChild(el("span", "gv-chip", g.nodes[idKey(u)] ? g.nodes[idKey(u)].text : fmt(u))); });
    }
  }

  // The places for a node tag, as angles in the SVG frame where y goes down: the four diagonals first, then above and below, then the sides.
  var SPOTS = [-Math.PI / 4, -3 * Math.PI / 4, Math.PI / 4, 3 * Math.PI / 4, -Math.PI / 2, Math.PI / 2, 0, Math.PI];
  // placeTag puts the tag of a node at the place that is farthest from the edges of the node. dirs holds the angle of each edge from the node. A tree does not use the sides, because a sibling is near.
  function placeTag(tag, dirs, sides) {
    var best = SPOTS[0], far = -1;
    SPOTS.slice(0, sides ? 8 : 6).forEach(function (a) {
      var near = Math.PI;
      dirs.forEach(function (b) { var d = Math.abs(a - b) % (2 * Math.PI); near = Math.min(near, d > Math.PI ? 2 * Math.PI - d : d); });
      if (near > far + 0.01) { far = near; best = a; }
    });
    var c = Math.cos(best), r = R + 4;
    tag.setAttribute("x", (c * r).toFixed(1));
    tag.setAttribute("y", (Math.sin(best) * r + 4 + Math.sin(best) * 3).toFixed(1));
    tag.style.textAnchor = c > 0.3 ? "start" : c < -0.3 ? "end" : "middle";
  }

  var COL = 38, ROW = 54;
  // buildTree makes a box with the size of the largest frame, so the stage keeps its height at each step.
  function buildTree(p, n) {
    var cols = 1, rows = 1;
    p.f.forEach(function (fr) { if (fr.trees[n]) { var l = treeLayout(fr.trees[n]); cols = Math.max(cols, l.cols); rows = Math.max(rows, l.rows); } });
    var W = Math.max(320, cols * COL + 12), H = rows * ROW + 12;
    var w = el("div", "gv tv"), lab = el("div", "arr-label");
    var box = svg("svg", { viewBox: "0 0 " + W + " " + H, role: "img", "class": "gv-svg" });
    box.style.maxWidth = Math.max(32, W / 10) + "rem";
    var eg = svg("g", { "class": "gv-edges" }, box), ng = svg("g", { "class": "gv-nodes" }, box);
    w.appendChild(lab); w.appendChild(box);
    p.stage.appendChild(w);
    return { kind: "tree", w: w, lab: lab, box: box, eg: eg, ng: ng, W: W, cols: cols, edges: {}, nodes: {} };
  }

  function drawTree(v, t, hot) {
    v.w.hidden = !t;
    if (!t) return;
    hot = hot || { n: {}, e: {} };
    v.lab.textContent = t.label;
    var l = treeLayout(t), x0 = (v.W - l.cols * COL) / 2 + COL / 2, words = [];
    function at(k) { var q = l.pos[k]; return { x: x0 + q.col * COL, y: ROW / 2 + 4 + q.row * ROW }; }
    Object.keys(v.nodes).forEach(function (k) { if (!t.nodes[k]) { v.nodes[k].g.style.display = "none"; v.edges[k].style.display = "none"; } });
    t.keys.forEach(function (k) {
      var o = t.nodes[k], x = v.nodes[k];
      if (!x) {
        x = v.nodes[k] = { g: svg("g", { "class": "gv-node" }, v.ng) };
        svg("circle", { r: R }, x.g);
        x.t = svg("text", { y: 4, "class": "gv-id" }, x.g);
        x.tag = svg("text", { "class": "gv-tag" }, x.g);
        v.edges[k] = svg("line", { "class": "gv-edge" }, v.eg);
      }
      var P = at(k), s = hot.n[k] || o.mark || "";
      x.g.style.display = "";
      x.g.setAttribute("transform", "translate(" + P.x.toFixed(1) + " " + P.y.toFixed(1) + ")");
      paint(x.g, s);
      x.t.textContent = fmt(o.value);
      x.tag.textContent = o.tag === undefined || o.tag === null ? "" : fmt(o.tag);
      if (x.tag.textContent) {
        var dirs = [];
        if (o.parent !== null && t.nodes[o.parent]) { var A = at(o.parent); dirs.push(Math.atan2(A.y - P.y, A.x - P.x)); }
        t.keys.forEach(function (c) { if (t.nodes[c].parent === k) { var C = at(c); dirs.push(Math.atan2(C.y - P.y, C.x - P.x)); } });
        placeTag(x.tag, dirs, false);
      }
      var e = v.edges[k];
      if (o.parent === null || !t.nodes[o.parent]) { e.style.display = "none"; }
      else {
        var Q = at(o.parent), dx = P.x - Q.x, dy = P.y - Q.y, len = Math.sqrt(dx * dx + dy * dy) || 1;
        e.style.display = "";
        e.setAttribute("x1", (Q.x + dx / len * R).toFixed(1)); e.setAttribute("y1", (Q.y + dy / len * R).toFixed(1));
        e.setAttribute("x2", (P.x - dx / len * R).toFixed(1)); e.setAttribute("y2", (P.y - dy / len * R).toFixed(1));
        paint(e, o.edge || "");
      }
      words.push(say(fmt(o.value), s, o.tag) + (o.parent === null ? ", a root" : ", under " + fmt(t.nodes[o.parent] ? t.nodes[o.parent].value : "")));
    });
    v.box.setAttribute("aria-label", t.label + ": " + (words.length ? words.join("; ") : "empty"));
  }

  Player.prototype.vars = function (fr) {
    var self = this;
    function dl(box, obj) {
      box.textContent = "";
      Object.keys(obj).forEach(function (k) {
        var w = el("div"); w.appendChild(el("dt", null, k)); w.appendChild(el("dd", null, fmt(obj[k])));
        box.appendChild(w);
      });
    }
    dl(this.varsEl, fr.vars);
    dl(this.countEl, fr.counters);
    void self;
  };

  Player.prototype.toggle = function () { if (this.timer) this.stop(); else this.play(); };
  Player.prototype.play = function () {
    var self = this;
    if (this.k >= this.f.length - 1) this.go(0);
    this.playBtn.textContent = "❚❚"; this.playBtn.setAttribute("aria-label", "Pause");
    var tick = function () {
      if (self.k >= self.f.length - 1) { self.stop(); return; }
      self.go(self.k + 1);
      self.timer = setTimeout(tick, 700 / (+self.speed.value || 1));
    };
    this.timer = setTimeout(tick, 350 / (+self.speed.value || 1));
  };
  Player.prototype.stop = function () {
    clearTimeout(this.timer); this.timer = null;
    if (this.playBtn) { this.playBtn.textContent = "▶"; this.playBtn.setAttribute("aria-label", "Play"); }
  };

  function start() { d.querySelectorAll(".player[data-scene]").forEach(function (p) { new Player(p); }); }
  if (d.readyState === "loading") d.addEventListener("DOMContentLoaded", start); else start();
})();
