// The TypeScript reference of the demo scene, for the V06 test.
export function walk(a: readonly number[]): number {
  let n = 0; // @line L1
  for (let i = 0; i < a.length; i++) { // @line L2
    n++;
  }
  return n;
}
