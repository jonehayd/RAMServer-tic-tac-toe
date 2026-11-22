# One command runs every test in the repository:
#
#   make test
#
# Each part is also available on its own. The client's targets run once client/ has a Makefile.

GO ?= go

.PHONY: test test-sdk test-go test-client check-deps

test: test-sdk test-go test-client

# The compliance tests' own tests
test-sdk:
	cd sdk && $(GO) test ./...

# The ruleset and any other Go code in this module
test-go:
	@if [ -n "$$($(GO) list ./... 2>/dev/null)" ]; then \
		$(GO) test ./...; \
	else \
		echo "test-go: no Go packages yet"; \
	fi

# The C client
test-client:
	@if [ -f client/Makefile ]; then \
		$(MAKE) -C client test; \
	else \
		echo "test-client: no client/Makefile yet"; \
	fi

# Install the system packages (Debian/Ubuntu); the Docker container already has them
check-deps:
	bash scripts/install-deps.sh
