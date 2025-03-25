#!/usr/bin/env bash

set -euox pipefail

ACTION=${ACTION:-}
REGION=${S3_REGION}

# AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
REGISTRY=${REGISTRY:-}
IMAGE_NAME=${IMAGE_NAME:-}
TAG=${TAG:-}

BASE_DIR=$(dirname "$(realpath "${BASH_SOURCE[0]}")")
source "${BASE_DIR}"/kops.sh
source "${BASE_DIR}"/eksctl.sh
source "${BASE_DIR}"/helm.sh

TEST_DIR=${BASE_DIR}/../csi-test-artifacts
BIN_DIR=${TEST_DIR}/bin

HELM_BIN=${BIN_DIR}/helm

ARCH=${ARCH:-x86}
SELINUX_MODE=${SELINUX_MODE:-}

CLUSTER_NAME="helm-test-cluster"

HELM_RELEASE_NAME=mountpoint-s3-csi-driver


MOUNTER_KIND=${MOUNTER_KIND:-systemd}

mkdir -p ${TEST_DIR}
mkdir -p ${BIN_DIR}
export PATH="$PATH:${BIN_DIR}"

# function kubectl_install() {
#   curl -LO "https://dl.k8s.io/release/v$K8S_VERSION/bin/linux/amd64/kubectl"
#   curl -LO "https://dl.k8s.io/release/v$K8S_VERSION/bin/linux/amd64/kubectl.sha256"
#   echo "$(cat kubectl.sha256)  kubectl" | sha256sum --check
#   sudo install -o root -g root -m 0755 kubectl ${KUBECTL_INSTALL_PATH}/kubectl
# }

function print_cluster_info() {
  $KUBECTL_BIN logs -l app=s3-csi-node -n kube-system --kubeconfig ${KUBECONFIG}
  $KUBECTL_BIN version --kubeconfig ${KUBECONFIG}
  $KUBECTL_BIN get nodes -o wide --kubeconfig ${KUBECONFIG}
}


function create_cluster() {
  if [[ "${CLUSTER_TYPE}" == "kops" ]]; then
    kops_create_cluster \
      "$CLUSTER_NAME" \
      "$KOPS_BIN" \
      "$ZONES" \
      "$NODE_COUNT" \
      "$INSTANCE_TYPE" \
      "$AMI_ID" \
      "$K8S_VERSION_KOPS" \
      "$CLUSTER_FILE" \
      "$KUBECONFIG" \
      "$KOPS_PATCH_FILE" \
      "$KOPS_PATCH_NODE_FILE" \
      "$KOPS_STATE_FILE" \
      "$SSH_KEY" \
      "$KOPS_PATCH_NODE_SELINUX_ENFORCING_FILE"
  elif [[ "${CLUSTER_TYPE}" == "eksctl" ]]; then
    eksctl_create_cluster \
      "$CLUSTER_NAME" \
      "$REGION" \
      "$KUBECONFIG" \
      "$CLUSTER_FILE" \
      "$EKSCTL_BIN" \
      "$KUBECTL_BIN" \
      "$EKSCTL_PATCH_FILE" \
      "$ZONES" \
      "$CI_ROLE_ARN" \
      "$INSTANCE_TYPE" \
      "$AMI_FAMILY" \
      "$K8S_VERSION_EKSCTL" \
      "$EKSCTL_PATCH_SELINUX_ENFORCING_FILE"
  fi
}

function delete_cluster() {
  if [[ "${CLUSTER_TYPE}" == "kops" ]]; then
    kops_delete_cluster \
      "${KOPS_BIN}" \
      "${CLUSTER_NAME}" \
      "${KOPS_STATE_FILE}" \
      "${FORCE:-}"
  elif [[ "${CLUSTER_TYPE}" == "eksctl" ]]; then
    eksctl_delete_cluster \
      "$EKSCTL_BIN" \
      "$CLUSTER_NAME" \
      "$REGION" \
      "${FORCE:-}"
  fi
}

