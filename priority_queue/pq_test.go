package priority_queue

import (
	"fmt"
	"testing"

	// "github.com/pschlump/godebug"
	// "github.com/pschlump/MiscLib"
	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/heap"
)

// Create a "heap of int" type called PqTest
type PqTest struct {
	value    string // The value of the item; arbitrary.
	priority int    // The priority of the item in the queue.
}

// At compile time verify that this is a correct type/interface setup.
var _ comparable.Comparable = (*PqTest)(nil)

// Compare implements the Compare function to satisfy the interface requirements.
func (aa PqTest) Compare(x comparable.Comparable) int {
	if bb, ok := x.(PqTest); ok {
		return int(aa.priority) - int(bb.priority)
	} else if bb, ok := x.(*PqTest); ok {
		return int(aa.priority) - int((*bb).priority)
	} else {
		panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
	}
	return 0
}

func TestCreateHeap(t *testing.T) {
	h := heap.NewHeap[PqTest]()
	_ = h
	return
}
