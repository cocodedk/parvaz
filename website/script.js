/* Slide-deck navigation — vanilla JS, no deps. Pattern ported from
   the twoday project: scroll-snap slides + dots + side arrows + keys.
   Loaded by both EN and FA index.html via <script src="..." defer>. */
(() => {
  document.body.classList.add('deck-mode');

  const slides   = Array.from(document.querySelectorAll('.slide'));
  if (!slides.length) return;

  const dotsEl   = document.getElementById('deck-dots');
  const currentEl= document.getElementById('deck-current');
  const totalEl  = document.getElementById('deck-total');
  const prevBtn  = document.getElementById('deck-prev');
  const nextBtn  = document.getElementById('deck-next');

  if (totalEl) totalEl.textContent = String(slides.length);

  // Generate dot per slide
  if (dotsEl) {
    slides.forEach((s, i) => {
      const b = document.createElement('button');
      b.type = 'button';
      b.setAttribute('aria-label', `Slide ${i + 1}`);
      b.addEventListener('click', () => s.scrollIntoView({ behavior: 'smooth' }));
      dotsEl.appendChild(b);
    });
  }
  const dots = dotsEl ? Array.from(dotsEl.querySelectorAll('button')) : [];

  let currentIdx = 0;
  const go = (delta) => {
    const next = Math.max(0, Math.min(slides.length - 1, currentIdx + delta));
    slides[next]?.scrollIntoView({ behavior: 'smooth' });
  };
  prevBtn?.addEventListener('click', () => go(-1));
  nextBtn?.addEventListener('click', () => go(1));

  const io = new IntersectionObserver((entries) => {
    entries.forEach((entry) => {
      if (entry.intersectionRatio < 0.5) return;
      const idx = slides.indexOf(entry.target);
      if (idx === -1) return;
      currentIdx = idx;
      if (currentEl) currentEl.textContent = String(idx + 1);
      dots.forEach((d, i) => d.classList.toggle('is-active', i === idx));
      if (prevBtn) prevBtn.disabled = idx === 0;
      if (nextBtn) nextBtn.disabled = idx === slides.length - 1;
    });
  }, { threshold: [0.5] });
  slides.forEach((s) => io.observe(s));

  // Keyboard navigation. Arrow keys map to slide motion in BOTH writing
  // modes — vertical scroll is the primary axis, so left/right are
  // legitimate aliases for prev/next regardless of LTR/RTL.
  document.addEventListener('keydown', (e) => {
    const tag = e.target?.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA') return;
    const next = ['ArrowDown', 'PageDown', ' ', 'ArrowRight'].includes(e.key);
    const prev = ['ArrowUp',   'PageUp',           'ArrowLeft' ].includes(e.key);
    if (next)      { e.preventDefault(); go(1); }
    else if (prev) { e.preventDefault(); go(-1); }
    else if (e.key === 'Home') slides[0]?.scrollIntoView({ behavior: 'smooth' });
    else if (e.key === 'End')  slides[slides.length - 1]?.scrollIntoView({ behavior: 'smooth' });
  });
})();