function update_kubeconfig() {
  if [[ "${CLUSTER_TYPE}" == "kops" ]]; then
    ${KOPS_BIN} export kubecfg --state "${KOPS_STATE_FILE}" "${CLUSTER_NAME}" --admin --kubeconfig "${KUBECONFIG}"
  elif [[ "${CLUSTER_TYPE}" == "eksctl" ]]; then
    aws eks update-kubeconfig --name ${CLUSTER_NAME} --region ${REGION} --kubeconfig=${KUBECONFIG}
  fi
}

function e2e_cleanup() {
  set -e
  if driver_installed ${HELM_BIN} ${HELM_RELEASE_NAME} ${KUBECONFIG}; then
    for ns in $($KUBECTL_BIN get namespaces -o custom-columns=":metadata.name" --kubeconfig "${KUBECONFIG}" | grep -E "^aws-s3-csi-e2e-.*|^volume-.*"); do
      $KUBECTL_BIN delete all --all -n $ns --timeout=2m --kubeconfig "${KUBECONFIG}"
      $KUBECTL_BIN delete namespace $ns --timeout=2m --kubeconfig "${KUBECONFIG}"
    done
  fi
  set +e

  for bucket in $(aws s3 ls --region ${REGION} | awk '{ print $3 }' | grep "^${CLUSTER_NAME}-e2e-kubernetes-.*"); do
    aws s3 rb "s3://${bucket}" --force --region ${REGION}
  done
}

function print_cluster_info() {
  $KUBECTL_BIN logs -l app=s3-csi-node -n kube-system --kubeconfig ${KUBECONFIG}
  $KUBECTL_BIN version --kubeconfig ${KUBECONFIG}
  $KUBECTL_BIN get nodes -o wide --kubeconfig ${KUBECONFIG}
}

if [[ "${ACTION}" == "create_cluster" ]]; then
  create_cluster
elif [[ "${ACTION}" == "update_kubeconfig" ]]; then
  update_kubeconfig
elif [[ "${ACTION}" == "install_driver" ]]; then
  helm_install_driver \
    "$HELM_BIN" \
    "$KUBECTL_BIN" \
    "$HELM_RELEASE_NAME" \
    "${REGISTRY}/${IMAGE_NAME}" \
    "${TAG}" \
    "${KUBECONFIG}" \
    "${MOUNTER_KIND}"
elif [[ "${ACTION}" == "run_tests" ]]; then
  set +e
  pushd tests/e2e-kubernetes
  KUBECONFIG=${KUBECONFIG} ginkgo -p -vv -timeout 60m -- --bucket-region=${REGION} --commit-id=${TAG} --bucket-prefix=${CLUSTER_NAME} --imds-available=true
  EXIT_CODE=$?
  print_cluster_info
  exit $EXIT_CODE
elif [[ "${ACTION}" == "run_perf" ]]; then
  set +e
  pushd tests/e2e-kubernetes
  KUBECONFIG=${KUBECONFIG} go test -ginkgo.vv --bucket-region=${REGION} --commit-id=${TAG} --bucket-prefix=${CLUSTER_NAME} --performance=true --imds-available=true
  EXIT_CODE=$?
  print_cluster_info
  popd
  cat tests/e2e-kubernetes/csi-test-artifacts/output.json
  exit $EXIT_CODE
elif [[ "${ACTION}" == "uninstall_driver" ]]; then
  helm_uninstall_driver \
    "$HELM_BIN" \
    "$KUBECTL_BIN" \
    "$HELM_RELEASE_NAME" \
    "${KUBECONFIG}"
elif [[ "${ACTION}" == "delete_cluster" ]]; then
  delete_cluster
elif [[ "${ACTION}" == "e2e_cleanup" ]]; then
  e2e_cleanup || true
else
  echo "ACTION := install_tools|create_cluster|install_driver|update_kubeconfig|run_tests|run_perf|e2e_cleanup|uninstall_driver|delete_cluster"
  exit 1
fi
