# Internal environments

Per-environment value overlays for the runtz team's own deployments. The
public chart in `helm/runtz/` keeps generic defaults; everything specific to
an environment lives here.

Layout per environment:

- `values.yaml` — non-secret, non-deployment-specific overrides (committed).
- Cloud config (OAuth client ids, Stripe price ids) — a **Kubernetes ConfigMap**
  referenced via `backend.existingConfigMap` (see below), applied to the cluster
  from the private `secrets-helm` folder.
- Routing — **Ingress objects applied by hand** from `secrets-helm`
  (`ingress-dev.yaml` / `ingress-prod.yaml`); `ingress.enabled` is `false` in
  both overlays (see below).
- Secrets — preferably a **Kubernetes Secret** referenced via
  `backend.secrets.existingSecret` (see below). A local gitignored
  `values.secrets.yaml` also works as a fallback.

`dev` and `prod` are fully independent: separate hostnames, namespaces,
databases, JWT secrets and Stripe accounts/modes (dev = sandbox, prod = live).
A payment approved on dev only ever upgrades a dev account.

## Cloud config as a ConfigMap (`backend.existingConfigMap`)

Values that belong to *our* deployments rather than to the product — the Google
and GitHub OAuth client ids and the four Stripe price ids — are not in this
repository. They live in a pre-created ConfigMap:

```yaml
backend:
  existingConfigMap: runtz-engine-config
```

The Deployment lists it **after** the chart's own ConfigMap in `envFrom`, so its
keys win. The chart renders those keys only when they are set in values, so an
empty default never shadows the external value. Self-hosted installations don't
need any of it (they activate paid plans through `https://engine.runtz.dev`) and
can leave `existingConfigMap` empty.

| Key | Purpose |
| --- | --- |
| `GOOGLE_CLIENT_ID` | Google sign-in; also served to the browser by `/api/v1/setup/status` |
| `GITHUB_CLIENT_ID` | GitHub sign-in (one OAuth app per environment — one callback URL each) |
| `STRIPE_PRICE_PRO_CLOUD` | Stripe price id, Pro on cloud |
| `STRIPE_PRICE_ENTERPRISE_CLOUD` | Stripe price id, Enterprise on cloud |
| `STRIPE_PRICE_PRO_SELF_HOSTED` | Stripe price id, Pro self-hosted license |
| `STRIPE_PRICE_ENTERPRISE_SELF_HOSTED` | Stripe price id, Enterprise self-hosted license |

The price ids must come from the same Stripe account **and mode** as
`STRIPE_SECRET_KEY` in the Secret. A ConfigMap change does not restart pods:

```bash
kubectl apply -f configmaps-<env>.yaml
kubectl -n <namespace> rollout restart deployment runtz-backend
```

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

## Routing lives outside the repository

Our hostnames and the `cloudflare-tunnel` ingress class say nothing to anyone
self-hosting runtz — nobody installing the chart routes `runtz.dev` through our
Cloudflare tunnel. So the chart ships generic ingress support (disabled, no
class, no hosts, plus a `tls` block and per-path `serviceName`/`servicePort`
for whatever the installation runs), and **our** Ingress objects live in
`secrets-helm/ingress-{dev,prod}.yaml`, applied with kubectl.

One Ingress per hostname, per environment:

| Host | Routes to |
| --- | --- |
| `runtz.dev` / `runtz-dev.runtz.dev` | `/home`, `/legal`, `/install.sh` → `runtz-landing`; everything else → `runtz-frontend` |
| `engine.runtz.dev` / `engine-dev.runtz.dev` | `runtz-backend` |
| `mcp.runtz.dev` / `mcp-dev.runtz.dev` | `runtz-mcp` |

Adding a route to the platform (a new top-level path) means editing those files
in `secrets-helm`, not this repository. Note that the cloudflare-tunnel ingress
controller only supports `pathType: Prefix` — an `Exact` path silently drops the
whole Ingress's DNS record.

## Resizing the MongoDB volume

`mongodb.persistence.size` lands in the StatefulSet's `volumeClaimTemplates`,
which Kubernetes treats as immutable — a `helm upgrade` that changes it fails
with `Forbidden: updates to statefulset spec for fields other than 'replicas'`.
Resize in three steps (the PVC keeps the data; the StorageClass must have
`allowVolumeExpansion: true`):

```bash
# 1. drop the StatefulSet object, keep the pod and the PVC
kubectl -n <namespace> delete statefulset runtz-mongodb --cascade=orphan

# 2. grow the existing claim
kubectl -n <namespace> patch pvc data-runtz-mongodb-0 \
  -p '{"spec":{"resources":{"requests":{"storage":"500Gi"}}}}'

# 3. redeploy — the StatefulSet is recreated with the new size and adopts the pod
helm upgrade --install runtz ... -f helm/environments/<env>/values.yaml
```

Shrinking a volume is not supported by Kubernetes; only grow.

## Fallback: local values.secrets.yaml

Copy `helm/runtz/values.secrets.example.yaml` next to the environment's
`values.yaml` as `values.secrets.yaml` (gitignored) and add a second `-f`
flag to the helm command.
