#!/usr/bin/env bash

set -euox pipefail

ACTION=${ACTION:-}
REGION=${S3_REGION}

# AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
REGISTRY=${REGISTRY:-}
IMAGE_NAME=${IMAGE_NAME:-}
TAG=${TAG:-}

BASE_DIR=$(dirname "$(realpath "${BASH_SOURCE[0]}")")
source "${BASE_DIR}"/helm.sh

TEST_DIR=${BASE_DIR}/../csi-test-artifacts
BIN_DIR=${TEST_DIR}/bin

ARCH=${ARCH:-x86}
SELINUX_MODE=${SELINUX_MODE:-}

CLUSTER_NAME="helm-test-cluster"

HELM_RELEASE_NAME=mountpoint-s3-csi-driver


MOUNTER_KIND=${MOUNTER_KIND:-systemd}

mkdir -p ${TEST_DIR}
mkdir -p ${BIN_DIR}
export PATH="$PATH:${BIN_DIR}"

function print_cluster_info() {
  kubectl logs -l app=s3-csi-node -n kube-system
  kubectl version
  kubectl get nodes -o wide
}


function e2e_cleanup() {
  set -e
  if driver_installed ${HELM_RELEASE_NAME}; then
    for ns in $(kubectl get namespaces -o custom-columns=":metadata.name" | grep -E "^aws-s3-csi-e2e-.*|^volume-.*"); do
      kubectl delete all --all -n $ns --timeout=2m
      kubectl delete namespace $ns --timeout=2m
    done
  fi
  set +e

  for bucket in $(aws s3 ls --region ${REGION} | awk '{ print $3 }' | grep "^${CLUSTER_NAME}-e2e-kubernetes-.*"); do
    aws s3 rb "s3://${bucket}" --force --region ${REGION}
  done
}

function print_cluster_info() {
  kubectl logs -l app=s3-csi-node -n kube-system
  kubectl version
  kubectl get nodes -o wide
}

if [[ "${ACTION}" == "create_cluster" ]]; then
  create_cluster
elif [[ "${ACTION}" == "update_kubeconfig" ]]; then
  update_kubeconfig
elif [[ "${ACTION}" == "install_driver" ]]; then
  helm_install_driver \
    "${HELM_RELEASE_NAME}" \
    "${REGISTRY}/${IMAGE_NAME}" \
    "${TAG}" \
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
    "$HELM_RELEASE_NAME"
elif [[ "${ACTION}" == "delete_cluster" ]]; then
  delete_cluster
elif [[ "${ACTION}" == "e2e_cleanup" ]]; then
  e2e_cleanup || true
else
  echo "ACTION := install_tools|create_cluster|install_driver|update_kubeconfig|run_tests|run_perf|e2e_cleanup|uninstall_driver|delete_cluster"
  exit 1
fi
