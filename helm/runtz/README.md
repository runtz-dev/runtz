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

## OpenTelemetry

Both the engine and the frontend can export traces and metrics over OTLP/HTTP.
It is off by default and nothing is sent anywhere until you set an endpoint:

```bash
helm install runtz runtz/runtz \
  --set backend.env.OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318 \
  --set frontend.env.OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318 \
  --set backend.env.OTEL_RESOURCE_ATTRIBUTES=deployment.environment=prod \
  --set frontend.env.OTEL_RESOURCE_ATTRIBUTES=deployment.environment=prod
```

| Value | Default | Description |
| --- | --- | --- |
| `backend.env.OTEL_EXPORTER_OTLP_ENDPOINT` | `""` (disabled) | OTLP/HTTP collector base URL. `http://` sends plaintext, `https://` uses TLS |
| `backend.env.OTEL_RESOURCE_ATTRIBUTES` | `""` | Extra resource attributes as `key=value` pairs |
| `frontend.env.OTEL_EXPORTER_OTLP_ENDPOINT` | `""` (disabled) | Same, for the Next.js server |
| `frontend.env.OTEL_RESOURCE_ATTRIBUTES` | `""` | Same, for the Next.js server |

The services report themselves as `runtz-engine` and `runtz-frontend`. The
engine traces HTTP requests by route and every MongoDB command (command name,
database and collection — never the query document itself), and exports Go
runtime metrics. The frontend propagates trace context across its `/api` proxy
hop, so one trace covers the browser request, the Next.js server, the engine
and the database query behind it.

Neither pod's ConfigMap gets the keys at all when the endpoint is empty.

## Secrets hygiene

`values.secrets.yaml` is ignored by git (repo `.gitignore`) and excluded from
the packaged chart (`.helmignore`). Never commit real secrets.
