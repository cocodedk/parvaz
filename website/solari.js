/* The departures board flips into place like a real Solari board: the first time it comes into
   view, each cell's letters rattle through the drum and settle one after another, row by row.
   Reduced motion, or a browser without IntersectionObserver, shows the board as it is. */
(function () {
  'use strict';
  var board = document.querySelector('.solari-board');
  if (!board || !('IntersectionObserver' in window)) return;
  if (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;

  var UPPER = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', LOWER = UPPER.toLowerCase(), DIGIT = '0123456789';
  /* A character flips through the drum that holds it; spaces and punctuation are fixed. */
  function drum(ch) {
    return UPPER.indexOf(ch) >= 0 ? UPPER : LOWER.indexOf(ch) >= 0 ? LOWER : DIGIT.indexOf(ch) >= 0 ? DIGIT : null;
  }
  var cells = [].slice.call(board.querySelectorAll('.solari-board__cell')).map(function (el, i) {
    var text = el.textContent, row = Math.floor(i / 4), col = i % 4;
    /* Hold the cell's width, so the columns don't jump while the letters turn. */
    el.style.minWidth = el.getBoundingClientRect().width + 'px';
    return { el: el, text: text, start: row * 260 + col * 90 };
  });

  function run() {
    var t0 = performance.now(), last = 0;
    (function frame(now) {
      var t = now - t0, busy = false;
      if (now - last >= 45) {          /* about 22 flips a second: mechanical, not a blur */
        last = now;
        cells.forEach(function (c) {
          var out = '';
          for (var i = 0; i < c.text.length; i++) {
            var ch = c.text[i], d = drum(ch), settle = c.start + 140 + i * 35;
            if (!d || t >= settle) { out += ch; } else { busy = true; out += d[Math.floor(Math.random() * d.length)]; }
          }
          if (c.el.textContent !== out) c.el.textContent = out;
        });
      } else { busy = true; }
      if (busy) requestAnimationFrame(frame);
      else cells.forEach(function (c) { c.el.style.minWidth = ''; });
    })(t0);
  }

  var io = new IntersectionObserver(function (entries) {
    if (entries.some(function (e) { return e.isIntersecting; })) { io.disconnect(); run(); }
  }, { threshold: 0.4 });
  io.observe(board);
})();
