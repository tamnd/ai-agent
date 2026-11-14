// SPDX-License-Identifier: Apache-2.0
// The script of a path page (16.12): the reader marks the steps they know, hides them, and follows the path, so that
// each page of the path shows the next step. The progress stays in this browser, in localStorage, with no account.
// Without the script the page is an ordered list.
(function () {
  "use strict";
  var d = document, page = d.querySelector("[data-path]");
  if (!page) return;
  function get(k, def) { try { var v = localStorage.getItem(k); return v ? JSON.parse(v) : def; } catch (e) { return def; } }
  function set(k, v) { try { localStorage.setItem(k, JSON.stringify(v)); } catch (e) {} }
  var known = get("ai-known", {}), cur = get("ai-path", null);
  var steps = [].slice.call(page.querySelectorAll("li[data-step]"));
  var prog = page.querySelector("[data-path-progress]"), tools = page.querySelector("[data-path-tools]");
  var hide = page.querySelector("[data-path-hide-known]"), follow = page.querySelector("[data-path-follow]");
  function draw() {
    var n = 0, next = null;
    steps.forEach(function (li) {
      var k = !!known[li.getAttribute("data-step")];
      li.classList.toggle("is-known", k);
      li.querySelector("input").checked = k;
      if (k) n++; else if (!next) next = li;
    });
    steps.forEach(function (li) { li.classList.toggle("is-next", li === next); });
    prog.textContent = n + " of " + steps.length + " marked as known, in this browser only.";
  }
  steps.forEach(function (li) {
    var label = li.querySelector("label.known");
    label.hidden = false;
    label.querySelector("input").addEventListener("change", function (e) {
      var k = li.getAttribute("data-step");
      if (e.target.checked) known[k] = 1; else delete known[k];
      set("ai-known", known);
      draw();
    });
  });
  function following() { return cur && cur.id === page.getAttribute("data-path"); }
  function drawFollow() {
    follow.textContent = following() ? "Stop following" : "Follow this path";
    follow.setAttribute("aria-pressed", following() ? "true" : "false");
  }
  follow.addEventListener("click", function () {
    if (following()) {
      cur = null;
      try { localStorage.removeItem("ai-path"); } catch (e) {}
    } else {
      cur = { id: page.getAttribute("data-path"), title: page.getAttribute("data-title"), url: location.pathname, steps: steps.filter(function (li) { return li.getAttribute("data-url"); }).map(function (li) { return { k: li.getAttribute("data-step"), n: li.getAttribute("data-name"), u: li.getAttribute("data-url") }; }) };
      set("ai-path", cur);
    }
    drawFollow();
  });
  hide.addEventListener("click", function () {
    var on = hide.getAttribute("aria-pressed") !== "true";
    hide.setAttribute("aria-pressed", on ? "true" : "false");
    page.classList.toggle("hide-known", on);
  });
  tools.hidden = false;
  drawFollow();
  draw();
})();
