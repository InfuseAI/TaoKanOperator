BINARY_PATH=bin
BINARY_NAME=taokan
CURRENT_OS=$(shell uname | tr A-Z a-z)

build: tidy
	GOOS=darwin go build -o ${BINARY_PATH}/${BINARY_NAME}-darwin main.go
	GOARCH=amd64 GOOS=linux go build -o ${BINARY_PATH}/${BINARY_NAME}-linux main.go

run-client: build
	./${BINARY_PATH}/${BINARY_NAME}-${CURRENT_OS} client

run-server: build
	./${BINARY_PATH}/${BINARY_NAME}-${CURRENT_OS} server

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
