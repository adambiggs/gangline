export const groups = [
  ['Getting started', [['readme', 'Overview'], ['quickstart', 'Your first team']]],
  ['Guides', [['guides', 'Work with a team'], ['troubleshooting', 'Troubleshooting']]],
  ['Concepts', [['concepts', 'How a team works']]],
  ['Reference', [['reference', 'CLI and configuration']]],
  ['Internals and contributing', [['internals', 'Internals'], ['contributing', 'Contributing'], ['security', 'Security']]],
];

export const sources = {
  readme: 'README.md',
  contributing: 'CONTRIBUTING.md',
  security: 'SECURITY.md',
  architecture: 'ARCHITECTURE.md',
  concepts: 'docs/concepts.md',
  design: 'docs/design.md',
  guides: 'docs/guides.md',
  internals: 'docs/internals.md',
  operations: 'docs/operations.md',
  quickstart: 'docs/quickstart.md',
  reference: 'docs/reference.md',
  troubleshooting: 'docs/troubleshooting.md',
};

export const slug = (id) => id === 'readme' ? '' : `${id}.html`;
export const href = (id) => `/docs/${slug(id)}`;
