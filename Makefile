#Copyright 2022 The Kubernetes Authors
#Copyright 2025 Scality, Inc.
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.
SHELL = /bin/bash

# MP CSI Driver version
VERSION=2.1.0

# List of allowed licenses in the CSI Driver's dependencies.
# See https://github.com/google/licenseclassifier/blob/e6a9bb99b5a6f71d5a34336b8245e305f5430f99/license_type.go#L28 for list of canonical names for licenses.
ALLOWED_LICENSES="Apache-2.0,BSD-2-Clause,BSD-3-Clause,ISC,MIT,MPL-2.0"

PKG=github.com/scality/mountpoint-s3-csi-driver
GIT_COMMIT?=$(shell git rev-parse HEAD)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS?="-X ${PKG}/pkg/driver/version.driverVersion=${VERSION} -X ${PKG}/pkg/driver/version.gitCommit=${GIT_COMMIT} -X ${PKG}/pkg/driver/version.buildDate=${BUILD_DATE}"

GO111MODULE=on
GOPROXY=direct
GOPATH=$(shell go env GOPATH)
GOOS=$(shell go env GOOS)
GOBIN=$(GOPATH)/bin

# Container image configuration
CONTAINER_IMAGE ?= scality/mountpoint-s3-csi-driver
CONTAINER_TAG ?= local

# Test configuration variables
E2E_REGION?=us-east-1
E2E_COMMIT_ID?=local
E2E_KUBECONFIG?=""

# Kubernetes version to use in envtest for controller tests.
ENVTEST_K8S_VERSION ?= 1.30.x

# Virtual environment activation
venv := .venv/bin/activate

.EXPORT_ALL_VARIABLES:

.PHONY: bin
bin:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags ${LDFLAGS} -o bin/scality-s3-csi-driver ./cmd/scality-csi-driver/
	CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags ${LDFLAGS} -o bin/scality-csi-controller ./cmd/scality-csi-controller/
	CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags ${LDFLAGS} -o bin/scality-s3-csi-mounter ./cmd/scality-csi-mounter/
	# TODO: `install-mp` component won't be necessary with the containerization.
	CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags ${LDFLAGS} -o bin/install-mp ./cmd/install-mp/

.PHONY: container
container:
	docker build -t ${CONTAINER_IMAGE}:${CONTAINER_TAG} .

.PHONY: unit-test
unit-test:
	go test -v -parallel 8 ./{cmd,pkg}/... -coverprofile=./coverage.out -covermode=atomic -coverpkg=./{cmd,pkg}/...

# Skip patterns for CSI sanity tests
# - ValidateVolumeCapabilities: stub implementation, tested in unit tests (see https://github.com/kubernetes-csi/csi-test/issues/214)
# - Node Service: requires real S3 storage infrastructure, tested in e2e tests
# - Specific tests using SINGLE_NODE_WRITER: S3 only supports multi-node access modes
CSI_SKIP_PATTERNS := ValidateVolumeCapabilities|Node Service|SingleNodeWriter|should not fail when requesting to create a volume with already existing name and same capacity|should fail when requesting to create a volume with already existing name and different capacity|should not fail when creating volume with maximum-length name|should return appropriate values.*no optional values added

.PHONY: csi-compliance-test
csi-compliance-test:
	go test -v ./tests/sanity/... -ginkgo.skip="$(CSI_SKIP_PATTERNS)"

.PHONY: test
test:
	go test -v -race ./{cmd,pkg}/... -coverprofile=./cover.out -covermode=atomic -coverpkg=./{cmd,pkg}/...
	go test -v ./tests/sanity/... -ginkgo.skip="$(CSI_SKIP_PATTERNS)"

.PHONY: cover
cover:
	go tool cover -html=coverage.out -o=coverage.html

.PHONY: fmt
fmt:
	go fmt ./...

# Validate Helm charts for correctness and requirements
.PHONY: validate-helm
validate-helm:
	@echo "Validating Helm charts..."
	@tests/helm/validate_charts.sh

################################################################
# Documentation commands
################################################################

.PHONY: docs
docs:
	@echo "Building documentation and starting server (strict mode)..."
	source $(venv) && mkdocs build --strict && mkdocs serve

.PHONY: docs-clean
docs-clean:
	@echo "Cleaning documentation build artifacts..."
	rm -rf site/

# Run controller tests with envtest.
.PHONY: controller-integration-test
controller-integration-test: envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(TESTBIN) -p path)" go test ./tests/controller/... -ginkgo.v -ginkgo.junit-report=../../controller-integration-tests-results.xml -test.v

.PHONY: lint
lint:
	test -z "$$(gofmt -d . | tee /dev/stderr)"

