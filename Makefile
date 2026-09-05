BINARY  ?= vortex
PLUGINS ?= plugins

# Loading plugins at runtime needs a dynamically linked binary, which means cgo.
export CGO_ENABLED = 1

.PHONY: build run tidy fmt vet test plugin plugins clean

build:
	go build -o $(BINARY) ./cmd/vortex

run:
	go run ./cmd/vortex -config config.yml

# Builds one plugin into the plugin directory:
#   make plugin DIR=./examples/plugins/greeter
# NAME defaults to the last element of DIR.
plugin:
	@test -n "$(DIR)" || (echo "usage: make plugin DIR=./path/to/plugin [NAME=name]"; exit 1)
	go build -buildmode=plugin -o $(PLUGINS)/$(if $(NAME),$(NAME),$(notdir $(DIR))).so $(DIR)

# Builds every example plugin into the plugin directory.
plugins:
	@for dir in examples/plugins/*/; do \
		name=$$(basename $$dir); \
		echo "building $$name"; \
		go build -buildmode=plugin -o $(PLUGINS)/$$name.so ./$$dir || exit 1; \
	done

tidy:
	go mod tidy

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -f $(BINARY)
