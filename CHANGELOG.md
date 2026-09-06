# Changelog

All notable changes to runtz are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).
Until `1.0.0` ships, public builds are tagged as release candidates
(`1.0.0-rc1`, `1.0.0-rc2`, ...).

## [Unreleased]

## [1.0.0-rc23] - 2026-09-06

### Added

- Cloud workspace owners on Pro and Enterprise can share access with existing
  Runtz accounts by email, view members and remove access. Members can view the
  roster of workspaces shared with them.
- The application header displays the current account plan.

### Changed

- Workspaces are selectable with a highlighted row and a minimal member list
  below. Share and Delete remain in each owned workspace row; adding a member
  uses a compact email-only dialog.
- Deleting the last cloud workspace leaves the account empty. Subsequent sign-in
  does not recreate it; users can create another workspace from Settings.
- Workspace creation uses a compact dialog, selects the new workspace, respects
  plan limits and permits different accounts to use the same display name.
- Improved empty-account navigation and responsive workspace layouts.

### Fixed

- Checkout activation now verifies the purchased plan and account ownership
  before reporting success.
- Subscription reconciliation handles duplicate and out-of-order Stripe webhook
  deliveries, preserving confirmed subscription state and checkout associations.

### Security

- Workspace sharing enforces ownership and account-wide plan seat limits on the
  server. Only owners may add or remove members.
- Removing a member revokes their workspace API keys. Cloud API keys require
  their creator to retain workspace membership.

## [1.0.0-rc22] - 2026-09-05

### Added

- Cloud workspace owners can permanently delete a workspace from Settings,
  including every scan and API key stored in it. Deleting the last workspace
  creates a new empty personal workspace so the account remains usable.
- Settings now includes a cloud-only **Account** tab for permanent account and
  owned-data deletion. The confirmation shows the affected workspaces, scans,
  API keys, shared access and subscription before proceeding.

### Changed

- Redesigned the passwordless sign-in email with runtz's dark brand identity
  (JetBrains Mono wordmark, accent-blue code) instead of the plain-text
  layout, and lowercased every "Runtz" mention to match the wordmark.

### Security

- Account deletion revokes sessions and API keys, removes authentication data,
  cancels active Stripe subscriptions and refuses to continue while an owned
  workspace is still shared with other users.

## [1.0.0-rc21] - 2026-08-16

### Changed

- Kept the API-key creation and success dialogs at the same width for a
  smoother transition after creating a key.

## [1.0.0-rc20] - 2026-08-16

### Changed

- Newly created API keys are masked by default and can be explicitly shown or
  hidden before copying.
- Removed the duplicate `RUNTZ_API_KEY=...` snippet from the API-key creation
  dialog.

## [1.0.0-rc19] - 2026-08-15

### Changed

- Replaced the onboarding host-scan step with an optional VS Code extension
  setup and a scanner-agnostic completion step, so onboarding works the same
  way on Linux, macOS and Windows.
- Clarified that the Windows CLI installation command runs in PowerShell.

## [1.0.0-rc18] - 2026-08-14

### Documentation

- Reworked the project README around a clearer product overview, accurate
  setup and deployment guidance, and real platform and VS Code extension
  visuals.

## [1.0.0-rc17] - 2026-08-12

### Added

- Onboarding's "Install the CLI" step now switches between the Linux/macOS
  `curl` command and the Windows `irm` command, matching runtz-cli's new
  Windows support.

## [1.0.0-rc16] - 2026-08-11

### Added

- Findings pages accept a `?scanId=` query param to open a specific past
  scan instead of always defaulting to the latest one.

### Docs

- Documented the new `runtz-dev/runtz-vscode-extension` repo alongside the
  CLI and MCP server repos.

## [1.0.0-rc14] - 2026-08-07

### Added

- Reusable first-scan and clean-scan states across SCA, SAST, container, host
  and Kubernetes views, with copyable CLI examples and continuous-monitoring
  guidance.
