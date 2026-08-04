# runtz Helm chart

Deploys the self-hosted runtz platform on Kubernetes: engine (API), frontend
and an optional in-cluster MongoDB.

## Install

```bash
helm repo add runtz https://helm.runtz.dev
helm repo update

helm install runtz runtz/runtz
```

Without an ingress, reach the platform with a port-forward:

```bash
kubectl port-forward svc/runtz-frontend 3000:3000
```

Open http://localhost:3000 and create the admin user on first access.

## Secrets

None are required — the engine issues and stores its own sessions, API keys
and login codes. Secrets only enable optional integrations: `resendApiKey`
(email login), `githubClientSecret` (OAuth), `stripeSecretKey` /
`stripeWebhookSecret` (billing) and `licensePrivateKey` (central engine only).

Alternatively, set `backend.secrets.existingSecret` to the name of a
pre-created Kubernetes Secret containing the keys `RESEND_API_KEY`,
`GITHUB_CLIENT_SECRET`, `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET` and
`RUNTZ_LICENSE_PRIVATE_KEY` (empty strings for unused ones).

## Common values

| Value | Default | Description |
| --- | --- | --- |
| `backend.image.repository` | `runtzdev/runtz-engine` | Engine image |
| `frontend.image.repository` | `runtzdev/runtz-frontend` | Frontend image |
| `backend.image.tag` / `frontend.image.tag` | chart `appVersion` | Image tag |
| `backend.env.RUNTZ_PUBLIC_URL` | `http://localhost:3000` | Public URL of the platform |
| `backend.env.CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Comma-separated origins |
| `backend.env.MONGODB_URI` | in-cluster MongoDB | Set to use an external MongoDB |
| `backend.existingConfigMap` | `""` | Pre-created ConfigMap with extra engine variables; layered after (and winning over) the chart's own |
| `mongodb.persistence.size` | `10Gi` | Data PVC size. Set it before the first install: it lands in the StatefulSet's `volumeClaimTemplates`, which Kubernetes will not let an upgrade change |
| `mongodb.enabled` | `true` | Deploy in-cluster MongoDB |
| `mongodb.persistence.storageClassName` | cluster default | StorageClass for the data PVC |
| `ingress.enabled` | `false` | Frontend ingress |
| `backend.ingress.enabled` | `false` | Engine ingress (direct API access) |

See [values.yaml](values.yaml) for the complete list.

## Secrets hygiene

`values.secrets.yaml` is ignored by git (repo `.gitignore`) and excluded from
the packaged chart (`.helmignore`). Never commit real secrets.
