GOLANGCI := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
ACTIONLINT := go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
GORELEASER := go run github.com/goreleaser/goreleaser/v2@v2.18.3
GOVULNCHECK := go run golang.org/x/vuln/cmd/govulncheck@v1.8.0

.PHONY: build test cross lint fmt check hooks actionlint pins release-check vuln test-installers trusted-root dist-check

build: ## build bin/rhythm
	go build -o bin/rhythm ./cmd/rhythm

test: ## run tests
	go test -race ./...

# Vets every package and compiles the router's tests for Windows and
# macOS, so a test that refers to a unix-only helper fails here and not
# only on the CI runners.
cross: ## vet for Windows and macOS and compile the router's tests there
	GOOS=windows go vet ./...
	GOOS=windows go test -c -o /dev/null ./cmd/rhythm
	GOOS=darwin go vet ./...

lint:
	$(GOLANGCI) run ./...

fmt:
	gofmt -w .

check: lint test cross ## everything CI runs

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

vuln: ## report known vulnerabilities in code rhythm calls
	$(GOVULNCHECK) ./...

# The installer tests live in scripts/test/. CI also runs them under dash
# and on Windows with PowerShell 7 and Windows PowerShell 5.1.
test-installers: ## run the install script tests
	@if [ -f scripts/test/run.sh ]; then sh scripts/test/run.sh; else echo "scripts/test/run.sh not found; nothing to run"; fi
	@if [ -f scripts/test/run.ps1 ] && command -v pwsh >/dev/null 2>&1; then pwsh -NoProfile -File scripts/test/run.ps1; fi

# Refreshes the Sigstore trusted root that kit/sigverify embeds, through
# TUF from Sigstore's CDN. Run it before a release and commit the result.
trusted-root: ## refresh kit/sigverify/trusted_root.json
	go generate ./kit/sigverify

# A local snapshot release (needs syft on PATH) checked like CI's packages
# job, minus the rpm listing and winget schemas when those tools are absent.
dist-check: ## build a snapshot release and check SBOMs, packages and manifests
	sh .github/scripts/check-dist_test.sh
	$(GORELEASER) release --snapshot --clean --skip=publish,sign
	sh .github/scripts/check-dist.sh dist rhythm
