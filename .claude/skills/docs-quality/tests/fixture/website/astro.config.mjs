// Trimmed stand-in for the real config: only the sidebar shape is read.
export default {
  sidebar: [
    { label: 'Tutorial', items: [{ autogenerate: { directory: 'tutorial' } }] },
    { label: 'Guides', items: [{ autogenerate: { directory: 'guides/frontend' } }] },
    // Two slugs, so the parser has to read past the first: templates exists and
    // must stay quiet, missing-page does not and must be reported.
    {
      label: 'Reference',
      translations: { ja: 'リファレンス' },
      items: ['guides/frontend/templates', 'reference/missing-page'],
    },
    { label: 'Gone', items: [{ autogenerate: { directory: 'guides/vanished' } }] },
  ],
};
