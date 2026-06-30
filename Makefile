BUILD_DIR ?= ./bin
REST_BINARY ?= $(BUILD_DIR)/rest
GO ?= go
VERSION ?=

.PHONY: build-rest test generated-examples runtime-e2e check benchmark clean setup hooks changelog release publish-release

build-rest:
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(REST_BINARY) ./cmd/rest

test:
	$(GO) test ./...

generated-examples:
	$(GO) test ./cmd/rest -run 'TestE2E'

runtime-e2e:
	REST_RUNTIME_E2E=1 $(GO) test ./cmd/rest -run 'TestRuntimeE2E' -count=1

check:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	$(GO) test ./...
	$(GO) build -trimpath -o $(REST_BINARY) ./cmd/rest

setup: hooks
	@command -v git-cliff >/dev/null 2>&1 || { \
		echo "git-cliff is missing. Install it with: brew install git-cliff"; \
		exit 1; \
	}
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
