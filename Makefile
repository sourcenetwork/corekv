.PHONY: deps\:test
deps\:test:
	go install github.com/agnivade/wasmbrowsertest@latest

.PHONY: test\:js
test\:js:
	GOOS=js GOARCH=wasm go test -v -timeout 30s -exec wasmbrowsertest ./indexed_db/...