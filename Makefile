GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
VERSION ?= '$(shell hack/version.sh)'

# Images management
REGISTRY?="ghcr.io/kosmos-io/clusterlink"
REGISTRY_USER_NAME?=""
REGISTRY_PASSWORD?=""
REGISTRY_SERVER_ADDRESS?=""

TARGETS :=  clusterlink-controller-manager  \
			clusterlink-operator \
			clusterlink-agent \
            clusterlink-elector \
			clusterlink-floater \
			clusterlink-network-manager \
			clusterlink-proxy \

CTL_TARGETS := linkctl

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
#   make make image-clusterlink-controller-manager
#   make make image-clusterlink-controller-manager GOARCH=arm64
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
	rm -rf _tmp _output

.PHONY: update
update:
	hack/update-all.sh

.PHONY: verify
verify:
	hack/verify-all.sh

.PHONY: release-chart
release-chart:
	hack/release-helm-chart.sh $(VERSION)

.PHONY: test
test:
	mkdir -p ./_output/coverage/
	go test --race --v ./pkg/... -coverprofile=./_output/coverage/coverage_pkg.txt -covermode=atomic
	go test --race --v ./cmd/... -coverprofile=./_output/coverage/coverage_cmd.txt -covermode=atomic
#   go test --race --v ./examples/... -coverprofile=./_output/coverage/coverage_examples.txt -covermode=atomic

upload-images: images
	@echo "push images to $(REGISTRY)"
ifneq ($(REGISTRY_USER_NAME), "")
	docker login -u ${REGISTRY_USER_NAME} -p ${REGISTRY_PASSWORD} ${REGISTRY_SERVER_ADDRESS}
endif
	docker push ${REGISTRY}/clusterlink-controller-manager:${VERSION}
	docker push ${REGISTRY}/clusterlink-operator:${VERSION}
	docker push ${REGISTRY}/clusterlink-agent:${VERSION}
	docker push ${REGISTRY}/clusterlink-proxy:${VERSION}
	docker push ${REGISTRY}/clusterlink-floater:${VERSION}

# Build and package binary
#
# Example
#   make release-linkctl
RELEASE_TARGET=$(addprefix release-, $(CTL_TARGETS))
.PHONY: $(RELEASE_TARGET)
$(RELEASE_TARGET):
	@set -e;\
	target=$$(echo $(subst release-,,$@));\
	make $$target;\
	hack/release.sh $$target $(GOOS) $(GOARCH)

# Build and package binary for all platforms
#
# Example
#   make release
release:
	@make release-linkctl GOOS=linux GOARCH=amd64
	@make release-linkctl GOOS=linux GOARCH=arm64
	@make release-linkctl GOOS=darwin GOARCH=amd64
	@make release-linkctl GOOS=darwin GOARCH=arm64

.PHONY: lint
lint: golangci-lint
	$(GOLANGLINT_BIN) run

.PHONY: lint-fix
lint-fix: golangci-lint
	$(GOLANGLINT_BIN) run --fix

golangci-lint:
ifeq (, $(shell which golangci-lint))
	GO111MODULE=on go install github.com/golangci/golangci-lint/cmd/golangci-lint@1.53.3
GOLANGLINT_BIN=$(shell go env GOPATH)/bin/golangci-lint
else
GOLANGLINT_BIN=$(shell which golangci-lint)
endif