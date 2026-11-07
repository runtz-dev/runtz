# AWS configuration checks

Use **Cloud → Add cloud → Amazon Web Services** to connect one commercial AWS account to a workspace. Enter the account ID and selected regions, create the generated CloudFormation stack, paste its RoleArn output and verify access. The wizard supports an S3 quick-create link when configured by your operator and a prefilled JSON download otherwise.

The service uses an IAM role, a workspace/account-bound External ID and temporary STS credentials. Customer access keys are not uploaded. Validation checks the exact role/account and rejects trust policies that allow the scanner to assume the role without the correct External ID. The first asynchronous scan runs immediately; scheduled scans repeat every 12 hours.

The 18 custom checks cover IAM, S3, EC2/EBS, RDS, CloudTrail and EKS. Findings and check evaluations carry region metadata. IAM/account checks run once; regional resource checks run only in selected regions. Empty findings do not imply full coverage: review skipped/error evaluations and the selected regions. Public-access-block and security-group rules describe configuration rather than effective public exposure.

Only workspace owners/self-hosted admins can onboard, validate, scan or disconnect. Members can read results. The engine authorizes the workspace before setting the Security tenant header. New AWS authorization is encrypted and stored only by Security; the engine persists a scoped reference, account/role/region metadata and summaries. Existing GCP/Azure connections remain compatible.

**Settings → Cloud** displays account, role and regions and links to IAM management. Role/regions are immutable in this release; disconnect and reconnect to change scope. Disconnect deletes Runtz results and stored authorization, not the AWS role. Delete its CloudFormation stack separately to revoke the integration's IAM resources.

Configure the companion service's runtime identity using [the Security AWS setup guide](https://github.com/runtz-dev/runtz-security/blob/dev/docs/aws-setup.md). No AWS secrets are needed in the engine/frontend. Use the existing `RUNTZ_SECURITY_API_URL` and `RUNTZ_SECURITY_API_KEY`; deploy the AWS-capable Security backend before the engine/frontend.

Engine routes:

- `POST /api/v1/cloud/aws/onboarding`: workspaceId, accountId.
- `POST /api/v1/cloud/aws/validate`: workspaceId, accountId, credentials.
- `POST /api/v1/cloud/aws/connections`: same fields plus name.

Credentials contain roleArn, generated externalId and selected regions. Internal project_id stores the AWS account scope for compatibility with existing indexes; accountId is explicit in the API. Shared routes serve scans, findings, coverage and deletion. Integration tests run the AWS/GCP/Azure lifecycle against disposable MongoDB and a fake Security service, including owner permissions, tenant-header isolation, partial coverage, scope mismatches, duplicate connections and deletion during onboarding.
