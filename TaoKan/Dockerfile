FROM golang:buster
ARG version=latest

RUN mkdir -p /go/src/app/TaoKan

COPY . /go/src/app/TaoKan

RUN cd /go/src/app/TaoKan && make build-linux VERSION=${version}

RUN ln -nfs /go/src/app/TaoKan/bin/taokan-linux /go/bin/taokan

