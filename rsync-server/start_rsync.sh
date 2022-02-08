#!/bin/bash
set -eo pipefail

RSYNC_BWLIMIT=${RSYNC_BWLIMIT:-250000}

start_rsync() {
  # Check installed rsync
  if command -v rsync &>/dev/null; then
    echo "[Start] Backup by rsync ..."
    echo "  CMD: rsync -azcrvhP --bwlimit ${RSYNC_BWLIMIT} --info=progress2 --no-i-r  --stats --log-file=/var/log/rsync.log /data/ remote-rsync-server:/data/"
    rsync -azcrvhP --bwlimit ${RSYNC_BWLIMIT} --info=progress2 --no-i-r  --stats --log-file=/var/log/rsync.log /data/ remote-rsync-server:/data/ | tee /var/log/rsync_progress.log
    echo "[Completed] Backup"

    ssh remote-rsync-server mkdir -p /data/backup_log/
    now=$(date +'%Y-%m-%d-%H%M')
    rsync_log="${REMOTE_PVC_NAME:-rsync-worker}-${now}.log"
    progress_log="${REMOTE_PVC_NAME:-rsync-worker}-${now}-progress.log"
    cp /var/log/rsync.log /var/log/${rsync_log}
    cp /var/log/rsync_progress.log /var/log/${progress_log}
    echo "[Start] Copy rsync log: ${rsync_log} & ${progress_log}"
    rsync -zvh --bwlimit ${RSYNC_BWLIMIT} /var/log/${rsync_log} remote-rsync-server:/data/backup_log/
    rsync -zvh --bwlimit ${RSYNC_BWLIMIT} /var/log/${progress_log} remote-rsync-server:/data/backup_log/
    rsync -zvh --bwlimit ${RSYNC_BWLIMIT} /root/log-analyzer/* remote-rsync-server:/data/
    echo "[Completed]"
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
