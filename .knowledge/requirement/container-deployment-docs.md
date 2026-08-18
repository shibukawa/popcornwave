---
id: requirement:container-deployment-docs
type: requirement
title: Container Deployment Documentation
---
The Deployment guide explains the Dockerfile requirement:container-image-scaffold wrote, line by line, so a reader can change it rather than only run it.

```yaml
audience: actor:application-developer holding a scaffolded project and a platform to put it on
motivation:
  - a scaffolded file that is never explained is a file nobody edits and everybody copies wrong into the second project
  - rule:container-build-inputs is the one thing about this build that is not standard Go, and it is invisible until it fails
  - decision:separate-tinygo-dockerfile puts a real trade in front of the reader, and the guide is where the trade is priced
  - the current Health and Readiness page carries a Dockerfile and a Compose example it inherited for lack of a page to hold them
pages:
  container_images:
    path: guides/deployment/container-images
    order: 2
    covers:
      - why a Popcorn Web build is not go build, per rule:container-build-inputs, stated before the first Dockerfile
      - the scaffolded Dockerfile walked stage by stage, naming what each line exists for
      - the runtime stage choices of policy:container-runtime-image: the base image, nonroot, the working directory, APP_ENV, and why no secret is copied
      - .dockerignore and what breaks when an entry is removed
      - Dockerfile.tinygo, its builder image, and the rule:tinygo-container-operations shutdown and migration consequences, priced against the size it buys
      - api:cli-migrate as a separate step, because policy:migration-safety keeps startup apply out of production
      - requirement:dockerless-image-builders as one note-level admonition, placed after the host-phase section it depends on rather than at the end
    links_out_to:
      - the pw build page for cross-compiling and the TinyGo netdev registration, rather than restating them
      - the Health and Readiness page for what the probe answers
      - the Configuration page for how APP_ENV selects a file
boundary_with_the_probe_page:
  rule: the Health and Readiness page owns the probe and every platform that invokes it; this page owns the build
  consequence: the Docker, Compose, and Kubernetes subsections stay where they are, because all three answer how a platform asks the question that page is about, and splitting Compose from Kubernetes would separate two answers to one question
  moved: the multi-stage build recipe only
  why_it_had_to_move: the recipe on that page ran plain go build, so a reader who followed it got the rule:container-build-inputs failure; it was wrong rather than merely misplaced
edits_to_existing_pages:
  operational_endpoints:
    keeps: the HEALTHCHECK instruction, the probe, Compose, and Kubernetes
    loses: the multi-stage build recipe
    gains: a link to the container images page
  package:
    change: sidebar order 2 becomes 3, since the one new page sits between it and Health and Readiness
  pw_build:
    change: the cross-compiling section names the command that leaves a compilable tree rather than the narrower generation that never did, and links to the container images page
    now: both are api:cli-generate, per requirement:cli-generate-check-rename, so the section names one command and the distinction it used to draw is gone
  pw_overview:
    change: the command that stops before the compiler was added to the project command table, and is api:cli-generate
localization: an English page and its Japanese counterpart for every page above, per the parity rule the documentation site already enforces
acceptance:
  - a reader who has never seen a Popcorn Web project can say why COPY . . and go build fails, after the first section
  - every line of the scaffolded Dockerfile is accounted for by the page
  - the TinyGo section states the SIGTERM consequence plainly rather than as a caveat
  - the Compose example appears once on the site
  - no page restates the pw build cross-compiling instructions
  - the ko and Buildpacks note fits in one admonition and names no builder version
  - no page shows a Dockerfile that reaches a compiler without the host phase
non_goals:
  - a platform tutorial for a specific cloud
  - a Kubernetes manifest reference, which is the platform's documentation and not this framework's
  - an image size comparison table, which goes stale between releases
```
