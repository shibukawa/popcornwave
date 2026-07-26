---
id: ui:devidp-login
type: ui
title: Development IdP Login Screen
---
The requirement:contrib-devidp login screen lists the data:devidp-config roster so a developer signs in by choosing an identity, with no credential input anywhere on the page.

```yaml
ui:
  root:
    kind: browser
    id: screen.devidp-login
    title: Select a development user
    children:
      - kind: banner
        id: dev-warning
        label: Development identity provider; no password is checked
        state: always visible
      - kind: text
        id: client-context
        label: requesting client id and granted scopes
      - kind: list
        id: user-roster
        columns:
          - display_name
          - subject
          - extra_claims summary
        children:
          - kind: button
            id: select-user
            label: Sign in as this user
            action: flow:devidp-user-selection
      - kind: button
        id: cancel
        label: Cancel
        action: return access_denied to the client
rules:
  - no password, passcode, or one-time-code field exists on this screen
  - selection is a POST carrying the pending authorization identifier, protected by policy:csrf-protection
  - the roster is rendered from configuration only and never from request parameters
  - claim values are escaped as text under policy:template-escaping
  - the screen is skipped entirely when automatic login is configured
  - the warning banner cannot be disabled by configuration
```
