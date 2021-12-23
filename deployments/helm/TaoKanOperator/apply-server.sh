#! /bin/bash

helm upgrade --install taokan-operator . --create-namespace --namespace hub --set taoKan.serverMode=true
