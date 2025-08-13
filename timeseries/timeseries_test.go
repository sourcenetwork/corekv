package timeseries

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	badgerds "github.com/dgraph-io/badger/v4"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/badger"
	"github.com/stretchr/testify/require"
)

func TestT(t *testing.T) {
	location := t.TempDir()
	start := time.Now().Add(-time.Hour)
	s := New(location, start, time.Second)

	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, uint64(start.UnixMilli()))

	value := make([]byte, 8)
	binary.LittleEndian.PutUint64(value, 1234)

	s.Set(context.Background(), key, value)

	result, _ := s.Get(context.Background(), key)

	require.Equal(t, uint64(1234), binary.LittleEndian.Uint64(result))
}

func Benchmark_ReadSingle_TCQL(b *testing.B) {
	nValues := 5000000 // badger (im or disk) is currently slower from about 4_000_000 (about 1.25 years of 10-sec data)
	location := b.TempDir()
	start := time.Now().Add(-time.Hour)
	s := New(location, start, time.Second)

	keys, _ := setup(s, nValues)

	//middleKey := keys[nValues/2]
	var result []byte

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, _ = s.Get(context.Background(), keys[(nValues/2)+i])
	}

	b.StopTimer()
	_ = result

	/*
		du, _ := exec.Command("du", "-hs", location).Output()
		b.Log(string(du))

		nValues:
		1000: 16Kb
		10000000: 77Mb
	*/
}

func Benchmark_ReadSingle_Badger(b *testing.B) {
	nValues := 5000000 // badger (im or disk) is currently slower from about 4_000_000 (about 1.25 years of 10-sec data)
	location := b.TempDir()
	s, _ := badger.NewDatastore(location, badgerds.DefaultOptions(""))

	keys, _ := setup(s, nValues)

	//middleKey := keys[nValues/2]
	var result []byte

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, _ = s.Get(context.Background(), keys[(nValues/2)+i])
	}

	b.StopTimer()
	_ = result

	/*
		du, _ := exec.Command("du", "-hs", location).Output()
		b.Log(string(du))

		nValues:
		1000: 88Kb
		10000000: 177Mb
	*/
}

func setup(store corekv.Store, nValues int) ([][]byte, [][]byte) {
	start := time.Now().Add(-time.Hour)
	resolution := time.Second

	keys := make([][]byte, 0, nValues)
	values := make([][]byte, 0, nValues)
	for i := 0; i < nValues; i++ {
		t := start.Add(time.Duration(i) * resolution)

		key := make([]byte, 8)
		binary.LittleEndian.PutUint64(key, uint64(t.UnixMilli()))
		keys = append(keys, key)

		value := make([]byte, 8)
		binary.LittleEndian.PutUint64(value, uint64(i*3))
		values = append(values, value)
	}

	for i := 0; i < nValues; i++ {
		store.Set(context.Background(), keys[i], values[i])
	}

	return keys, values
}

func writeJunk(store corekv.Store, nValues int) { // only works for badger atm
	start := time.Now().Add(-time.Hour)
	resolution := time.Second

	for i := 0; i < nValues; i++ {
		t := start.Add(time.Duration(i) * resolution)

		key := make([]byte, 8)
		binary.LittleEndian.PutUint64(key, uint64(t.UnixMilli()))

		value := make([]byte, 8)
		binary.LittleEndian.PutUint64(value, uint64(i*3))

		key = append([]byte("nonsensekeyyyyyyyyyyyyyyyyyyyyyy/"), key...)
		value = append([]byte("nonsensevalueeeeeeeeeeeeeeeeeeeeeeee/"), value...)

		store.Set(context.Background(), key, value)
	}
}
