# Changelog

All notable changes to runtz are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).
Until `1.0.0` ships, public builds are tagged as release candidates
(`1.0.0-rc1`, `1.0.0-rc2`, ...).

## [Unreleased]

### Added

- Settings → **Usage**: weekly (7-day) and monthly (30-day) counters of scans
  sent, with a per-scan-type breakdown, backed by `GET /api/v1/usage`.
- Account menu: shortcut back to the onboarding guide.
- Chart: `backend.existingConfigMap` layers a pre-created ConfigMap of
  non-secret engine variables (OAuth client ids, Stripe price ids) on top of
  the chart's own, keeping deployment-specific config out of the repository.

### Fixed

- Billing read the subscription renewal date from `current_period_end` on the
  subscription, which Stripe moved onto the subscription items in API version
  `2025-03-31.basil`; both payload shapes are now handled, so the renewal date
  shows up again.

### Changed

- Settings → Billing is now a single card: current plan, cycle and one upgrade
  button wired to Stripe Checkout (plus the portal for paying customers).
- Sessions last 7 days instead of 24 hours, and opening `/login` with a valid
  session goes straight to the app instead of asking for credentials again.
- Sidebar: the wordmark (which already ends in the cursor block) no longer
  shows the mark tile beside it; the tile appears only when collapsed to icons.

## [1.0.0-rc4] - 2026-08-01

### Changed

- Onboarding now scans the host itself (`runtz host`) instead of running an
  SCA scan, and only shows `--endpoint` for self-hosted deployments.

### Added

- API keys support an optional expiration; the onboarding key now expires
  after 90 days.

## [1.0.0-rc2] - 2026-07-28

### Changed

- CLI installer moved from the standalone `get.runtz.dev` host to
  `https://runtz.dev/install.sh`, served by the existing landing app.

## [1.0.0-rc1] - 2026-07-11

First public release candidate.

### Added

- Engine (Go + MongoDB): first-run setup, JWT auth, optional Google and GitHub
  sign-in, workspaces, users, workspace-scoped API keys.
- Stripe Billing: checkout, customer portal and webhook handling for paid
  plans; self-hosted Pro/Enterprise activation with license signing and
  central heartbeat validation.
- Scan ingestion and dashboards for SCA, SAST, host packages, container
  packages and Kubernetes findings.
- CLI scanners: `sca`, `sast`, `host`, `container`, `k8s`, with environment
  variable configuration and CI-friendly output.
- Web platform: Next.js + shadcn/ui frontend.
- Distribution: Docker Hub images (`runtzdev/runtz-engine`,
  `runtzdev/runtz-frontend`, `runtzdev/runtz-cli`), Docker Compose for
  self-hosting, Helm chart, and a `curl | bash` CLI installer.

### Security

- The engine refuses to start with empty or placeholder `JWT_SECRET` /
  `RUNTZ_INGEST_TOKEN` values.

[Unreleased]: https://github.com/runtz-dev/runtz/compare/v1.0.0-rc1...HEAD
[1.0.0-rc1]: https://github.com/runtz-dev/runtz/releases/tag/v1.0.0-rc1
