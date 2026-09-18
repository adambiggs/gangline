/* SPDX-License-Identifier: Apache-2.0 */
/* Gangline's background: a screen of fixed character cells, and a snowfield
 * projected onto it from above.
 *
 * The cells never move. Each frame every cell asks the scene what lies under
 * it and paints that one glyph, the way adambig.gs samples its wave field —
 * so motion is only ever a change in what a cell shows. The one exception is
 * scrolling, which slides the whole screen as a unit for parallax.
 *
 * The scene is a trail seen from overhead: the paired lines of a sled's
 * runners winding across the snow, and older passes beside them packed down
 * to a dotted line, all kept faint under the gradient. Over them drifts
 * adambig.gs's block-glyph wave field, read here as low cloud, a little slower
 * than the ground; a trail keeps clear of cloud rather than crossing it.
 * Lines are built from the box-drawing set only — ─ │ and the four round
 * corners — so a track is continuous from cell to cell however it bends.
 */
(() => {
  const canvas = document.getElementById('field');
  if (!canvas || !canvas.getContext) return;
  const ctx = canvas.getContext('2d');
  const reduced = matchMedia('(prefers-reduced-motion: reduce)');
  const dark = matchMedia('(prefers-color-scheme: dark)');

  const SIZE = 13;
  const FONT = SIZE + 'px ui-monospace, SFMono-Regular, Menlo, Consolas, monospace';
  const REACH = 240;

  /* Ground passes under the sled at this many cells per second, and the page
   * adds to it: scrolling is travelling. */
  const SPEED = 1.6, SCROLL_TRAVEL = 0.012;
  /* Parallax: how far the whole screen slides per pixel of scroll. */
  const PARALLAX = 0.16;

  /* One trail per band of world rows. Which kind it is comes from its index,
   * so the field is the same on every visit and endless in either direction. */
  const BAND = 14, SLED_SHARE = 0.34;
  const A = {
    runner: 0.14, centre: 0.06,   /* a sled: two runner lines and the team's line between */
    old: 0.07,                    /* an older pass, packed to a dotted line */
  };

  /* Cloud: adambig.gs's drifting wave field, crossing the screen a little
   * slower than the ground below it. Cells at or above CLOUD show cloud;
   * the band between CLEAR and CLOUD is left empty around it, so a trail
   * stops short of a cloud rather than running into its edge. */
  const CLOUD_SPEED = 0.62, CLOUD = 0.22, CLEAR = 0.17;
  const CLOUDS = ' ·░▒▓█';

  let W = 0, H = 0, cw = 8, ch = 16, asc = 12, cols = 0, rows = 0;
  let glyph = [], alpha = new Float32Array(0), tone = new Uint8Array(0), cloud = new Float32Array(0);
  let dx2 = new Float32Array(0), dy2 = new Float32Array(0);
  let ink = '232,240,248', line = '132,170,214';
  let styles = [[], []], styleKey = '';

  const s = {
    t: 0, last: 0, travel: 0, cur: 0,
    px: -1e4, py: -1e4, tx: -1e4, ty: -1e4,
    scroll: 0, lastY: 0, sv: 0, energy: 0,
  };

  const hash = (x, y) => {
    let h = Math.imul(x | 0, 374761393) ^ Math.imul(y | 0, 668265263);
    h = Math.imul(h ^ (h >>> 13), 1274126177);
    return ((h ^ (h >>> 16)) >>> 0) / 4294967296;
  };
  /* Split an offset into the whole cells the grid steps by and the remainder
   * the canvas is translated by. The remainder is what the whole part has
   * overshot, not what is left of the offset: the grid steps a full cell the
   * moment `o` passes one, and the translate has to give that cell back until
   * `o` catches up. Getting this backwards still drifts the right way on
   * average, but slides against itself between every step. */
  const step = (o) => { const whole = Math.floor(o); return { whole, frac: whole - o }; };

  const readColors = () => {
    const css = getComputedStyle(document.documentElement);
    ink = css.getPropertyValue('--glyph').trim() || ink;
    line = css.getPropertyValue('--trail').trim() || line;
  };

  const resize = () => {
    const dpr = Math.min(2, devicePixelRatio || 1);
    W = innerWidth; H = innerHeight;
    canvas.width = Math.round(W * dpr); canvas.height = Math.round(H * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.font = FONT;
    ctx.textBaseline = 'alphabetic';
    /* Cell pitch is the font's own advance and line height, so box glyphs in
     * neighbouring cells meet with nothing between them. */
    const m = ctx.measureText('─');
    cw = m.width || SIZE * 0.6;
    const a = m.fontBoundingBoxAscent, d = m.fontBoundingBoxDescent;
    ch = a && d ? Math.round(a + d) : Math.round(SIZE * 1.3);
    asc = a ? Math.round(a) : Math.round(SIZE * 1.05);
    cols = Math.ceil(W / cw) + 2; rows = Math.ceil(H / ch) + 3;
    glyph = new Array(cols * rows).fill(' ');
    alpha = new Float32Array(cols * rows); tone = new Uint8Array(cols * rows);
    cloud = new Float32Array(cols * rows);
    dx2 = new Float32Array(cols); dy2 = new Float32Array(rows);
    /* A still scene has no next frame to repaint the resized canvas. */
    if (reduced.matches) draw(0);
  };

  const style = (which, a) => {
    const q = Math.round(a * 100);
    if (q < 1) return null;
    const cache = styles[which];
    return cache[q] || (cache[q] = 'rgba(' + (which ? line : ink) + ',' + q / 100 + ')');
  };

  /* Where a trail crosses column x, in world rows. Two slow bends and one
   * quicker one, phased by the band, so no two trails wander alike. */
  const centre = (band, x) => {
    const p = hash(band, 77) * 6.28, q = hash(band, 91) * 6.28;
    return band * BAND + BAND * 0.5
      + Math.sin(x * 0.017 + p) * 2.8
      + Math.sin(x * 0.0071 + q) * 3.0
      + Math.sin(x * 0.041 + p * 2) * 0.35;
  };

  const put = (x, g, top, c, a, which) => {
    const y = g - top;
    if (y < 0 || y >= rows) return;
    const i = y * cols + x;
    if (cloud[i] >= CLEAR) return;
    glyph[i] = c; alpha[i] = a; tone[i] = which;
  };

  /* Lay one continuous line: ─ where it holds its row, and a round corner,
   * any │ needed, and the matching corner where it steps to another. */
  const lay = (x, from, to, top, flat, a) => {
    if (from === to) { put(x, from, top, flat, a, 1); return; }
    const down = to > from;
    put(x, from, top, down ? '╮' : '╯', a, 1);
    for (let g = Math.min(from, to) + 1; g < Math.max(from, to); g++) put(x, g, top, '│', a, 1);
    put(x, to, top, down ? '╰' : '╭', a, 1);
  };

  const draw = (dt) => {
    ctx.clearRect(0, 0, W, H);
    const t = s.t;
    s.travel += dt * SPEED * (1 + s.energy * 0.6);
    const u0 = s.travel + s.scroll * SCROLL_TRAVEL;
    const slide = step(s.scroll * PARALLAX / ch);
    const top = slide.whole - 1;
    const r2 = REACH * REACH;
    if (styleKey !== ink + line) { styleKey = ink + line; styles = [[], []]; }

    /* Screen row y is painted one cell up plus the parallax remainder. */
    for (let y = 0; y < rows; y++) { const d = (y - 1 + slide.frac) * ch + ch * 0.5 - s.py; dy2[y] = d * d; }
    for (let x = 0; x < cols; x++) { const d = x * cw + cw * 0.5 - s.px; dx2[x] = d * d; }

    /* Cloud first, everywhere. Its value is kept so the trails can keep
     * clear of it. */
    const cu = s.travel * CLOUD_SPEED + s.scroll * SCROLL_TRAVEL, en = s.energy;
    for (let y = 0; y < rows; y++) {
      const g = y + top, row = y * cols;
      for (let x = 0; x < cols; x++) {
        const u = x + cu;
        let v = Math.sin(u * 0.11 + t * 0.35) * Math.cos(g * 0.13 - t * 0.22)
          + Math.sin((u + g) * 0.07 + t * 0.15) * 0.6;
        v = (v + 1.6) / 3.2;
        let near = 0;
        if (s.cur) {
          const d2 = dx2[x] + dy2[y];
          if (d2 < r2) { near = 1 - d2 / r2; near *= near * s.cur; }
        }
        v = v * (0.62 + en * 0.3) + near * 0.35;
        const i = row + x;
        cloud[i] = v; tone[i] = 0;
        if (v < CLOUD) { glyph[i] = ' '; alpha[i] = 0; continue; }
        glyph[i] = CLOUDS[Math.min(CLOUDS.length - 1, Math.floor(v * (CLOUDS.length - 1)))];
        alpha[i] = Math.min(0.22, 0.05 + (v - CLOUD) * 0.3 + near * 0.08);
      }
    }

    /* Then the trails, in the clear between the clouds. */
    const first = Math.floor((top - BAND) / BAND), last = Math.floor((top + rows + BAND) / BAND);
    for (let band = first; band <= last; band++) {
      const sled = hash(band, 13) < SLED_SHARE;
      let prev = Math.round(centre(band, u0));
      for (let x = 0; x < cols; x++) {
        const next = Math.round(centre(band, x + 1 + u0));
        let lift = 0;
        if (s.cur) {
          const d2 = dx2[x] + dy2[Math.max(0, Math.min(rows - 1, prev - top))];
          if (d2 < r2) { const n = 1 - d2 / r2; lift = n * n * s.cur * 0.14; }
        }
        if (sled) {
          /* The team's line goes down first so a runner stepping rows lands
           * over it, never under: the runners are the lines that must hold. */
          lay(x, prev, next, top, '╌', A.centre + lift * 0.5);
          lay(x, prev - 1, next - 1, top, '─', A.runner + lift);
          lay(x, prev + 1, next + 1, top, '─', A.runner + lift);
        } else {
          lay(x, prev, next, top, '╌', A.old + lift);
        }
        prev = next;
      }
    }

    /* Paint: each row in runs of one style, so a stretch of line or a drift of
     * cloud goes down in a single call. */
    ctx.save();
    ctx.translate(0, (slide.frac - 1) * ch);
    for (let y = 0; y < rows; y++) {
      const row = y * cols, py = y * ch + asc;
      let run = '', runStyle = null, runX = 0;
      for (let x = 0; x < cols; x++) {
        const i = row + x, c = glyph[i];
        const st = c === ' ' ? null : style(tone[i], alpha[i]);
        if (st !== runStyle) {
          if (runStyle) { ctx.fillStyle = runStyle; ctx.fillText(run, runX * cw, py); }
          run = ''; runStyle = st; runX = x;
        }
        if (st) run += c;
      }
      if (runStyle) { ctx.fillStyle = runStyle; ctx.fillText(run, runX * cw, py); }
    }
    ctx.restore();
  };

  readColors();
  resize();
  if (dark.addEventListener) dark.addEventListener('change', readColors);
  addEventListener('resize', resize);

  if (reduced.matches) { draw(0); return; }

  addEventListener('pointermove', (e) => { s.tx = e.clientX; s.ty = e.clientY; s.cur = 1; }, { passive: true });
  document.addEventListener('pointerleave', () => { s.cur = 0; });
  addEventListener('scroll', () => {
    const y = scrollY; s.sv += y - s.lastY; s.lastY = y; s.scroll = y;
  }, { passive: true });

  const loop = (now) => {
    const dt = s.last ? Math.min(0.05, (now - s.last) / 1000) : 0.016;
    s.last = now; s.t += dt;
    if (s.px < -1e3) { s.px = s.tx; s.py = s.ty; }
    const ease = Math.min(1, dt * 8);
    s.px += (s.tx - s.px) * ease; s.py += (s.ty - s.py) * ease;
    s.sv *= 0.88;
    s.energy += (Math.max(-1, Math.min(1, s.sv / 500)) - s.energy) * 0.1;
    draw(dt);
    requestAnimationFrame(loop);
  };
  requestAnimationFrame(loop);
})();
