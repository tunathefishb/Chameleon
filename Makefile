PATH := $(shell go env GOPATH)/bin:$(PATH)
export PATH

.PHONY: build run test lint clean

run:
	go run ./cmd/chameleon

build:
	@mkdir -p build
	go build -o build/chameleon ./cmd/chameleon

test:
	go test -v ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf build/
