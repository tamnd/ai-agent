import { scene } from "@aik/viz";

interface Input {
  values: number[];
  pops: number;
}

// A binary min-heap: insert each value with a sift up, then take the minimum some times with a sift down.
export default scene({
  slug: "heap-demo",
  input: { type: "list" },
  run(v, input: Input) {
    const h = v.heap("H", [], { label: "min-heap" });
    for (const x of input.values) {
      v.line("L1", `Insert ${x}.`);
      let i = h.push(x);
      while (i > 0 && h.compare(i, h.parent(i)) < 0) {
        h.swap(i, h.parent(i));
        i = h.parent(i);
      }
    }
    const out: number[] = [];
    for (let k = 0; k < input.pops && h.length > 0; k++) {
      v.line("L2", "Take the minimum.");
      out.push(h.get(0) as number);
      h.swap(0, h.length - 1);
      h.pop();
      let i = 0;
      for (;;) {
        let c = h.left(i);
        if (c >= h.length) break;
        if (c + 1 < h.length && h.compare(c + 1, c) < 0) c++;
        if (h.compare(c, i) >= 0) break;
        h.swap(c, i);
        i = c;
      }
    }
    return out;
  },
});
