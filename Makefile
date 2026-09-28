# make test          fast suite (Go unit + integration with -race, client unit). Run before every commit.
# make test-browser  real browser end to end tests (needs Chromium/Chrome, set CHROME_PATH if not found)
# make check         everything CI runs

.PHONY: test test-go test-client test-browser check generate bench icons

test: test-go test-client

test-go:
	go vet ./...
	go test -race ./...

test-client:
	npm test

test-browser:
	npm run test:browser

check: generate test test-browser
	@git diff --exit-code client/src/protocol.gen.ts || (echo "protocol.gen.ts is stale: run make generate and commit it"; exit 1)

generate:
	go generate ./internal/protocol

# Redraws the ability icons in client/static/assets/abilities from cmd/icongen
icons:
	go run ./cmd/icongen

bench:
	go test -run xxx -bench . -benchmem ./internal/game/
