package array

import (
	"context"
	"fmt"
	"testing"
)

func TestT(t *testing.T) {
	s := &Store{}
	panic(fmt.Sprint(s.Get(context.Background(), []byte{1, 0, 0, 0, 0, 0, 0, 0})))
}
