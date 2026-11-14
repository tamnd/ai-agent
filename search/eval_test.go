// SPDX-License-Identifier: Apache-2.0

package search_test

import (
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/search"
	"github.com/tamnd/ai-agent/site"
)

const index = `[
{"n":"Transformer","u":"/transformer/","k":"Architecture","s":"A network of attention layers.","a":["transformer architecture"],"r":"Deep learning","t":"2B7QDA"},
{"n":"Grouped-query attention","u":"/grouped-query-attention/","k":"Method","s":"Share key and value heads.","a":["GQA"],"r":"Deep learning","t":"3C8RDB"},
{"n":"General question answering","u":"/general-qa/","k":"Task","s":"Answer questions about anything.","a":["GQA"],"r":"NLP","t":"4D9SEC"},
{"n":"Café attention","u":"/cafe-attention/","k":"Method","s":"Attention with accents.","r":"Deep learning","t":"5EATFD"},
{"n":"Dropout","u":"/dropout/","k":"Method","s":"Zero random units while training to curb overfitting.","r":"Deep learning","t":"6FBUGE"}
]`

const set = `
- q: transformer
  class: title
  want: transformer
- q: CAFE ATTENTION
  class: title
  want: /cafe-attention/
- q: 6fbuge
  class: id
  want: 6FBUGE
- q: gqa
  class: acronym
  want: [grouped-query-attention, general-qa, gqa-benchmark]
- q: how to regularize a model
  class: question
  want: dropout
- q: attention
  class: alias
  want: cafe-attention
- q: mamba
  class: title
  want: mamba
`