.PHONY: precommit
precommit:
	pre-commit run --all-files

.PHONY: clean
clean:
	rm -rf bin/ && docker system prune

################################################################
# License checking and generation
################################################################

# Download Go tools and dependencies (required for CI)
.PHONY: download-tools
download-tools:
	@echo "Downloading Go tools and dependencies..."
	go mod download

# Check that all dependencies use allowed licenses
.PHONY: check-licenses
check-licenses:
	@echo "Checking licenses for all dependencies..."
	go tool go-licenses check --allowed_licenses ${ALLOWED_LICENSES} ./...

# Generate license files for all dependencies
.PHONY: generate-licenses
generate-licenses: download-tools
	@echo "Generating license files..."
	@rm -rf LICENSES/
	@mkdir -p LICENSES/
	go tool go-licenses save --save_path="./LICENSES" --force ./...

# Generate CRD manifests and deepcopy functions
# Note: Currently the CRD is placed directly in crds/ subdirectory
# AWS upstream uses a different approach with patches for K8s version compatibility
.PHONY: generate
generate:
	@echo "Generating deepcopy functions..."
	@controller-gen object paths="./pkg/api/v2/..."
	@echo "Generating CRD manifests..."
	@controller-gen crd paths="./pkg/api/v2/..." output:crd:artifacts:config=charts/scality-mountpoint-s3-csi-driver/crds
	@echo "Generation complete. Note: selectableFields requires K8s >= 1.30 for our CRD"
	@# Rename to simpler filename since we only have one CRD
	@mv charts/scality-mountpoint-s3-csi-driver/crds/s3.csi.scality.com_mountpoints3podattachments.yaml \
	    charts/scality-mountpoint-s3-csi-driver/crds/mountpoints3podattachments.yaml 2>/dev/null || true

## Binaries used in tests.

TESTBIN ?= $(shell pwd)/tests/bin
$(TESTBIN):
	mkdir -p $(TESTBIN)

ENVTEST ?= $(TESTBIN)/setup-envtest
ENVTEST_VERSION ?= release-0.19

