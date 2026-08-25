.PHONY: test race bench vet tidy e2e up down

tidy:
	go mod tidy

vet:
	go vet ./...

test:
	go test ./...

race:
	go test -race ./...

bench:
	go test -bench=. -benchmem ./internal/window ./internal/engine

up:
	docker compose up --build -d

down:
	docker compose down
