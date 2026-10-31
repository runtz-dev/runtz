# Google Cloud configuration checks

Runtz connects GCP projects to the Runtz Security service and runs 32 custom,
read-only configuration checks. Findings are grouped by severity and include
rule IDs, evidence, remediation and documentation. Security Command Center
activation, its findings API and a GCP organization are not prerequisites.

## Connect a project

Use **Cloud → Add cloud → Google Cloud**. The same dialog guides you through:

1. Select the Runtz workspace and target GCP project ID.
2. Enable the resource APIs needed by the resources you want to check.
3. Create a dedicated service account and start with `roles/iam.securityReviewer`
   on the target project. Each check reports additional missing read permissions.
4. Create a JSON key, upload it and verify access to the target project.
5. Name the connection and start its first scan.

Connections finish onboarding while their scan runs in the background. Results
and status refresh automatically. Each workspace can connect multiple projects,
including projects accessed by the same service account. Only the workspace
owner (or a self-hosted admin) can connect, scan, rotate credentials or disconnect;
authorized workspace members can read findings and check coverage.

## Results and coverage

The Findings tab supports severity, literal search and pagination. Open a
finding to review evidence, the affected resource and remediation. The Checks
tab shows passed, failed, skipped, error and not-applicable evaluations.

A skipped check is not a compliance pass. Missing APIs/permissions and incomplete
collection remain visible in the coverage summary. A result with no findings
only describes the checks that were evaluated. A failed later scan preserves
the last published result. Scheduled scans repeat every 12 hours.

The engine supplies the authorized workspace ID as the service tenant. Browser
headers cannot choose another service tenant. Every connection lookup is scoped
to workspace membership before the engine calls Security.

## Configure the companion service

Deploy `runtz-security` separately. Configure these **engine-only** variables:

- `RUNTZ_SECURITY_API_URL`: the Security HTTP(S) origin, for example
  `http://runtz-security:8080` inside the same Kubernetes namespace.
- `RUNTZ_SECURITY_API_KEY`: the matching Security service bearer key.

The Helm chart exposes `backend.cloudSecurity.url` and
`backend.cloudSecurity.existingSecret`. The referenced Secret must contain
`RUNTZ_SECURITY_API_KEY`. Hosted dev and prod each use their namespace-local
Security Service and runtime Secret; databases and encryption keys are isolated.
The public chart leaves this integration optional for self-hosted deployments.

New service account JSON is encrypted and stored only in Security's MongoDB.
The engine stores a workspace-scoped connection reference, safe display
metadata and cached scan summaries. No service bearer key is exposed to the
frontend or included in a Helm values file. Keep Security's encryption key
stable and backed up separately from its database.

Older connections can retain their original encrypted engine credentials.
Their owner can start a scan or replace the JSON key to migrate them to
Security. This verifies the project and removes the old credential copy from
the engine after the service reference is stored. Preserve
`RUNTZ_CLOUD_ENCRYPTION_KEY` until legacy connections have been migrated.

Disconnecting removes the remote encrypted credential and jobs before deleting
the engine reference. Workspace/account deletion also cleans up the matching
Security tenant. An unavailable Security service prevents deletion of known
remote credentials so they are not silently orphaned; retry when connectivity
is restored.

## Verification

Engine tests cover asynchronous onboarding, tenant-header isolation, owner-only
operations, skipped coverage, preserving previous results on failure, credential
migration, disconnection and workspace deletion during an in-flight connection.
Set `RUNTZ_TEST_MONGO_URI` to a disposable MongoDB server to run them.

An optional live integration test accepts `RUNTZ_TEST_SECURITY_URL`,
`RUNTZ_TEST_SECURITY_API_KEY` and `RUNTZ_TEST_GCP_KEY_FILE`. It creates an isolated
workspace/tenant, verifies a real read-only GCP scan through the engine adapter,
then disconnects the test cloud and removes its test data. Never enable it in CI
or place the service account JSON in repository fixtures.
