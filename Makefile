BIN_NAME := nolejje

.PHONY: build run test vet fmt release docker

build:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o $(BIN_NAME) .

release:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o $(BIN_NAME)-linux-amd64 .

run:
	go run .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

docker:
	docker compose up --build
