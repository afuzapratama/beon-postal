.PHONY: run build download tidy

## run: start the server (downloads data automatically if missing)
run:
	go run .

## build: compile a production binary
build:
	go build -ldflags="-s -w" -o postal-api .

## download: pre-download and cache the Japan Post CSV (skips download on next run)
download:
	@mkdir -p data
	@echo "Downloading KEN_ALL.ZIP from Japan Post..."
	@curl -fL "https://www.post.japanpost.jp/zipcode/dl/kogaki/zip/ken_all.zip" -o /tmp/ken_all.zip
	@unzip -o /tmp/ken_all.zip -d data/
	@echo "Done. Run 'make run' to start the server."

## tidy: sync go.mod / go.sum
tidy:
	go mod tidy

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/## //'
