# kblog Makefile

BINARY_NAME=kblog
BUILD_DIR=bin
INSTALL_PATH=/usr/local/bin
PLUGIN_SRC=plugin/plugins.yaml

# Detect k9s config directory (macOS Library path takes precedence over XDG)
K9S_CONFIG_DIR := $(shell \
  if [ -d "$$HOME/Library/Application Support/k9s" ]; then \
    echo "$$HOME/Library/Application Support/k9s"; \
  elif [ -n "$$XDG_CONFIG_HOME" ] && [ -d "$$XDG_CONFIG_HOME/k9s" ]; then \
    echo "$$XDG_CONFIG_HOME/k9s"; \
  else \
    echo "$$HOME/.config/k9s"; \
  fi)

.PHONY: all build test clean install uninstall install-plugin uninstall-plugin help

all: build

help:
	@echo "kblog Open Source Build Tool"
	@echo "Available commands:"
	@echo "  make build            - Compile kblog to bin/kblog"
	@echo "  make install          - Install binary + k9s plugin"
	@echo "  make uninstall        - Remove binary and k9s plugin"
	@echo "  make install-plugin   - Install k9s plugin only"
	@echo "  make uninstall-plugin - Remove k9s plugin only"
	@echo "  make clean            - Remove build artifacts"
	@echo "  make test             - Run Go tests"

build:
	@echo "🔨 Compiling kblog..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@echo "✅ Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

test:
	@echo "🧪 Running tests..."
	go test -v ./...

clean:
	@echo "🧹 Cleaning build directories..."
	rm -rf $(BUILD_DIR)
	@echo "✅ Clean finished."

install: build install-plugin
	@echo "🚀 Installing binary to $(INSTALL_PATH)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "🎉 kblog installed. Run 'kblog --help' to verify."

uninstall: uninstall-plugin
	@echo "🗑️ Removing binary from $(INSTALL_PATH)..."
	@rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "✅ kblog uninstalled."

install-plugin:
	@echo "🔌 Installing k9s plugin to $(K9S_CONFIG_DIR)..."
	@mkdir -p "$(K9S_CONFIG_DIR)"
	@if [ -f "$(K9S_CONFIG_DIR)/plugins.yaml" ]; then \
	  if grep -q "kblog-pod" "$(K9S_CONFIG_DIR)/plugins.yaml"; then \
	    echo "   Plugin already present — skipping."; \
	  else \
	    grep -v '^plugins:' $(PLUGIN_SRC) >> "$(K9S_CONFIG_DIR)/plugins.yaml" && \
	    echo "   Merged into existing plugins.yaml."; \
	  fi \
	else \
	  cp $(PLUGIN_SRC) "$(K9S_CONFIG_DIR)/plugins.yaml" && \
	  echo "   Created plugins.yaml."; \
	fi
	@echo "✅ k9s plugin installed. Press Shift-L on any pod or deployment."

uninstall-plugin:
	@echo "🗑️ Removing kblog entries from k9s plugins..."
	@if [ -f "$(K9S_CONFIG_DIR)/plugins.yaml" ]; then \
	  python3 -c " \
import yaml, sys; \
data = yaml.safe_load(open('$(K9S_CONFIG_DIR)/plugins.yaml')) or {}; \
plugins = data.get('plugins', {}); \
[plugins.pop(k, None) for k in ['kblog-pod', 'kblog-deployment']]; \
yaml.dump(data, open('$(K9S_CONFIG_DIR)/plugins.yaml', 'w'), default_flow_style=False) \
  " && echo "   Removed kblog entries." || echo "   Could not parse plugins.yaml — remove manually."; \
	fi
