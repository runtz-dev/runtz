#!/usr/bin/env bash
#
# Package the runtz Helm chart and push it to the public ChartMuseum.
#
#   ./scripts/helm-release.sh
#
# Environment overrides:
#   CHART_REPO_URL        ChartMuseum base URL (default: https://helm.runtz.dev)
#   CHARTMUSEUM_USER      Basic auth user (optional)
#   CHARTMUSEUM_PASSWORD  Basic auth password (optional)
#   FORCE                 Set to 1 to overwrite an existing chart version

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHART_DIR="${ROOT}/helm/runtz"
DIST_DIR="${ROOT}/.helm-dist"
CHART_REPO_URL="${CHART_REPO_URL:-https://helm.runtz.dev}"

info() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

command -v helm >/dev/null || fail "helm is required"
command -v curl >/dev/null || fail "curl is required"

info "Linting chart..."
helm lint "$CHART_DIR"

VERSION="$(helm show chart "$CHART_DIR" | sed -n 's/^version: //p')"
[ -n "$VERSION" ] || fail "could not read chart version"

mkdir -p "$DIST_DIR"
info "Packaging runtz-${VERSION}..."
# .helmignore keeps values.secrets.yaml out of the package.
helm package "$CHART_DIR" --destination "$DIST_DIR" >/dev/null
PACKAGE="${DIST_DIR}/runtz-${VERSION}.tgz"
[ -f "$PACKAGE" ] || fail "package not found: ${PACKAGE}"

AUTH=()
if [ -n "${CHARTMUSEUM_USER:-}" ]; then
  AUTH=(-u "${CHARTMUSEUM_USER}:${CHARTMUSEUM_PASSWORD:-}")
fi

QUERY=""
if [ "${FORCE:-0}" = "1" ]; then
  QUERY="?force"
fi

info "Pushing runtz-${VERSION}.tgz to ${CHART_REPO_URL}..."
HTTP_CODE="$(curl -sS -o /tmp/helm-release-response.$$ -w '%{http_code}' \
  "${AUTH[@]}" \
  --data-binary "@${PACKAGE}" \
  "${CHART_REPO_URL}/api/charts${QUERY}")"

if [ "$HTTP_CODE" != "201" ] && [ "$HTTP_CODE" != "200" ]; then
  cat /tmp/helm-release-response.$$ >&2 || true
  rm -f /tmp/helm-release-response.$$
  fail "ChartMuseum push failed with HTTP ${HTTP_CODE}"
fi
rm -f /tmp/helm-release-response.$$

info "Published. Users can now run:"
info "  helm repo add runtz ${CHART_REPO_URL}"
info "  helm repo update"
info "  helm install runtz runtz/runtz -f values.secrets.yaml"
