// A reference that runs L2 before L1, so V06 fails.
export function walk(a: readonly number[]): number {
  let n = 0;
  for (const x of a) { // @line L2
    n += x;
  }
  return n; // @line L1
}
