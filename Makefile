.PHONY: run build test clean web protocol-generate protocol-check

APP_NAME := agent-native-im
BUILD_DIR := bin

run:
	go run ./cmd/server

build: web
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/server

web:
	cd web && npm install && npm run build

test:
	go test ./...

protocol-generate:
	node scripts/generate-route-contract.mjs

protocol-check:
	node scripts/generate-route-contract.mjs --check
	node scripts/check-protocol-contract.mjs

clean:
	rm -rf $(BUILD_DIR) data/*.db web/dist
