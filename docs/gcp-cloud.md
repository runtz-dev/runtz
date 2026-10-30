# Google Cloud security findings

Runtz Cloud imports **native Security Command Center (SCC) findings** from GCP.
It does not run its own cloud posture checks, start Google scanners, enable APIs,
or modify cloud resources. AWS and Azure are shown as coming soon.

## Configure credential encryption

Before connecting a project, generate an encryption key with `openssl rand -base64 32`
and store it as `RUNTZ_CLOUD_ENCRYPTION_KEY` in the engine's environment or a
Kubernetes Secret. Docker Compose reads it from `.env`. Never commit the value.
Without it, the rest of Runtz works, but connecting cloud credentials returns a
configuration error.

For Helm, add `RUNTZ_CLOUD_ENCRYPTION_KEY` to the engine's existing Secret, then
reference that Secret without putting its value in Helm values:

```yaml
backend:
  secrets:
    existingSecret: runtz-engine-secrets
```

The engine deployment reads the key automatically. The Secret must also contain
the chart's other integration keys, using empty values for unused integrations
as described in the chart README. For a chart-managed Secret, set
`backend.secrets.cloudEncryptionKey` in an ignored private values file instead.

MongoDB stores the service account JSON encrypted with AES-256-GCM, a random
nonce per encryption, and authenticated binding to its workspace, connection
and provider. API responses never include the ciphertext or original key.
Credentials are not written to files, browser storage, logs or fixtures.
Fixed Google OAuth and SCC endpoints prevent uploaded JSON from selecting
arbitrary credential endpoints or external credential providers.

Back up the encryption key **separately from MongoDB**, and preserve it across
engine replicas, restarts and restores. Losing or changing it makes existing
credentials unreadable. There is no automatic master-key migration in this
version: after changing the master key, replace every connection's JSON through
Configure. Service account key rotation is available independently in the UI.
A future KMS-backed encryption provider can replace this operator-managed key.

## Connect a project

Use **Cloud → Add cloud → Google Cloud**. The same dialog walks through:

1. Select a Runtz workspace and GCP project ID. Choose the SCC data location
   (`global` by default, or `eu`, `us`, `sa` for data residency).
2. Activate SCC on the project, or inherit its organization activation. Enable
   `securitycenter.googleapis.com` in the service account's project. API
   enablement alone does not activate SCC or its detectors.
3. Create a service account and grant
   `roles/securitycenter.findingsViewer` on the **target project**. This is the
   narrower read-only role needed by this importer. An existing
   `roles/iam.securityReviewer` grant also includes `securitycenter.findings.list`.
   Runtz does not need Owner, Editor or permission to modify cloud resources.
4. Create a JSON key under **Service Accounts → Keys → Add key → Create new key**,
   upload it, and verify access. Organization policies that prohibit JSON keys
   must be addressed by the customer's administrator; workload identity
   federation is not implemented in this version.
5. Name the connection and connect. Runtz imports findings before reporting
   success, then shows a summary and the link to the project's findings.

The API defaults to the credential's project only when the target is omitted.
The UI requires an explicit target. A service account can access other projects
if granted IAM permissions there. Add a separate connection for each target
project/location. The same project/location cannot be added twice to a workspace.

## Review and refresh

The sidebar shows Google Cloud once a connection exists in the selected
workspace. **Add cloud** stays underneath. Google Cloud lists connections,
project IDs, workspaces, status, last successful import and severity totals.

Open a connection to search and filter findings, inspect provider descriptions,
resource, source, event time and remediation when supplied, or follow its link.
Counts include **active, unmuted** findings; All states also shows inactive and
muted findings. Unknown provider severity stays unspecified.

**Import findings** refreshes existing SCC results. It does not trigger a new
Google detector run. Imports are manual in this version. Pagination and stable
provider IDs prevent duplicates. A new snapshot becomes visible only after all
pages and database writes succeed. Failures preserve the previous snapshot and
show an error. A database lease prevents concurrent imports, credential
replacement or disconnection of the same connection.

Retired snapshots remain for five minutes to let in-flight reads finish and are
then removed by MongoDB TTL. The current snapshot is not subject to TTL.

An import has a 90-second budget, a 10,000-finding limit, a 100-page limit and a
64 MB total response budget (8 MB per GCP response).
Exceeding a limit fails explicitly. Runtz serves findings in pages of 50 records,
sorted by severity. API access is scoped to the authenticated user's workspaces.
Only a cloud workspace owner or self-hosted administrator can connect, rescan,
replace credentials or disconnect. Workspace members/viewers can read results.

Use **Configure → Replace JSON key** to validate and save a new key. Revoke the
previous key in GCP afterwards. Disconnect removes saved credentials and findings
from Runtz; it does not revoke GCP keys. Deleting a workspace/account also removes
its cloud connections and findings.

## Coverage and errors

- `SERVICE_DISABLED`: enable the SCC API in the credential project and check SCC
  activation on the target. Runtz does not change this automatically.
- Access denied: check the JSON key, IAM grant on the target, SCC activation and
  the selected SCC location.
- Zero findings: a successful read returned no applicable findings in that
  location. It does **not** prove the project is secure. Check detector coverage,
  tier, activation and initial processing time in Google Cloud.
- Alerts produced by another product's custom checks (for example audit logging
  checks) appear only if GCP SCC itself reports corresponding findings.

Official references: [List findings API](https://docs.cloud.google.com/security-command-center/docs/reference/rest/v2/projects.sources.locations.findings/list),
[IAM roles](https://docs.cloud.google.com/iam/docs/roles-permissions/securitycenter),
[SCC locations](https://docs.cloud.google.com/security-command-center/docs/how-to-api-list-findings).

## Verification

```sh
cd engine
go test ./...
go vet ./...
RUNTZ_TEST_MONGO_URI=mongodb://127.0.0.1:27017 go test ./...
# Optional authorized, read-only live smoke test; never prints the key:
RUNTZ_TEST_GCP_KEY_FILE=/secure/path/service-account.json go test ./internal/cloudgcp -run TestLiveGCPReadOnly -v -count=1
cd ../frontend
npm run lint
npm run build
```

Database tests use disposable databases and cover tenant isolation, owner/member
permissions, encryption, finding filters, snapshot replacement, retries, failed
imports, key rotation, concurrency protection, multiple projects, disconnection
and workspace cleanup.
