.PHONY: deps\:test-ci
deps\:test-ci:
	go install gotest.tools/gotestsum@latest

.PHONY: clean
clean:
	go clean -testcache

.PHONY: test
test:
	@$(MAKE) clean
	go test ./test/...

.PHONY: test\:all
test\:all:
# Environment variable changes do not invalidate the go test cache, so it is important
# for us to clean between each run.
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory" go test ./test/...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger" go test ./test/...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,memory" go test ./test/...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,badger" go test ./test/...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger,txn-commit" go test ./test/...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,badger,txn-commit" go test ./test/...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger,txn-discard" go test ./test/...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,badger,txn-discard" go test ./test/...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger,txn-multi" go test ./test/...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,badger,txn-multi" go test ./test/...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory,txn-commit" go test ./test/...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,memory,txn-commit" go test ./test/...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory,txn-discard" go test ./test/...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,memory,txn-discard" go test ./test/...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory,txn-multi" go test ./test/...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,memory,txn-multi" go test ./test/...

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
	(cd ./test && go mod tidy)
	go mod tidy
