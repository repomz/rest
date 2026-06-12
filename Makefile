-include .env

APP_NAME ?= app
BUILD_DIR ?= ./bin
DB_SCRIPT ?= init_db.sh
DB_NAME ?= app_db
DB_USER ?= app_user
DB_PASS ?= app_password
DB_DRIVER ?= postgres
MIGRATIONS_DIR ?= ./internal/sql/migrations
DB_DSN ?= postgres://$(DB_USER):$(DB_PASS)@localhost:5432/$(DB_NAME)?sslmode=disable
HTTP_ADDR ?= :8080
DEBUG_ERRORS ?= 1
GOCACHE ?= $(CURDIR)/.cache/go-build

export

.PHONY: build run test clean db migrate-status migrate-up migrate-down migrate-create

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd

run:
	@mkdir -p $(BUILD_DIR) && \
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd && \
	HTTP_ADDR=$(HTTP_ADDR) \
	DB_DSN=$(DB_DSN) \
	DEBUG_ERRORS=$(DEBUG_ERRORS) \
	$(BUILD_DIR)/$(APP_NAME)

test:
	go test -race -v ./...

clean:
	rm -rf $(BUILD_DIR)

db:
	@test -f $(DB_SCRIPT) || { echo "Ошибка: $(DB_SCRIPT) отсутствует"; exit 1; }
	@chmod +x $(DB_SCRIPT)
	@./$(DB_SCRIPT)

migrate-status:
	goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) $(DB_DSN) status

migrate-up:
	goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) $(DB_DSN) up

migrate-down:
	goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) $(DB_DSN) down

migrate-create:
	@read -p "Название миграции: " name; \
	goose -dir $(MIGRATIONS_DIR) create $$name sql
