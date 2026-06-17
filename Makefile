BUILD_DIR ?= ./bin
REST_BINARY ?= $(BUILD_DIR)/rest
GO ?= go

.PHONY: build-rest test clean

build-rest:
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(REST_BINARY) ./cmd/rest

test:
	$(GO) test ./cmd/rest ./internal/appgen ./internal/config ./internal/generator ./internal/sqlcconfig ./internal/config/templates

clean:
	rm -rf $(BUILD_DIR)
