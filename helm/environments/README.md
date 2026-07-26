# Internal environments

Per-environment value overlays for the runtz team's own deployments. The
public chart in `helm/runtz/` keeps generic defaults; everything specific to
an environment lives here.

Layout per environment:

- `values.yaml` — non-secret overrides (committed).
- Secrets — preferably a **Kubernetes Secret** referenced via
  `backend.secrets.existingSecret` (see below). A local gitignored
  `values.secrets.yaml` also works as a fallback.

## Recommended: secrets as a Kubernetes Secret

The engine Deployment reads **all seven keys** from the Secret — every key
must exist (use an empty string for features you don't use):

| Key | Purpose | How to obtain |
| --- | --- | --- |
| `JWT_SECRET` | Session tokens (required) | `openssl rand -base64 32` |
| `RUNTZ_INGEST_TOKEN` | Shared ingest token (required) | `openssl rand -base64 24` |
| `RESEND_API_KEY` | Email sign-in codes / invites | Resend dashboard |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth login | GitHub OAuth app settings |
| `STRIPE_SECRET_KEY` | Billing (central engine only) | Stripe dashboard → API keys |
| `STRIPE_WEBHOOK_SECRET` | Billing webhooks (central only) | Stripe dashboard → Webhooks |
| `RUNTZ_LICENSE_PRIVATE_KEY` | Signs self-hosted licenses (central only) | `openssl rand -base64 32` |

```bash
kubectl create secret generic runtz-engine-secrets \
  --namespace <namespace> \
  --from-literal=JWT_SECRET="$(openssl rand -base64 32)" \
  --from-literal=RUNTZ_INGEST_TOKEN="$(openssl rand -base64 24)" \
  --from-literal=RESEND_API_KEY="re_..." \
  --from-literal=GITHUB_CLIENT_SECRET="..." \
  --from-literal=STRIPE_SECRET_KEY="sk_live_..." \
  --from-literal=STRIPE_WEBHOOK_SECRET="whsec_..." \
  --from-literal=RUNTZ_LICENSE_PRIVATE_KEY="$(openssl rand -base64 32)"
```

Then set in the environment's `values.yaml`:

```yaml
backend:
  secrets:
    existingSecret: runtz-engine-secrets
```

and deploy without any secrets file:

```bash
helm upgrade --install runtz ./helm/runtz \
  -f helm/environments/prod/values.yaml
```

To rotate a single key later:

```bash
kubectl patch secret runtz-engine-secrets -n <namespace> \
  --type merge -p '{"stringData":{"RESEND_API_KEY":"re_new..."}}'
kubectl rollout restart deployment/runtz-backend -n <namespace>
```

## Fallback: local values.secrets.yaml

Copy `helm/runtz/values.secrets.example.yaml` next to the environment's
`values.yaml` as `values.secrets.yaml` (gitignored) and add a second `-f`
flag to the helm command.
