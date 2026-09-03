/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bloom_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/bloom_ts"
)

// ExampleBloom is the shared-filter use: goroutines Add and MayContain
// through the same filter, the RWMutex serializing the writes.  The
// outputs are deterministic (the hashes are frozen constants).
func ExampleBloom() {
	f := bloom_ts.NewBloom(1000, 0.01)
	done := make(chan struct{})
	go func() { // a writer goroutine sharing the filter
		for i := range 500 {
			f.Add([]byte(fmt.Sprintf("seen-%d", i)))
		}
		close(done)
	}()
	<-done
	fmt.Println(f.MayContain([]byte("seen-499")), f.MayContain([]byte("never-seen")))
	// Output: true false
}

// ExampleBloom_Lock is the canonical compound: an admission check and
// the batch of adds it admits run as one consistent view under Lock +
// the Nl* forms (a regular method inside the held lock would deadlock).
func ExampleBloom_Lock() {
	f := bloom_ts.NewBloom(100, 0.01)
	batch := [][]byte{[]byte("k1"), []byte("k2"), []byte("k3")}
	f.Lock()
	if f.NlSaturation() < 0.9 { // admission: room in the filter
		for _, k := range batch {
			f.NlAdd(k)
		}
	}
	f.Unlock()
	for _, k := range batch {
		fmt.Println(f.MayContain(k))
	}
	// Output:
	// true
	// true
	// true
}
