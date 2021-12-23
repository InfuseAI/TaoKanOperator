#! /bin/bash

if [[ $1 == "" ]]; then
  echo "Usage: `basename $0` <Remote-Cluster>"
  exit 1
fi

helm upgrade --install taokan-operator . --create-namespace --namespace hub --set taoKan.remoteCluster=$1
