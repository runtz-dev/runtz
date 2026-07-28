<picture>
  <source media="(prefers-color-scheme: dark)" srcset="frontend/public/brand/runtz-logo-dark.svg">
  <img src="frontend/public/brand/runtz-logo-light.svg" alt="runtz" width="220">
</picture>

# runtz

[![License: BUSL-1.1](https://img.shields.io/badge/License-BUSL--1.1-blue.svg)](LICENSE)
[![Docker Hub](https://img.shields.io/badge/Docker%20Hub-runtzdev-2496ED?logo=docker&logoColor=white)](https://hub.docker.com/u/runtzdev)
[![Helm](https://img.shields.io/badge/Helm-helm.runtz.dev-0F1689?logo=helm&logoColor=white)](https://helm.runtz.dev)
[![Docs](https://img.shields.io/badge/Docs-runtz.dev-2f7eff)](https://runtz.dev/home/docs)

runtz is a source-available DevSecOps scans platform. This repository holds the
web platform and backend engine:

- `frontend`: Next.js, React and shadcn/ui web platform.
- `engine`: Go backend engine used by the frontend and CLI.

The scanner CLI lives in its own repository,
[runtz-dev/runtz-cli](https://github.com/runtz-dev/runtz-cli), so it can evolve
and release independently. The MCP server lives in
[runtz-dev/runtz-mcp](https://github.com/runtz-dev/runtz-mcp).

## Current Scope

Implemented:

- First-run admin and workspace setup.
- Login with JWT and optional Google and GitHub Sign-In.
- Stripe Billing checkout, customer portal and webhook handling for paid plans.
- Self-hosted Pro/Enterprise activation with one installation per license and central heartbeat validation.
- Workspaces, users and profile settings.
- Workspace-scoped API keys for CLI ingestion.
- SCA scan ingestion and SCA dashboard.
- Go CLI for npm `package.json` SCA using GitHub Global Security Advisories.
- SAST finding ingestion and dashboard.
- Host package scan ingestion and dashboard for dpkg-based systems.
- Container package scan ingestion and dashboard for dpkg-based images.
- Kubernetes cluster/manifest finding ingestion and dashboard.
- Docker Compose and Helm deployments for self-hosting.

Visible but not implemented yet:

- DAST

## Install the CLI

```bash
curl -fsSL https://runtz.dev/install.sh | bash
runtz version
```

Or download a binary from the
[releases page](https://github.com/runtz-dev/runtz-cli/releases), or run it from
the Docker image:

```bash
docker run --rm runtzdev/runtz-cli:latest --help
```

Full CLI documentation, the `runtz update` self-updater and the CI/CD severity
gates (`--critical-threshold`, `--high-threshold`, ...) are documented in the
[runtz-cli](https://github.com/runtz-dev/runtz-cli) repository.

## Self-Host with Docker Compose

Uses the published images (`runtzdev/runtz-engine`, `runtzdev/runtz-frontend`):

```bash
git clone https://github.com/runtz-dev/runtz
cd runtz
cp .env.example .env
# fill JWT_SECRET (openssl rand -base64 32) and RUNTZ_INGEST_TOKEN (openssl rand -base64 24)
docker compose up -d
```

Open:

- Frontend: http://localhost:3000
- Engine health: http://localhost:8080/health
- MongoDB: localhost:27017

If `8080`, `3000` or `27017` are already in use, change `BACKEND_PORT`,
`FRONTEND_PORT`, `MONGODB_PORT` and `NEXT_PUBLIC_API_URL` in `.env`.

On the first access, create the admin user and the initial workspace.

To build the images from source instead:

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
```

For Google Sign-In, set `GOOGLE_CLIENT_ID` on the engine — the frontend picks
it up at runtime.

For paid plans, Stripe is only configured on the central/cloud engine.
Self-hosted installations start Checkout from the in-app Billing screen,
return to the same installation after payment, and activate Pro or Enterprise
automatically through the fixed central engine at `https://engine.runtz.dev`.

## Deploy with Helm

```bash
helm repo add runtz https://helm.runtz.dev
helm repo update

cp helm/runtz/values.secrets.example.yaml values.secrets.yaml
# fill jwtSecret and ingestToken

helm install runtz runtz/runtz -f values.secrets.yaml
```

Non-secret config lives in the chart values and is rendered as ConfigMaps;
secrets live in `values.secrets.yaml` (ignored by git and by Helm packaging)
or in an existing Kubernetes Secret via `backend.secrets.existingSecret`.
See [helm/runtz/README.md](helm/runtz/README.md) for the full reference, and
[helm/environments/](helm/environments/) for how we run our own dev/prod
environments from the same chart.

## Run the CLI

With the CLI installed and pointed at this engine, the five scans are:

```bash
runtz sca ./         --endpoint http://localhost:8080 --token rtz_live_...
runtz sast ./        --endpoint http://localhost:8080 --token rtz_live_...
runtz host           --endpoint http://localhost:8080 --token rtz_live_...
runtz container ubuntu:22.04 --endpoint http://localhost:8080 --token rtz_live_...
runtz k8s            --endpoint http://localhost:8080 --token rtz_live_...
```

`--endpoint`/`--token` also read from `RUNTZ_ENDPOINT`/`RUNTZ_TOKEN`. Every scan
accepts severity gates (`--critical-threshold`, `--high-threshold`, ...) that
exit non-zero to fail a CI pipeline. See the
[runtz-cli](https://github.com/runtz-dev/runtz-cli) repository for the full flag
and environment reference, `runtz update`, and CI examples.

## Development

Backend engine:

```bash
cd engine
go test ./...
JWT_SECRET=$(openssl rand -base64 32) RUNTZ_INGEST_TOKEN=$(openssl rand -base64 24) \
  go run ./cmd/server
```

Frontend:

```bash
cd frontend
npm install
npm run dev
```

Releases are manual for now — see [RELEASING.md](RELEASING.md). Versions
follow `1.0.0-rc1 → 1.0.0-rc2 → ... → 1.0.0`, then regular semver
([CHANGELOG.md](CHANGELOG.md)).

## API Overview

- `GET /health`
- `GET /api/v1/setup/status`
- `POST /api/v1/setup`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/google`
- `GET /api/v1/me`
- `GET /api/v1/workspaces`
- `POST /api/v1/workspaces`
- `GET /api/v1/api-keys`
- `POST /api/v1/api-keys`
- `PATCH /api/v1/api-keys/{id}/revoke`
- `POST /api/v1/billing/checkout`
- `POST /api/v1/billing/portal`
- `GET /api/v1/billing/status`
- `GET /api/v1/billing/checkout-session/{id}`
- `POST /api/v1/billing/webhook`
- `POST /api/v1/license/activate`
- `POST /api/v1/license/refresh`
- `POST /api/v1/licenses/validate`
- `GET /api/v1/users`
- `POST /api/v1/users`
- `PATCH /api/v1/users/{id}`
- `POST /api/v1/users/{id}/invite`
- `POST /api/v1/ingest/sca`
- `POST /api/v1/ingest/sast`
- `POST /api/v1/ingest/host`
- `POST /api/v1/ingest/container`
- `POST /api/v1/ingest/k8s`
- `GET /api/v1/scans/sca`
- `GET /api/v1/scans/sca/{id}`
- `GET /api/v1/scans/sast`
- `GET /api/v1/scans/sast/{id}`
- `GET /api/v1/scans/host`
- `GET /api/v1/scans/host/{id}`
- `GET /api/v1/scans/container`
- `GET /api/v1/scans/container/{id}`
- `GET /api/v1/scans/k8s`
- `GET /api/v1/scans/k8s/{id}`

## Community

- [Contributing](CONTRIBUTING.md)
- [Security policy](SECURITY.md) — report vulnerabilities to security@runtz.dev
- [Code of Conduct](CODE_OF_CONDUCT.md)

## License

runtz is **source-available** under the
[Business Source License 1.1](LICENSE) (the same license model used by
Terraform):

- **Free to use, self-host and modify** — including production use inside
  your organization.
- **What you cannot do:** offer runtz to third parties as a competing hosted
  or embedded commercial product. Only RAW DEVOPS LTDA may commercialize the
  platform.
- **It becomes open source over time:** each release converts to the
  [MPL-2.0](https://www.mozilla.org/en-US/MPL/2.0/) four years after it is
  published.

### Paid plans and fair use

Pro and Enterprise are gated by a cryptographic license: the central engine at
`https://engine.runtz.dev` signs short-lived certificates that every runtz
binary verifies against a public key compiled into it. Modifying the source to
remove, weaken or bypass that verification — to unlock paid features without an
active subscription, point the central-engine URL at a look-alike signer, or
forge a stored license — breaches the BUSL-1.1. See [NOTICE](NOTICE). The code
is open so you can audit and self-host it, not so paid plans can be cracked.

For commercial licensing, contact licensing@runtz.dev.

---

Copyright © 2026 Runtz · RAW DEVOPS LTDA (CNPJ 51.460.107/0001-53). All rights reserved.
