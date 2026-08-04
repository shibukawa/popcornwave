// Trimmed stand-in for the real config: only the sidebar shape is read.
export default {
  sidebar: [
    { label: 'Tutorial', items: [{ autogenerate: { directory: 'tutorial' } }] },
    { label: 'Guides', items: [{ autogenerate: { directory: 'guides/frontend' } }] },
    { label: 'Reference', items: ['reference/missing-page'] },
    { label: 'Gone', items: [{ autogenerate: { directory: 'guides/vanished' } }] },
  ],
};
