// SPDX-License-Identifier: Apache-2.0
// Runs in the head, before the page paints, so that the saved theme shows with no flash.
(function () {
  var d = document.documentElement;
  d.classList.add("js");
  try {
    var t = localStorage.getItem("al-theme");
    if (t === "light" || t === "dark") d.setAttribute("data-theme", t);
  } catch (e) {}
})();
