import { scene } from "@aik/viz";

interface Input {
  n: number;
  edges: [number, number, number][];
}

// Dijkstra on a small directed graph, with a frontier and distance labels.
export default scene({
  slug: "graph-demo",
  input: { type: "graph" },
  run(v, input: Input) {
    const ids = Array.from({ length: input.n }, (_, i) => i);
    const g = v.graph("G", { nodes: ids, edges: input.edges, directed: true, weighted: true, layout: "layered" });
    const dist = ids.map(() => Infinity);
    const done = ids.map(() => false);
    dist[0] = 0;
    v.line("L1", "Start at node 0.");
    ids.forEach((u) => g.nodeLabel(u, dist[u]));
    g.frontier([0], "queue");
    for (;;) {
      let u = -1;
      for (const x of ids) if (!done[x] && dist[x] < Infinity && (u < 0 || dist[x] < dist[u])) u = x;
      if (u < 0) break;
      v.line("L2", `Take node ${u}.`);
      done[u] = true;
      g.visit(u);
      g.mark(u, "visited");
      for (const w of g.neighbors(u)) {
        v.line("L3");
        const nd = dist[u] + (g.weight(u, w) ?? 0);
        if (g.relax(u, w, dist[w], nd)) dist[w] = nd;
      }
      g.frontier(ids.filter((x) => !done[x] && dist[x] < Infinity), "queue");
    }
    return dist.map((d) => (Number.isFinite(d) ? d : null));
  },
});
