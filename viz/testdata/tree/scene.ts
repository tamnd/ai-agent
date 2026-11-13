import { scene } from "@aik/viz";

interface Input {
  n: number;
  unions: [number, number][];
}

// A disjoint-set forest with union by rank and path compression.
export default scene({
  slug: "tree-demo",
  input: { type: "unions" },
  run(v, input: Input) {
    const parent = Array.from({ length: input.n }, (_, i) => i);
    const rank = parent.map(() => 0);
    const t = v.tree("F", { nodes: parent.map((i) => ({ id: i })), label: "forest" });
    v.line("L1", "Each item is its own set.");
    const find = (x: number): number => {
      t.visit(x);
      if (parent[x] === x) return x;
      const r = find(parent[x]);
      if (parent[x] !== r) { parent[x] = r; t.insert(r, x); }
      return r;
    };
    for (const [a, b] of input.unions) {
      v.line("L2", `Union ${a} and ${b}.`);
      let x = find(a), y = find(b);
      if (x === y) continue;
      if (rank[x] < rank[y]) [x, y] = [y, x];
      parent[y] = x;
      t.insert(x, y);
      t.edge(y, "path");
      if (rank[x] === rank[y]) { rank[x]++; t.nodeLabel(x, rank[x]); }
    }
    return parent;
  },
});
