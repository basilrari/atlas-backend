# Local dev only (no Docker). Requires Postgres and Redis available (e.g. Supabase + local Redis).
.PHONY: run build test deps

run:
	go run ./api

build:
	go build -o api ./api

test:
	go test ./...

deps:
	go mod download
	go mod tidy
