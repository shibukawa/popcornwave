---
id: policy:asset-content-signature
type: policy
title: Asset Content Signature
---

How a static file's bytes are compared against the type its extension declares, and what the comparison may conclude when the format carries no signature at all.

```yaml
subject:
  checked: a file in the authored public tree, and any file api:public-asset-middleware serves from disk without a data:public-asset-manifest entry
  never: build output, meaning a policy:public-asset-precompression sidecar, a converted file, and a media variant
  why_output_is_exempt: the build labels what it wrote and the manifest carries that label, so re-deriving a type from the name would only test the build against itself
  disk_name_not_url: the avif variant of policy:asset-transform-matrix is stored as {url}.avif and served under a .webp URL with its own media type, so comparing bytes against the URL extension would flag the build's own output; the comparison is always against the extension of the file on disk
window:
  read: the first 64 bytes, which covers every signature below including those at a non-zero offset
  cost: nothing on the build path, because the tree walk already holds the whole file to digest it
verdicts:
  confirmed: the extension declares a signature-bearing format and the bytes carry that signature
  contradicted: the comparison failed, by either rule below
  unconstrained: the extension declares a format with no signature and no known signature is present
positive_rule:
  states: an extension in signatures whose bytes do not carry its signature is contradicted
  catches: the motivating case of requirement:asset-content-verification, a .png holding svg, since svg is text and matches no binary signature
negative_rule:
  states: an extension in signature_less whose bytes carry any signature in the table is contradicted
  why: css, js, json, and svg have nothing to assert, so the positive rule alone would exempt exactly the extensions a browser treats as executable or as same-origin markup
  catches: a .css holding a zip, a .js holding a pdf
  cost: the same table read in the other direction
signatures:
  offset_0:
    png: 89 50 4E 47 0D 0A 1A 0A
    gif: GIF87a or GIF89a
    jpeg: FF D8 FF
    bmp: BM
    tiff: 49 49 2A 00 or 4D 4D 00 2A
    ico: 00 00 01 00
    pdf: "%PDF-"
    zip_family: 50 4B 03 04, which is .zip and every container built on it
    gzip: 1F 8B
    zstd: 28 B5 2F FD
    wasm: 00 61 73 6D
    woff: wOFF
    woff2: wOF2
    ttf: 00 01 00 00
    otf: OTTO
    ogg: OggS
    flac: fLaC
  offset_4:
    isobmff: ftyp, whose brand then separates mp4, avif, and heic
  offset_8:
    webp: WEBP, after RIFF at offset 0
  absent_by_design:
    brotli: a .br sidecar has no magic number at all, which is a second reason build output is exempt rather than checked
signature_less:
  extensions: .css .js .mjs .json .map .txt .md .csv .html .xml .svg
  meaning: subject to the negative rule only
no_text_parsing:
  rejected: a second tier that parses a signature-less file to confirm its shape, such as an svg whose root element must be svg or a json that must parse
  reason: a correct text check is a parser per format, which is a large surface for the narrow gain of catching a file that is malformed rather than mislabelled
  consequence: a signature-less extension is judged by the negative rule alone, which is the cheap half and the half that catches a disguised binary
  the_case_it_gives_up: a .svg holding plain prose or broken xml ships, and fails at the consumer instead of at the build
  the_case_it_does_not: a .png holding svg is still caught, because that verdict comes from the png signature being absent and needs no reading of the svg
edges:
  empty_file: contradicted when the extension declares a signature, because zero bytes are not a png
  bom: never skipped, since every rule here reads a binary signature at a fixed offset
  unknown_extension: unconstrained and silent, because the table is not a completeness claim and guessing would fail a build over a name nobody declared
  double_extension: only the final extension is read, so a.tar.gz is judged as gzip
determinism: the verdict is a function of the bytes alone, so policy:generated-artifacts --check still compares two builds of one tree
exemption:
  form: data:project-config assets.verify.allow, a list of path globs under the authored tree
  why: a check with no exemption is turned off wholesale the first time it is wrong, which costs more than the false positive did
  reported: an exempted path is named in the build report, so the list cannot go quiet
```
