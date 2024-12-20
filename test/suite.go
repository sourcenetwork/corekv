package kvtest

import (
	"context"
	"encoding/binary"
	"fmt"
	"reflect"
	"runtime"
	"testing"

	"github.com/sourcenetwork/corekv"
)

// BasicSubtests is a list of all basic tests.
var BasicSubtests = []func(t *testing.T, store corekv.Store){
	// SubtestBasicPutGet,
	SubtestBackendsGetSetDelete,
	// SubtestDBIterator,
	// SubtestNotFounds,
	// SubtestPrefix,
	// SubtestLimit,
	// SubtestManyKeysAndQuery,
}

// BatchSubtests is a list of all basic batching datastore tests.
// var BatchSubtests = []func(t *testing.T, batcher corekv.Batchable){
// 	RunBatchTest,
// 	RunBatchDeleteTest,
// 	RunBatchPutAndDeleteTest,
// }

func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func clearDs(t *testing.T, store corekv.Store) {
	t.Log("clearing DS...")
	ctx := context.Background()

	it := store.Iterator(ctx, corekv.DefaultIterOptions)
	defer it.Close(ctx)
	t.Log("clearDS: iterator all")
	res, err := all(it)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("clearDS: iterate through res, items:", len(res))

	for _, item := range res {
		fmt.Println("item:", item)
		if err := store.Delete(ctx, item.key); err != nil {
			t.Fatal(err)
		}
	}

	t.Log("DS Cleared!")
}

// SubtestAll tests the given datastore against all the subtests.
func SubtestAll(t *testing.T, store corekv.Store) {
	for _, f := range BasicSubtests {
		t.Run(getFunctionName(f), func(t *testing.T) {
			f(t, store)
			clearDs(t, store)
		})
	}
	// if bs, ok := store.(corekv.Batchable); ok {
	// 	for _, f := range BatchSubtests {
	// 		t.Run(getFunctionName(f), func(t *testing.T) {
	// 			f(t, bs)
	// 			clearDs(t, store)
	// 		})
	// 	}
	// }
}

type item struct {
	key, value []byte
}

func all(it corekv.Iterator) ([]item, error) {
	res := make([]item, 0)
	fmt.Println("iterator valid:", it.Valid())
	for ; it.Valid(); it.Next() {
		fmt.Println("all - iterate")
		val, err := it.Value()
		if err != nil {
			return nil, err
		}

		res = append(res, item{it.Key(), val})
	}
	return res, nil
}

func int642Bytes(i int64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(i))
	return buf
}

func bytes2Int64(buf []byte) int64 {
	return int64(binary.BigEndian.Uint64(buf))
}
