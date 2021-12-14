#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"

kubectl create secret generic -n hub rsync-ssh-key --from-file=privatekey=$DIR/id_rsa --from-file=publickey=$DIR/id_rsa.pub
