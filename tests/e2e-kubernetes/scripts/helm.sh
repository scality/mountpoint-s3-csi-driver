#!/usr/bin/env bash

set -euox pipefail

function helm_uninstall_driver() {
  RELEASE_NAME=${3}
  if driver_installed helm ${RELEASE_NAME}; then
    helm uninstall $RELEASE_NAME --namespace kube-system
    kubectl wait --for=delete pod --selector="app=s3-csi-node" -n kube-system --timeout=60s
  else
    echo "driver does not seem to be installed"
  fi
  kubectl get pods -A
  kubectl get CSIDriver
}

function helm_install_driver() {
  RELEASE_NAME=${1}
  REPOSITORY=${2}
  TAG=${3}
  MOUNTER_KIND=${4}

  if [ "$MOUNTER_KIND" = "pod" ]; then
    USE_POD_MOUNTER=true
  else
    USE_POD_MOUNTER=false
  fi

  helm_uninstall_driver \
    "$RELEASE_NAME"
  helm upgrade --install $RELEASE_NAME --namespace kube-system ./charts/aws-mountpoint-s3-csi-driver --values \
    ./charts/aws-mountpoint-s3-csi-driver/values.yaml \
    --set image.repository=${REPOSITORY} \
    --set image.tag=${TAG} \
    --set node.serviceAccount.create=true \
    --set node.podInfoOnMountCompat.enable=true \
    --set experimental.podMounter=${USE_POD_MOUNTER}
  kubectl rollout status daemonset s3-csi-node -n kube-system --timeout=60s
  kubectl get pods -A
  echo "s3-csi-node-image: $(kubectl get daemonset s3-csi-node -n kube-system -o jsonpath="{$.spec.template.spec.containers[:1].image}")"
}

function driver_installed() {
  RELEASE_NAME=${2}
  set +e
  if [[ $(helm list -A | grep $RELEASE_NAME) == *deployed* ]]; then
    set -e
    return 0
  else
    set -e
    return 1
  fi
}
