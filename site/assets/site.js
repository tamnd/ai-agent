// SPDX-License-Identifier: Apache-2.0
// The script of every page: the theme switch, the search palette, the code tabs, the table of contents and the copy buttons. The pages work without it.
(function () {
  "use strict";
  var d = document, root = d.documentElement;

  // The theme switch goes through the system theme, light and dark.
  var themeBtn = d.querySelector("[data-theme-toggle]");
  if (themeBtn) {
    themeBtn.addEventListener("click", function () {
      var cur = root.getAttribute("data-theme");
      var sysDark = matchMedia("(prefers-color-scheme: dark)").matches;
      var next = !cur ? (sysDark ? "light" : "dark") : null;
      try {
        if (next) { root.setAttribute("data-theme", next); localStorage.setItem("al-theme", next); }
        else { root.removeAttribute("data-theme"); localStorage.removeItem("al-theme"); }
      } catch (e) {}
    });
  }

  // Search. The index is search.json. It loads when the palette opens for the first time.
  var index = null, loading = null;
  function load() {
    if (index) return Promise.resolve(index);
    if (!loading) loading = fetch("/search.json").then(function (r) { return r.json(); }).then(function (j) { index = j; return j; });
    return loading;
  }
  function norm(s) { return (s || "").toLowerCase().normalize("NFD").replace(/[̀-ͯ]/g, ""); }
  function score(e, q, words) {
    var n = norm(e.n), best = 0;
    if (n === q) best = 1000;
    else if (n.indexOf(q) === 0) best = 600;
    else if (n.indexOf(q) > 0) best = 400;
    (e.a || []).forEach(function (a) {
      var x = norm(a);
      if (x === q) best = Math.max(best, 900);
      else if (x.indexOf(q) === 0) best = Math.max(best, 500);
      else if (x.indexOf(q) >= 0) best = Math.max(best, 300);
    });
    if (!best) {
      var hay = n + " " + norm((e.a || []).join(" ")) + " " + norm(e.s) + " " + norm(e.k) + " " + norm(e.r);
      var all = words.every(function (w) { return hay.indexOf(w) >= 0; });
      if (all) best = 100;
    }
    if (e.t && q.toUpperCase() === e.t) best = 1000;
    return best;
  }
  function find(q) {
    q = norm(q).trim();
    if (!q || !index) return [];
    var words = q.split(/\s+/);
    var out = [];
    for (var i = 0; i < index.length; i++) {
      var s = score(index[i], q, words);
      if (s) out.push([s - i / 10000, index[i]]);
    }
    out.sort(function (a, b) { return b[0] - a[0]; });
    return out.map(function (x) { return x[1]; });
  }
  window.AlgoSearch = { load: load, find: find };

  var pal = d.getElementById("palette");
  if (pal && pal.showModal) {
    var input = pal.querySelector("input"), list = pal.querySelector("ul"), sel = 0, hits = [];
    function esc(s) { return String(s || "").replace(/[&<>"]/g, function (c) { return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]; }); }
    function draw() {
      hits = find(input.value).slice(0, 12);
      list.innerHTML = hits.map(function (e, i) {
        return '<li role="option" aria-selected="' + (i === sel) + '"><a href="' + esc(e.u) + '"><span class="k">' + esc(e.k) + "</span>" + esc(e.n) + '<span class="s">' + esc(e.s) + "</span></a></li>";
      }).join("") || (input.value.trim() ? '<li><a href="/search/?q=' + encodeURIComponent(input.value) + '">No page has this name. Search the full text.</a></li>' : "");
    }
    function open() { pal.showModal(); input.value = ""; sel = 0; list.innerHTML = ""; input.focus(); load().then(draw); }
    d.querySelectorAll("[data-palette]").forEach(function (a) {
      a.addEventListener("click", function (e) { if (e.metaKey || e.ctrlKey) return; e.preventDefault(); open(); });
    });
    d.addEventListener("keydown", function (e) {
      var t = e.target, typing = t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable);
      if ((e.key === "k" && (e.metaKey || e.ctrlKey)) || (e.key === "/" && !typing)) { e.preventDefault(); open(); }
    });
    input.addEventListener("input", function () { sel = 0; draw(); });
    input.addEventListener("keydown", function (e) {
      if (e.key === "ArrowDown") { sel = Math.min(sel + 1, hits.length - 1); draw(); e.preventDefault(); }
      else if (e.key === "ArrowUp") { sel = Math.max(sel - 1, 0); draw(); e.preventDefault(); }
      else if (e.key === "Enter") { if (hits[sel]) { e.preventDefault(); location.href = hits[sel].u; } }
    });
    pal.addEventListener("click", function (e) { if (e.target === pal) pal.close(); });
  }

  // Code tabs. The choice of language is kept for the next page.
  var tabs = d.querySelector(".tabs");
  function pick(lang, save) {
    var b = d.getElementById("tab-" + lang);
    if (!b) return false;
    tabs.querySelectorAll("button").forEach(function (x) { x.setAttribute("aria-selected", String(x === b)); x.tabIndex = x === b ? 0 : -1; });
    d.querySelectorAll(".code .panel").forEach(function (p) { p.classList.toggle("on", p.id === "code-" + lang); });
    if (save) try { localStorage.setItem("al-lang", lang); } catch (e) {}
    return true;
  }
  if (tabs) {
    var want = location.hash.indexOf("#code-") === 0 ? location.hash.slice(6) : null;
    var saved = null;
    try { saved = localStorage.getItem("al-lang"); } catch (e) {}
    if (!(want && pick(want, false)) && saved) pick(saved, false);
    tabs.addEventListener("click", function (e) { var b = e.target.closest("button[data-lang]"); if (b) pick(b.dataset.lang, true); });
    tabs.addEventListener("keydown", function (e) {
      if (e.key !== "ArrowRight" && e.key !== "ArrowLeft") return;
      var bs = Array.prototype.slice.call(tabs.querySelectorAll("button"));
      var i = bs.indexOf(d.activeElement);
      if (i < 0) return;
      var j = (i + (e.key === "ArrowRight" ? 1 : bs.length - 1)) % bs.length;
      bs[j].focus(); pick(bs[j].dataset.lang, true);
    });
  }

  // Hover a line of the pseudocode to see the matching lines of the code.
  function codeLines(panel) { return panel ? panel.querySelectorAll("pre .line") : []; }
  function mark(id) {
    d.querySelectorAll(".panel").forEach(function (p) {
      var m = {};
      try { m = JSON.parse(p.dataset.markers || "{}"); } catch (e) {}
      var ls = codeLines(p);
      ls.forEach(function (l) { l.classList.remove("hl"); });
      (m[id] || []).forEach(function (n) { if (ls[n - 1]) ls[n - 1].classList.add("hl"); });
    });
  }
  window.AlgoMark = mark;
  d.querySelectorAll(".prose ol.pseudo li[data-line]").forEach(function (li) {
    li.addEventListener("mouseenter", function () { mark(li.dataset.line); });
    li.addEventListener("mouseleave", function () { mark(""); });
  });

  // Copy buttons.
  d.querySelectorAll("[data-copy]").forEach(function (b) {
    b.addEventListener("click", function () {
      var pre = b.closest(".panel").querySelector("pre");
      if (!pre || !navigator.clipboard) return;
      navigator.clipboard.writeText(pre.innerText).then(function () {
        var t = b.textContent; b.textContent = "Copied"; setTimeout(function () { b.textContent = t; }, 1400);
      });
    });
  });

  // The table of contents shows the current section.
  var toc = d.querySelector(".toc");
  if (toc && "IntersectionObserver" in window) {
    var links = {};
    toc.querySelectorAll("a").forEach(function (a) { links[a.getAttribute("href").slice(1)] = a; });
    var heads = Array.prototype.filter.call(d.querySelectorAll("main h2[id]"), function (h) { return links[h.id]; });
    var cur = null;
    var io = new IntersectionObserver(function () {
      var best = null;
      heads.forEach(function (h) { if (h.getBoundingClientRect().top < 120) best = h; });
      if (best !== cur) {
        if (cur && links[cur.id]) links[cur.id].classList.remove("on");
        cur = best;
        if (cur && links[cur.id]) links[cur.id].classList.add("on");
      }
    }, { rootMargin: "0px 0px -70% 0px", threshold: [0, 1] });
    heads.forEach(function (h) { io.observe(h); });
  }
  // The 404 page looks up the path in the map of moved paths, the tags and the aliases of the nodes, and sends the reader on.
  if (d.body.classList.contains("page-notfound")) {
    var path = location.pathname.replace(/\/+$/, "") || "/";
    fetch("/api/moved.json").then(function (r) { return r.ok ? r.json() : {}; }).then(function (m) {
      var to = m[path] || m[path.toLowerCase()];
      if (to) location.replace(to + location.search + location.hash);
    }).catch(function () {});
  }
})();
