.PHONY: build install test lint clean

BINARY := bin/cicd
VERSION ?= dev

build:
	go build -ldflags "-X github.com/cicd-automate/cicd-automate/internal/cli.Version=$(VERSION)" -o $(BINARY) ./cmd/cicd

install:
	go install -ldflags "-X github.com/cicd-automate/cicd-automate/internal/cli.Version=$(VERSION)" ./cmd/cicd

test:
	go test -race -v ./...

vet:
	go vet ./...

clean:
	rm -rf bin dist coverage.txt
