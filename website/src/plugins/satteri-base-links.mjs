/**
 * Sätteri hast plugin that rewrites root-absolute internal links in content so
 * that they include the configured Astro `base`.
 *
 * Astro applies `base` to assets and to Starlight-generated navigation, but not
 * to hrefs written by hand inside a page. Without this plugin every page would
 * have to repeat `/popcornweb/` in its links, which silently breaks as soon as
 * the base changes. With it, pages link with plain `/guides/testing/`.
 *
 * @param {{ base: string }} options
 */
export function satteriBaseLinks({ base }) {
  const prefix = base.replace(/\/+$/, '');

  return {
    name: 'base-links',
    element: {
      filter: ['a'],
      visit(node, ctx) {
        if (!prefix) return;

        const href = node.properties?.href;
        if (typeof href !== 'string') return;

        // Leave external (`//host`), protocol, anchor and relative links alone.
        if (!href.startsWith('/') || href.startsWith('//')) return;
        // Already based — do not double-prefix.
        if (href === prefix || href.startsWith(`${prefix}/`)) return;

        ctx.setProperty(node, 'href', prefix + href);
      },
    },
  };
}
