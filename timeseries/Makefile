BIN_DIR := ./libs

.PHONY: compile_rust
compile_rust: $(BIN_DIR)
	cargo build --release --manifest-path=./internal/abi/Cargo.toml
	(cd ./internal/abi && cargo run --features headers --bin generate-headers)
	cp ./internal/abi/target/release/libabi.a $(BIN_DIR)/libabi.a
	cp ./internal/abi/target/abi.h $(BIN_DIR)/abi.h

.PHONY: test
test:
	@$(MAKE) compile_rust
	go test ./...
