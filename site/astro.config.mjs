import { defineConfig } from 'astro/config';
import { unified } from '@astrojs/markdown-remark';
import { rewriteRepoLinks } from './src/lib/rewrite-repo-links.mjs';

export default defineConfig({
  site: 'https://gangline.ai',
  outDir: './dist',
  markdown: {
    processor: unified({ remarkPlugins: [rewriteRepoLinks] }),
    syntaxHighlight: false,
  },
  build: { format: 'file' },
});
