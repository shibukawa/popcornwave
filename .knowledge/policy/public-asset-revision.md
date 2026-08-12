---
id: policy:public-asset-revision
type: policy
title: Public Asset Revision Segment
---
An asset that kept the name its author wrote is also served under a digest of its own bytes, so a document naming it through the runtime function gets an immutable URL without the file being renamed.

```yaml
problem:
  what: requirement:derived-asset-pipeline hashes the name of every URL the build invents, and nothing else, so a stylesheet revalidates on every page load forever
  why_not_rename: hashing a name works only where every reference is rewritten; nothing rewrites a link href, and a renamed public/app.css is simply gone
  cost_without_it: one conditional round trip per authored asset per page load; the ETag keeps the body off the wire, so what is spent is latency rather than bytes
shape:
  url: mount + revision + "/" + tree path, so /public/app.css is also /public/<16 hex>/app.css
  precedent: pwbrowser serves the framework's own script set the same way, under the same policy, for the same reason
  derivation: sha256 over each representation's path and etag of one URL, NUL-separated, first 16 hex characters
  covers: the URL rather than a representation, so a change to the avif moves the segment its webp is served under
  length: middlewares.RevisionLength, read by the build rather than repeated, since a drift 404s every revisioned URL
who_gets_one:
  authored_urls: yes, this is what the policy exists for
  invented_urls: no, the name already carries the digest and a segment would say it twice under a longer path
  external_tree: no, per requirement:external-public-tree the build never read those bytes and claims no validator over them
  development: no manifest exists under the pwdev build mode, so every asset is named plainly and revalidates, which is what a loop wants
naming:
  runtime: pw.PublicAssetURL, over middlewares.PublicAssetURL, which reads the manifest and the configured mount
  template: an external declaration calling it, scaffolded by api:cli-init beside the RuntimeScriptURL one it already writes
  argument: the tree path, since the mount is runtime configuration and a template spelling it out is a second place to change; the whole URL is accepted too, so migrating a literal is mechanical
  scope: for assets nothing else renames, which is a stylesheet and a plain script; an img src and a typescript script src are rewritten by policy:asset-transform-matrix and must stay literal for the hook to claim them
serving:
  order: the whole path is looked up first, so a URL that predates revisions answers as it did and a real directory is never read as a segment
  match: the segment must equal the entry's own revision, not merely look like one
  stale: not found, never answered from the current tree, which is what makes the immutable promise sound rather than a way to serve new bytes under an old name
  shared: the resolution and the policy come back together from the shared leaf, so the two transports cannot disagree; a transport reading entry.CacheControl directly would serve the revalidating policy for a URL a document promised was immutable
rules:
  - a revisioned answer still carries its ETag, so a client that revalidates anyway is answered 304 rather than resent the body
  - the plain URL keeps working and keeps its revalidating policy, so this is additive and no existing page breaks
  - an unknown name is returned as the plain URL rather than reshaped, so a bad reference 404s where a reader can see it
open_questions:
  - whether api:cli-doctor should report an authored asset a document still names by literal, which is the silent case: the page renders and every load spends a round trip
```
