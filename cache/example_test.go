/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package cache_test

import (
	"context"
	"fmt"

	"github.com/pschlump/pluto/cache"
)

func ExampleLoadingCache() {
	value := cache.NewLoading(func(_ context.Context, key string) (int, error) {
		return len(key), nil
	}, cache.WithMaximumSize[string, int](2))
	fmt.Println(value.Get(context.Background(), "pluto"))
	value.Put("a", 1)
	value.Put("b", 2)
	fmt.Println(value.Len())
	// Output:
	// 5 <nil>
	// 2
}
