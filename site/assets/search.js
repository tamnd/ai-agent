// SPDX-License-Identifier: Apache-2.0
// The search page. With no script, the page shows every page from A to Z.
(function () {
  "use strict";
  var d = document, q = d.getElementById("search-q"), out = d.getElementById("search-results"), all = d.getElementById("search-all"), count = d.getElementById("search-count");
  if (!q || !window.AlgoSearch) return;
  function esc(s) { return String(s || "").replace(/[&<>"]/g, function (c) { return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]; }); }
  function run() {
    var v = q.value.trim();
    if (!v) { out.innerHTML = ""; all.hidden = false; count.textContent = ""; history.replaceState(null, "", "/search/"); return; }
    all.hidden = true;
    window.AlgoSearch.load().then(function () {
      var hits = window.AlgoSearch.find(v);
      count.textContent = hits.length === 1 ? "1 page" : hits.length + " pages";
      out.innerHTML = hits.slice(0, 100).map(function (e) {
        return '<li class="item"><div class="item-main"><a href="' + esc(e.u) + '">' + esc(e.n) + "</a><p>" + esc(e.s) + '</p></div><div class="item-side"><span class="chip">' + esc(e.k) + "</span></div></li>";
      }).join("");
      history.replaceState(null, "", "/search/?q=" + encodeURIComponent(v));
    });
  }
  var p = new URLSearchParams(location.search).get("q");
  if (p) q.value = p;
  q.addEventListener("input", run);
  q.form.addEventListener("submit", function (e) { e.preventDefault(); run(); });
  run();
  q.focus();
})();
