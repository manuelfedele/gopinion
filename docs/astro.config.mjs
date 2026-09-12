import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://manuelfedele.github.io',
  base: '/gopinion',
  integrations: [
    starlight({
      title: 'GOpinion',
      description: 'The fail-closed framework for Go web applications.',
      logo: {
        src: './src/assets/logo.svg',
        replacesTitle: true,
      },
      favicon: '/favicon.svg',
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/manuelfedele/gopinion',
        },
      ],
      editLink: {
        baseUrl: 'https://github.com/manuelfedele/gopinion/edit/main/docs/',
      },
      lastUpdated: true,
      customCss: ['./src/styles/custom.css'],
      sidebar: [
        {
          label: 'Start Here',
          items: [
            { label: 'Overview', slug: 'index' },
            { label: 'Installation', slug: 'getting-started/installation' },
            { label: 'Your First App', slug: 'getting-started/first-app' },
            { label: 'Policy Model', slug: 'getting-started/policy-model' },
          ],
        },
        {
          label: 'Guides',
          items: [{ autogenerate: { directory: 'guides' } }],
        },
        {
          label: 'Reference',
          items: [{ autogenerate: { directory: 'reference' } }],
        },
      ],
    }),
  ],
});
