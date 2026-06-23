.PHONY: dev test build

build:
	go build ./...

test:
	go test ./...

dev:
	@trap 'kill %1 %2' INT; \
	go run ./cmd/trader & \
	cd web && npm run dev & \
	wait
