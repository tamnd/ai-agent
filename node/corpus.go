package node

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tamnd/ai-agent/computed"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/bridge"
	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/exp"
	"go.yaml.in/yaml/v3"
)

// Area is one entry of topics/areas.yaml (spec 05.2). A top area has the letter of its group; a sub-area has none.
type Area struct {
	ID       string `yaml:"id" json:"id"`
	Name     string `yaml:"name" json:"name"`
	Group    string `yaml:"group,omitempty" json:"group,omitempty"`
	Summary  string `yaml:"summary,omitempty" json:"summary,omitempty"`
	Children []Area `yaml:"children,omitempty" json:"children,omitempty"`
	Parent   string `yaml:"-" json:"parent,omitempty"`
}

// Path is "top/sub" for a sub-area and "top" for a top area.
func (a Area) Path() string {
	if a.Parent == "" {
		return a.ID
	}
	return a.Parent + "/" + a.ID
}

// Lang is one entry of registers/languages.yaml (spec 08.2).
type Lang struct {
	ID        string   `yaml:"id" json:"id"`
	Name      string   `yaml:"name" json:"name"`
	Tier      int      `yaml:"tier" json:"tier"`
	Ext       []string `yaml:"ext" json:"ext"`
	FileCase  string   `yaml:"file_case" json:"file_case,omitempty"`
	Test      LangTool `yaml:"test" json:"test"`
	Format    LangTool `yaml:"format" json:"format"`
	Lint      LangTool `yaml:"lint" json:"lint"`
	Toolchain string   `yaml:"toolchain" json:"toolchain,omitempty"` // the pinned version, for the language page
	Run       string   `yaml:"run" json:"run,omitempty"`             // how a reader runs the tests without aik
	Comment   string   `yaml:"comment" json:"comment,omitempty"`
	Highlight string   `yaml:"highlight" json:"highlight"` // the Chroma lexer
	Int       string   `yaml:"int" json:"int,omitempty"`
	List      string   `yaml:"list" json:"list,omitempty"`
	Since     string   `yaml:"since" json:"since,omitempty"`

	// The fields below are the fields of a framework of registers/frameworks.yaml (spec 08.2).
	Language     string   `yaml:"language" json:"language,omitempty"` // the language of the code, such as python for pytorch
	Packages     string   `yaml:"packages" json:"packages,omitempty"` // the pinned package set of spec 08.4
	Types        LangTool `yaml:"types" json:"types,omitzero"`
	Mutation     LangTool `yaml:"mutation" json:"mutation,omitzero"`
	Image        string   `yaml:"image" json:"image,omitempty"`
	DefaultDType string   `yaml:"default_dtype" json:"default_dtype,omitempty"`
	Determinism  []string `yaml:"determinism" json:"determinism,omitempty"`
}

// Lang gives the language of the code of the entry: the language of a framework, or the id of a language.
func (l *Lang) Lang() string {
	if l.Language != "" {
		return l.Language
	}
	return l.ID
}

// LangTool is a tool of a language: the test framework, the formatter or the linter.
type LangTool struct {
	Tool      string   `yaml:"tool,omitempty" json:"tool,omitempty"`
	Framework string   `yaml:"framework,omitempty" json:"framework,omitempty"`
	File      string   `yaml:"file,omitempty" json:"file,omitempty"`
	Args      []string `yaml:"args,omitempty" json:"args,omitempty"`
}

// Tag is one line of registers/tags.reg.
type Tag struct {
	Tag  string
	Slug string
	Line int
}

