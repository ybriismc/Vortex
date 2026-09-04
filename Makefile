BINARY ?= vortex

.PHONY: build run tidy fmt vet clean

build:
	go build -o $(BINARY) ./cmd/vortex

run:
	go run ./cmd/vortex -config config.yml

tidy:
	go mod tidy

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
