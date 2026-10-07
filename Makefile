.PHONY: build test screenshot

build:
	go build -o super-shell .

test:
	go vet ./... && go test -race ./...

# Regenerates docs/screenshot.png and docs/demo.gif (see scripts/screenshot/run.sh).
screenshot:
	scripts/screenshot/run.sh
