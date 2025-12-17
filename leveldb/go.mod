module github.com/sourcenetwork/corekv/leveldb

go 1.24.6

require (
	github.com/sourcenetwork/corekv v0.0.0
	github.com/sourcenetwork/goleveldb v0.0.0-20251217012629-27249d06b81b
)

replace github.com/sourcenetwork/corekv v0.0.0 => ./..

require github.com/golang/snappy v0.0.4 // indirect
