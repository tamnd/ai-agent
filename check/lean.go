// SPDX-License-Identifier: Apache-2.0

package check

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tamnd/ai-agent/formal"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

func leanFile(mod string) string {
	return "lean/" + strings.ReplaceAll(mod, ".", "/") + ".lean"
}

// leanNode runs the L rules of one node. F02 needs only the files. The others need the declarations of aik lean build.
func (r *run) leanNode(n *node.Node) {
	mod := n.Lean["module"]
	if mod == "" {
		return
	}
	p := n.Rel + "/node.md"
	file := leanFile(mod)
	if _, err := os.Stat(filepath.Join(r.root, filepath.FromSlash(file))); err != nil {
		r.add("F01", Block, p, 0, "lean.module is %s, but %s does not exist", mod, file)
		return
	}
	if n.Level == "core" {
		// The review of a theorem's statement is lean-statement (11.7), and the review of an algorithm's spec is lean-spec.
		part, what := "lean-spec", "spec"
		if status.Formal(n) {
			part, what = "lean-statement", "statement"
		}
		if ok, why := status.Reviewed(r.root, n, part, filepath.Join(r.root, filepath.FromSlash(file))); !ok {
			r.add("F02", Level, p, 0, "the Lean %s of a core node needs a fidelity review by a person: %s", what, why)
		}
	}
	if r.opt.Decls == nil {
		return
	}
	fs := formal.CheckNode(n, r.opt.Decls)
	fs = append(fs, formal.CheckClaims(n, r.opt.Decls)...)
	for _, f := range fs {
		r.add(f.Rule, Severity(f.Sev()), p, 0, "%s: %s", strings.TrimPrefix(f.Where, n.Slug+" "), f.Msg)
	}
}

// leanDecls runs the axiom rule on each declaration (F03).
func (r *run) leanDecls() {
	if r.opt.Decls == nil || len(r.opt.Only) > 0 {
		return
	}
	for _, f := range formal.Check(nil, r.opt.Decls) {
		d := r.opt.Decls[f.Decl]
		line := 0
		if d.Pos != nil {
			line = d.Pos.Line
		}
		r.add(f.Rule, Severity(f.Sev()), leanFile(d.Module), line, "%s %s", f.Decl, f.Msg)
	}
}
