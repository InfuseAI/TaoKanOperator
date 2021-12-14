#!/bin/bash

ensure_sshd() {
  # Check installed sshd
  if command -v sshd &>/dev/null; then
    echo "Starting sshd"
    mkdir -p /run/sshd
    $(command -v sshd) -D &
  fi
}

prepare_user_volume() {
  # Fix the home directory too open issue
  if [ -e $HOME ]; then
    chmod 755 $HOME
  fi

  # Prepare default .ssh directory and setup permission
  if [ ! -f $HOME/.ssh/authorized_keys ]; then
    mkdir -p $HOME/.ssh
    chmod 700 $HOME/.ssh
    touch $HOME/.ssh/authorized_keys
    if [[ ! -z ${SSH_PUBLIC_KEY=+x} ]]; then
      echo "$SSH_PUBLIC_KEY" >> $HOME/.ssh/authorized_keys
    fi
    chmod 644 $HOME/.ssh/authorized_keys
  fi
}

ensure_sshd &
prepare_user_volume

# Start publickey api server
if command -v nohup &>/dev/null; then
  nohup python3 /root/publickey_api.py &
fi

inotifywait -rm -e close_write,create /data/
