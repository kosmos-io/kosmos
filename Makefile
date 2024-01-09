GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
VERSION ?= '$(shell hack/version.sh)'

# Images management
REGISTRY?="ghcr.io/kosmos-io"
REGISTRY_USER_NAME?=""
REGISTRY_PASSWORD?=""
REGISTRY_SERVER_ADDRESS?=""
KIND_IMAGE_TAG?="v1.25.3"

MACOS_TARGETS := clusterlink-controller-manager  \
				kosmos-operator \
				 clusterlink-elector \
				clusterlink-network-manager \
				clusterlink-proxy \
				clustertree-cluster-manager \
				scheduler \

# clusterlink-agent and clusterlink-floater only support linux platform
TARGETS :=  clusterlink-controller-manager  \
			kosmos-operator \
			clusterlink-agent \
            clusterlink-elector \
			clusterlink-floater \
			clusterlink-network-manager \
			clusterlink-proxy \
			clustertree-cluster-manager \
			scheduler \

# If GOOS is macOS, assign the value of MACOS_TARGETS to TARGETS
ifeq ($(GOOS), darwin)
	TARGETS := $(MACOS_TARGETS)
endif

CTL_TARGETS := kosmosctl

# Build code.
#
# Args:
#   GOOS:   OS to build.
#   GOARCH: Arch to build.
#
# Example:
#   make
#   make all
#   make clusterlink-controller-manager
#   make clusterlink-controller-manager GOOS=linux
CMD_TARGET=$(TARGETS) $(CTL_TARGETS)

.PHONY: all
all: $(CMD_TARGET)

.PHONY: $(CMD_TARGET)
$(CMD_TARGET):
	BUILD_PLATFORMS=$(GOOS)/$(GOARCH) hack/build.sh $@

# Build image.
#
# Args:
#   GOARCH:      Arch to build.
#   OUTPUT_TYPE: Destination to save image(docker/registry).
#
# Example:
#   make images
#   make image-clusterlink-controller-manager
#   make image-clusterlink-controller-manager GOARCH=arm64
IMAGE_TARGET=$(addprefix image-, $(TARGETS))
.PHONY: $(IMAGE_TARGET)
$(IMAGE_TARGET):
	set -e;\
	target=$$(echo $(subst image-,,$@));\
	make $$target GOOS=linux;\
	VERSION=$(VERSION) REGISTRY=$(REGISTRY) BUILD_PLATFORMS=linux/$(GOARCH) hack/docker.sh $$target

images: $(IMAGE_TARGET)

# Build and push multi-platform image to DockerHub
#
# Example
#   make multi-platform-images
#   make mp-image-clusterlink-controller-manager
MP_TARGET=$(addprefix mp-image-, $(TARGETS))
.PHONY: $(MP_TARGET)
$(MP_TARGET):
	set -e;\
	target=$$(echo $(subst mp-image-,,$@));\
	make $$target GOOS=linux GOARCH=amd64;\
	make $$target GOOS=linux GOARCH=arm64;\
	VERSION=$(VERSION) REGISTRY=$(REGISTRY) \
		OUTPUT_TYPE=registry \
		BUILD_PLATFORMS=linux/amd64,linux/arm64 \
		hack/docker.sh $$target

multi-platform-images: $(MP_TARGET)

.PHONY: clean
clean:
	hack/clean.sh

.PHONY: update
update:
	hack/update-all.sh

# verify-all.sh can not found
.PHONY: verify
verify:
	hack/verify-all.sh

.PHONY: test
test:
	mkdir -p ./_output/coverage/
	go test --race --v ./pkg/... -coverprofile=./_output/coverage/coverage_pkg.txt -covermode=atomic
	go test --race --v ./cmd/... -coverprofile=./_output/coverage/coverage_cmd.txt -covermode=atomic

upload-images: images
	@echo "push images to $(REGISTRY)"
	docker push ${REGISTRY}/clusterlink-controller-manager:${VERSION}
	docker push ${REGISTRY}/kosmos-operator:${VERSION}
	docker push ${REGISTRY}/clusterlink-agent:${VERSION}
	docker push ${REGISTRY}/clusterlink-proxy:${VERSION}
	docker push ${REGISTRY}/clusterlink-network-manager:${VERSION}
	docker push ${REGISTRY}/clusterlink-floater:${VERSION}
	docker push ${REGISTRY}/clusterlink-elector:${VERSION}
	docker push ${REGISTRY}/clustertree-cluster-manager:${VERSION}
    docker push ${REGISTRY}/scheduler:${VERSION}

.PHONY: release
release:
	@make release-kosmosctl GOOS=linux GOARCH=amd64
	@make release-kosmosctl GOOS=linux GOARCH=arm64
	@make release-kosmosctl GOOS=darwin GOARCH=amd64
	@make release-kosmosctl GOOS=darwin GOARCH=arm64

release-kosmosctl:
	hack/release.sh kosmosctl ${GOOS} ${GOARCH}

.PHONY: lint
lint: golangci-lint
	$(GOLANGLINT_BIN) run

.PHONY: lint-fix
lint-fix: golangci-lint
	$(GOLANGLINT_BIN) run --fix

golangci-lint:
ifeq (, $(shell which golangci-lint))
	GO111MODULE=on go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.54.2
GOLANGLINT_BIN=$(shell go env GOPATH)/bin/golangci-lint
else
GOLANGLINT_BIN=$(shell which golangci-lint)
endif

image-base-kind-builder:
	docker buildx build \
	    -t $(REGISTRY)/node:$(KIND_IMAGE_TAG) \
        --platform=linux/amd64,linux/arm64 \
        --push \
        -f cluster/images/buildx.kind.Dockerfile .
