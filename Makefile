# One command runs every test in the repository:
#
#   make test
#
# Each part is also available on its own. Go code needs Go 1.24 or newer and nothing else: the server's
# game package is provided in ./sdk, so no modules are downloaded. The client's tests run once client/
# has a Makefile with a test target.

GO ?= go

.PHONY: test fmt-check vet test-sdk test-go test-client check-deps

test: fmt-check vet test-sdk test-go test-client

# Fails if any Go file outside sdk/ (the server team's code) is not gofmt-formatted
fmt-check:
	@unformatted="$$(find . -name '*.go' -not -path './sdk/*' -print0 | xargs -0 -r gofmt -l)"; \
	if [ -n "$$unformatted" ]; then echo "gofmt needed on:"; echo "$$unformatted"; exit 1; fi

vet:
	@if [ -n "$$($(GO) list ./... 2>/dev/null)" ]; then $(GO) vet ./...; fi

# The SDK's own tests (sdk/ is a separate module, so ./... in this module doesn't include it)
test-sdk:
	cd sdk && $(GO) test -count=1 ./...

# The ruleset, including its sdk/game/gametest compliance tests, and any other Go code in this module
test-go:
	@if [ -n "$$($(GO) list ./... 2>/dev/null)" ]; then \
		$(GO) test -count=1 ./...; \
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
