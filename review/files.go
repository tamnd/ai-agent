// SPDX-License-Identifier: Apache-2.0

package review

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/ai-agent/node"
)

// Files are the files that a review of the part of n hashes, as paths under root. The file that a ladder checks is
// one of them.
func Files(root string, n *node.Node, part string) ([]string, error) {
	dir := filepath.Join(root, filepath.FromSlash(n.Rel))
	var out []string
	add := func(rel string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	walk := func(rel string) error {
		top := filepath.Join(dir, filepath.FromSlash(rel))
		if _, err := os.Stat(top); errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return filepath.WalkDir(top, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == "__pycache__" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			if !d.IsDir() {
				out = append(out, p)
			}
			return nil
		})
	}
	switch {
	case part == "text", part == "positions":
		add("node.md")
		add("claims.yaml")
		for _, d := range n.Derivations {
			out = append(out, filepath.Join(root, filepath.FromSlash(d.Path)))
		}
	case part == "facts":
		add("facts.yaml")
	case part == "contract":
		add("contract.yaml")
	case strings.HasPrefix(part, "impl-"):
		if err := walk("impl/" + strings.TrimPrefix(part, "impl-")); err != nil {
			return nil, err
		}
	case strings.HasPrefix(part, "exp/"):
		if err := walk(part); err != nil {
			return nil, err
		}
	case part == "scene":
		if err := walk("viz"); err != nil {
			return nil, err
		}
	case part == "lean-spec", part == "lean-statement":
		mod := n.Lean["module"]
		if mod == "" {
			return nil, errors.New("the node names no Lean module")
		}
		out = append(out, filepath.Join(root, "lean", filepath.FromSlash(strings.ReplaceAll(mod, ".", "/"))+".lean"))
	case strings.HasPrefix(part, "proof-"):
		id := strings.TrimPrefix(part, "proof-")
		for _, d := range n.Derivations {
			if d.ID == id {
				out = append(out, filepath.Join(root, filepath.FromSlash(d.Path)))
			}
		}
		for _, p := range n.Proofs {
			if p.ID == id {
				out = append(out, filepath.Join(root, filepath.FromSlash(p.Path)))
			}
		}
	default:
		return nil, fmt.Errorf("no part %q: text, positions, facts, contract, impl-<framework>, exp/<slug>, scene, lean-spec, lean-statement or proof-<id>", part)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("the node has no files for the part %s", part)
	}
	sort.Strings(out)
	return out, nil
}

// ChecklistDir is the directory of the checklists of a part, and of a kind under it (15.7).
func ChecklistDir(part string) string {
	switch {
	case strings.HasPrefix(part, "impl-"):
		return "impl/" + strings.TrimPrefix(part, "impl-")
	case strings.HasPrefix(part, "exp/"):
		return "experiment"
	case strings.HasPrefix(part, "proof-"):
		return "proof"
	}
	return part
}

var versionRE = regexp.MustCompile(`^v([0-9]+)\.md$`)

// Latest is the newest checklist of the part for the kind of n, "text/method/v2", or of the part, "scene/v1"; it is
// empty when there is none.
func Latest(root string, n *node.Node, part string) string {
	base := ChecklistDir(part)
	for _, d := range []string{base + "/" + n.Kind, base} {
		es, err := os.ReadDir(filepath.Join(root, "checklists", filepath.FromSlash(d)))
		if err != nil {
			continue
		}
		best := 0
		for _, e := range es {
			if m := versionRE.FindStringSubmatch(e.Name()); m != nil {
				if v, _ := strconv.Atoi(m[1]); v > best {
					best = v
				}
			}
		}
		if best > 0 {
			return d + "/v" + strconv.Itoa(best)
		}
	}
	return ""
}

// Template is a record for the reviewer to fill in: the hashes of the current files, and one item for each question
// of the checklist, with the question as a comment.
func Template(root string, n *node.Node, part, reviewer, checklist, date string) (string, error) {
	files, err := Files(root, n, part)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "tag: %s\npart: %s\nhashes:\n", n.Tag, part)
	for _, f := range files {
		h, err := Hash(f)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "  %s: %s\n", strconv.Quote(Key(root, n, f)), h)
	}
	fmt.Fprintf(&b, "checklist: %s\nreviewer: %s\nverdict: changes  # pass when every item is pass or n/a\ndate: %s\ninterests: \"\"  # \"none\", or your employer or funder if it made what the page is about (15.5)\nitems:\n", checklist, strconv.Quote(reviewer), date)
	text, err := os.ReadFile(ChecklistFile(root, checklist))
	if err != nil {
		return "", fmt.Errorf("the checklist %s does not exist: write checklists/%s.md (15.7)", checklist, checklist)
	}
	for _, l := range strings.Split(string(text), "\n") {
		if m := questionRE.FindStringSubmatch(l); m != nil {
			fmt.Fprintf(&b, "  # %s\n  - {check: %q, result: \"\"}\n", strings.TrimSpace(l), m[1])
		}
	}
	return b.String(), nil
}
