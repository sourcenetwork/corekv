.PHONY: deps\:test-ci
deps\:test-ci:
	go install gotest.tools/gotestsum@latest

.PHONY: deps\:test-js
deps\:test-js:
	go install github.com/agnivade/wasmbrowsertest@latest

.PHONY: clean
clean:
	go clean -testcache

.PHONY: test
test:
	@$(MAKE) clean
	go test ./...

.PHONY: test\:all
test\:all:
# Environment variable changes do not invalidate the go test cache, so it is important
# for us to clean between each run.
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory" go test ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger" go test ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,memory" go test ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,badger" go test ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger,txn-commit" go test ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,badger,txn-commit" go test ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger,txn-discard" go test ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,badger,txn-discard" go test ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="badger,txn-multi" go test ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,badger,txn-multi" go test ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory,txn-commit" go test ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,memory,txn-commit" go test ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory,txn-discard" go test ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,memory,txn-discard" go test ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="memory,txn-multi" go test ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,memory,txn-multi" go test ./...

.PHONY: test\:ci
test\:ci:
# We do not make the deps here, the ci does that seperately to avoid compiling stuff
# multiple times etc.
	gotestsum --format testname ./...

.PHONY: test\:js
test\:js:
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="indexed-db" GOOS=js GOARCH=wasm go test -exec wasmbrowsertest ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="namespace,indexed-db" GOOS=js GOARCH=wasm go test -exec wasmbrowsertest ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="indexed-db,txn-commit" GOOS=js GOARCH=wasm go test -exec wasmbrowsertest ./...
	@$(MAKE) clean
	CORE_KV_MULTIPLIERS="indexed-db,txn-discard" GOOS=js GOARCH=wasm go test -exec wasmbrowsertest ./...
# @$(MAKE) clean
# CORE_KV_MULTIPLIERS="indexed-db,txn-multi" GOOS=js GOARCH=wasm go test -exec wasmbrowsertest ./...
# @$(MAKE) clean
# CORE_KV_MULTIPLIERS="namespace,indexed-db,txn-multi" GOOS=js GOARCH=wasm go test -exec wasmbrowsertest ./...

.PHONY: test\:scripts
test\:scripts:
	@$(MAKE) -C ./tools/scripts/ test
