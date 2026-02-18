.PHONY: build test run tidy

build:
	go build ./...

test:
	go test ./... -v

run:
	go run cmd/api/main.go

tidy:
	go mod tidy

lint:
	golangci-lint run ./...