// Corpus is the content tree.
type Corpus struct {
	Root     string
	Nodes    []*Node
	BySlug   map[string]*Node
	ByTag    map[string]*Node
	Areas    []Area
	AreaByID map[string]*Area // keyed by Area.Path()
	// Langs are the entries of the framework register, registers/frameworks.yaml (spec 08.2). A content tree that has no framework register has the language register, registers/languages.yaml, of the algorithm site.
	Langs    []Lang
	LangByID map[string]*Lang
	// LangFile is the path of the register that Langs came from, relative to the root.
	LangFile string
	// Frameworks is true when Langs came from the framework register.
	Frameworks bool
	Reserved   map[string]bool
	Tags       []Tag
	// OldSlugs maps each old slug of registers/aliases.reg to the slug it moved to.
	OldSlugs map[string]string
	// Errors are the node directories that could not be read. Package check reports them as T01.
	Errors []error
	// Cards are the cards of cards/ (spec 04.6), in the order of the files and their lines.
	Cards []*card.Card
	// CardByKey maps the key of each card (its slug, or its owner and slug) to the first card with that key.
	CardByKey map[string]*card.Card
	// CardErrors are the lines of cards/ that are not cards. Package check reports them as T01.
	CardErrors []error
	// Bridges are the bridge cards of cards/bridge (spec 5.9).
	Bridges *bridge.Set
	// BridgeErrors are the lines of cards/bridge that are not bridge cards.
	BridgeErrors []error
	// Pins are the datasets of registers/data-pins.yaml (spec 9.5), by id. It is nil when the register does not exist.
	Pins map[string]*exp.Pin
	// ComputedDefs are the definitions of topics/computed.yaml (04.9), Computed their results by id, and ComputedErrs
	// the definitions that have no result, with the reason.
	ComputedDefs []computed.Def
	Computed     map[string]*computed.Result
	ComputedErrs map[string]error
}

