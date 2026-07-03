BUILD_DIR ?= ./bin
REST_BINARY ?= $(BUILD_DIR)/rest
GO ?= go
VERSION ?=
GOIMPORTS ?= $(GO) run golang.org/x/tools/cmd/goimports
GOVULNCHECK ?= $(GO) run golang.org/x/vuln/cmd/govulncheck@v1.1.4

.PHONY: build-rest test race vuln generated-examples golden docker-smoke runtime-e2e format format-check check ci-check benchmark clean setup hooks changelog release publish-release

build-rest:
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(REST_BINARY) ./cmd/rest

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

vuln:
	$(GOVULNCHECK) ./...

generated-examples:
	$(GO) test ./internal/cli -run 'TestE2E'

golden:
	$(GO) test ./internal/cli -run 'TestE2EGoldenSnapshots' -count=1

docker-smoke:
	REST_DOCKER_SMOKE=1 $(GO) test ./internal/cli -run 'TestE2EDockerBuildSmoke' -count=1

runtime-e2e:
	REST_RUNTIME_E2E=1 $(GO) test ./internal/cli -run 'TestRuntimeE2E' -count=1 -timeout 6m

format:
	$(GOIMPORTS) -w .

format-check:
	@files="$$($(GOIMPORTS) -l .)"; \
	status="$$?"; \
	if [ "$$status" -ne 0 ]; then exit "$$status"; fi; \
	test -z "$$files" || (echo "$$files" && exit 1)

check:
	$(MAKE) format-check
	$(GO) test ./...
	$(GO) build -trimpath -o $(REST_BINARY) ./cmd/rest

ci-check: check race vuln generated-examples golden

setup: hooks
	@echo "Development tools are configured."

hooks:
	git config core.hooksPath .githooks
	@echo "Git hooks enabled from .githooks/"

changelog:
	@command -v git-cliff >/dev/null 2>&1 || { echo "git-cliff is required"; exit 1; }
	git-cliff --unreleased

release:
	./scripts/release.sh "$(VERSION)"

publish-release:
	./scripts/publish-release.sh "$(VERSION)"

benchmark:
	$(GO) test -run '^$$' -bench '^BenchmarkGenerate$$' -benchmem ./internal/generator

clean:
	rm -rf $(BUILD_DIR)
