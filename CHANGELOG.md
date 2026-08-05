# Changelog

All notable changes to runtz are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).
Until `1.0.0` ships, public builds are tagged as release candidates
(`1.0.0-rc1`, `1.0.0-rc2`, ...).

## [Unreleased]

## [1.0.0-rc9] - 2026-08-05

### Added

- Rolling scan allowances per plan, enforced at ingest: Free workspaces get
  2,500 scans per 7 days and 10,000 per 30 days; Pro and Enterprise get
  1,000,000 in both windows. **Ingestion returns HTTP 429 once either window is
  exhausted** — CI that scans above the Free allowance now fails until the
  window rolls or the workspace upgrades. Playground-generated data does not
  count towards usage.
- **Settings → Usage** shows the plan, both windows as progress bars with a
  `scans/limit` counter, and a refresh action.
- API keys can be renamed and permanently deleted, from a top-level create flow
  and a searchable list with an Edit/Delete menu.
- An "Access host scanning page" link and a "Don't show onboarding again"
  checkbox in onboarding. Dismissing it persists completion; the page stays
  reachable by direct URL and from the account menu.

### Fixed

- A host whose scan reported `"vulnerabilities": null` no longer breaks its
  detail page. macOS/Homebrew scans have no OSV advisory feed, so the CLI sent
  `null`, the engine stored it verbatim, and the page crashed on
  `vulnerabilities.length` with "This page couldn't load". Ingest now normalizes
  `dependencies` and `vulnerabilities` to empty arrays, so every client reads a
  list instead of guarding each access.

### Removed

- The unused `CVETable` component, which was the one that crashed above.

## [1.0.0-rc8] - 2026-08-04

Supersedes `1.0.0-rc7`, which was tagged but never produced its own artifacts:
the version files still read `1.0.0-rc5`, so that build overwrote the
`1.0.0-rc5` images and chart instead of publishing new ones. `1.0.0-rc8` is the
first build of this work under its own version.

### Removed

- **Breaking:** `JWT_SECRET` and `RUNTZ_INGEST_TOKEN` no longer exist. Both
  `docker compose up -d` and `helm install` now run with no secrets at all —
  browser sessions, API keys and email login codes are issued and stored by the
  engine. Existing deployments can drop both variables; they are ignored.
- **Breaking:** the ingest endpoints (`POST /api/v1/ingest/*`) require an API
  key. Set `RUNTZ_TOKEN` in CI to an `rtz_...` key created in **Settings → API
  keys**. The workspace now always comes from the key, so `workspaceId` /
  `workspace` in the request body are accepted and ignored — a key issued for
  one workspace can no longer write into another.

### Added

- `POST /api/v1/auth/logout` ends a session server-side; signing out now
  actually revokes, rather than only forgetting the token client-side.
- Password login is rate limited: ten wrong passwords lock the username for an
  hour, reusing the lockout that already guarded email login codes.

### Changed

- **Breaking:** browser sessions moved from a JWT in `localStorage` to an
  opaque token in an `HttpOnly`, `SameSite=Lax` cookie, stored as a SHA-256
  hash in a new `sessions` collection with a TTL index. Script injected into
  the dashboard can no longer read the session, and a leaked session can be
  revoked. Everyone is signed out once on upgrade.
- In cloud mode the `admin` role no longer widens access to data. It still
  gates administration (users, workspaces, licensing), but scans, workspaces,
  usage and API keys stay scoped to the workspaces a user belongs to, so one
  tenant's source-code findings are not reachable from another.
- Email login codes are hashed with bcrypt instead of HMAC-SHA256, which
  removed the last use of `JWT_SECRET` while keeping a six-digit code
  impractical to reverse from a database dump.
- The compose file no longer publishes MongoDB on `27017`; only the backend
  reaches it, over the compose network.

## [1.0.0-rc5] - 2026-08-02

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
- Chart: the ingress examples are neutral (nginx, `example.com`) instead of our
  own hostnames and ingress class.

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

[Unreleased]: https://github.com/runtz-dev/runtz/compare/v1.0.0-rc8...HEAD
[1.0.0-rc9]: https://github.com/runtz-dev/runtz/releases/tag/v1.0.0-rc9
[1.0.0-rc8]: https://github.com/runtz-dev/runtz/releases/tag/v1.0.0-rc8
[1.0.0-rc5]: https://github.com/runtz-dev/runtz/releases/tag/v1.0.0-rc5
[1.0.0-rc1]: https://github.com/runtz-dev/runtz/releases/tag/v1.0.0-rc1
