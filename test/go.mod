module github.com/sourcenetwork/corekv/test

go 1.24.6

require (
	github.com/dgraph-io/badger/v4 v4.8.0
	github.com/sourcenetwork/corekv v0.0.0
	github.com/sourcenetwork/corekv/badger v0.0.0
	github.com/sourcenetwork/corekv/memory v0.0.0
	github.com/sourcenetwork/corekv/namespace v0.0.0
	github.com/sourcenetwork/immutable v0.3.0
	github.com/stretchr/testify v1.10.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgraph-io/ristretto/v2 v2.2.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/flatbuffers v25.2.10+incompatible // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/tidwall/btree v1.7.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/sourcenetwork/corekv v0.0.0 => ./..
	github.com/sourcenetwork/corekv/badger v0.0.0 => ../badger
	github.com/sourcenetwork/corekv/memory v0.0.0 => ../memory
	github.com/sourcenetwork/corekv/namespace v0.0.0 => ../namespace
)
