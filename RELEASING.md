# Releasing runtz

Releases of the platform (engine + frontend + Helm chart) are driven by CI.
Publishing a GitHub Release runs `.github/workflows/runtz-pipeline-prod-k8s.yml`,
which builds the public images, pushes the Helm chart to ChartMuseum and deploys
prod. Versions follow `1.0.0-rc1 → 1.0.0-rc2 → ... → 1.0.0`, then regular semver.

The scanner CLI releases from its own repository,
[runtz-dev/runtz-cli](https://github.com/runtz-dev/runtz-cli) (GitHub Releases +
`runtzdev/runtz-cli` image). This repo no longer ships the CLI.

## 0. Prerequisites (one-time)

Configured on the `runtz-dev` org, not on your machine:

- GitHub Actions **variables**: `DOCKER_LOGIN` (Docker Hub user).
- GitHub Actions **secrets**: `DOCKER_PASS`, `CHARTMUSEUM_USER`,
  `CHARTMUSEUM_PASSWORD`.
- Self-hosted runners labelled `runtz-runners` and the private deploy image
  `runtzdev/deploy-k8s:v1` (kubectl + helm + kubeconfig).

## 1. Prepare the version (on `dev`)

1. Update `VERSION` (single source of truth), e.g. `1.0.0-rc2`.
2. Update `helm/runtz/Chart.yaml` (`version` and `appVersion`) to match.
3. Update the pinned default in `docker-compose.yml` (`RUNTZ_VERSION:-...`)
   and `.env.example`.
4. Move the `[Unreleased]` notes in `CHANGELOG.md` into a new section.
5. Open a PR into `dev` and let the dev pipeline deploy + smoke it.

## 2. Promote and publish

1. Promote `dev → main` (open a PR from `dev` into `main` and merge it).
2. Publish the GitHub Release for the tag `v$(cat VERSION)` (Release Drafter
   keeps a draft ready — set its tag to `v$(cat VERSION)` and publish, marking it
   a pre-release for `-rc` versions).

Publishing the release triggers the prod pipeline, which:

- builds and pushes multi-arch `runtzdev/runtz-engine` and
  `runtzdev/runtz-frontend` (and moves `latest` for stable versions);
- builds the cloud frontend variant to the in-cluster registry;
- lints, packages and pushes the chart to https://helm.runtz.dev;
- runs `helm upgrade --install` against the prod namespace.

## 3. After the release

- Verify the prod deploy: `https://runtz.dev` and
  `https://engine.runtz.dev/health`.
- Verify the chart: `helm repo update && helm search repo runtz`.
- Verify the image: `docker pull runtzdev/runtz-engine:$(cat VERSION)`.
- Cut the matching CLI release in
  [runtz-dev/runtz-cli](https://github.com/runtz-dev/runtz-cli) if the CLI
  changed, and update the docs site (`runtz.dev` repo) if commands or versions
  changed.

## Manual fallback

If CI is unavailable, the same steps can be run by hand from a checkout with
Docker + helm + goreleaser: `make release-images` (images) and
`make helm-release` (chart). Deploy with `helm upgrade --install runtz
runtz/runtz -n prod -f helm/environments/prod/values.yaml`.
