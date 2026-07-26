VERSION := $(shell cat VERSION)
ENGINE_LDFLAGS := -s -w -X github.com/runtz-dev/runtz/engine/internal/version.Version=$(VERSION)

.PHONY: version build build-engine test lint fmt images release-images helm-lint helm-template helm-release

version:
	@echo $(VERSION)

build: build-engine

build-engine:
	cd engine && go build -ldflags "$(ENGINE_LDFLAGS)" -o ../bin/runtz-engine ./cmd/server

test:
	cd engine && go test ./...
	cd frontend && npm run lint

fmt:
	cd engine && gofmt -w . && go vet ./...

# Build the platform Docker images locally (single arch, no push).
# The CLI ships from the runtz-dev/runtz-cli repository.
images:
	docker build -t runtzdev/runtz-engine:$(VERSION) engine
	docker build -t runtzdev/runtz-frontend:$(VERSION) frontend

# Build and push multi-arch images to Docker Hub (requires docker login).
release-images:
	./scripts/release-images.sh

helm-lint:
	helm lint helm/runtz

helm-template:
	helm template runtz helm/runtz

# Package the chart and push it to the ChartMuseum repository.
helm-release:
	./scripts/helm-release.sh
