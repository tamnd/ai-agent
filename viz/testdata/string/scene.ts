import { scene } from "@aik/viz";

interface Input {
  text: string;
  pattern: string;
}

// Knuth-Morris-Pratt: the failure function, then the search with the pattern under the text.
export default scene({
  slug: "string-demo",
  input: { type: "strings" },
  run(v, input: Input) {
    const T = v.string("T", input.text, { label: "text" });
    const P = v.string("P", input.pattern, { label: "pattern" });
    const m = P.length;
    const fail = v.array("fail", Array.from({ length: m }, () => 0), { view: "cells", label: "failure function" });
    v.line("L1", "Compute the failure function.");
    let k = 0;
    for (let q = 1; q < m; q++) {
      while (k > 0 && !P.match(k, P, q)) k = fail.peek(k - 1) as number;
      if (P.match(k, P, q)) k++;
      fail.set(q, k);
    }
    const found: number[] = [];
    let j = 0;
    for (let i = 0; i < T.length; i++) {
      v.line("L2");
      P.shift(i - j);
      T.window(i - j, i - j + m - 1);
      T.ptr("i", i);
      while (j > 0 && !T.match(i, P, j)) {
        j = fail.get(j - 1) as number;
        P.shift(i - j);
        T.window(i - j, i - j + m - 1);
      }
      if (T.match(i, P, j)) j++;
      if (j === m) {
        v.line("L3", `A match at ${i - m + 1}.`);
        T.mark([i - m + 1, i], "path");
        found.push(i - m + 1);
        j = fail.peek(j - 1) as number;
      }
    }
    return found;
  },
});
