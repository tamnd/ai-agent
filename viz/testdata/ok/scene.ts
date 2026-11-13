import { scene } from "@aik/viz";

export default scene({
  slug: "demo",
  input: { type: "list<int>", length: [0, 8] },
  run(v, a: number[]) {
    const arr = v.array("a", a);
    v.line("L1", "Start.");
    for (let i = 0; i < arr.length; i++) {
      v.line("L2");
      arr.mark(i, "visited");
    }
    return a.length;
  },
});
