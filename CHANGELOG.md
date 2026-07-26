# Changelog

All notable changes to runtz are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).
Until `1.0.0` ships, public builds are tagged as release candidates
(`1.0.0-rc1`, `1.0.0-rc2`, ...).

## [Unreleased]

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
