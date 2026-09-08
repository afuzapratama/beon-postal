.PHONY: run build test check download tidy

## run: start the server (downloads data automatically if missing)
run:
	go run .

## build: compile a production binary
build:
	go build -ldflags="-s -w" -o postal-api .

## test: run the automated test suite
test:
	go test ./...

## check: run formatting, tests, vet, and dependency verification
check:
	@test -z "$$(gofmt -l *.go)" || (echo "Run gofmt on:"; gofmt -l *.go; exit 1)
	go test ./...
	go vet ./...
	go mod verify

## download: pre-download and cache the Japan Post CSV (skips download on next run)
download:
	@mkdir -p data
	@echo "Downloading KEN_ALL.ZIP from Japan Post..."
	@curl -fL "https://www.post.japanpost.jp/service/search/zipcode/download/kogaki/zip/ken_all.zip" -o /tmp/ken_all.zip
	@unzip -o /tmp/ken_all.zip -d data/
	@echo "Done. Run 'make run' to start the server."

## tidy: sync go.mod / go.sum
tidy:
	go mod tidy

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/## //'
