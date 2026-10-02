// SPDX-License-Identifier: Apache-2.0
// The home page and every docs page carry a left navigation column. A rewrite
// that drops it still builds and links cleanly, so the build checks for it.
import { readFileSync, readdirSync } from 'node:fs';
import { join, resolve } from 'node:path';

const root = resolve('dist');
const pages = ['index.html', ...readdirSync(join(root, 'docs')).filter((name) => name.endsWith('.html')).map((name) => `docs/${name}`)];
const failures = [];
for (const page of pages) {
  const html = readFileSync(join(root, page), 'utf8');
  const column = html.match(/<(aside|header)\b[^>]*\bclass="(?:side|sticky)"[^>]*>([\s\S]*?)<\/\1>/);
  const nav = column && column[2].match(/<nav\b[^>]*\baria-label=(['"])[^'"]+\1[^>]*>([\s\S]*?)<\/nav>/);
  if (!nav) failures.push(`${page}: no labelled <nav> in the left column`);
  else if (!/<a\b[^>]*\bhref=/.test(nav[2])) failures.push(`${page}: left-column <nav> has no links`);
}
for (const failure of failures) console.error(`site: missing navigation ${failure}`);
console.log(`site: navigation column checked on ${pages.length} pages, ${failures.length} missing`);
if (failures.length) process.exitCode = 1;
