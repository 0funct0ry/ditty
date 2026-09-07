import { defineConfig } from 'astro/config';
import mdx from '@astrojs/mdx';
import starlight from '@astrojs/starlight';

// Base defaults to '/' so `make docs-dev` / `make docs-build` / `npm run
// preview` all just work locally with no path-prefix surprises — the build
// output on disk is never actually nested under a /ditty directory, GitHub
// Pages supplies that prefix only at serve time for a project site. The
// real deploy (docs-deploy in .github/workflows/release.yml) sets
// DITTY_DOCS_BASE=/ditty explicitly before building, since that's the only
// build whose output is actually served with that prefix.
export default defineConfig({
  site: 'https://0funct0ry.github.io',
  base: process.env.DITTY_DOCS_BASE ?? '/',
  integrations: [
    starlight({
      title: 'ditty',
      description: 'Share a terminal over the web from a single Go binary.',
      favicon: '/favicon.svg',
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/0funct0ry/ditty' }],
      customCss: ['./src/styles/custom.css'],
      editLink: {
        baseUrl: 'https://github.com/0funct0ry/ditty/edit/main/docs/',
      },
      sidebar: [
        {
          label: 'Start here',
          items: ['install', 'quickstart', 'sharing-a-terminal'],
        },
        {
          label: 'Security',
          items: ['security'],
        },
        {
          label: 'Recipes',
          items: [{ autogenerate: { directory: 'recipes' } }],
        },
        {
          label: 'Reference',
          items: [
            'configuration',
            { label: 'CLI reference', items: [{ autogenerate: { directory: 'reference/cli' } }] },
            'reference/protocol',
            'themes',
          ],
        },
        {
          label: 'Guides',
          items: ['deploying-behind-a-proxy', 'docker', 'troubleshooting'],
        },
      ],
    }),
    mdx(),
  ],
});
