import { defineCollection } from 'astro:content';
import { glob } from 'astro/loaders';

export const collections = {
  docs: defineCollection({
    loader: glob({
      base: '..',
      pattern: ['docs/*.md', 'README.md', 'CONTRIBUTING.md', 'SECURITY.md', 'ARCHITECTURE.md'],
      generateId: ({ entry }) => entry.split('/').at(-1).replace(/\.md$/, '').toLowerCase(),
    }),
  }),
};