.PHONY: envtest
envtest: $(ENVTEST)
$(ENVTEST): $(TESTBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

# Copied from https://github.com/kubernetes-sigs/kubebuilder/blob/c32f9714456f7e5e7cc6c790bb87c7e5956e710b/pkg/plugins/golang/v4/scaffolds/internal/templates/makefile.go#L275-L289.
# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(TESTBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef


################################################################
# Scality CSI driver configuration
################################################################

# Image tag for the CSI driver (optional)
CSI_IMAGE_TAG ?=

# Custom image repository for the CSI driver (optional)
CSI_IMAGE_REPOSITORY ?=

# Namespace to deploy the CSI driver in (optional, defaults to kube-system)
CSI_NAMESPACE ?=

# S3 endpoint URL (REQUIRED)
# Example: https://s3.your-scality.com
S3_ENDPOINT_URL ?=

# Note: AWS/S3 credentials are loaded from environment variables (ACCOUNT1_ACCESS_KEY, ACCOUNT1_SECRET_KEY)
# Run 'source tests/e2e/scripts/load-credentials.sh' before using these targets

# Set to 'true' to validate S3 credentials before installation (optional)
# Checks endpoint connectivity and validates credentials (if AWS CLI is available)
VALIDATE_S3 ?= false

# Additional arguments to pass to the script (optional)
ADDITIONAL_ARGS ?=

# Additional Helm arguments for CSI driver installation (optional)
# Example: ADDITIONAL_HELM_ARGS="--set tls.caCertSecret=my-ca-secret"
ADDITIONAL_HELM_ARGS ?=

################################################################
# Scality CSI driver commands
################################################################

# Show help for CSI driver commands
.PHONY: help-csi
help-csi:
	@echo "Scality CSI Driver Make Targets:"
	@echo ""
	@echo "Installation:"
	@echo "  csi-install         - Install the CSI driver"
	@echo "  csi-uninstall       - Uninstall the CSI driver (interactive)"
	@echo "  csi-uninstall-clean - Uninstall and delete custom namespace"
	@echo "  csi-uninstall-force - Force uninstall the CSI driver"
	@echo ""
	@echo "Building:"
	@echo "  bin                - Build binaries"
	@echo "  container          - Build container image (default tag: local)"
	@echo ""
	@echo "Testing:"
	@echo "  e2e                - Run tests on installed driver"
	@echo "  e2e-go             - Run only Go-based tests"
	@echo "  e2e-verify         - Run only verification tests"
	@echo "  e2e-all            - Install driver and run all tests"
	@echo ""
	@echo "Prerequisites:"
	@echo "  1. Run 'source tests/e2e/scripts/load-credentials.sh' to load credentials"
	@echo "  2. Provide S3_ENDPOINT_URL parameter to installation/test commands"
	@echo "  3. Ensure KUBECONFIG is set or ~/.kube/config exists"
	@echo ""
	@echo "Example workflow:"
	@echo "  # Build container with default tag (local):"
	@echo "  make container"
	@echo "  # Build container with custom tag:"
	@echo "  make container CONTAINER_TAG=1.1.3"
	@echo ""
	@echo "  # E2E testing:"
	@echo "  source tests/e2e/scripts/load-credentials.sh"
	@echo "  make e2e-all S3_ENDPOINT_URL=https://s3.example.com"
	@echo "  # Or with custom kubeconfig:"
	@echo "  make e2e-all S3_ENDPOINT_URL=https://s3.example.com KUBECONFIG=/path/to/kubeconfig"



# Install the Scality CSI driver
#
# Prerequisites:
#   Run 'source tests/e2e/scripts/load-credentials.sh' to load credentials
#
# Required parameters:
#   S3_ENDPOINT_URL - Your Scality S3 endpoint
#
# Optional parameters:
#   CSI_IMAGE_TAG - Specific version of the driver
#   CSI_IMAGE_REPOSITORY - Custom image repository for the driver
#   CSI_NAMESPACE - Namespace to deploy the CSI driver in (defaults to kube-system)
#   VALIDATE_S3 - Set to "true" to verify S3 credentials
#
# Example:
#   source tests/e2e/scripts/load-credentials.sh
#   make csi-install S3_ENDPOINT_URL=https://s3.example.com
.PHONY: csi-install
csi-install:
	@if [ -z "$(S3_ENDPOINT_URL)" ]; then \
		echo "Error: S3_ENDPOINT_URL is required. Please provide it with 'make S3_ENDPOINT_URL=https://s3.example.com csi-install'"; \
		exit 1; \
	fi; \
	INSTALL_ARGS=""; \
	if [ ! -z "$(CSI_IMAGE_TAG)" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS --image-tag $(CSI_IMAGE_TAG)"; \
	fi; \
	if [ ! -z "$(CSI_IMAGE_REPOSITORY)" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS --image-repository $(CSI_IMAGE_REPOSITORY)"; \
	fi; \
	if [ ! -z "$(CSI_NAMESPACE)" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS --namespace $(CSI_NAMESPACE)"; \
	fi; \
	INSTALL_ARGS="$$INSTALL_ARGS --endpoint-url $(S3_ENDPOINT_URL)"; \
	if [ "$(VALIDATE_S3)" = "true" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS --validate-s3"; \
	fi; \
	if [ ! -z "$(ADDITIONAL_HELM_ARGS)" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS --additional-helm-args '$(ADDITIONAL_HELM_ARGS)'"; \
	fi; \
	if [ ! -z "$(ADDITIONAL_ARGS)" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS $(ADDITIONAL_ARGS)"; \
	fi; \
	./tests/e2e/scripts/run.sh install $$INSTALL_ARGS

# Uninstall the Scality CSI driver (interactive mode)
# This will uninstall from the default namespace (kube-system) or the specified namespace
# Note: kube-system namespace will NOT be deleted even with --delete-ns
.PHONY: csi-uninstall
csi-uninstall:
	@UNINSTALL_ARGS=""; \
	if [ ! -z "$(CSI_NAMESPACE)" ]; then \
		UNINSTALL_ARGS="$$UNINSTALL_ARGS --namespace $(CSI_NAMESPACE)"; \
	fi; \
	./tests/e2e/scripts/run.sh uninstall $$UNINSTALL_ARGS

# Uninstall the Scality CSI driver and delete custom namespace
# This automatically deletes namespace without prompting ONLY for custom namespaces
# Note: kube-system namespace will NOT be deleted even with --delete-ns
.PHONY: csi-uninstall-clean
csi-uninstall-clean:
	@UNINSTALL_ARGS="--delete-ns"; \
	if [ ! -z "$(CSI_NAMESPACE)" ]; then \
		UNINSTALL_ARGS="$$UNINSTALL_ARGS --namespace $(CSI_NAMESPACE)"; \
	fi; \
	./tests/e2e/scripts/run.sh uninstall $$UNINSTALL_ARGS

# Force uninstall the Scality CSI driver
# Use this when standard uninstall methods aren't working
# Note: kube-system namespace will NOT be deleted even with --force
.PHONY: csi-uninstall-force
csi-uninstall-force:
	@UNINSTALL_ARGS="--force"; \
	if [ ! -z "$(CSI_NAMESPACE)" ]; then \
		UNINSTALL_ARGS="$$UNINSTALL_ARGS --namespace $(CSI_NAMESPACE)"; \
	fi; \
	./tests/e2e/scripts/run.sh uninstall $$UNINSTALL_ARGS

################################################################
# E2E test commands for Scality
################################################################

# Run tests on an already installed CSI driver
# Tests use credentials from Kubernetes secrets created during installation
.PHONY: e2e
e2e:
	@TEST_ARGS=""; \
	if [ ! -z "$(CSI_NAMESPACE)" ]; then \
		TEST_ARGS="$$TEST_ARGS --namespace $(CSI_NAMESPACE)"; \
	fi; \
	if [ ! -z "$(S3_ENDPOINT_URL)" ]; then \
		TEST_ARGS="$$TEST_ARGS --endpoint-url $(S3_ENDPOINT_URL)"; \
	fi; \
	if [ ! -z "$(KUBECONFIG)" ]; then \
		TEST_ARGS="$$TEST_ARGS --kubeconfig $(KUBECONFIG)"; \
	fi; \
	./tests/e2e/scripts/run.sh test $$TEST_ARGS

# Run only the Go-based e2e tests (skips verification checks)
# Tests use credentials from Kubernetes secrets created during installation
#
# Usage: make e2e-go
.PHONY: e2e-go
e2e-go:
	@TEST_ARGS=""; \
	if [ ! -z "$(CSI_NAMESPACE)" ]; then \
		TEST_ARGS="$$TEST_ARGS --namespace $(CSI_NAMESPACE)"; \
	fi; \
	if [ ! -z "$(S3_ENDPOINT_URL)" ]; then \
		TEST_ARGS="$$TEST_ARGS --endpoint-url $(S3_ENDPOINT_URL)"; \
	fi; \
	if [ ! -z "$(KUBECONFIG)" ]; then \
		TEST_ARGS="$$TEST_ARGS --kubeconfig $(KUBECONFIG)"; \
	fi; \
	./tests/e2e/scripts/run.sh go-test $$TEST_ARGS

# Run the verification tests only (skips Go tests)
# Makes sure the CSI driver is properly installed
.PHONY: e2e-verify
e2e-verify:
	@TEST_ARGS="--skip-go-tests"; \
	if [ ! -z "$(CSI_NAMESPACE)" ]; then \
		TEST_ARGS="$$TEST_ARGS --namespace $(CSI_NAMESPACE)"; \
	fi; \
	./tests/e2e/scripts/run.sh test $$TEST_ARGS

# Install CSI driver and run all tests in one command
#
# Prerequisites:
#   Run 'source tests/e2e/scripts/load-credentials.sh' to load credentials
#
# Required parameters:
#   S3_ENDPOINT_URL - Your Scality S3 endpoint
#
# Optional parameters:
#   CSI_IMAGE_TAG - Specific version of the driver
#   CSI_IMAGE_REPOSITORY - Custom image repository for the driver
#   CSI_NAMESPACE - Namespace to deploy the CSI driver in (defaults to kube-system)
#   VALIDATE_S3 - Set to "true" to verify S3 credentials
#
# Example:
#   source tests/e2e/scripts/load-credentials.sh
#   make e2e-all S3_ENDPOINT_URL=https://s3.example.com
.PHONY: e2e-all
e2e-all:
	@if [ -z "$(S3_ENDPOINT_URL)" ]; then \
		echo "Error: S3_ENDPOINT_URL is required. Please provide it with 'make S3_ENDPOINT_URL=https://s3.example.com e2e-all'"; \
		exit 1; \
	fi; \
	INSTALL_ARGS=""; \
	if [ ! -z "$(CSI_IMAGE_TAG)" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS --image-tag $(CSI_IMAGE_TAG)"; \
	fi; \
	if [ ! -z "$(CSI_IMAGE_REPOSITORY)" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS --image-repository $(CSI_IMAGE_REPOSITORY)"; \
	fi; \
	if [ ! -z "$(CSI_NAMESPACE)" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS --namespace $(CSI_NAMESPACE)"; \
	fi; \
	INSTALL_ARGS="$$INSTALL_ARGS --endpoint-url $(S3_ENDPOINT_URL)"; \
	if [ "$(VALIDATE_S3)" = "true" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS --validate-s3"; \
	fi; \
	if [ ! -z "$(ADDITIONAL_HELM_ARGS)" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS --additional-helm-args '$(ADDITIONAL_HELM_ARGS)'"; \
	fi; \
	if [ ! -z "$(ADDITIONAL_ARGS)" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS $(ADDITIONAL_ARGS)"; \
	fi; \
	./tests/e2e/scripts/run.sh all $$INSTALL_ARGS