// Load reads the whole content tree at root.
func Load(root string) (*Corpus, error) {
	c := &Corpus{
		Root:      root,
		BySlug:    map[string]*Node{},
		ByTag:     map[string]*Node{},
		AreaByID:  map[string]*Area{},
		LangByID:  map[string]*Lang{},
		Reserved:  map[string]bool{},
		CardByKey: map[string]*card.Card{},
	}
	if err := readYAML(filepath.Join(root, "topics", "areas.yaml"), &c.Areas); err != nil {
		return nil, err
	}
	for i := range c.Areas {
		top := &c.Areas[i]
		c.AreaByID[top.Path()] = top
		for j := range top.Children {
			sub := &top.Children[j]
			sub.Parent = top.ID
			c.AreaByID[sub.Path()] = sub
		}
	}
	pins, err := exp.LoadPins(filepath.Join(root, filepath.FromSlash(exp.PinsFile)))
	if err != nil {
		return nil, err
	}
	c.Pins = pins
	c.LangFile = "registers/languages.yaml"
	if _, err := os.Stat(filepath.Join(root, "registers", "frameworks.yaml")); err == nil {
		c.LangFile, c.Frameworks = "registers/frameworks.yaml", true
	}
	if err := readYAML(filepath.Join(root, filepath.FromSlash(c.LangFile)), &c.Langs); err != nil {
		return nil, err
	}
	sort.SliceStable(c.Langs, func(i, j int) bool { return c.Langs[i].Tier < c.Langs[j].Tier })
	for i := range c.Langs {
		c.LangByID[c.Langs[i].ID] = &c.Langs[i]
	}
	var reserved []string
	if err := readYAML(filepath.Join(root, "registers", "reserved.yaml"), &reserved); err != nil {
		return nil, err
	}
	for _, w := range reserved {
		c.Reserved[w] = true
	}
	tags, err := readTags(filepath.Join(root, "registers", "tags.reg"))
	if err != nil {
		return nil, err
	}
	c.Tags = tags
	if c.OldSlugs, err = ReadAliases(filepath.Join(root, "registers", "aliases.reg")); err != nil {
		return nil, err
	}

	nodes := filepath.Join(root, "nodes")
	err = filepath.WalkDir(nodes, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "node.md" {
			return nil
		}
		n, err := Read(root, filepath.Dir(p))
		if err != nil {
			c.Errors = append(c.Errors, err)
			return nil
		}
		c.Nodes = append(c.Nodes, n)
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	sort.Slice(c.Nodes, func(i, j int) bool { return c.Nodes[i].Slug < c.Nodes[j].Slug })
	for _, n := range c.Nodes {
		if n.Slug != "" {
			if _, dup := c.BySlug[n.Slug]; !dup {
				c.BySlug[n.Slug] = n
			}
		}
		if n.Tag != "" {
			if _, dup := c.ByTag[n.Tag]; !dup {
				c.ByTag[n.Tag] = n
			}
		}
	}
	c.Cards, c.CardErrors = card.Read(root)
	for _, cd := range c.Cards {
		if _, dup := c.CardByKey[cd.Key()]; !dup && cd.Slug != "" {
			c.CardByKey[cd.Key()] = cd
		}
	}
	for _, n := range c.Nodes {
		if n.Kind != "work" || n.Slug == "" || c.CardByKey[n.Slug] != nil {
			continue
		}
		if cd := n.WorkCard(); cd != nil {
			c.CardByKey[cd.Key()] = cd
		}
	}
	var bs []*bridge.Card
	bs, c.BridgeErrors = bridge.Read(root)
	c.Bridges = bridge.NewSet(bs)
	defs, err := computed.Read(root)
	if err != nil {
		c.Errors = append(c.Errors, err)
	}
	c.ComputedDefs = defs
	c.compute()
	return c, nil
}

// InArea returns the nodes whose primary area is path or is under path.
func (c *Corpus) InArea(path string) []*Node {
	var out []*Node
	for _, n := range c.Nodes {
		if n.Area == path || strings.HasPrefix(n.Area, path+"/") {
			out = append(out, n)
		}
	}
	return out
}

// Kind returns the nodes of one kind.
func (c *Corpus) Kind(kind string) []*Node {
	var out []*Node
	for _, n := range c.Nodes {
		if n.Kind == kind {
			out = append(out, n)
		}
	}
	return out
}

// Solvers returns the nodes with a `solves` edge to the problem.
func (c *Corpus) Solvers(problem string) []*Node {
	var out []*Node
	for _, n := range c.Nodes {
		for _, s := range n.Solves {
			if s.Problem == problem {
				out = append(out, n)
				break
			}
		}
	}
	return out
}

// Leads returns the nodes that list slug in prereqs or uses.
func (c *Corpus) Leads(slug string) []*Node {
	var out []*Node
	for _, n := range c.Nodes {
		for _, p := range append(append([]string{}, n.Prereqs...), n.Uses...) {
			if p == slug {
				out = append(out, n)
				break
			}
		}
	}
	return out
}

func readYAML(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// readTags reads registers/tags.reg. Each line is "TAG slug". A line that starts with # is a comment.
// ReadAliases reads registers/aliases.reg. Each line is an old slug and the slug it moved to.
func ReadAliases(path string) (map[string]string, error) {
	out := map[string]string{}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	line := 0
	for s.Scan() {
		line++
		t := strings.TrimSpace(s.Text())
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		fs := strings.Fields(t)
		if len(fs) != 2 {
			return nil, fmt.Errorf("%s:%d: a line needs an old slug and a new slug", path, line)
		}
		out[fs[0]] = fs[1]
	}
	return out, s.Err()
}

func readTags(path string) ([]Tag, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Tag
	s := bufio.NewScanner(f)
	line := 0
	for s.Scan() {
		line++
		t := strings.TrimSpace(s.Text())
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		fs := strings.Fields(t)
		if len(fs) < 2 {
			return nil, fmt.Errorf("%s:%d: a line needs a tag and a slug", path, line)
		}
		out = append(out, Tag{Tag: fs[0], Slug: fs[1], Line: line})
	}
	return out, s.Err()
}

// WorkCard is the card of a work that is a node, made from its front matter: a promoted card leaves cards/ (4.6), and
// its fields move to the node, where aik cite and the checks read them as they read a card. It is in CardByKey and
// not in Cards, so the card checks do not see it twice.
func (n *Node) WorkCard() *card.Card {
	m := map[string]any{"tag": n.Tag, "kind": n.Kind, "slug": n.Slug, "name": n.Name, "summary": n.Summary}
	if n.SameAs != nil {
		m["same_as"] = n.SameAs
	}
	for k, v := range n.Fields {
		m[k] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	cd, err := card.Parse(b)
	if err != nil {
		return nil
	}
	cd.File = n.Rel + "/node.md"
	return cd
}
