# Microsoft Azure configuration checks

Use **Cloud → Add cloud → Microsoft Azure** to connect an Azure subscription
to the companion Runtz Security service. The integration runs 14 custom,
read-only configuration checks through Azure Resource Manager. Defender for
Cloud and Microsoft Graph permissions are not required.

## Connect a subscription

1. Select a Runtz workspace that you own and enter the target Subscription ID.
2. Select or register a dedicated Microsoft Entra application.
3. Have an authorized administrator assign its service principal **Reader**
   on the target subscription, or an equivalent custom role for the checks.
4. Enter the Directory (tenant) ID, Application (client) ID and client secret
   **value** from Certificates & secrets. The secret ID is not the credential.
5. Verify subscription access, name the connection, and start the first scan.

See Microsoft's [application setup guide](https://learn.microsoft.com/entra/identity-platform/howto-create-service-principal-portal)
and [Reader role reference](https://learn.microsoft.com/azure/role-based-access-control/built-in-roles/general#reader).
The scanner never assigns roles, registers providers or changes Azure resources.
Only Azure public cloud and application client-secret credentials are supported.

Each subscription is a separate connection. The application's Entra tenant
differs from the Runtz workspace tenant. Runtz authorizes the workspace before
forwarding its own trusted tenant header to Security.

## Findings, scans and coverage

Azure connections appear under **Cloud → Microsoft Azure**, separately from
GCP projects. Open a connection for findings, severity/search filters,
pagination, evidence, remediation and documentation. The Checks tab shows
passed, failed, skipped, error and not-applicable results.

The initial scan runs asynchronously; results refresh automatically. Scans
repeat every 12 hours, with manual scans available to the workspace owner
or self-hosted admin. Members can read results. Missing permissions and
unavailable resource providers remain incomplete coverage, never a pass.
A failed later scan preserves the previous published results.

## Credentials and disconnection

The same engine-only `RUNTZ_SECURITY_API_URL` and `RUNTZ_SECURITY_API_KEY`
configuration used by [GCP](gcp-cloud.md) enables Azure. Security encrypts the
application credential before persistence. The engine stores a scoped reference
and display metadata; it does not persist the client secret. The service bearer
key is never exposed to the browser.

**Settings → Cloud** supports replacing the client secret while preserving
the same directory and application identity. Security verifies access before
accepting the replacement. Revoke the previous secret in Entra after replacement.

Disconnecting deletes the saved credential, jobs and results in Security
before removing the engine reference. This does not delete the Azure application,
its role assignment or subscription resources. Workspace/account cleanup
handles both GCP and Azure references.

## API and verification

- `POST /api/v1/cloud/azure/validate`: workspaceId, subscriptionId, credentials.
- `POST /api/v1/cloud/azure/connections`: the same fields plus name.
- Shared connection routes handle scans, findings, checks, credentials and deletion.

Credentials are an object with tenantId, clientId and clientSecret. IDs use UUID
syntax and normalize to lowercase. The existing indexed project_id field stores
the subscription scope for Azure, matching Security; subscriptionId is also
explicitly exposed alongside directoryTenantId and clientId.

Automated integration tests cover both providers with a disposable MongoDB and
a fake Security server: asynchronous creation, workspace isolation, owner-only
management, partial coverage, results filtering, preservation after failed scans,
secret rotation, duplicate connections and deletion during connection creation.
Azure tests also reject results from another subscription and identity changes
during rotation. Set RUNTZ_TEST_MONGO_URI to run these tests. They do not call Azure.
