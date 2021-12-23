FROM golang:buster

RUN mkdir -p /go/src/app/TaoKan

COPY . /go/src/app/TaoKan

RUN cd /go/src/app/TaoKan && make build-linux

RUN ln -nfs /go/src/app/TaoKan/bin/taokan-linux /go/bin/taokan

