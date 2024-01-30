install:
	go get ./...
	
test:
	go test

build:
	go build .

all: install test build

.PHONY: install test build all
