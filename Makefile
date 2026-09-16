BINARY  := x-cyber-lrc-hub
PKG     := ./cmd/server
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the binary (static, no CGO)
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

.PHONY: test
test: ## Run the offline test suite with the race detector
	go test -short -race -count=1 ./...

.PHONY: test-all
test-all: ## Run every test, including the live provider smoke test (needs network)
	go test -race -count=1 ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: fmt
fmt: ## Format the source
	@gofmt -w $$(git ls-files '*.go')

.PHONY: fmt-check
fmt-check: ## Fail if anything is not gofmt-clean
	@files="$$(git ls-files '*.go')"; \
	if [ -z "$$files" ]; then echo "no tracked Go files found"; exit 1; fi; \
	out="$$(gofmt -l $$files)"; \
	if [ -n "$$out" ]; then echo "not gofmt-clean:"; echo "$$out"; exit 1; fi; \
	echo "gofmt: clean"

.PHONY: lint
lint: ## Run golangci-lint (must be installed; see .golangci.yml)
	golangci-lint run

.PHONY: run
run: build ## Build and run locally
	./$(BINARY)

.PHONY: tidy
tidy: ## Tidy the module graph
	go mod tidy

.PHONY: docker
docker: ## Build the container image
	docker build --build-arg VERSION=$(VERSION) -t $(BINARY):$(VERSION) .

.PHONY: clean
clean: ## Remove build artifacts and the local cache
	rm -rf $(BINARY) .evalcache data
