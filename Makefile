.PHONY: deps\:test-ci
deps\:test-ci:
	go install gotest.tools/gotestsum@latest

.PHONY: clean
clean:
	go clean -testcache

.PHONY: test
test:
	@$(MAKE) clean
# Workspace submodule tests must be run from their root directory
	(cd ./test && go test ./...)

.PHONY: test\:memory
test\:memory:
# Environment variable changes do not invalidate the go test cache, so it is important
# for us to clean between each run.
	@$(MAKE) clean
	(cd ./memory && go test ./...)
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,memory" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory,txn-commit" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory,txn-commit,txn-context" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,memory,txn-commit" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory,txn-discard" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory,txn-discard,txn-context" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,memory,txn-discard" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory,txn-multi" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,memory,txn-multi" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="chunk,memory" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,chunk,memory" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory,chunk,txn-commit" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,chunk,memory,txn-commit" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory,chunk,txn-discard" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,chunk,memory,txn-discard" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory,chunk,txn-multi" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,chunk,memory,txn-multi" sh -c 'cd ./test && go test ./...'

.PHONY: test\:badger
test\:badger:
# Environment variable changes do not invalidate the go test cache, so it is important
# for us to clean between each run.
	@$(MAKE) clean
	(cd ./badger && go test ./...)
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,badger" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger,txn-commit" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger,txn-commit,txn-context" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,badger,txn-commit" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger,txn-discard" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger,txn-discard,txn-context" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,badger,txn-discard" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger,txn-multi" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,badger,txn-multi" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="chunk,badger" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,chunk,badger" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger,chunk,txn-commit" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,chunk,badger,txn-commit" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger,chunk,txn-discard" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,chunk,badger,txn-discard" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger,chunk,txn-multi" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,chunk,badger,txn-multi" sh -c 'cd ./test && go test ./...'

.PHONY: test\:level
test\:level:
# Environment variable changes do not invalidate the go test cache, so it is important
# for us to clean between each run.
	@$(MAKE) clean
	(cd ./leveldb && go test ./...)
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="level" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,level" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="level,txn-commit" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="level,txn-commit,txn-context" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,level,txn-commit"  sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="level,txn-discard" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="level,txn-discard,txn-context" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,level,txn-discard" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="chunk,level" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,chunk,level" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="level,chunk,txn-commit" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,chunk,level,txn-commit" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="level,chunk,txn-discard" sh -c 'cd ./test && go test ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,chunk,level,txn-discard" sh -c 'cd ./test && go test ./...'

.PHONY: test\:regolith
test\:regolith:
# The regolith store links the Rust staticlib owned by go-regolith, which the
# whole test module then depends on, so it must be built before any test binary
# in it can be linked.
	@$(MAKE) -C ../go-regolith ffi
# Environment variable changes do not invalidate the go test cache, so it is important
# for us to clean between each run.
	@$(MAKE) clean
	(cd ./regolith && go test -tags regolith ./...)
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="regolith" sh -c 'cd ./test && go test -tags regolith ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,regolith" sh -c 'cd ./test && go test -tags regolith ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="regolith,txn-commit" sh -c 'cd ./test && go test -tags regolith ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="regolith,txn-commit,txn-context" sh -c 'cd ./test && go test -tags regolith ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,regolith,txn-commit" sh -c 'cd ./test && go test -tags regolith ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="regolith,txn-discard" sh -c 'cd ./test && go test -tags regolith ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="regolith,txn-discard,txn-context" sh -c 'cd ./test && go test -tags regolith ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,regolith,txn-discard" sh -c 'cd ./test && go test -tags regolith ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="regolith,txn-multi" sh -c 'cd ./test && go test -tags regolith ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,regolith,txn-multi" sh -c 'cd ./test && go test -tags regolith ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="chunk,regolith" sh -c 'cd ./test && go test -tags regolith ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,chunk,regolith" sh -c 'cd ./test && go test -tags regolith ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="regolith,chunk,txn-commit" sh -c 'cd ./test && go test -tags regolith ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,chunk,regolith,txn-commit" sh -c 'cd ./test && go test -tags regolith ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="regolith,chunk,txn-discard" sh -c 'cd ./test && go test -tags regolith ./...'
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,chunk,regolith,txn-discard" sh -c 'cd ./test && go test -tags regolith ./...'

.PHONY: test\:all
test\:all:
	@$(MAKE) test:memory
	@$(MAKE) test:badger
	@$(MAKE) test:level
	@$(MAKE) test:regolith

.PHONY: test\:ci
test\:ci:
# We do not make the deps here, the ci does that seperately to avoid compiling stuff
# multiple times etc.
	(cd ./test && gotestsum --format testname ./...)

.PHONY: test\:scripts
test\:scripts:
	@$(MAKE) -C ./tools/scripts/ test

.PHONY: test\:wasm
test\:wasm:
	GOOS=js GOARCH=wasm gotestsum --format testname -- -exec="$(shell go env GOROOT)/lib/wasm/go_js_wasm_exec" ./...

.PHONY: tidy
tidy:
	(cd ./memory && go mod tidy)
	(cd ./namespace && go mod tidy)
	(cd ./badger && go mod tidy)
	(cd ./blockstore && go mod tidy)
	(cd ./chunk && go mod tidy)
	(cd ./leveldb && go mod tidy)
	(cd ./regolith && go mod tidy)
# The regolith store is behind the `regolith` build tag, so the tag must be
# enabled here or tidy would drop its require/replace from test/go.mod.
	GOFLAGS=-tags=regolith sh -c 'cd ./test && go mod tidy'
	go mod tidy
