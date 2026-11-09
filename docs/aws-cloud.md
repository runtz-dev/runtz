# AWS configuration checks

Use **Cloud → Add cloud → Amazon Web Services** to connect one commercial AWS account to a workspace. Choose IAM role (recommended) or Access key, and enter the account ID and selected regions. For a role, create the generated CloudFormation stack, paste its RoleArn output and verify access. For a key, create a dedicated IAM user with the downloaded catalog-scoped read policy and enter its Access key ID and Secret access key. The wizard supports an S3 quick-create link when configured by your operator and a prefilled JSON download otherwise.

Role connections use a workspace/account-bound External ID and temporary STS credentials. Direct IAM-user access keys are also supported, independently of the platform AWS identity. STS verifies their account and rejects root credentials. Keys are encrypted in Security Mongo, never returned to the browser and never stored by the engine. Validation checks the exact role/account and rejects trust policies that allow the scanner to assume the role without the correct External ID. The first asynchronous scan runs immediately; scheduled scans repeat every 12 hours.

The 18 custom checks cover IAM, S3, EC2/EBS, RDS, CloudTrail and EKS. Findings and check evaluations carry region metadata. IAM/account checks run once; regional resource checks run only in selected regions. Empty findings do not imply full coverage: review skipped/error evaluations and the selected regions. Public-access-block and security-group rules describe configuration rather than effective public exposure.

Only workspace owners/self-hosted admins can onboard, validate, scan or disconnect. Members can read results. The engine authorizes the workspace before setting the Security tenant header. New AWS authorization is encrypted and stored only by Security; the engine persists a scoped reference, authentication/account/role/region metadata and summaries. Existing GCP/Azure connections remain compatible.

**Settings → Cloud** displays authentication method, account, role (when used) and regions. Direct keys can be replaced after creating a new IAM user key, without losing results. Authentication method/role/regions are immutable; disconnect and reconnect to change scope. Disconnect deletes Runtz results and stored authorization, not the AWS role. Delete its CloudFormation stack or revoke its IAM user key separately in AWS.

Configure the companion service's runtime identity using [the Security AWS setup guide](https://github.com/runtz-dev/runtz-security/blob/dev/docs/aws-setup.md). No AWS secrets are needed in the engine/frontend. Use the existing `RUNTZ_SECURITY_API_URL` and `RUNTZ_SECURITY_API_KEY`; deploy the AWS-capable Security backend before the engine/frontend.

Engine routes:

- `POST /api/v1/cloud/aws/onboarding`: workspaceId, accountId, optional authMethod (`role` or `access_key`, default `role`).
- `POST /api/v1/cloud/aws/validate`: workspaceId, accountId, credentials.
- `POST /api/v1/cloud/aws/connections`: same fields plus name.

Role credentials contain optional authMethod=`role`, roleArn, generated externalId and selected regions. Key credentials contain authMethod=`access_key`, accessKeyId, secretAccessKey and selected regions; do not mix the two shapes. PATCH on the shared credentials route replaces keys and preserves authentication, account and regions. Internal project_id stores the AWS account scope for compatibility with existing indexes; accountId is explicit in the API. Shared routes serve scans, findings, coverage and deletion. Integration tests run the AWS/GCP/Azure lifecycle against disposable MongoDB and a fake Security service, including owner permissions, tenant-header isolation, partial coverage, scope mismatches, duplicate connections and deletion during onboarding.
