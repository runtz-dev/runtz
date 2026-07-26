# Security Policy

runtz is a security product, and we treat reports about runtz itself with
the highest priority.

## Reporting a vulnerability

**Do not open public issues for vulnerabilities.**

Email **security@runtz.dev** with:

- A description of the issue and its impact.
- Steps to reproduce (proof of concept if possible).
- Affected component (engine, CLI, frontend, Helm chart, installer) and
  version (`runtz version`, image tag or chart version).

We will acknowledge your report within 3 business days and keep you updated
while we investigate.

## Coordinated disclosure

- Please give us up to **90 days** to release a fix before public
  disclosure.
- We will credit reporters in the release notes unless you prefer to remain
  anonymous.
- We currently do not run a paid bug bounty program.

## Supported versions

Until `1.0.0` ships, only the **latest release candidate** receives security
fixes. After `1.0.0`, we will support the latest minor release.

## Scope notes

- The Platform ships with secure-by-default settings: the engine refuses to
  start with missing or placeholder `JWT_SECRET` / `RUNTZ_INGEST_TOKEN`
  values. Reports about deployments that deliberately weaken these controls
  are out of scope.
- Vulnerabilities in third-party dependencies should also be reported
  upstream; we still want to know so we can ship updated builds.
