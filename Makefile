.PHONY: all test build clean

REGISTRY_BASE ?= quay.io/luzuccar
IMAGE_NAME ?= golang-oci-mirror
IMAGE_VERSION ?= v0.0.1

all: clean test build

build: 
	mkdir -p build
	go build -o build ./...

build-dev:
	mkdir -p build
	GOOS=linux go build -ldflags="-s -w" -o build -tags real./...

verify:
	golangci-lint run -c .golangci.yaml --deadline=30m

test:
	mkdir -p tests/results
	go test -v -coverprofile=tests/results/cover.out ./...

cover:
	go tool cover -html=tests/results/cover.out -o tests/results/cover.html

clean:
	rm -rf build/*
	go clean ./...
