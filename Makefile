.PHONY: build test vet
build:
	go build -o bin/meshgrid ./cmd/meshgrid
test:
	go test ./...
vet:
	go vet ./...

