// SPDX-License-Identifier: Apache-2.0
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';

const root = resolve('dist');
const files = (dir) => readdirSync(dir, { withFileTypes: true }).flatMap((item) => {
  const path = join(dir, item.name);
  return item.isDirectory() ? files(path) : item.name.endsWith('.html') ? [path] : [];
});
const failures = [];
let count = 0;
for (const file of files(root)) {
  const html = readFileSync(file, 'utf8');
  for (const [, name, , value] of html.matchAll(/\b(href|src|srcset)=(['"])(.*?)\2/gi)) {
    const links = name.toLowerCase() === 'srcset'
      ? value.split(',').map((item) => item.trim().split(/\s+/)[0]) : [value];
    for (const link of links) {
      if (/^(?:[a-z][a-z\d+.-]*:|\/\/)/i.test(link)) continue;
      count++;
      const url = new URL(link, 'https://gangline.ai/' + file.slice(root.length + 1));
      let target = url.pathname.startsWith('/') ? join(root, decodeURIComponent(url.pathname)) : resolve(dirname(file), decodeURIComponent(url.pathname));
      if (existsSync(target) && statSync(target).isDirectory()) target = join(target, 'index.html');
      if (!existsSync(target)) {
        failures.push(`${file.slice(root.length + 1)}: ${link} (missing file)`);
        continue;
      }
      if (url.hash && target.endsWith('.html')) {
        const ids = new Set([...readFileSync(target, 'utf8').matchAll(/\bid=(['"])(.*?)\1/g)].map((match) => match[2]));
        if (!ids.has(decodeURIComponent(url.hash.slice(1)))) failures.push(`${file.slice(root.length + 1)}: ${link} (missing anchor)`);
      }
    }
  }
}
for (const failure of failures) console.error(`site: broken link ${failure}`);
console.log(`site: ${files(root).length} pages, ${count} local links, ${failures.length} broken`);
if (failures.length) process.exitCode = 1;
