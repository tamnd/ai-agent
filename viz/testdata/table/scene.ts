import { scene } from "@aik/viz";

interface Input {
  a: string;
  b: string;
}

// The length of a longest common subsequence, then the walk back along the arrows.
export default scene({
  slug: "table-demo",
  input: { type: "strings" },
  run(v, input: Input) {
    const n = input.a.length, m = input.b.length;
    const L = v.table("L", n + 1, m + 1, { rowHeads: ["", ...input.a], colHeads: ["", ...input.b], label: "LCS lengths" });
    v.line("L1", "Row 0 and column 0 are 0.");
    for (let i = 0; i <= n; i++) L.fill(i, 0, 0);
    for (let j = 1; j <= m; j++) L.fill(0, j, 0);
    for (let i = 1; i <= n; i++) {
      for (let j = 1; j <= m; j++) {
        v.line("L2");
        if (input.a[i - 1] === input.b[j - 1]) L.fill(i, j, (L.get(i - 1, j - 1) as number) + 1, { from: [[i - 1, j - 1]] });
        else if ((L.get(i - 1, j) as number) >= (L.get(i, j - 1) as number)) L.fill(i, j, L.peek(i - 1, j), { from: [[i - 1, j], [i, j - 1]] });
        else L.fill(i, j, L.peek(i, j - 1), { from: [[i, j - 1], [i - 1, j]] });
      }
    }
    v.line("L3", "Walk back along the arrows.");
    let i = n, j = m, out = "";
    while (i > 0 && j > 0) {
      L.mark([i, j], "path");
      if (input.a[i - 1] === input.b[j - 1]) { out = input.a[i - 1] + out; i--; j--; }
      else if ((L.peek(i - 1, j) as number) >= (L.peek(i, j - 1) as number)) i--;
      else j--;
    }
    return out;
  },
});
