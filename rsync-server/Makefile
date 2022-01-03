VERSION ?= latest

build:
	docker buildx build --platform linux/amd64 -t infuseai/rsync-server:$(VERSION) .
deploy: build
	docker push infuseai/rsync-server:$(VERSION)
