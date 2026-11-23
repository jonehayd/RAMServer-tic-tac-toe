# The tic-tac-toe client for RAMServer.
#
#   make            build build/<os>/ttt (and a ./ttt link to it)
#   make test       the unit tests, then the integration test (no server, network or display needed)
#   make unit       only the unit tests
#   make integration  only the integration test: real clients play complete matches against
#                   tests/fakeserver, a stand-in server running the real ruleset, over local TCP
#   make memcheck   run the unit tests under valgrind
#   make clean
#
# Needs: a C11 compiler, protoc with protoc-c, protobuf-c, libwebsockets (with OpenSSL), and cmocka for
# the tests, and Go for the integration test's stand-in server. scripts/install-deps.sh at the repository root installs them all; the Docker image has them.

CC      ?= cc
PKGS    := libprotobuf-c libwebsockets
CFLAGS  ?= -O2 -g
CFLAGS  += -std=gnu11 -Wall -Wextra -Wshadow -Wformat=2 -Wno-format-nonliteral -Wno-format-truncation
CPPFLAGS += -Isrc -I$(GEN) -MMD -MP
LDLIBS  := $(shell pkg-config --libs $(PKGS)) -lpthread
TEST_LDLIBS := $(shell pkg-config --libs libprotobuf-c cmocka)
CFLAGS  += $(shell pkg-config --cflags $(PKGS) cmocka)

# One build directory per OS, so building on WSL and in the Docker container (which share this
# directory) never mixes binaries linked against different library versions
OS_TAG  := $(shell . /etc/os-release 2>/dev/null && echo "$$ID$$VERSION_ID" || uname -s)
BUILD   ?= build/$(OS_TAG)
GEN     := $(BUILD)/gen
PROTO_ROOT := ../proto
PROTO   := $(PROTO_ROOT)/ramserver/v1/ramserver.proto
PB_C    := $(GEN)/ramserver/v1/ramserver.pb-c.c
PB_H    := $(GEN)/ramserver/v1/ramserver.pb-c.h
PB_O    := $(BUILD)/ramserver.pb-c.o

# Protocol, framing and game state: no networking or terminal code, so the tests use only these
CORE    := frame view proto client addr bot
# Networking, the event loop and the user interfaces
APP     := transport app tui plain main
TESTS   := test_frame test_view test_proto test_client test_addr test_bot

CORE_O  := $(CORE:%=$(BUILD)/%.o) $(PB_O)
APP_O   := $(APP:%=$(BUILD)/%.o)
TEST_BIN := $(TESTS:%=$(BUILD)/%)

.PHONY: all ttt test unit integration memcheck clean
.SECONDARY:
all: ttt

ttt: $(BUILD)/ttt
	ln -sf $(BUILD)/ttt ttt

$(BUILD)/ttt: $(CORE_O) $(APP_O)
	$(CC) $(CFLAGS) -o $@ $^ $(LDLIBS)

# Generated protobuf code (not committed; see the repository's .gitignore)
$(PB_C) $(PB_H): $(PROTO)
	@mkdir -p $(GEN)
	protoc -I$(PROTO_ROOT) --c_out=$(GEN) $(PROTO)

$(PB_O): $(PB_C)
	$(CC) $(CPPFLAGS) $(filter-out -Wshadow,$(CFLAGS)) -w -c -o $@ $<

$(BUILD)/%.o: src/%.c $(PB_H)
	@mkdir -p $(BUILD)
	$(CC) $(CPPFLAGS) $(CFLAGS) -c -o $@ $<

$(BUILD)/test_%.o: tests/test_%.c $(PB_H)
	@mkdir -p $(BUILD)
	$(CC) $(CPPFLAGS) -Itests $(CFLAGS) -c -o $@ $<

$(BUILD)/test_helpers.o: tests/helpers.c $(PB_H)
	@mkdir -p $(BUILD)
	$(CC) $(CPPFLAGS) -Itests $(CFLAGS) -c -o $@ $<

$(BUILD)/test_%: $(BUILD)/test_%.o $(BUILD)/test_helpers.o $(CORE_O)
	$(CC) $(CFLAGS) -o $@ $^ $(TEST_LDLIBS)

test: unit integration
	@echo "All client tests passed."

unit: $(TEST_BIN)
	@set -e; for t in $(TEST_BIN); do echo "== $$t"; ./$$t; done

integration: $(BUILD)/ttt $(BUILD)/fakeserver
	tests/integration.sh $(BUILD)

# The integration test's stand-in server, in this repository's Go module (it runs the real ruleset)
GO ?= go
$(BUILD)/fakeserver: $(wildcard tests/fakeserver/*.go) $(wildcard ../rulesets/tictactoe/*.go)
	@mkdir -p $(BUILD)
	cd .. && $(GO) build -o client/$(BUILD)/fakeserver ./client/tests/fakeserver

memcheck: $(TEST_BIN)
	@set -e; for t in $(TEST_BIN); do echo "== valgrind $$t"; \
		valgrind -q --error-exitcode=1 --leak-check=full --errors-for-leak-kinds=definite ./$$t; done

clean:
	rm -rf build ttt

-include $(wildcard $(BUILD)/*.d)
