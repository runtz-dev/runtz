// Matches the read-only permission catalog in runtz-security/internal/aws/catalog.go.
export const awsReadOnlyPolicy = {
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "cloudtrail:DescribeTrails",
        "cloudtrail:GetTrailStatus",
        "ec2:DescribeInstances",
        "ec2:DescribeRegions",
        "ec2:DescribeSecurityGroups",
        "ec2:DescribeVolumes",
        "eks:DescribeCluster",
        "eks:ListClusters",
        "iam:GetAccountSummary",
        "iam:GetLoginProfile",
        "iam:ListMFADevices",
        "iam:ListUsers",
        "rds:DescribeDBInstances",
        "s3:GetAccountPublicAccessBlock",
        "s3:GetBucketLocation",
        "s3:GetBucketPublicAccessBlock",
        "s3:GetBucketVersioning",
        "s3:ListAllMyBuckets"
      ],
      "Resource": "*"
    }
  ]
} as const

export function cloudSetupCommands(provider: "gcp" | "aws" | "azure", scope: string) {
 if (provider === "gcp") return `gcloud iam service-accounts create runtz-security --project=${scope} --display-name="Runtz Security"

gcloud projects add-iam-policy-binding ${scope} --member="serviceAccount:runtz-security@${scope}.iam.gserviceaccount.com" --role="roles/iam.securityReviewer"

gcloud iam service-accounts keys create runtz-security-key.json --iam-account="runtz-security@${scope}.iam.gserviceaccount.com" --project=${scope}`
 if (provider === "azure") return `az login
az account set --subscription ${scope}
az ad sp create-for-rbac --name runtz-security --role Reader --scopes /subscriptions/${scope}`
 return `aws sts get-caller-identity

# Save the read-only policy below as runtz-readonly-policy.json.
aws iam create-user --user-name runtz-security
aws iam put-user-policy --user-name runtz-security --policy-name RuntzReadOnly --policy-document file://runtz-readonly-policy.json
aws iam create-access-key --user-name runtz-security`
}
