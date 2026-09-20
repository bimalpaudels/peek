# peek - Universal Fast In-File Scratchpad
BINARY_NAME := peek
CMD_DIR     := ./cmd/peek

# Installation directory (defaults to ~/.local/bin, override via: make install INSTALL_DIR=/usr/local/bin)
INSTALL_DIR ?= $(HOME)/.local/bin

VERSION     ?= 0.1.0

# Go build flags: strip DWARF and symbol table for minimal binary footprint, inject version
LDFLAGS := -s -w -X main.Version=$(VERSION)

.PHONY: all build install uninstall remove update clean test help

all: build

## Build clean minimal binary
build:
	@echo "==> Building $(BINARY_NAME)..."
	go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) $(CMD_DIR)
	@echo "✓ Built $(BINARY_NAME) ($$(ls -lh $(BINARY_NAME) | awk '{print $$5}'))"

## Install binary to $(INSTALL_DIR)
install: build
	@echo "==> Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	@install -m 755 $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "✓ Successfully installed $(BINARY_NAME) to $(INSTALL_DIR)/$(BINARY_NAME)"
	@./$(BINARY_NAME) --config >/dev/null 2>&1 || true
	@if ! echo "$$PATH" | grep -q "$(INSTALL_DIR)"; then \
		echo "⚠ Warning: $(INSTALL_DIR) does not appear to be in your \$$PATH"; \
	fi

## Remove binary from $(INSTALL_DIR)
uninstall:
	@echo "==> Removing $(BINARY_NAME) from $(INSTALL_DIR)..."
	@rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "✓ Removed $(INSTALL_DIR)/$(BINARY_NAME)"

## Alias for uninstall
remove: uninstall

## Rebuild and reinstall binary
update: clean build install
	@echo "✓ Updated $(BINARY_NAME) in $(INSTALL_DIR)/$(BINARY_NAME)"

## Remove local compiled binary
clean:
	@echo "==> Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@echo "✓ Clean complete"

## Run all unit tests
test:
	@echo "==> Running tests..."
	go test -v ./...

## Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk '/^[a-zA-Z\-_0-9]+:/ { \
		helpMessage = match(lastLine, /^## (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpDesc = substr(lastLine, RSTART + 3, RLENGTH); \
			printf "  %-12s %s\n", helpCommand, helpDesc; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)
