---
id: requirement:extension-distribution
type: requirement
title: Extension Distribution
---
The system:vscode extension is published as a versioned package to both marketplaces, built from decision:extension-in-repository by CI, and never hand-uploaded.

```yaml
status: pipeline implemented at .github/workflows/vscode.yml; first publish pending the registry tokens
stage: 1 of vision:editor-support, because an unpublished extension helps nobody
artifact: a vsix built from tools/vscode
registries:
  - Visual Studio Marketplace
  - Open VSX, so a VSCodium or a remote editor user is not excluded
identity:
  publisher: the same account requirement:cli-distribution already uses where a registry allows it
  name: fixed at first publish, because requirement:editor-language-registration proposes recommending it from a scaffold
versioning:
  scheme: the extension has its own semantic version and does not track the CLI version
  reason: a grammar fix must ship without a CLI release, and a CLI release must not force an extension release
  compatibility: the manifest records the minimum pw version each stage-2 feature needs, checked at activation by policy:editor-tool-execution
  stage_1: declares no pw requirement at all
trigger:
  tag: vscode-v*, distinct from the flow:cli-release v* tags, so one push publishes one thing
  reason: sharing the release trigger would republish an unchanged extension on every CLI patch
pipeline:
  - build the extension from a clean checkout
  - rebuild the decision:formatter-delivery WebAssembly module and fail if it differs from the committed one
  - run the rule:template-grammar-scopes tokenization fixtures
  - package the vsix and attach it to the tag
  - publish to both registries from the same vsix
  - a registry failure leaves the other published and is repaired by rerunning that job
contents:
  included: the manifest, the grammars, the language configuration, the snippets, the client, and the decision:formatter-delivery WebAssembly module
  excluded: any pw binary, per policy:editor-tool-execution
  license: the repository license, restated in the package
documentation:
  readme: states what each stage does and what it needs, so a stage-1 install does not read as a broken language server
  changelog: required by both registries and generated from the tag range
acceptance:
  - a published version installs and highlights with no other software present
  - the same vsix is what both registries serve
  - a CLI release publishes no extension version
  - a grammar fixture failure blocks publication
```
