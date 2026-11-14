// SPDX-License-Identifier: Apache-2.0
// The ranking of the search palette and the search page. It reads no page and no network: it ranks the entries of
// search.json for a query. aik search eval runs this file on the built index (spec 17.10), so the evaluation measures
// the code that the reader runs.
//
// An entry has the keys n (name), a (aliases), s (summary), k (kind), r (area), t (tag) and i (the ids of same_as, as
// scheme:value). A query is ranked in tiers: an exact id or tag; then the name and the aliases, exact, as a prefix, as a
// part, or the name followed by a version or size (ResNet-50); then one typo away from a name or an alias, or from
// its start while the reader types; then the words of the query against the fields, weighted as in 17.2 and by how
// rare each word is in the index.
(function (g) {
  "use strict";
  var STOP = {};
  ("a an and are as at be by can do does for from how i in into is it its of on or that the this to was what when where which who why will with").split(" ").forEach(function (w) { STOP[w] = true; });
  // The weights of the fields, from 17.2.
  var W = { n: 10, a: 8, s: 4, r: 1, k: 1 };

  function norm(s) { return (s || "").toLowerCase().normalize("NFD").replace(/[̀-ͯ]/g, ""); }
  // key folds the punctuation, so that q-learning, q learning and "Q Learning" are one key.
  function key(s) { return norm(s).replace(/[^a-z0-9.]+/g, " ").replace(/\.(?![0-9])|(^|\s)\./g, " ").replace(/\s+/g, " ").trim(); }
  function stem(w) {
    if (w.length > 4 && /ies$/.test(w)) return w.slice(0, -3) + "y";
    if (w.length > 4 && /(sses|xes|ches|shes)$/.test(w)) return w.slice(0, -2);
    if (w.length > 3 && /[^su]s$/.test(w)) return w.slice(0, -1);
    if (w.length > 5 && /ing$/.test(w)) return w.slice(0, -3);
    if (w.length > 4 && /ed$/.test(w)) return w.slice(0, -2);
    return w;
  }
  function words(s) {
    return key(s).split(" ").filter(function (w) { return w && !STOP[w]; }).map(stem);
  }
  // like tells whether a word of the query matches a word of a field: the same stem, one a prefix of the other when
  // the shorter has 4 letters or more, or a shared start of 7 letters (intelligent, intelligence).
  function like(q, f) {
    if (q === f) return true;
    var a = q.length < f.length ? q : f, b = a === q ? f : q;
    if (a.length >= 4 && b.indexOf(a) === 0) return true;
    var i = 0;
    while (i < a.length && a[i] === b[i]) i++;
    return i >= 7;
  }
  // dist is the edit distance of two strings with the swap of two neighbours as one edit, or max+1 when it is larger.
  function dist(a, b, max) {
    if (Math.abs(a.length - b.length) > max) return max + 1;
    var p2 = null, p = [], c, i, j;
    for (j = 0; j <= b.length; j++) p[j] = j;
    for (i = 1; i <= a.length; i++) {
      c = [i];
      var low = i;
      for (j = 1; j <= b.length; j++) {
        var v = Math.min(p[j] + 1, c[j - 1] + 1, p[j - 1] + (a[i - 1] === b[j - 1] ? 0 : 1));
        if (p2 && i > 1 && j > 1 && a[i - 1] === b[j - 2] && a[i - 2] === b[j - 1]) v = Math.min(v, p2[j - 2] + 1);
        c[j] = v;
        if (v < low) low = v;
      }
      if (low > max) return max + 1;
      p2 = p; p = c;
    }
    return p[b.length];
  }
  function id(s) {
    return norm(s).trim().replace(/^https?:\/\/(www\.)?(arxiv\.org\/(abs|pdf)\/|doi\.org\/|dx\.doi\.org\/|wikidata\.org\/wiki\/)/, "").replace(/^(arxiv|doi|wikidata):\s*/, "").replace(/v[0-9]+$/, "");
  }

  // prep computes once what the ranking needs of an index: the folded fields of each entry and the rarity of each word.
  function prep(index) {
    if (index.__prep) return index.__prep;
    var df = {}, es = [];
    index.forEach(function (e) {
      var x = { n: key(e.n), a: (e.a || []).map(key), ids: (e.i || []).map(function (v) { return id(v.slice(v.indexOf(":") + 1)); }), f: {} };
      var seen = {};
      ["n", "a", "s", "r", "k"].forEach(function (f) {
        var ws = words(f === "a" ? (e.a || []).join(" ") : e[f]);
        x.f[f] = ws;
        ws.forEach(function (w) { if (!seen[w]) { seen[w] = true; df[w] = (df[w] || 0) + 1; } });
      });
      es.push(x);
    });
    var p = { es: es, df: df, n: index.length };
    try { Object.defineProperty(index, "__prep", { value: p }); } catch (err) { index.__prep = p; }
    return p;
  }

  function score(e, x, q, qk, qw, p) {
    if (e.t && q.toUpperCase() === e.t) return 1000;
    var qi = id(q);
    // An id of a paper names a work; a method that the paper introduced may hold the id too, and comes second (04.2).
    for (var i = 0; i < x.ids.length; i++) if (x.ids[i] === qi) return e.k === "work" ? 1000 : 990;
    var best = 0, n = x.n;
    if (n === qk) return 1000;
    if (n.indexOf(qk) === 0) best = 600;
    else if (n.indexOf(qk) > 0) best = 400;
    else if (qk.indexOf(n + " ") === 0 && /^[0-9]/.test(qk.slice(n.length + 1))) best = 450; // a version or a size
    x.a.forEach(function (a) {
      if (a === qk) best = Math.max(best, 900);
      else if (a.indexOf(qk) === 0) best = Math.max(best, 500);
      else if (a.indexOf(qk) >= 0) best = Math.max(best, 300);
      else if (qk.indexOf(a + " ") === 0 && /^[0-9]/.test(qk.slice(a.length + 1))) best = Math.max(best, 350);
    });
    if (best) return best;
    if (qk.length >= 4) {
      var max = qk.length >= 8 ? 2 : 1;
      if (dist(qk, n, max) <= max) return 250;
      for (var j = 0; j < x.a.length; j++) if (dist(qk, x.a[j], max) <= max) return 200;
      // The reader is still typing: one typo away from the start of a name or an alias.
      if (n.length > qk.length && dist(qk, n.slice(0, qk.length), max) <= max) return 150;
      for (j = 0; j < x.a.length; j++) if (x.a[j].length > qk.length && dist(qk, x.a[j].slice(0, qk.length), max) <= max) return 120;
    }
    // The words of the query against the fields, as a share of the best that the words could score.
    var got = 0, top = 0;
    qw.forEach(function (w) {
      var idf = Math.log(1 + p.n / (p.df[w] || 0.5)), m = 0;
      for (var f in W) {
        if (W[f] <= m) continue;
        if (x.f[f].some(function (v) { return like(w, v); })) m = W[f];
      }
      got += m * idf; top += W.n * idf;
    });
    return top ? Math.round(100 * got / top * 100) / 100 : 0;
  }

  // rank gives the entries that match a query, best first, each as [score, entry]. The index is in the order of the
  // build, the more important kinds first, and that order breaks ties.
  function rank(index, q) {
    q = norm(q).trim();
    if (!q || !index) return [];
    var p = prep(index), qk = key(q), qw = words(q);
    var out = [];
    for (var i = 0; i < index.length; i++) {
      var s = score(index[i], p.es[i], q, qk, qw, p);
      if (s) out.push([s - i / 10000, index[i]]);
    }
    out.sort(function (a, b) { return b[0] - a[0]; });
    return out;
  }
  g.AlgoFind = { norm: norm, rank: rank };
})(typeof window !== "undefined" ? window : this);