func TestEval(t *testing.T) {
	qs, err := search.Parse([]byte(set))
	if err != nil {
		t.Fatal(err)
	}
	rep, err := search.Eval(qs, site.FindJS(), []byte(index), nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Entries != 5 || rep.Queries != 7 {
		t.Fatalf("entries %d queries %d", rep.Entries, rep.Queries)
	}
	failed := map[string]search.Outcome{}
	for _, o := range rep.Failed {
		failed[o.Q] = o
	}
	// "how to regularize a model" is not in any field word for word, so the palette finds nothing: a question fails.
	for _, q := range []string{"how to regularize a model", "attention"} {
		if _, ok := failed[q]; !ok {
			t.Errorf("%q passed; want a failure", q)
		}
	}
	if len(failed) != 2 {
		t.Errorf("failed %v; want 2", failed)
	}
	if len(rep.Pending) != 1 || rep.Pending[0].Q != "mamba" {
		t.Errorf("pending %v; want mamba", rep.Pending)
	}
	o := failed["attention"]
	// Both names hold "attention" (400); the order of the index breaks the tie.
	if o.Ranks[0] != 2 || len(o.Top) != 3 || o.Top[0].URL != "/grouped-query-attention/" {
		t.Errorf("attention ranks %v top %v", o.Ranks, o.Top)
	}
	gates := map[string]search.Gate{}
	for _, g := range rep.Gates {
		gates[strings.SplitN(g.Name, ":", 2)[0]] = g
	}
	if g := gates["exact title, alias and id queries"]; g.N != 4 || g.OK || g.Got != 0.75 {
		t.Errorf("exact gate %+v", g)
	}
	if g := gates["acronym queries"]; g.N != 1 || !g.OK {
		t.Errorf("acronym gate %+v", g)
	}
	if g := gates["the whole set"]; g.N != 6 {
		t.Errorf("acronym gate %+v", g)
	}
	if g := gates["model and benchmark version queries"]; g.N != 0 || !g.OK || g.Note != "no queries" {
		t.Errorf("version gate %+v", g)
	}
	if g := gates["question, task and log queries"]; g.N != 1 || g.OK {
		t.Errorf("question gate %+v", g)
	}
	if rep.OK() {
		t.Error("the report passed")
	}
	if g := gates["palette transfer before the first result (gzip)"]; !g.OK || g.N == 0 {
		t.Errorf("transfer gate %+v", g)
	}

	// The baseline: a drop of more than 0.01 fails the MRR gate even above 0.85.
	good, _ := search.Parse([]byte("- q: transformer\n  class: title\n  want: transformer\n"))
	hi := 1.02
	rep, err = search.Eval(good, site.FindJS(), []byte(index), &hi)
	if err != nil {
		t.Fatal(err)
	}
	if rep.MRR != 1 || rep.OK() {
		t.Errorf("mrr %v ok %v; want 1 and a failed drop", rep.MRR, rep.OK())
	}
	lo := 1.005
	rep, _ = search.Eval(good, site.FindJS(), []byte(index), &lo)
	if !rep.OK() {
		t.Errorf("a drop of 0.005 failed: %+v", rep.Gates)
	}
}

func TestParse(t *testing.T) {
	for _, bad := range []string{
		"- q: x\n  class: guess\n  want: a\n",
		"- q: ''\n  class: title\n  want: a\n",
		"- q: x\n  class: title\n",
		"- q: x\n  class: title\n  want: a\n- q: x\n  class: title\n  want: b\n",
		"q: x\n",
	} {
		if _, err := search.Parse([]byte(bad)); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
	qs, err := search.Parse([]byte("# none yet\n"))
	if err != nil || qs != nil {
		t.Errorf("an empty set: %v %v", qs, err)
	}
}

// ids holds two entries with the same arXiv id: the paper and the method it introduced.
const ids = `[
{"n":"Transformer","u":"/transformer/","k":"architecture","s":"A network of attention layers.","i":["arxiv:1706.03762"]},
{"n":"ResNet","u":"/resnet/","k":"architecture","s":"A deep convolutional network of residual blocks.","a":["residual network"],"i":["arxiv:1512.03385","wikidata:Q43744058"]},
{"n":"Attention Is All You Need (Vaswani et al. 2017)","u":"/vaswani-2017-attention/","k":"work","s":"The paper that introduced the Transformer.","i":["arxiv:1706.03762"]},
{"n":"Q-learning","u":"/q-learning/","k":"method","s":"An off-policy temporal-difference control method that moves the value of each state-action pair toward the reward."}
]`

func TestRanker(t *testing.T) {
	r, err := search.NewRanker(site.FindJS(), []byte(ids))
	if err != nil {
		t.Fatal(err)
	}
	for q, want := range map[string]string{
		"1706.03762":                       "/vaswani-2017-attention/", // an id of a paper goes to the work first
		"arXiv:1706.03762v5":               "/vaswani-2017-attention/",
		"https://arxiv.org/abs/1512.03385": "/resnet/",
		"q43744058":                        "/resnet/",
		"q learning":                       "/q-learning/",  // punctuation folds
		"transfomer":                       "/transformer/", // one typo
		"resdiual network":                 "/resnet/",      // one swap in an alias
		"resdiu":                           "/resnet/",      // a typo in the start of an alias, while typing
		"ResNet-50":                        "/resnet/",      // a name with a size
		"value of state-action pairs":      "/q-learning/",  // the words against the summary
	} {
		res, err := r.Rank(q)
		if err != nil {
			t.Fatal(err)
		}
		if len(res) == 0 || res[0].URL != want {
			t.Errorf("%q gave %+v; want %s first", q, res, want)
		}
	}
	if res, _ := r.Rank("1706.03762"); len(res) < 2 || res[1].URL != "/transformer/" {
		t.Errorf("1706.03762 gave %+v; want the method second", res)
	}
	if res, _ := r.Rank("tr"); len(res) != 1 || res[0].URL != "/transformer/" {
		t.Errorf("tr gave %+v; want only the transformer", res)
	}
}

func TestRankerDiacritics(t *testing.T) {
	r, err := search.NewRanker(site.FindJS(), []byte(index))
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.Rank("café")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 || res[0].URL != "/cafe-attention/" || res[0].Sc < 599 {
		t.Errorf("café gave %+v", res)
	}
	if _, err := search.NewRanker(site.FindJS(), []byte(`{"n":1}`)); err == nil {
		t.Error("an index that is not a list loaded")
	}
}
