BINARY_PATH=bin
BINARY_NAME=taokan
CURRENT_OS=$(shell uname | tr A-Z a-z)
IMAGE_NAME=infuseai/taokan
VERSION ?= latest
DOCKER_BUILD_CMD=

ifeq ($(CURRENT_OS),darwin)
	DOCKER_BUILD_CMD=buildx build --platform linux/amd64
else
	DOCKER_BUILD_CMD=build
endif

build-mac: tidy
	GOOS=darwin go build -ldflags '-X TaoKan/cmd.version=${VERSION}' -o ${BINARY_PATH}/${BINARY_NAME}-darwin main.go

build-linux: tidy
	GOARCH=amd64 GOOS=linux go build -ldflags '-X TaoKan/cmd.version=${VERSION}' -o ${BINARY_PATH}/${BINARY_NAME}-linux  main.go

build: build-mac build-linux
	@ echo "Completed"

build-static: tidy
	GOARCH=amd64 GOOS=linux go build -tags netgo -ldflags '-extldflags "-static" -X TaoKan/cmd.version=${VERSION}' -o ${BINARY_PATH}/${BINARY_NAME}-static-linux main.go

build-image:
	docker ${DOCKER_BUILD_CMD} --build-arg version=${VERSION} -t ${IMAGE_NAME}:${VERSION} .

deploy-image: build-image
	docker push ${IMAGE_NAME}:${VERSION}

run-client: build
	./${BINARY_PATH}/${BINARY_NAME}-${CURRENT_OS} client -r localhost --retry 0 --daemon --debug

run-server: build
	./${BINARY_PATH}/${BINARY_NAME}-${CURRENT_OS} server --storage-class-rwx efs-sc --debug

dev-client:
	reflex --start-service  -r '\.go$$' make run-client

dev-server:
	reflex --start-service  -r '\.go$$' make run-server

clean:
	go clean
	rm ${BINARY_PATH}/${BINARY_NAME}-darwin
	rm ${BINARY_PATH}/${BINARY_NAME}-linux

test:
	go test ./...

test_coverage:
	go test ./... -coverprofile=coverage.out


tidy:
	go mod tidy

dep:
	go mod download

vet:
	go vet

lint:
	golangci-lint run --enable-all
