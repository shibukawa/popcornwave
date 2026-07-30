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
      components: {
        // Applies `base` to hero action links, which come from frontmatter and
        // are therefore out of reach of the base-links Markdown plugin.
        Hero: './src/components/Hero.astro',
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
          label: 'Tutorial',
          translations: { ja: 'チュートリアル' },
          items: [{ autogenerate: { directory: 'tutorial' } }],
        },
        {
          label: 'Guides',
          translations: { ja: 'ガイド' },
          items: [
            {
              label: 'For Frontend',
              translations: { ja: 'フロントエンド' },
              items: [{ autogenerate: { directory: 'guides/frontend' } }],
            },
            {
              label: 'As a Backend',
              translations: { ja: 'バックエンド' },
              items: [{ autogenerate: { directory: 'guides/backend' } }],
            },
            {
              label: 'Architecture',
              translations: { ja: 'アーキテクチャ' },
              items: [{ autogenerate: { directory: 'guides/architecture' } }],
            },
          ],
        },
        {
          label: 'Advanced Features',
          translations: { ja: '応用機能' },
          items: [{ autogenerate: { directory: 'advanced' } }],
        },
        {
          label: 'Productivity Support',
          translations: { ja: '開発支援' },
          items: [{ autogenerate: { directory: 'productivity' } }],
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
        {
          label: 'Reference',
          translations: { ja: 'リファレンス' },
          items: [{ autogenerate: { directory: 'reference' } }],
        },
        {
          label: 'Appendix',
          translations: { ja: '付録' },
          items: [{ autogenerate: { directory: 'appendix' } }],
        },
      ],
      // Publishes /llms.txt, /llms-full.txt and /llms-small.txt for LLM consumers.
      plugins: [starlightLlmsTxt()],
    }),
  ],
});
