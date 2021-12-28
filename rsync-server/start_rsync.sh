#!/bin/bash
set -eo pipefail

start_rsync() {
  # Check installed rsync
  if command -v rsync &>/dev/null; then
    echo "Starting rsync ..."
    rsync -azcrvhP --bwlimit 250000 --info=progress2 --no-i-r /data/ remote-rsync-server:/data/ | tee /var/log/rsync.log
  fi
}

prepare_ssh_config() {
  # Fix the home directory too open issue
  if [ -e $HOME ]; then
    chmod 755 $HOME
  fi

  # Prepare default .ssh directory and setup permission
  mkdir -p $HOME/.ssh
  chmod 700 $HOME/.ssh

  if [[ ! -z ${SSH_PRIVATE_KEY=+x} ]]; then
    echo "$SSH_PRIVATE_KEY" > $HOME/.ssh/id_rsa
  fi
  chmod 400 $HOME/.ssh/id_rsa

  cat > $HOME/.ssh/config << EOF
HOST *
  StrictHostKeyChecking no
HOST remote-rsync-server
  User root
  Hostname ${REMOTE_SERVER_NAME:-rsync-server}.${REMOTE_NAMESPACE:-hub}
  Port 22
  ForwardAgent yes
  ProxyCommand ssh -W %h:%p -i ~/.ssh/id_rsa limited-user@${REMOTE_K8S_CLUSTER} -p 2222
  IdentityFile ~/.ssh/id_rsa
  StrictHostKeyChecking no
  UserKnownHostsFile=/dev/null
EOF
  chmod 644 $HOME/.ssh/config
}

if [[ ${REMOTE_K8S_CLUSTER:-} != '' ]]; then
  prepare_ssh_config
  start_rsync
else
  echo "[Error] No specific remote k8s cluster"
  exit 1
fi
