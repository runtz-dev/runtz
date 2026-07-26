#!/usr/bin/env bash
#
# Build and push the runtz Docker images to Docker Hub (multi-arch).
#
#   docker login   # as a user with push access to the runtzdev org
#   ./scripts/release-images.sh
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

# Stable releases (no -rc/-beta suffix) also move the :latest tag.
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
  local args=(-t "${ORG}/${image}:${VERSION}")
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

info "Done. Published ${ORG}/{runtz-engine,runtz-frontend}:${VERSION}$([ "$LATEST" = true ] && echo ' and :latest' || true)"
