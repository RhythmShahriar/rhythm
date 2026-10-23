GOLANGCI := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0

.PHONY: build test lint fmt check hooks

build: ## build bin/rhythm
	go build -o bin/rhythm ./cmd/rhythm

test: ## run tests
	go test -race ./...

lint:
	$(GOLANGCI) run ./...

fmt:
	gofmt -w .

check: lint test ## everything CI runs

hooks: ## enable the Conventional Commits hook
	git config core.hooksPath .githooks
