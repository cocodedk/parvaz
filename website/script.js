/* Slide-deck navigation — vanilla JS, no deps.
   scroll-snap inside .deck-track + dots + prev/next + keyboard.
   Loaded by both EN and FA index.html via <script src="..." defer>.
   Counter is zero-padded ("01 / 13") and mirrored to two slots:
   one in the fixed boarding-pass header, one in the bottom controls.
   URL hash carries the active slide so EN↔FA lang-switch lands on
   the same slide and so direct links can deep-link a specific slide. */
(() => {
  const slides = Array.from(document.querySelectorAll('.slide'));
  if (!slides.length) return;

  const dotsEl    = document.getElementById('deck-dots');
  const currentEl = document.getElementById('deck-current');
  const mirrorEl  = document.getElementById('deck-current-mirror');
  const totalEl   = document.getElementById('deck-total');
  const prevBtn   = document.getElementById('deck-prev');
  const nextBtn   = document.getElementById('deck-next');
  const track     = document.getElementById('deck-track');

  // Persian-digit awareness: if the existing total reads in Persian
  // numerals, format the current counter the same way.
  const FA_DIGITS = '۰۱۲۳۴۵۶۷۸۹';
  const usesFa = totalEl ? /[۰-۹]/.test(totalEl.textContent) : false;
  const pad = (n) => {
    const s = String(n).padStart(2, '0');
    return usesFa ? s.replace(/\d/g, (d) => FA_DIGITS[+d]) : s;
  };

  if (totalEl) totalEl.textContent = pad(slides.length);

  if (dotsEl) {
    slides.forEach((s, i) => {
      const b = document.createElement('button');
      b.type = 'button';
      b.setAttribute('aria-label', `Slide ${i + 1}`);
      b.addEventListener('click', () => goTo(i));
      dotsEl.appendChild(b);
    });
  }
  const dots = dotsEl ? Array.from(dotsEl.querySelectorAll('button')) : [];

  // Cache the original lang-switch hrefs once so we can re-append the
  // current hash on every slide change.
  const langLinks = Array.from(document.querySelectorAll('.deck-lang-pill, .deck-controls__lang'));
  langLinks.forEach((a) => {
    if (!a.dataset.langBase) {
      a.dataset.langBase = (a.getAttribute('href') || '').split('#')[0];
    }
  });

  let currentIdx = 0;
  const goTo = (idx) => {
    const next = Math.max(0, Math.min(slides.length - 1, idx));
    const target = slides[next];
    if (!target) return;
    // Each slide is exactly 100vh; the track is also 100vh. So slide N's
    // top in the track's scroll-space is N * track.clientHeight. We avoid
    // target.offsetTop because it's measured against the closest positioned
    // ancestor, which is the body (not the track) — so on mobile, where
    // slides are themselves scroll containers, the math is wrong.
    if (track && typeof track.scrollTo === 'function') {
      track.scrollTo({ top: next * track.clientHeight, behavior: 'smooth' });
    } else {
      target.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  };
  const go = (delta) => goTo(currentIdx + delta);

  // Pointerdown fires before click and isn't swallowed by Chrome Android's
  // 300ms tap-delay heuristics on fixed elements.
  const wireNav = (btn, delta) => {
    if (!btn) return;
    btn.addEventListener('click', (e) => { e.preventDefault(); go(delta); });
  };
  wireNav(prevBtn, -1);
  wireNav(nextBtn, 1);

  const setActive = (idx) => {
    currentIdx = idx;
    const padded = pad(idx + 1);
    if (currentEl) currentEl.textContent = padded;
    if (mirrorEl)  mirrorEl.textContent  = padded;
    dots.forEach((d, i) => d.classList.toggle('is-active', i === idx));
    if (prevBtn) prevBtn.disabled = idx === 0;
    if (nextBtn) nextBtn.disabled = idx === slides.length - 1;

    // Reflect into URL hash without polluting browser history.
    const wantHash = '#' + (idx + 1);
    if (location.hash !== wantHash) {
      history.replaceState(null, '', wantHash);
    }
    // Forward the hash to lang-switch links so EN↔FA preserves slide.
    langLinks.forEach((a) => {
      a.setAttribute('href', a.dataset.langBase + wantHash);
    });
  };

  const idxFromHash = () => {
    const m = (location.hash || '').match(/^#(\d+)$/);
    if (!m) return 0;
    return Math.max(0, Math.min(slides.length - 1, parseInt(m[1], 10) - 1));
  };

  const io = new IntersectionObserver((entries) => {
    entries.forEach((entry) => {
      if (entry.intersectionRatio < 0.5) return;
      const idx = slides.indexOf(entry.target);
      if (idx !== -1) setActive(idx);
    });
  }, { root: track || null, threshold: [0.5] });
  slides.forEach((s) => io.observe(s));

  // Land on the slide named by the URL hash (or slide 1 by default).
  // Use 'auto' (instant) on initial load — animating from slide 1 to
  // slide N on every page open is jarring.
  const initial = idxFromHash();
  if (initial > 0) {
    slides[initial].scrollIntoView({ behavior: 'auto', block: 'start' });
  }
  setActive(initial);

  // Handle browser back/forward and manual hash edits.
  window.addEventListener('hashchange', () => {
    const idx = idxFromHash();
    if (idx !== currentIdx) goTo(idx);
  });

  document.addEventListener('keydown', (e) => {
    const tag = e.target?.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA') return;
    const next = ['ArrowDown', 'PageDown', ' ', 'ArrowRight'].includes(e.key);
    const prev = ['ArrowUp',   'PageUp',           'ArrowLeft' ].includes(e.key);
    if (next)      { e.preventDefault(); go(1); }
    else if (prev) { e.preventDefault(); go(-1); }
    else if (e.key === 'Home') goTo(0);
    else if (e.key === 'End')  goTo(slides.length - 1);
  });

  // Self-test hook for test-deck.sh — only runs when URL has ?test=nav.
  // Wraps scrollTo so the smooth-scroll behavior in goTo() runs instantly
  // (otherwise we'd race the animation), clicks the next arrow, and writes
  // the result to data-nav-test on <body>. Catches the offsetTop-vs-
  // clientHeight regression we hit on Chrome Android.
  if (track && /[?&]test=nav\b/.test(location.search)) {
    // Force instant scroll for the test — both the CSS scroll-behavior
    // (which makes scrollTop=N animate) and the explicit option in goTo's
    // scrollTo({behavior:'smooth'}) need to be neutralized.
    track.style.scrollBehavior = 'auto';
    const realScrollTo = track.scrollTo.bind(track);
    track.scrollTo = (opts) => realScrollTo({ ...opts, behavior: 'auto' });
    setTimeout(() => {
      const h = track.clientHeight;
      const before = track.scrollTop;
      nextBtn?.click();
      const after = track.scrollTop;
      const drift = Math.abs(after - h);
      document.body.dataset.navTest = (drift < 4) ? 'pass' : 'fail';
      document.body.dataset.navTestExpected = String(h);
      document.body.dataset.navTestActual = String(after);
      document.body.dataset.navTestBefore = String(before);
    }, 50);
  }
})();
