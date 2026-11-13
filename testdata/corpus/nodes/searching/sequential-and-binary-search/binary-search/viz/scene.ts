// SPDX-License-Identifier: MIT
// Part of https://ai.tamnd.com/binary-search/. The authors are listed in prov/2B7QDX.jsonl.
import { scene } from "@aik/viz";

interface Input {
  a: number[];
  x: number;
}

export default scene({
  slug: "binary-search",
  input: { type: "sorted list<int> and key", length: [1, 24], values: [-999, 999] },
  run(v, input: Input) {
    for (let i = 1; i < input.a.length; i++) {
      if (input.a[i - 1] > input.a[i]) throw new Error("The array must be sorted.");
    }
    const x = input.x;
    const a = v.array("A", input.a, { view: "cells", label: "A" });
    let lo = 0;
    let hi = a.length;
    v.line("L1", `Search for ${x}. At the start, the range is the full array.`);
    v.vars({ x, lo, hi });
    a.bracket(lo, null, hi - 1);
    a.ptr("lo", lo);
    a.ptr("hi", hi <= a.length - 1 ? hi : null);
    for (;;) {
      v.line("L2", lo < hi ? `The range has ${hi - lo} items, so the loop continues.` : "The range is empty, so the loop stops.");
      v.vars({ lo, hi });
      if (lo >= hi) break;
      const mid = lo + Math.floor((hi - lo) / 2);
      v.line("L3", `The middle of the range [${lo}, ${hi}) is index ${mid}.`);
      v.vars({ mid });
      a.bracket(lo, mid, hi - 1);
      a.ptr("mid", mid);
      v.line("L4");
      const c = a.compareWith(mid, x);
      if (c < 0) {
        v.note(`A[${mid}] = ${a.peek(mid)} is less than ${x}. So ${x} can only be to the right of index ${mid}.`);
        v.line("L5", `Remove the indices ${lo} to ${mid} from the range.`);
        a.mark([lo, mid], "rejected");
        lo = mid + 1;
        a.ptr("lo", lo < a.length ? lo : null);
        continue;
      }
      v.note(`A[${mid}] = ${a.peek(mid)} is not less than ${x}.`);
      v.line("L6");
      if (c > 0) {
        v.note(`A[${mid}] = ${a.peek(mid)} is larger than ${x}. So ${x} can only be to the left of index ${mid}.`);
        v.line("L7", `Remove the indices ${mid} to ${hi - 1} from the range.`);
        a.mark([mid, hi - 1], "rejected");
        hi = mid;
        a.ptr("hi", hi);
        continue;
      }
      v.note(`A[${mid}] = ${x}. The key is found.`);
      v.line("L8", `Return ${mid}.`);
      a.mark(mid, "path");
      a.ptr("lo", null);
      a.ptr("hi", null);
      return mid;
    }
    v.line("L9", `No item is equal to ${x}. Return NIL. The key can go at index ${lo}.`);
    a.bracket(null);
    a.ptr("mid", null);
    return null;
  },
});
