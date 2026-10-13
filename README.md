# semantic-release-test

## Helm chart release proof of concept

This repository demonstrates independent semantic releases for charts in a monorepo using go-semantic-release with a chart-aware GitHub provider plugin.

Each chart has its own Conventional Commit history and release tags:

- `charts/api/vX.Y.Z`
- `charts/worker/vX.Y.Z`
- `charts/web/vX.Y.Z`
- `charts/scheduler/vX.Y.Z`

The GitHub Actions workflow runs one release job per chart on pushes to `main`. The provider scopes commit lookup and release discovery to the selected chart. A release updates only that chart's `Chart.yaml`, creates its GitHub Release, and publishes to Docker Hub as `gaurarpit/<chart>`.

Use chart paths in commit scopes for readable history, for example:

```text
feat(api): add request metrics
fix(worker): handle shutdown signal
```

The provider uses GitHub's commit-list path filter, so changed files determine which chart receives a release. A commit touching more than one chart can release each affected chart. The standard go-semantic-release commit analyzer determines the bump from Conventional Commit types and breaking-change markers.

## Test in GitHub

1. Push this folder to a disposable GitHub repository with the default branch named `main`.
2. Add repository Actions secrets `DOCKERHUB_USERNAME` (`gaurarpit`) and `DOCKERHUB_TOKEN` (a Docker Hub access token with write access). The token is used only by the workflow and should not be committed or shared in chat.
3. Enable GitHub Actions and allow write access to repository contents. The workflow uses the repository-scoped `GITHUB_TOKEN` for GitHub Releases; no GitHub personal access token is required.
4. Push a change under one chart using a Conventional Commit message, such as `feat(api): add readiness probe`.
5. Confirm only that chart receives a `charts/<name>/vX.Y.Z` tag/release and a Docker Hub OCI package at `oci://registry-1.docker.io/gaurarpit/<name>`. `Chart.yaml` in the source branch is not committed back by this demo; the released package uses the updated version produced in the workflow workspace.

This is a proof of concept, not a production-ready chart validation pipeline.
