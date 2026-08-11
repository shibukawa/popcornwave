---
id: ui:devidp-device-verification
type: ui
title: Development Device Verification Screen
---
The development provider lets a browser user locate a pending device request, select a roster identity, and explicitly approve or deny it.

```yaml
ui:
  root:
    kind: browser
    id: screen.devidp-device-verification
    title: Authorize a development device
    children:
      - kind: banner
        id: dev-warning
        label: Development identity provider; no password is checked
        state: always visible
      - kind: input
        id: user-code
        label: Code shown on the device
      - kind: text
        id: authorization-context
        label: normalized user code, requesting client id, and requested scopes
      - kind: list
        id: user-roster
        columns:
          - display_name
          - subject
        children:
          - kind: button
            id: approve
            label: Approve for this user
            action: flow:oidc-device-authorization
      - kind: button
        id: deny
        label: Deny
        action: terminate with access_denied
rules:
  - verification_uri_complete pre-fills the code but still displays it for device comparison
  - an unknown, expired, or exhausted code returns one generic result
  - every mutation is a POST protected by policy:csrf-protection
  - no password or device_code field exists
  - configuration cannot disable the warning, context, or explicit decision
```
