GOLANGCI := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
ACTIONLINT := go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
GORELEASER := go run github.com/goreleaser/goreleaser/v2@v2.18.3

.PHONY: build test lint fmt check hooks actionlint pins release-check

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

actionlint: ## lint the GitHub workflows
	$(ACTIONLINT)

# Same rule as the "pinned actions" step in the shared ci-go workflow:
# third-party actions name a full commit SHA. Local actions and the shared
# RhythmShahriar/.github workflows are exempt until the signing identity
# cut-over.
pins: ## every third-party action is pinned to a commit SHA
	@bad=$$(grep -nE '^[[:space:]-]*uses:' .github/workflows/*.yml \
		| grep -vE 'uses:[[:space:]]*"?(\./|RhythmShahriar/\.github/)' \
		| grep -vE '@[0-9a-f]{40}"?([[:space:]]|$$)' || true); \
	if [ -n "$$bad" ]; then echo "These uses: lines don't pin a 40-hex commit SHA:"; echo "$$bad"; exit 1; fi

release-check: ## validate .goreleaser.yaml
	$(GORELEASER) check