- Fix-availability filtering across overview metrics, asset lists, scan
  history, vulnerability trends and package details. Fixable CVEs are shown by
  default, with a **Show unfixed CVEs** switch for findings without a published
  fix.
- Severity distributions and scan-over-scan vulnerability deltas in the
  latest-scan cards.
- Production autoscaling for the engine and frontend, with CPU and memory
  targets, resource requests and limits, and three to ten replicas.

### Fixed

- Vulnerability trends now retain the latest known state per asset each day
  instead of summing repeated submissions of the same inventory.
- Historical package scans backfill fix-availability severity counts so the
  new filters remain consistent with existing data.
- Unchanged scan totals now display a single dash instead of `- 0`.

## [1.0.0-rc13] - 2026-08-06

### Fixed

- `POST /api/v1/ingest/sca`: every SCA scan was rejected with `400 invalid
  json body`. The engine's `Dependency` model was missing the per-dependency
  `file` field that the CLI has always sent, and the strict JSON decoder
  (`DisallowUnknownFields`) rejected the whole payload as a result. `runtz
  sca` has been unusable against both dev and prod since the initial
  release; this was never caught because the ingest endpoint had no test
  coverage.

## [1.0.0-rc12] - 2026-08-06

### Added

- `GET /api/v1/keys/verify`: the CLI's new `runtz login` verifies a workspace
  key before storing it locally. Authenticates exactly like ingest (active-key
  lookup, constant-time hash compare, `ingest:write` scope) without counting
  scan usage, and returns only the workspace id/name and the key's name,
  prefix and expiry.

### Changed

- Onboarding now teaches the login-once flow: generate key, install the CLI,
  `runtz login`, then scan with plain `runtz host`.

## [1.0.0-rc11] - 2026-08-06

### Added

- OpenTelemetry instrumentation for the engine and the frontend. Traces and
  metrics are exported over OTLP/HTTP and are off unless
  `OTEL_EXPORTER_OTLP_ENDPOINT` is set, so self-hosted deployments are
  unaffected by default. `OTEL_SDK_DISABLED=true` turns the SDK off outright.
- MongoDB command tracing in the engine, so database calls appear as spans on
  the request they belong to.
- Trace context propagation from the frontend to the engine: a browser request
  and the engine work it triggers now land in the same trace.

### Changed

- The DAST coming-soon card links to the roadmap and its copy was simplified.
- Bumped `mongo-driver` to v2.8.0.

### Fixed

- The `mongodb.persistence` overlays pointed at the `openebs-ssd` StorageClass,
  which no longer exists, with sizes that no longer matched the provisioned
  volumes. Because `volumeClaimTemplates` is immutable, every `helm upgrade`
  failed with `Forbidden: updates to statefulset spec ... are forbidden`. Both
  overlays now match what is provisioned (dev 20Gi, prod 500Gi on
  `openebs-ssd-wd`).

## [1.0.0-rc10] - 2026-08-05

### Changed

- Standardized all user-facing platform copy and validation messages in
  English, removing the remaining Portuguese text.

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

[Unreleased]: https://github.com/runtz-dev/runtz/compare/v1.0.0-rc12...HEAD
[1.0.0-rc12]: https://github.com/runtz-dev/runtz/releases/tag/v1.0.0-rc12
[1.0.0-rc11]: https://github.com/runtz-dev/runtz/releases/tag/v1.0.0-rc11
[1.0.0-rc10]: https://github.com/runtz-dev/runtz/releases/tag/v1.0.0-rc10
[1.0.0-rc9]: https://github.com/runtz-dev/runtz/releases/tag/v1.0.0-rc9
[1.0.0-rc8]: https://github.com/runtz-dev/runtz/releases/tag/v1.0.0-rc8
[1.0.0-rc5]: https://github.com/runtz-dev/runtz/releases/tag/v1.0.0-rc5
[1.0.0-rc1]: https://github.com/runtz-dev/runtz/releases/tag/v1.0.0-rc1
