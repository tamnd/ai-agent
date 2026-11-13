// SPDX-License-Identifier: Apache-2.0

package check

import (
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/tamnd/ai-agent/mutate"
	"github.com/tamnd/ai-agent/node"
)

// packageSets are the modules that each pinned package set of spec 08.4 lets the code import, in addition to the standard library.
var packageSets = map[string][]string{
	"numpy-ref":   {"numpy"},
	"pytorch-ref": {"torch", "numpy"},
	"jax-ref":     {"jax", "numpy"},
}

// testPackages are the modules that the tests of a Python framework can also import (spec 08.4).
var testPackages = []string{"pytest", "hypothesis", "scipy"}

// labels are the labels of the versions of spec 08.5 that are not the reference. A labelled version has the label at the end of its file name.
var labels = []string{"efficient", "library", "historical"}

// MaxReference is the number of lines of a reference version over which rule X14 warns.
const MaxReference = 200

// implData are the files of an implementation that are not code.
var implData = map[string]bool{mutate.IgnoreFile: true, "versions.yaml": true}

var (
	snakeRE  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	importRE = regexp.MustCompile(`(?m)^[ \t]*import[ \t]+([^#\n]+)`)
	fromRE   = regexp.MustCompile(`(?m)^[ \t]*from[ \t]+(\.*[A-Za-z_][\w.]*|\.+)[ \t]+import\b`)
)

// framework checks one implementation against the framework register (spec 08.2 to 08.5 and 08.12).
func (r *run) framework(n *node.Node, im node.Impl, fw *node.Lang) {
	dir := n.Rel + "/impl/" + im.Lang
	if im.Lang != "numpy" && r.c.LangByID["numpy"] != nil && !slices.ContainsFunc(n.Impls, func(o node.Impl) bool { return o.Lang == "numpy" }) {
		r.add("X01", Level, dir, 0, "the node has %s code but no NumPy reference; the NumPy reference comes first, because it is the oracle of the other frameworks (8.2.2)", fw.Name)
	}
	local := map[string]bool{}
	for _, f := range im.Files {
		local[stem(f.Name)] = true
	}
	ref := map[string]bool{}
	for _, o := range n.Impls {
		if o.Lang == "numpy" {
			for _, f := range o.Files {
				ref[stem(f.Name)] = true
			}
		}
	}
	for _, f := range im.Files {
		if implData[path.Base(f.Name)] {
			continue
		}
		r.header(n, f)
		ext := strings.TrimPrefix(path.Ext(f.Name), ".")
		if !slices.Contains(fw.Ext, ext) {
			r.add("X12", Level, f.Path, 0, "the file has the extension %q; the code of %s has the extensions %s (8.2.1)", ext, fw.Name, strings.Join(fw.Ext, ", "))
			continue
		}
		if fw.FileCase == "snake" && !snakeRE.MatchString(stem(f.Name)) {
			r.add("X12", Level, f.Path, 0, "the file name %q is not snake_case, as the register asks for %s", path.Base(f.Name), fw.Name)
		}
		if fw.Lang() == "python" {
			r.imports(f, fw, local, ref)
		}
		if !f.Test && !labelled(f.Name) {
			if k := strings.Count(strings.TrimRight(f.Content, "\n"), "\n") + 1; k > MaxReference {
				r.add("X14", Warn, f.Path, 0, "the reference version has %d lines; a reference of more than %d lines is hard to read, so move the fast parts to a labelled version (8.3.1)", k, MaxReference)
			}
		}
	}
}

// header checks the licence header of a file of the framework register (spec 08.12): the SPDX line, and the line that names the page and the tag of the node.
func (r *run) header(n *node.Node, f node.File) {
	top := firstLines(f.Content, 4)
	if !strings.Contains(top, "SPDX-License-Identifier: MIT") {
		r.add("X19", Level, f.Path, 1, "the file does not start with the licence header `SPDX-License-Identifier: MIT` (8.12)")
		return
	}
	want := "Part of https://ai.tamnd.com/" + n.Slug + "/ (ai://" + n.Tag + ")"
	if !strings.Contains(top, want) {
		r.add("X19", Level, f.Path, 2, "the header does not name the page and the tag of the node; write `%s.` under the SPDX line (8.12)", want)
	}
}

// imports is rule X07: the code imports only the packages of the pinned set of its framework.
func (r *run) imports(f node.File, fw *node.Lang, local, ref map[string]bool) {
	set, ok := packageSets[fw.Packages]
	if !ok {
		return
	}
	for _, m := range pyImports(f.Content) {
		top, _, _ := strings.Cut(m, ".")
		switch {
		case top == "", pyStdlib[top], slices.Contains(set, top), local[top]:
		case f.Test && (slices.Contains(testPackages, top) || ref[top]):
		default:
			who := "the code imports"
			if f.Test {
				who = "the tests import"
			}
			r.add("X07", Block, f.Path, 0, "%s %s, which is not in the package set %s of %s (8.4)", who, top, fw.Packages, fw.Name)
		}
	}
}

// pyImports gives the modules that Python code imports. A relative import gives "".
func pyImports(src string) []string {
	var out []string
	for _, m := range importRE.FindAllStringSubmatch(src, -1) {
		for part := range strings.SplitSeq(m[1], ",") {
			name, _, _ := strings.Cut(strings.TrimSpace(strings.Trim(part, "() \t\\")), " ")
			if name != "" {
				out = append(out, name)
			}
		}
	}
	for _, m := range fromRE.FindAllStringSubmatch(src, -1) {
		if strings.HasPrefix(m[1], ".") {
			out = append(out, "")
			continue
		}
		out = append(out, m[1])
	}
	return out
}

func stem(name string) string {
	b := path.Base(name)
	return strings.TrimSuffix(b, path.Ext(b))
}

// labelled tells if a file is a labelled version of spec 08.5, such as attention_efficient.py.
func labelled(name string) bool {
	s := stem(name)
	return slices.ContainsFunc(labels, func(l string) bool { return strings.HasSuffix(s, "_"+l) })
}
