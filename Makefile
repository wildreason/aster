# Aster Makefile
# Simple, deterministic build and release management

.PHONY: build test clean install release help

VERBOSE ?= 0
BINARY := aster

# Default target
all: build

help:
	@echo "Aster - Terminal File Reader"
	@echo ""
	@echo "Development:"
	@echo "  make build     - Build with version injection"
	@echo "  make test      - Run tests"
	@echo "  make install   - Install aster to ~/.local/bin"
	@echo "  make clean     - Clean build artifacts"
	@echo ""
	@echo "Release Management:"
	@echo "  make release   - RELEASE-NOTES-driven release workflow"
	@echo ""
	@echo "Add VERBOSE=1 for detailed logs"

# Build with version injection
build:
	@DIRTY_HASH=$$(git diff --quiet && git diff --cached --quiet && echo "" || (git diff --cached; git diff; git ls-files --others --exclude-standard | sed 's/^/?? /') 2>&1 | shasum | cut -c1-8); \
	if git diff --quiet && git diff --cached --quiet; then \
		true; \
	else \
		if [ "$(VERBOSE)" = "1" ]; then \
			echo "warn dirty dirty-$$DIRTY_HASH"; \
			git status --short; \
		else \
			echo "warn dirty dirty-$$DIRTY_HASH"; \
		fi; \
	fi
	@VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	BRANCH=$$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown"); \
	DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ"); \
	if [ "$(VERBOSE)" = "1" ]; then \
		echo "build $(BINARY)"; \
		echo "version: $$VERSION"; \
		echo "commit:  $$COMMIT"; \
		echo "branch:  $$BRANCH"; \
		echo "time:    $$DATE"; \
	else \
		echo "build $(BINARY) @ $$VERSION"; \
	fi; \
	go build -ldflags "-X main.Version=$$VERSION -X main.Commit=$$COMMIT -X main.Date=$$DATE" -o $(BINARY) .; \
	if [ "$(VERBOSE)" = "1" ]; then \
		echo "done"; \
	fi

# Run tests
test:
	@if [ "$(VERBOSE)" = "1" ]; then \
		echo "test go test ./..."; \
		go test ./...; \
	else \
		echo "test go test ./..."; \
		go test ./... >/dev/null; \
	fi

# Install locally as aster-local (avoids conflicting with Homebrew aster)
install: build
	@killall -9 $(BINARY) 2>/dev/null || true
	@rm -f ~/.local/bin/aster-local 2>/dev/null || true
	@mkdir -p ~/.local/bin
	@cp $(BINARY) ~/.local/bin/aster-local
	@echo "install ~/.local/bin/aster-local"

# Clean build artifacts
clean:
	@rm -f $(BINARY)
	@go clean
	@echo "clean"

# RELEASE-NOTES-driven release workflow
release:
	@BRANCH=$$(git rev-parse --abbrev-ref HEAD 2>/dev/null); \
	if [ "$$BRANCH" != "main" ] && [ "$$BRANCH" != "master" ]; then \
		echo "error: must be on main branch (currently on $$BRANCH)"; \
		exit 1; \
	fi
	@if ! git diff --quiet || ! git diff --cached --quiet; then \
		echo "error: uncommitted changes detected"; \
		git status --short; \
		exit 1; \
	fi
	@if ! go test ./... >/dev/null 2>&1; then \
		echo "error: tests failed"; \
		exit 1; \
	fi
	@VERSION=$$(grep -m1 '^## ' RELEASE-NOTES.md | sed -E 's/## (v[0-9]+\.[0-9]+\.[0-9]+).*/\1/'); \
	if [ -z "$$VERSION" ]; then \
		echo "error: no version found in RELEASE-NOTES.md"; \
		exit 1; \
	fi; \
	if git rev-parse "refs/tags/$$VERSION" >/dev/null 2>&1; then \
		echo "error: tag $$VERSION already exists"; \
		exit 1; \
	fi; \
	git tag -a "$$VERSION" -m "Release $$VERSION"; \
	git push origin main 2>&1 | grep -v "^To " || true; \
	git push origin "$$VERSION" 2>&1 | grep -v "^To " || true; \
	if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then \
		NOTES_FILE=$$(mktemp); \
		awk -v ver="$$VERSION" 'BEGIN{f=0}/^## v[0-9]+\./{if(f)exit;if($$0~"^## "ver){f=1;next}}f&&/^---$$/{exit}f{print}' RELEASE-NOTES.md > "$$NOTES_FILE" || true; \
		if [ -s "$$NOTES_FILE" ]; then \
			gh release create "$$VERSION" --title "$$VERSION" --notes-file "$$NOTES_FILE" --latest >/dev/null 2>&1 || true; \
		else \
			gh release create "$$VERSION" --title "$$VERSION" --notes "Release $$VERSION" --latest >/dev/null 2>&1 || true; \
		fi; \
		rm -f "$$NOTES_FILE"; \
	fi; \
	echo "release $$VERSION complete"
