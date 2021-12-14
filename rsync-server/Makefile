build:
	docker buildx build --platform linux/amd64 -t infuseai/rsync-server .
deploy: build
	docker push infuseai/rsync-server
