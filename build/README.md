# Release build

This directory was adopted from
`gemyago/golang-backend-boilerplate` at
`798f0dc9fd753481d0d698d8232ea08df44185b6` and minimally adapted for the
single Sumweave binary.

```sh
make -C build test
make -C build dist
make -C build docker/local-image
```

`dist` builds the Svelte UI once, embeds it in CGO-disabled Linux amd64 and
arm64 binaries, and copies `.platform-agents/skills` to
`build/dist/platform-agents/skills`. These are the only Docker inputs. The
Dockerfile never compiles Go or JavaScript and has no default command, so an
image run with no arguments displays Cobra help.

The publication workflows push `ghcr.io/gemyago/sumweave`. `main` emits
`latest-main` and an immutable `git-commit-<sha7>` tag; trusted manual branch
publication emits a sanitized branch tag plus that commit tag. Published stable
SemVer releases remotely retag the commit image with `git-tag-*`, full version,
minor/major latest tags, and `latest`; prereleases get only `git-tag-*` and the
full prerelease tag. `cleanup-docker-images.yml` preserves stable/release tags
and their multi-platform children while expiring branch and commit-only images
after seven days.

After a successful automatic `main` image push, the workflow runs
`bash build/scripts/advance-docker-image-head.sh` in the checkout. Manual
publications do not advance this Git tag. The script reads only environment
inputs: required `SOURCE_SHA` (published commit), optional `TAG_NAME` (default
`docker-image-head`), and optional `GIT_REMOTE` (default `origin`). It requires
Git history and push credentials, leaves equal/older/divergent sources unchanged,
and fails on a concurrent tag update rather than overwriting it. Empty optional
values use their defaults. `make -C build test` exercises the real script in
disposable local Git repositories, including compare-and-swap races.
