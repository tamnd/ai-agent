// SPDX-License-Identifier: Apache-2.0

package shapes

// JSON gives a formula as plain data for the evaluator of the architecture scene (spec 12.6), which runs in the
// browser and in aik viz check. A number is a decimal string, so that no size loses digits in JavaScript; a name
// is {"r": name}; a negation is {"n": x}; an operation is {"o": "+", "a": left, "b": right}.
func JSON(e Expr) any {
	switch x := e.(type) {
	case num:
		return x.v.String()
	case ref:
		return map[string]any{"r": x.name}
	case neg:
		return map[string]any{"n": JSON(x.x)}
	case bin:
		return map[string]any{"o": string(x.op), "a": JSON(x.l), "b": JSON(x.r)}
	}
	return nil
}
