package bloom_filter_ts

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/pschlump/pluto/hashing"
)

func TestConcurrentUseAndSnapshots(t *testing.T) {
	filter := New(5000, 0.01, hashing.StringFunnel)
	const workers = 8
	const perWorker = 500
	var wait sync.WaitGroup
	for worker := range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for i := range perWorker {
				value := fmt.Sprintf("%d-%d", worker, i)
				filter.Put(value)
				if !filter.MightContain(value) {
					t.Errorf("false negative for %q", value)
				}
			}
		}()
	}
	wait.Wait()
	copyFilter := filter.Copy()
	encoded := filter.Bytes()
	decoded, err := FromBytes(encoded, hashing.StringFunnel)
	if err != nil || !bytes.Equal(encoded, decoded.Bytes()) || !filter.Compatible(copyFilter) {
		t.Fatalf("snapshot/encoding mismatch: %v", err)
	}
	for worker := range workers {
		for i := range perWorker {
			if !copyFilter.MightContain(fmt.Sprintf("%d-%d", worker, i)) {
				t.Fatalf("copy lost %d-%d", worker, i)
			}
		}
	}
}

func TestConcurrentOppositeMerges(t *testing.T) {
	left := New(1000, 0.01, hashing.StringFunnel)
	right := New(1000, 0.01, hashing.StringFunnel)
	left.Put("left")
	right.Put("right")
	var wait sync.WaitGroup
	wait.Add(2)
	go func() { defer wait.Done(); _ = left.Merge(right) }()
	go func() { defer wait.Done(); _ = right.Merge(left) }()
	wait.Wait()
	if !left.MightContain("left") || !left.MightContain("right") || !right.MightContain("left") || !right.MightContain("right") {
		t.Fatal("opposite merges lost values")
	}
}
