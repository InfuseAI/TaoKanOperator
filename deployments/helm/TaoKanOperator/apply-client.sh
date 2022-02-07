#! /bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"

if [[ $1 == "" ]]; then
  echo "Usage: `basename $0` <Remote-Cluster>"
  exit 1
fi

WHITE_LIST_FLAGS=""
WHITE_LIST_DIR="${DIR}/whiteList"
if [[ -d "${WHITE_LIST_DIR}" ]]; then
  if [[ -f "${WHITE_LIST_DIR}/users.txt" ]]; then
    WHITE_LIST_FLAGS="--set-file user.whiteList=${WHITE_LIST_DIR}/users.txt"
  fi
  if [[ -f "${WHITE_LIST_DIR}/projects.txt" ]]; then
    WHITE_LIST_FLAGS="${WHITE_LIST_FLAGS} --set-file project.whiteList=${WHITE_LIST_DIR}/projects.txt"
  fi
  if [[ -f "${WHITE_LIST_DIR}/datasets.txt" ]]; then
    WHITE_LIST_FLAGS="${WHITE_LIST_FLAGS} --set-file dataset.whiteList=${WHITE_LIST_DIR}/datasets.txt"
  fi
fi

EXCLUSIVE_LIST_FLAGS=""
EXCLUSIVE_LIST_DIR="${DIR}/exclusiveList"
if [[ -d "${EXCLUSIVE_LIST_DIR}" ]]; then
    if [[ -f "${EXCLUSIVE_LIST_DIR}/users.txt" ]]; then
      EXCLUSIVE_LIST_FLAGS="--set-file user.exclusiveList=${EXCLUSIVE_LIST_DIR}/users.txt"
    fi
    if [[ -f "${EXCLUSIVE_LIST_DIR}/projects.txt" ]]; then
      EXCLUSIVE_LIST_FLAGS="${EXCLUSIVE_LIST_FLAGS} --set-file project.exclusiveList=${EXCLUSIVE_LIST_DIR}/projects.txt"
    fi
    if [[ -f "${EXCLUSIVE_LIST_DIR}/datasets.txt" ]]; then
      EXCLUSIVE_LIST_FLAGS="${EXCLUSIVE_LIST_FLAGS} --set-file dataset.exclusiveList=${EXCLUSIVE_LIST_DIR}/datasets.txt"
    fi
fi

HELM_OVERRIDE_FLAGS=""
if [[ -f "${DIR}/helm_override/client.yaml" ]]; then
  HELM_OVERRIDE_FLAGS="-f ${DIR}/helm_override/client.yaml"
fi

helm upgrade --install taokan-operator . --create-namespace --namespace hub --set taoKan.remoteCluster=$1 \
  ${WHITE_LIST_FLAGS} \
  ${EXCLUSIVE_LIST_FLAGS} \
  ${HELM_OVERRIDE_FLAGS}
