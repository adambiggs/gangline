import { existsSync, statSync } from 'node:fs';
import { dirname, relative, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { href, sources } from './docs.mjs';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '../../..');
const pages = new Map(Object.entries(sources).map(([id, source]) => [source, href(id)]));
const github = 'https://github.com/adambiggs/gangline';

function rewrite(url, source) {
  if (/^(?:[a-z][a-z\d+.-]*:|\/|#)/i.test(url)) return url;
  const match = /^([^?#]*)(\?[^#]*)?(#.*)?$/.exec(url);
  if (!match?.[1]) return url;
  const target = resolve(repo, dirname(source), decodeURIComponent(match[1]));
  const path = relative(repo, target).split(sep).join('/');
  if (path.startsWith('../') || !existsSync(target)) {
    throw new Error(`${source}: missing link ${url}`);
  }
  const suffix = `${match[2] || ''}${match[3] || ''}`;
  if (pages.has(path)) return pages.get(path) + suffix;
  if (/^docs\/[^/]+\.md$/.test(path)) return href(path.slice(5, -3)) + suffix;
  if (path.startsWith('site/') && !path.startsWith('site/src/')) {
    return `/${path.slice(5)}${suffix}`;
  }
  return `${github}/${statSync(target).isDirectory() ? 'tree' : 'blob'}/main/${path}${suffix}`;
}

export function rewriteRepoLinks() {
  return (tree, file) => {
    const source = relative(repo, file.path).split(sep).join('/');
    const visit = (node) => {
      if (node.type === 'link' || node.type === 'image') node.url = rewrite(node.url, source);
      if (node.type === 'html') {
        node.value = node.value.replace(/\b(href|src|srcset)=(['"])(.*?)\2/g, (_, name, quote, value) => {
          const replaced = name === 'srcset'
            ? value.split(',').map((part) => {
                const [, url, rest] = /^(\S+)(.*)$/.exec(part.trim());
                return rewrite(url, source) + rest;
              }).join(', ')
            : rewrite(value, source);
          return `${name}=${quote}${replaced}${quote}`;
        });
      }
      for (const child of node.children || []) visit(child);
    };
    visit(tree);
  };
}
