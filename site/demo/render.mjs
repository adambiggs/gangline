// SPDX-License-Identifier: Apache-2.0
// npm install --prefix /your/scratch playwright
// PLAYWRIGHT_MODULE=/your/scratch/node_modules/playwright CHROMIUM=/path/to/chromium \
//   FFMPEG=/path/to/ffmpeg node site/demo/render.mjs /your/scratch/frames
import { createRequire } from 'node:module';
import { readFileSync, writeFileSync, mkdirSync, copyFileSync } from 'node:fs';
import { resolve, dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const here = dirname(fileURLToPath(import.meta.url));
const output = resolve(here, '..');
const frames = resolve(process.argv[2]);
mkdirSync(frames, { recursive: true });
const slides = JSON.parse(readFileSync(join(here, 'session.json')));
const receipts = JSON.parse(readFileSync(join(here, 'receipts.json')));
// A shown envelope must name a registered sender and have a native receipt.
for (const slide of slides.filter(s => s.envelope)) {
  const e = receipts.find(r => r.envelope?.id === slide.envelope)?.envelope;
  if (!e || e.from.kind !== 'agent' || !e.from.hitch_id ||
      !receipts.some(r => r.type === 'delivery_succeeded' && r.id === e.id && r.hitch_id === e.recipient))
    throw Error(`Missing verified delivery: ${slide.envelope}`);
  const expected = `[gang:${e.from.name}#${e.id}]\n${e.message.text}\n[/gang:${e.from.name}#${e.id}]`;
  if (slide.text !== expected) throw Error(`Changed message: ${slide.envelope}`);
}
const escape = s => s.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
const font = readFileSync(join(output, 'fonts/jetbrains-mono-latin.woff2')).toString('base64');
const browser = await chromium.launch({ headless: true, ...(process.env.CHROMIUM ? { executablePath: process.env.CHROMIUM } : {}) });
function ffmpeg(args) {
  const result = spawnSync(process.env.FFMPEG || 'ffmpeg', ['-hide_banner', '-loglevel', 'error', '-y', ...args], { stdio: 'inherit' });
  if (result.status !== 0) throw Error(`ffmpeg failed: ${result.status}`);
}
try {
  const page = await browser.newPage({ viewport: { width: 560, height: 724 }, deviceScaleFactor: 1 });
  for (const light of [false, true]) {
    const suffix = light ? '-light' : '';
    const colors = light ? ['#e9f0f8', '#12202e', '#42546a', '#b4531c', '#b7c9db'] : ['#06090f', '#e9eef7', '#b2bed0', '#f0a23c', '#324052'];
    const [bg, ink, soft, accent, border] = colors;
    const files = [];
    for (const [i, slide] of slides.entries()) {
      await page.setContent(`<!doctype html><meta charset="utf-8"><style>
        @font-face{font-family:mono;src:url(data:font/woff2;base64,${font})}
        *{box-sizing:border-box}body{margin:0;background:${bg};color:${ink};font:25.5px/1.4 mono}
        header{padding:24px;border-bottom:1px solid ${border};font-size:22px;color:${soft}}
        header b{color:${accent};font-weight:400}main{padding:24px}
        h1{margin:0 0 8px;font:26px/1.4 mono;color:${accent}}
        h2{margin:0 0 24px;font:22px/1.4 mono;color:${soft}}
        pre{margin:0;white-space:pre-wrap;overflow-wrap:anywhere;font:inherit}
        footer{position:absolute;bottom:64px;left:0;right:0;padding:20px 24px;border-top:1px solid ${border};font-size:18px;color:${soft}}
        .progress{display:flex;gap:8px;margin-top:12px}.progress i{height:3px;flex:1;background:${border}}.progress i.active{background:${accent}}
      </style><header>Claude Code <b>↔</b> Codex</header><main>
      <h1>${i + 1}. ${escape(slide.title)}</h1><h2>${escape(slide.harness)}</h2><pre>${escape(slide.text)}</pre></main>
      <footer>Recorded session · reflowed excerpts<div class="progress">${slides.map((_, n) => `<i class="${n === i ? 'active' : ''}"></i>`).join('')}</div></footer>`);
      await page.evaluate(() => document.fonts.ready);
      const fits = await page.evaluate(() => {
        const pre = document.querySelector('pre').getBoundingClientRect();
        const footer = document.querySelector('footer').getBoundingClientRect();
        return pre.bottom + 24 <= footer.top && document.documentElement.scrollWidth === innerWidth;
      });
      if (!fits) throw Error(`Text does not fit: ${slide.title}${suffix}`);
      const file = join(frames, `frame${suffix}-${i}.png`);
      await page.screenshot({ path: file });
      files.push(`file '${file.replaceAll("'", "'\\''")}'\nduration ${slide.seconds}`);
    }
    files.push(`file '${join(frames, `frame${suffix}-${slides.length - 1}.png`).replaceAll("'", "'\\''")}'`);
    const list = join(frames, `frames${suffix}.txt`);
    writeFileSync(list, files.join('\n') + '\n');
    ffmpeg(['-f', 'concat', '-safe', '0', '-i', list, '-vf', 'fps=12', '-c:v', 'libx264', '-crf', '18', '-pix_fmt', 'yuv420p', '-movflags', '+faststart', join(output, `demo${suffix}.mp4`)]);
    ffmpeg(['-i', join(output, `demo${suffix}.mp4`), '-vf', 'fps=1,split[a][b];[a]palettegen[p];[b][p]paletteuse', '-loop', '0', join(output, `demo${suffix}.gif`)]);
    ffmpeg(['-i', join(frames, `frame${suffix}-0.png`), '-frames:v', '1', '-q:v', '2', join(output, `demo-poster${suffix}.jpg`)]);
  }
} finally { await browser.close(); }
writeFileSync(join(output, 'demo.txt'), 'Real native session excerpts, reflowed for readability.\n\n' + slides.map(s => `${s.title} — ${s.harness}\n\n${s.text}`).join('\n\n') + '\n');
copyFileSync(join(here, 'session.json'), join(frames, 'session.json'));
console.log('PASS: verified envelopes and all text fits; rendered both themes.');
