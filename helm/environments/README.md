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

The engine Deployment reads **all five keys** from the Secret — every key
must exist (use an empty string for features you don't use). None of them are
required to run: sessions, API keys and login codes are issued and stored by
the engine itself.

| Key | Purpose | How to obtain |
| --- | --- | --- |
| `RESEND_API_KEY` | Email sign-in codes / invites | Resend dashboard |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth login | GitHub OAuth app settings |
| `STRIPE_SECRET_KEY` | Billing (central engine only) | Stripe dashboard → API keys |
| `STRIPE_WEBHOOK_SECRET` | Billing webhooks (central only) | Stripe dashboard → Webhooks |
| `RUNTZ_LICENSE_PRIVATE_KEY` | Signs self-hosted licenses (central only) | `openssl rand -base64 32` |

```bash
kubectl create secret generic runtz-engine-secrets \
  --namespace <namespace> \
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

## The MongoDB volume size is nominal here

`mongodb.persistence.size` in both overlays matches what is already provisioned
(dev 20Gi, prod 500Gi) and should stay that way. Two reasons:

1. **It is not a limit.** `openebs-ssd-wd` is OpenEBS LocalPV in hostpath mode —
   the volume is a directory on the node's disk (`/data/ssd-wd/...`) with no
   quota. Both MongoDBs already see the full 938G disk with ~890G free, so the
   difference between the two numbers buys neither of them a byte.
2. **Changing it breaks the deploy.** The value lands in the StatefulSet's
   `volumeClaimTemplates`, which Kubernetes treats as immutable, so
   `helm upgrade` fails with `Forbidden: updates to statefulset spec for fields
   other than 'replicas' ... are forbidden`. And the usual escape hatch —
   orphan the StatefulSet, patch the PVC, redeploy — does not work either:
   this StorageClass has `allowVolumeExpansion` unset and LocalPV hostpath
   cannot expand in place.

Making the number match a target would therefore mean recreating the volume:

```bash
# Data loss unless you dump and restore around it (reclaim policy is Delete)
kubectl -n <ns> exec runtz-mongodb-0 -- mongodump --archive=/tmp/runtz.gz --gzip
kubectl -n <ns> cp runtz-mongodb-0:/tmp/runtz.gz ./runtz.gz
kubectl -n <ns> delete statefulset runtz-mongodb --cascade=orphan
kubectl -n <ns> delete pod runtz-mongodb-0
kubectl -n <ns> delete pvc data-runtz-mongodb-0
helm upgrade --install runtz ... --set mongodb.persistence.size=<new size>
kubectl -n <ns> cp ./runtz.gz runtz-mongodb-0:/tmp/runtz.gz
kubectl -n <ns> exec runtz-mongodb-0 -- mongorestore --archive=/tmp/runtz.gz --gzip
```

If you want a size that is actually **enforced**, that is a StorageClass
decision (LVM- or ZFS-backed OpenEBS with `allowVolumeExpansion: true`), not a
chart one.

## Fallback: local values.secrets.yaml

Copy `helm/runtz/values.secrets.example.yaml` next to the environment's
`values.yaml` as `values.secrets.yaml` (gitignored) and add a second `-f`
flag to the helm command.

## Testing premium purchases in dev

Use the dev frontend with Stripe sandbox keys and prices from the same sandbox.
Before testing, verify the running engine has `STRIPE_SECRET_KEY` in test mode,
the four `STRIPE_PRICE_*` values refer to active recurring test prices, and
`STRIPE_WEBHOOK_SECRET` contains the signing secret for the **dev** endpoint.
Never copy the live signing secret or live prices into dev.

Register the sandbox webhook at the dev engine's `/api/v1/billing/webhook`,
using the same API version as `stripeAPIVersion` in `engine/internal/api/billing.go`.
Enable `checkout.session.completed`, `customer.subscription.created`,
`customer.subscription.updated`, and `customer.subscription.deleted`.
After changing Kubernetes Secrets or ConfigMaps, restart the dev engine so it
loads the new values. Configure the sandbox Customer Portal too, to test
subscription management from Billing.

Sign in on dev, open **Settings → Billing**, and select a paid plan. In the
Stripe test checkout use `4242 4242 4242 4242`, any future expiration date, and
any three-digit CVC. After returning, verify the plan and limits in Billing and
Usage, then exercise the premium features. Also test cancellation in the portal
and verify the matching webhook returns HTTP 200 and the account status updates.
The checkout return status check can recover the initial purchase without a webhook;
renewals and cancellations still require the webhook to stay synchronized.
The return page waits for an active or trialing subscription, verifies the account's
entitlement, and refreshes the plan displayed throughout the app. Pending payments
remain pending; synchronization failures are reported and can be retried by
refreshing the return page. Checkout and subscription webhooks reconcile the same
billing record, including when deliveries arrive concurrently or in either order.

References: [Stripe test cards](https://docs.stripe.com/testing) and
[subscription webhooks](https://docs.stripe.com/billing/subscriptions/webhooks).
