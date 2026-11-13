// SPDX-License-Identifier: MIT
// @aik/viz core: the trace protocol of spec 10.2. A scene records steps and events here. The player of the site draws them.

export type Value = number | string | boolean | null | Value[] | { [k: string]: Value };
export type Event = [string, ...unknown[]];

export type State =
  | "default" | "active" | "compare" | "swap" | "pivot"
  | "frontier" | "visited" | "path" | "rejected" | "invariant";

export interface Step {
  line: string;
  note?: string;
  phase?: string;
  vars?: Record<string, unknown>;
  ev: Event[];
}

export interface Trace {
  v: 1;
  scene: string;
  input: unknown;
  steps: Step[];
  result?: unknown;
  counters: Record<string, number>;
}

export interface InputSpec {
  type: string;
  length?: [number, number];
  values?: [number, number];
  [k: string]: unknown;
}

export interface Scene<I = any, R = any> {
  slug: string;
  input: InputSpec;
  run(v: Viz, input: I): R;
}

export function scene<I, R>(s: Scene<I, R>): Scene<I, R> {
  return s;
}

// mulberry32, seeded from the input, so that a scene is pure (rule V01).
function rng(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function hashString(s: string): number {
  let h = 2166136261;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return h >>> 0;
}

export type Range = number | [number, number];

export class ArrayView {
  constructor(protected v: Viz, readonly name: string, protected data: Value[]) {}
  get length(): number { return this.data.length; }
  get(i: number): Value {
    this.v.emit(["array.get", this.name, i]);
    return this.data[i];
  }
  // peek reads a value with no event, for the logic of the scene.
  peek(i: number): Value { return this.data[i]; }
  set(i: number, value: Value, opts?: { from?: [string, number] }): void {
    this.data[i] = value;
    this.v.count("write");
    this.v.emit(["array.set", this.name, i, value, opts?.from ?? null]);
  }
  swap(i: number, j: number): void {
    const t = this.data[i];
    this.data[i] = this.data[j];
    this.data[j] = t;
    this.v.count("swap");
    this.v.emit(["array.swap", this.name, i, j]);
  }
  compare(i: number, j: number): number {
    this.v.count("compare");
    this.v.emit(["array.compare", this.name, i, j]);
    const a = this.data[i] as number, b = this.data[j] as number;
    return a < b ? -1 : a > b ? 1 : 0;
  }
  // compareWith compares the cell i with a value that is not in the array, such as a key.
  compareWith(i: number, x: Value): number {
    this.v.count("compare");
    this.v.emit(["array.compare", this.name, i, -1]);
    const a = this.data[i] as number, b = x as number;
    return a < b ? -1 : a > b ? 1 : 0;
  }
  mark(range: Range, state: State): void { this.v.emit(["array.mark", this.name, range, state]); }
  unmark(range: Range): void { this.v.emit(["array.unmark", this.name, range]); }
  ptr(name: string, i: number | null): void { this.v.emit(["array.ptr", this.name, name, i]); }
  bracket(lo: number | null, mid?: number | null, hi?: number | null): void {
    this.v.emit(["array.bracket", this.name, lo, mid ?? null, hi ?? null]);
  }
  label(text: string): void { this.v.emit(["array.label", this.name, text]); }
  values(): Value[] { return this.data.slice(); }
}

// StringView is a string as a row of cells (spec 10.2.4). All the strings of a scene use one column grid, so a pattern that is shifted under a text stays aligned with it.
export class StringView extends ArrayView {
  private offset = 0;
  // text gives the string.
  text(): string { return this.data.join(""); }
  // window shows the cells from lo to hi as the window that a pattern covers. hi can be past the end, when the pattern sticks out. Give null to hide it.
  window(lo: number | null, hi?: number): void {
    if (lo !== null && (!Number.isInteger(lo) || lo < 0 || lo >= this.data.length || (hi ?? lo) < lo)) throw new Error(`the string ${this.name} has no window ${lo}..${hi}`);
    this.v.emit(["array.window", this.name, lo, lo === null ? null : (hi ?? lo)]);
  }
  // shift draws the string k columns to the right, for a pattern under a text.
  shift(k: number): void {
    if (!Number.isInteger(k) || k < 0) throw new Error(`the string ${this.name} cannot shift by ${k}`);
    this.offset = k;
    this.v.emit(["array.shift", this.name, k]);
  }
  get shifted(): number { return this.offset; }
  // match compares the character i of this string with the character j of other. It counts one compare.
  match(i: number, other: StringView, j: number): boolean {
    if (i < 0 || i >= this.data.length || j < 0 || j >= other.data.length) throw new Error(`the match ${this.name}[${i}] = ${other.name}[${j}] is out of range`);
    const eq = this.data[i] === other.data[j];
    this.v.count("compare");
    this.v.emit(["array.match", this.name, i, other.name, j, eq]);
    return eq;
  }
}

export type Cell = [number, number];

export interface TableInit {
  rowHeads?: Value[];
  colHeads?: Value[];
  values?: Value[][];
  label?: string;
}

// TableView is a matrix with row and column headers (spec 10.2.4), for dynamic programming. fill records the cells that a value comes from, and the player draws an arrow to the first of them.
export class TableView {
  private cells: Value[][];
  constructor(private v: Viz, readonly name: string, readonly rows: number, readonly cols: number, init?: Value[][]) {
    this.cells = Array.from({ length: rows }, (_, r) => Array.from({ length: cols }, (_, c) => init?.[r]?.[c] ?? null));
  }
  private need(r: number, c: number): void {
    if (!Number.isInteger(r) || !Number.isInteger(c) || r < 0 || c < 0 || r >= this.rows || c >= this.cols) throw new Error(`the table ${this.name} has no cell [${r}, ${c}]`);
  }
  get(r: number, c: number): Value {
    this.need(r, c);
    this.v.emit(["table.get", this.name, r, c]);
    return this.cells[r][c];
  }
  peek(r: number, c: number): Value { this.need(r, c); return this.cells[r][c]; }
  fill(r: number, c: number, value: Value, opts?: { from?: Cell[] }): void {
    this.need(r, c);
    for (const [fr, fc] of opts?.from ?? []) this.need(fr, fc);
    this.cells[r][c] = value;
    this.v.count("write");
    this.v.emit(["table.fill", this.name, r, c, value, (opts?.from ?? []).map((x) => [x[0], x[1]])]);
  }
  mark(cell: Cell, state: State): void { this.need(cell[0], cell[1]); this.v.emit(["table.mark", this.name, [cell[0], cell[1]], state]); }
  unmark(cell: Cell): void { this.need(cell[0], cell[1]); this.v.emit(["table.unmark", this.name, [cell[0], cell[1]]]); }
  label(text: string): void { this.v.emit(["table.label", this.name, text]); }
}

// HeapView is a binary heap that the player draws as an array and as a tree at the same time (spec 10.2.4). The children of the cell i are the cells 2i+1 and 2i+2, and a mark on a cell is also a mark on its node.
export class HeapView {
  private data: Value[];
  constructor(private v: Viz, readonly name: string, values: Value[]) { this.data = values.slice(); }
  get length(): number { return this.data.length; }
  private need(i: number): void {
    if (!Number.isInteger(i) || i < 0 || i >= this.data.length) throw new Error(`the heap ${this.name} has no cell ${i}`);
  }
  parent(i: number): number { return (i - 1) >> 1; }
  left(i: number): number { return 2 * i + 1; }
  right(i: number): number { return 2 * i + 2; }
  get(i: number): Value { this.need(i); this.v.emit(["heap.get", this.name, i]); return this.data[i]; }
  peek(i: number): Value { this.need(i); return this.data[i]; }
  set(i: number, value: Value): void {
    this.need(i);
    this.data[i] = value;
    this.v.count("write");
    this.v.emit(["heap.set", this.name, i, value, null]);
  }
  swap(i: number, j: number): void {
    this.need(i); this.need(j);
    const t = this.data[i];
    this.data[i] = this.data[j];
    this.data[j] = t;
    this.v.count("swap");
    this.v.emit(["heap.swap", this.name, i, j]);
  }
  compare(i: number, j: number): number {
    this.need(i); this.need(j);
    this.v.count("compare");
    this.v.emit(["heap.compare", this.name, i, j]);
    const a = this.data[i] as number, b = this.data[j] as number;
    return a < b ? -1 : a > b ? 1 : 0;
  }
  // push adds a value at the end, as a new leaf.
  push(value: Value): number {
    this.data.push(value);
    this.v.emit(["heap.push", this.name, value]);
    return this.data.length - 1;
  }
  // pop removes the last cell and gives its value.
  pop(): Value {
    if (!this.data.length) throw new Error(`the heap ${this.name} is empty`);
    this.v.emit(["heap.pop", this.name]);
    return this.data.pop() as Value;
  }
  mark(range: Range, state: State): void { this.v.emit(["heap.mark", this.name, range, state]); }
  unmark(range: Range): void { this.v.emit(["heap.unmark", this.name, range]); }
  ptr(name: string, i: number | null): void { this.v.emit(["heap.ptr", this.name, name, i]); }
  label(text: string): void { this.v.emit(["heap.label", this.name, text]); }
  values(): Value[] { return this.data.slice(); }
}

export type Id = number | string;

export interface GraphNode { id: Id; label?: string; x?: number; y?: number }
export type GraphEdge = [Id, Id] | [Id, Id, number];

export interface GraphInit {
  nodes: (Id | GraphNode)[];
  edges?: GraphEdge[];
  directed?: boolean;
  weighted?: boolean;
  // fixed uses the x and y of each node, from 0 to 1. layered puts the nodes in the levels of a breadth-first search from the first node. circle is the default.
  layout?: "fixed" | "layered" | "circle";
  label?: string;
}

function key(u: Id, w: Id, directed: boolean): string {
  const a = JSON.stringify(u), b = JSON.stringify(w);
  return directed || a <= b ? a + ">" + b : b + ">" + a;
}

// GraphView is a graph of nodes and edges (spec 10.2.4). The player computes the place of each node once, from all the nodes that the trace adds, so a node never moves.
export class GraphView {
  private nodes = new Map<string, GraphNode>();
  private edges = new Map<string, number | null>();
  private adj = new Map<string, Id[]>();
  constructor(private v: Viz, readonly name: string, readonly directed: boolean) {}
  private need(u: Id): void {
    if (!this.nodes.has(JSON.stringify(u))) throw new Error(`the graph ${this.name} has no node ${JSON.stringify(u)}`);
  }
  private needEdge(u: Id, w: Id): string {
    const k = key(u, w, this.directed);
    if (!this.edges.has(k)) throw new Error(`the graph ${this.name} has no edge ${JSON.stringify(u)}-${JSON.stringify(w)}`);
    return k;
  }
  // init adds the nodes and edges of the create event, with no events of their own.
  init(g: GraphInit): void {
    for (const n of g.nodes) this.put(typeof n === "object" ? n : { id: n });
    for (const e of g.edges ?? []) this.link(e[0], e[1], e.length > 2 ? (e[2] as number) : null);
  }
  private put(n: GraphNode): void {
    const k = JSON.stringify(n.id);
    if (this.nodes.has(k)) throw new Error(`the graph ${this.name} has the node ${k} twice`);
    this.nodes.set(k, n);
    this.adj.set(k, []);
  }
  private link(u: Id, w: Id, weight: number | null): void {
    this.need(u); this.need(w);
    const k = key(u, w, this.directed);
    if (this.edges.has(k)) throw new Error(`the graph ${this.name} has the edge ${JSON.stringify(u)}-${JSON.stringify(w)} twice`);
    this.edges.set(k, weight);
    this.adj.get(JSON.stringify(u))!.push(w);
    if (!this.directed) this.adj.get(JSON.stringify(w))!.push(u);
  }
  get size(): number { return this.nodes.size; }
  ids(): Id[] { return [...this.nodes.values()].map((n) => n.id); }
  // neighbors gives the nodes that an edge from u reaches, in the order of the edges. It is a read with no event; use edge to show it.
  neighbors(u: Id): Id[] { this.need(u); return this.adj.get(JSON.stringify(u))!.slice(); }
  weight(u: Id, w: Id): number | null { return this.edges.get(this.needEdge(u, w)) ?? null; }
  addNode(n: Id | GraphNode): void {
    const node = typeof n === "object" ? n : { id: n };
    this.put(node);
    this.v.emit(["graph.addNode", this.name, node]);
  }
  addEdge(u: Id, w: Id, weight?: number): void {
    this.link(u, w, weight ?? null);
    this.v.emit(["graph.addEdge", this.name, u, w, weight ?? null]);
  }
  removeEdge(u: Id, w: Id): void {
    const k = this.needEdge(u, w);
    this.edges.delete(k);
    const drop = (a: Id, b: Id) => {
      const l = this.adj.get(JSON.stringify(a))!;
      const i = l.findIndex((x) => JSON.stringify(x) === JSON.stringify(b));
      if (i >= 0) l.splice(i, 1);
    };
    drop(u, w);
    if (!this.directed) drop(w, u);
    this.v.emit(["graph.removeEdge", this.name, u, w]);
  }
  // visit reads the node u. It adds 1 to the counter visit.
  visit(u: Id): void {
    this.need(u);
    this.v.count("visit");
    this.v.emit(["graph.visit", this.name, u]);
  }
  // edge shows the edge u-w in a state, or reads it when the state is not given. A read adds 1 to the counter edge.
  edge(u: Id, w: Id, state?: State): void {
    this.needEdge(u, w);
    if (state === undefined) this.v.count("edge");
    this.v.emit(["graph.edge", this.name, u, w, state ?? null]);
  }
  // relax shows the relaxation of the edge u-w: the label of w changes from before to after when after is smaller. It adds 1 to the counter relax.
  relax(u: Id, w: Id, before: number, after: number): boolean {
    this.needEdge(u, w);
    this.v.count("relax");
    const better = after < before;
    this.v.emit(["graph.relax", this.name, u, w, num(before), num(after), better]);
    return better;
  }
  nodeLabel(u: Id, text: string | number | null): void {
    this.need(u);
    this.v.emit(["graph.nodeLabel", this.name, u, text === null ? null : typeof text === "number" ? num(text) : text]);
  }
  mark(u: Id, state: State): void { this.need(u); this.v.emit(["graph.mark", this.name, u, state]); }
  unmark(u: Id): void { this.need(u); this.v.emit(["graph.unmark", this.name, u]); }
  // path marks the nodes and the edges of a path as path.
  path(nodes: Id[]): void {
    nodes.forEach((u, i) => { this.need(u); if (i > 0) this.needEdge(nodes[i - 1], u); });
    this.v.emit(["graph.path", this.name, nodes.slice()]);
  }
  // frontier shows the nodes that wait, such as the queue of a breadth-first search, in order. An empty list hides it.
  frontier(nodes: Id[], label = "frontier"): void {
    nodes.forEach((u) => this.need(u));
    this.v.emit(["graph.frontier", this.name, nodes.slice(), label]);
  }
  label(text: string): void { this.v.emit(["graph.label", this.name, text]); }
}

// num keeps Infinity in the JSON of a trace, as the text "∞".
function num(x: number): number | string {
  return Number.isFinite(x) ? x : x > 0 ? "∞" : "-∞";
}

export interface TreeNode { id: Id; value?: Value; parent?: Id | null; side?: "left" | "right" }

export interface TreeInit {
  nodes?: TreeNode[];
  // binary keeps a left and a right place for the children of each node, as in a search tree or a heap.
  binary?: boolean;
  label?: string;
}

// TreeView is a rooted tree, or a forest of rooted trees (spec 10.2.4). The player draws it with a tidy layout in a box that has the same size for each step.
export class TreeView {
  private parent = new Map<string, Id | null>();
  private side = new Map<string, "left" | "right" | null>();
  private ids = new Map<string, Id>();
  constructor(private v: Viz, readonly name: string, readonly binary: boolean) {}
  private need(u: Id): string {
    const k = JSON.stringify(u);
    if (!this.ids.has(k)) throw new Error(`the tree ${this.name} has no node ${k}`);
    return k;
  }
  init(t: TreeInit): void {
    for (const n of t.nodes ?? []) this.attach(n.parent ?? null, n.id, n.side ?? null, true);
  }
  // attach puts child under parent. A child that is in the tree already moves there with its subtree.
  private attach(parent: Id | null, child: Id, side: "left" | "right" | null, fresh: boolean): void {
    const ck = JSON.stringify(child);
    if (fresh && this.ids.has(ck)) throw new Error(`the tree ${this.name} has the node ${ck} twice`);
    if (parent !== null) {
      this.need(parent);
      for (let p: Id | null = parent; p !== null; p = this.parent.get(JSON.stringify(p)) ?? null) {
        if (JSON.stringify(p) === ck) throw new Error(`the tree ${this.name}: ${ck} cannot go under its own subtree`);
      }
      if (this.binary) {
        if (side === null) throw new Error(`the tree ${this.name} is binary, so insert needs the side left or right`);
        for (const [k, p] of this.parent) {
          if (k !== ck && p !== null && JSON.stringify(p) === JSON.stringify(parent) && this.side.get(k) === side) {
            throw new Error(`the tree ${this.name}: the node ${JSON.stringify(parent)} has a ${side} child already`);
          }
        }
      }
    }
    this.ids.set(ck, child);
    this.parent.set(ck, parent);
    this.side.set(ck, parent === null ? null : side);
  }
  has(u: Id): boolean { return this.ids.has(JSON.stringify(u)); }
  parentOf(u: Id): Id | null { return this.parent.get(this.need(u)) ?? null; }
  children(u: Id): Id[] {
    const k = this.need(u), out: Id[] = [];
    for (const [c, p] of this.parent) if (p !== null && JSON.stringify(p) === k) out.push(this.ids.get(c)!);
    return out;
  }
  child(u: Id, side: "left" | "right"): Id | null {
    const k = this.need(u);
    for (const [c, p] of this.parent) if (p !== null && JSON.stringify(p) === k && this.side.get(c) === side) return this.ids.get(c)!;
    return null;
  }
  // insert adds child under parent, or moves it there with its subtree when it is in the tree. A null parent makes the child a root.
  insert(parent: Id | null, child: Id, side?: "left" | "right", value?: Value): void {
    const fresh = !this.has(child);
    this.attach(parent, child, side ?? null, false);
    this.v.emit(["tree.insert", this.name, parent, child, side ?? null, fresh ? (value ?? null) : null, fresh]);
  }
  // remove takes out the node. Its children go to its parent, in its place.
  remove(u: Id): void {
    const k = this.need(u), p = this.parent.get(k) ?? null, s = this.side.get(k) ?? null;
    const kids = this.children(u);
    if (this.binary && kids.length > 1) throw new Error(`the tree ${this.name}: remove a node with two children after a replace`);
    for (const c of kids) {
      this.parent.set(JSON.stringify(c), p);
      this.side.set(JSON.stringify(c), p === null ? null : s);
    }
    this.parent.delete(k); this.side.delete(k); this.ids.delete(k);
    this.v.emit(["tree.remove", this.name, u]);
  }
  // rotate turns the edge between u and its child: "left" lifts the right child of u, and "right" lifts the left child.
  rotate(u: Id, dir: "left" | "right"): void {
    if (!this.binary) throw new Error(`the tree ${this.name} is not binary, so it cannot rotate`);
    const up = dir === "left" ? "right" : "left";
    const x = this.child(u, up);
    if (x === null) throw new Error(`the tree ${this.name}: ${JSON.stringify(u)} has no ${up} child to rotate`);
    const uk = JSON.stringify(u), xk = JSON.stringify(x);
    const p = this.parent.get(uk) ?? null, ps = this.side.get(uk) ?? null;
    const inner = this.child(x, dir);
    this.parent.set(xk, p); this.side.set(xk, ps);
    this.parent.set(uk, x); this.side.set(uk, dir);
    if (inner !== null) { this.parent.set(JSON.stringify(inner), u); this.side.set(JSON.stringify(inner), up); }
    this.v.count("rotate");
    this.v.emit(["tree.rotate", this.name, u, dir]);
  }
  replace(u: Id, value: Value): void { this.need(u); this.v.emit(["tree.replace", this.name, u, value]); }
  // visit reads the node u. It adds 1 to the counter visit.
  visit(u: Id): void { this.need(u); this.v.count("visit"); this.v.emit(["tree.visit", this.name, u]); }
  nodeLabel(u: Id, text: string | number | null): void { this.need(u); this.v.emit(["tree.nodeLabel", this.name, u, text]); }
  mark(u: Id, state: State): void { this.need(u); this.v.emit(["tree.mark", this.name, u, state]); }
  unmark(u: Id): void { this.need(u); this.v.emit(["tree.unmark", this.name, u]); }
  // edge shows the edge from u to its parent in a state.
  edge(u: Id, state: State | null): void {
    if (this.parentOf(u) === null) throw new Error(`the tree ${this.name}: ${JSON.stringify(u)} is a root, so it has no edge`);
    this.v.emit(["tree.edge", this.name, u, state]);
  }
  label(text: string): void { this.v.emit(["tree.label", this.name, text]); }
}

export class Viz {
  steps: Step[] = [];
  counters: Record<string, number> = {};
  private pending: Event[] = [];
  private phaseName: string | undefined;
  result: unknown;
  readonly random: () => number;

  constructor(input: unknown) {
    this.random = rng(hashString(JSON.stringify(input) ?? ""));
  }

  private cur(): Step | undefined { return this.steps[this.steps.length - 1]; }

  emit(e: Event): void {
    const s = this.cur();
    if (s) s.ev.push(e); else this.pending.push(e);
  }

  // line starts a new step at the pseudocode line id.
  line(id: string, note?: string): void {
    const s: Step = { line: id, ev: this.pending };
    this.pending = [];
    if (note) s.note = note;
    if (this.phaseName) s.phase = this.phaseName;
    this.steps.push(s);
  }
  note(text: string): void {
    const s = this.cur();
    if (s) s.note = text;
  }
  phase(title: string): void { this.phaseName = title; }
  vars(o: Record<string, unknown>): void {
    const s = this.cur();
    if (s) s.vars = { ...(s.vars ?? {}), ...o };
  }
  count(name: string, delta = 1): void { this.counters[name] = (this.counters[name] ?? 0) + delta; }
  array(name: string, values: Value[], opts: { view?: "bars" | "cells"; label?: string } = {}): ArrayView {
    this.emit(["array.create", name, values.slice(), { view: opts.view ?? "bars", label: opts.label ?? null }]);
    return new ArrayView(this, name, values.slice());
  }
  graph(name: string, g: GraphInit): GraphView {
    const view = new GraphView(this, name, !!g.directed);
    view.init(g);
    const nodes = g.nodes.map((n) => (typeof n === "object" ? { ...n } : { id: n }));
    const layout = g.layout ?? (nodes.every((n) => n.x !== undefined && n.y !== undefined) ? "fixed" : "circle");
    this.emit(["graph.create", name, { nodes, edges: (g.edges ?? []).map((e) => e.slice()), directed: !!g.directed, weighted: !!g.weighted, layout, label: g.label ?? null }]);
    return view;
  }
  tree(name: string, t: TreeInit = {}): TreeView {
    const view = new TreeView(this, name, !!t.binary);
    view.init(t);
    this.emit(["tree.create", name, { nodes: (t.nodes ?? []).map((n) => ({ ...n })), binary: !!t.binary, label: t.label ?? null }]);
    return view;
  }
  string(name: string, text: string, opts: { label?: string } = {}): StringView {
    const chars = [...text];
    this.emit(["array.create", name, chars, { view: "str", label: opts.label ?? null }]);
    return new StringView(this, name, chars);
  }
  table(name: string, rows: number, cols: number, t: TableInit = {}): TableView {
    if (!Number.isInteger(rows) || !Number.isInteger(cols) || rows < 1 || cols < 1) throw new Error(`the table ${name} must have at least one row and one column`);
    if (t.rowHeads && t.rowHeads.length !== rows) throw new Error(`the table ${name} has ${rows} rows but ${t.rowHeads.length} row heads`);
    if (t.colHeads && t.colHeads.length !== cols) throw new Error(`the table ${name} has ${cols} columns but ${t.colHeads.length} column heads`);
    const view = new TableView(this, name, rows, cols, t.values);
    const values = Array.from({ length: rows }, (_, r) => Array.from({ length: cols }, (_, c) => view.peek(r, c)));
    this.emit(["table.create", name, { rows, cols, rowHeads: t.rowHeads ?? null, colHeads: t.colHeads ?? null, values, label: t.label ?? null }]);
    return view;
  }
  heap(name: string, values: Value[], opts: { label?: string } = {}): HeapView {
    this.emit(["heap.create", name, values.slice(), { view: "heap", label: opts.label ?? null }]);
    return new HeapView(this, name, values);
  }
  destroy(name: string): void { this.emit(["destroy", name]); }
}

// trace runs a scene on an input and returns the trace.
export function trace(s: Scene, input: unknown): Trace {
  const v = new Viz(input);
  const copy = JSON.parse(JSON.stringify(input));
  const result = s.run(v, copy);
  return { v: 1, scene: s.slug, input, steps: v.steps, result, counters: v.counters };
}
