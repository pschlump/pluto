/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
//
// The default NewShardedHash hashing uses a per-process random seed, so
// examples that iterate a table sort their output first — stripe-then-
// bucket order is never asserted.
package sharded_hash_ts_test

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/pschlump/pluto/sharded_hash_ts"
)

// MarshalJSON encodes the table as a JSON array of its elements in
// stripe-then-bucket order.  That order is hash-dependent and varies from
// process to process, so the decoded elements are sorted here — a hash
// table is a set, and the round trip preserves membership, not order.
func ExampleShardedHash_MarshalJSON() {
	tt := sharded_hash_ts.NewShardedHash[int](4, 16, 0.75)
	for _, v := range []int{3, 1, 2} {
		tt.Insert(v)
	}

	b, err := json.Marshal(tt)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	var got []int
	if err := json.Unmarshal(b, &got); err != nil {
		fmt.Println("error:", err)
		return
	}
	sort.Ints(got)
	fmt.Println(got)
	// Output:
	// [1 2 3]
}

// UnmarshalJSON replaces the contents of the table from a JSON array of
// elements; the equality and hash functions are kept, so the table stays
// usable afterward.
func ExampleShardedHash_UnmarshalJSON() {
	tt := sharded_hash_ts.NewShardedHash[string](4, 16, 0.75)
	if err := json.Unmarshal([]byte(`["c","a","b"]`), tt); err != nil {
		fmt.Println("error:", err)
		return
	}

	got := []string{}
	for v := range tt.Values() { // stripe-then-bucket order — sort before printing
		got = append(got, v)
	}
	sort.Strings(got)
	fmt.Println(got, "len:", tt.Len())
	// Output:
	// [a b c] len: 3
}
