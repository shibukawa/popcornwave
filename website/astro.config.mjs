// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightLlmsTxt from 'starlight-llms-txt';
import { satteri } from '@astrojs/markdown-satteri';
import { satteriBaseLinks } from './src/plugins/satteri-base-links.mjs';

// Deployed to https://shibukawa.github.io/popcornwave/ by .github/workflows/docs.yml.
// This is the only place the base path is declared.
const base = '/popcornwave';

export default defineConfig({
  site: 'https://shibukawa.github.io',
  base,
  markdown: {
    // Lets content link with plain `/guides/testing/` instead of repeating `base`.
    processor: satteri({ hastPlugins: [satteriBaseLinks({ base })] }),
  },
  integrations: [
    starlight({
      title: 'Popcorn Wave',
      description:
        'A small, TinyGo-oriented web application framework for Go, built directly on net/http.',
      logo: {
        src: './src/assets/logo.png',
        alt: 'Popcorn Wave',
        replacesTitle: true,
      },
      defaultLocale: 'root',
      locales: {
        root: { label: 'English', lang: 'en' },
        ja: { label: '日本語', lang: 'ja' },
      },
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/shibukawa/popcornwave',
        },
      ],
      editLink: {
        baseUrl: 'https://github.com/shibukawa/popcornwave/edit/main/website/',
      },
      // Groups are declared once; pages appear by dropping a Markdown file into
      // the matching directory (and its ja/ counterpart). No config change needed.
      sidebar: [
        {
          label: 'Start Here',
          translations: { ja: 'はじめに' },
          items: [{ autogenerate: { directory: 'start' } }],
        },
        {
          label: 'Guides',
          translations: { ja: 'ガイド' },
          items: [{ autogenerate: { directory: 'guides' } }],
        },
        {
          label: 'pw command',
          translations: { ja: 'pw コマンド' },
          items: [
            'pw/overview',
            {
              label: 'Project',
              translations: { ja: 'プロジェクト' },
              items: [{ autogenerate: { directory: 'pw/project' } }],
            },
            {
              label: 'Database',
              translations: { ja: 'データベース' },
              items: [{ autogenerate: { directory: 'pw/database' } }],
            },
          ],
        },
      ],
      // Publishes /llms.txt, /llms-full.txt and /llms-small.txt for LLM consumers.
      plugins: [starlightLlmsTxt()],
    }),
  ],
});
