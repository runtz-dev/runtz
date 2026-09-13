#!/usr/bin/env bash
#
# Build and push the runtz Docker images to Docker Hub (multi-arch).
#
#   docker login   # as a user with push access to the runtzdev org
#   ./scripts/release-images.sh
#
# Every run also tags :rc (always moves — the newest published build) and,
# for a stable version, :latest too. See the LATEST check below.
#
# Environment overrides:
#   RUNTZ_VERSION   Version to tag (default: contents of the VERSION file)
#   DOCKER_ORG      Docker Hub organization (default: runtzdev)
#   PLATFORMS       Target platforms (default: linux/amd64,linux/arm64)

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${RUNTZ_VERSION:-$(cat "${ROOT}/VERSION")}"
ORG="${DOCKER_ORG:-runtzdev}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"
BUILDER="runtz-builder"

info() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

[ -n "$VERSION" ] || fail "VERSION is empty"

# Every release moves :rc — the newest published build, stable or not. Only
# stable releases (no -rc/-beta suffix) also move :latest, which stays
# reserved for the real thing so self-hosted installs can trust that name
# once we cut 1.0.0.
LATEST=false
case "$VERSION" in
  *-*) ;;
  *) LATEST=true ;;
esac

command -v docker >/dev/null || fail "docker is required"
docker buildx inspect "$BUILDER" >/dev/null 2>&1 ||
  docker buildx create --name "$BUILDER" --driver docker-container >/dev/null

# build <image> <context> [ldflags]
build() {
  local image="$1" context="$2" ldflags="${3:-}"
  local args=(-t "${ORG}/${image}:${VERSION}" -t "${ORG}/${image}:rc")
  if [ "$LATEST" = true ]; then
    args+=(-t "${ORG}/${image}:latest")
  fi
  if [ -n "$ldflags" ]; then
    args+=(--build-arg "LDFLAGS=${ldflags}")
  fi

  info "Building and pushing ${ORG}/${image}:${VERSION} (${PLATFORMS})..."
  docker buildx build \
    --builder "$BUILDER" \
    --platform "$PLATFORMS" \
    "${args[@]}" \
    --push \
    "$context"
}

build runtz-engine "${ROOT}/engine" \
  "-s -w -X github.com/runtz-dev/runtz/engine/internal/version.Version=${VERSION}"
build runtz-frontend "${ROOT}/frontend"

info "Done. Published ${ORG}/{runtz-engine,runtz-frontend}:${VERSION} and :rc$([ "$LATEST" = true ] && echo ' and :latest' || true)"
