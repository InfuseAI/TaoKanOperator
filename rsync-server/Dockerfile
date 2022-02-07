FROM ubuntu:18.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    rsync \
    inotify-tools \
    openssh-client \
    openssh-server \
    bash \
    python3 \
    && apt-get purge && apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

RUN mkdir /var/run/sshd

RUN echo 'root:root' |chpasswd

RUN sed -ri 's/^#?PermitRootLogin\s+.*/PermitRootLogin yes/' /etc/ssh/sshd_config
# RUN sed -ri 's/UsePAM yes/#UsePAM yes/g' /etc/ssh/sshd_config

RUN mkdir /root/.ssh
RUN mkdir /root/log-analyzer
RUN mkdir /data

EXPOSE 22

ADD publickey_api.py /root/
ADD ensure_sshd.sh /root/
ADD start_rsync.sh /root/
ADD log-analyzer/* /root/log-analyzer/

CMD ["/bin/bash", "/root/ensure_sshd.sh"]
