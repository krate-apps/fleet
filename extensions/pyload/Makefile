.PHONY: build test pack docs-config
build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -o bin/pyload-worker ./cmd/pyload-worker

test:
	go test ./...

pack:
	bash scripts/pack.sh dist

docs-config:
	python3 scripts/gen-readme-config.py .
