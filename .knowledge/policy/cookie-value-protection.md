---
id: policy:cookie-value-protection
type: policy
title: Cookie Value Protection
---
A cookie carries application data under one of three protections, chosen by what the client may read and what it may change.

```yaml
modes:
  plain:
    client_reads: yes
    client_writes: yes
    encoding: base64url of the payload, decodable in one browser call
    use: display preferences and other client-owned state
    rule: a decoded value is request input and is validated like any other
  signed:
    client_reads: yes
    client_writes: no
    mechanism: HMAC-SHA256 over the versioned value
    use: values the client may see but not choose
  sealed:
    client_reads: no
    client_writes: no
    mechanism: AES-256-GCM
    use: anything the client must not read, including a data:session-record
format:
  layout: format version, mode tag, then the body
  encoding: base64url without padding, so no value needs cookie quoting
  authenticated_context: format version, mode, cookie name, and one caller binding string
  expiry: absolute stamp inside the authenticated payload of a protected value
keys:
  secret: 32 or more random bytes, carried as base64
  derivation: one purpose-separated subkey per mode, so one secret serves both
  rotation: the first secret writes and retired secrets only read
rules:
  - the reader fixes the mode: a value written in another mode is rejected, never decoded
  - a value moved to another cookie name or presented under another binding is rejected
  - a protected value past its embedded expiry is rejected whatever the cookie attributes say
  - a plain value carries no expiry, because a client that keeps it can also rewrite it
  - reject a secret shorter than 256 bits and an empty keyring
  - never log a cookie value, a secret, or the text of a rejected secret
  - refuse a value beyond the browser size budget instead of writing one the browser drops
  - promoting a cookie to a stronger mode invalidates values written under the weaker one
surface: api:cookie-jar
tier_selection: requirement:state-storage-tiers decides which mode a piece of state needs
session_records: policy:session-security still governs what a session cookie may hold
```
