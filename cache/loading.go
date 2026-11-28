/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package cache

import (
	"context"
	"errors"
	"iter"
	"time"
)

var ErrBulkLoad = errors.New("cache: bulk loader returned missing, duplicate, or extra keys")

// Get delegates to the configured loader.
func (c *LoadingCache[K, V]) Get(ctx context.Context, key K) (V, error) { return c.cache.Get(ctx, key) }

// GetAll returns entries in key order. Without a bulk loader it uses coalesced
// per-key Get. A bulk loader receives only currently missing distinct keys.
// Returned entries are retained even on partial failure; missing results and a
// non-nil error are returned together. Bulk loads do not coalesce with concurrent
// per-key loads.
func (c *LoadingCache[K, V]) GetAll(ctx context.Context, keys []K) ([]Entry[K, V], error) {
	if c.bulkLoader() == nil {
		result := make([]Entry[K, V], len(keys))
		for index, key := range keys {
			value, err := c.Get(ctx, key)
			result[index] = Entry[K, V]{Key: key, Value: value}
			if err != nil {
				return result, err
			}
		}
		return result, nil
	}
	result := make([]Entry[K, V], len(keys))
	present := make([]bool, len(keys))
	missing := []K{}
	for index, key := range keys {
		value, found := c.cache.GetIfPresent(key)
		result[index] = Entry[K, V]{Key: key, Value: value}
		if found {
			present[index] = true
			continue
		}
		seen := false
		for _, candidate := range missing {
			if c.cache.equal(candidate, key) {
				seen = true
				break
			}
		}
		if !seen {
			missing = append(missing, key)
		}
	}
	if len(missing) == 0 {
		return result, nil
	}
	// Bulk-load statistics have no single key, so they are kept in shard 0;
	// Stats sums every shard.
	statsShard := c.cache.shards[0]
	started := c.cache.nanos()
	statsShard.lock.Lock()
	statsShard.stats.Loads++
	statsShard.lock.Unlock()
	entries, err := c.invokeBulk(ctx, missing)
	finished := c.cache.nanos()
	statsShard.lock.Lock()
	statsShard.stats.TotalLoadTime += time.Duration(finished - started)
	if err != nil {
		statsShard.stats.LoadFailures++
	} else {
		statsShard.stats.LoadSuccesses++
	}
	statsShard.lock.Unlock()
	loaded := make([]bool, len(missing))
	for _, entry := range entries {
		found := false
		for index, key := range missing {
			if c.cache.equal(key, entry.Key) {
				if loaded[index] {
					return result, ErrBulkLoad
				}
				loaded[index], found = true, true
			}
		}
		if !found {
			return result, ErrBulkLoad
		}
	}
	for _, entry := range entries {
		c.cache.Put(entry.Key, entry.Value)
	}
	for _, found := range loaded {
		if !found && err == nil {
			err = ErrBulkLoad
		}
	}
	for index, key := range keys {
		if present[index] {
			continue
		}
		value, found := c.cache.Peek(key)
		if found {
			result[index].Value = value
			present[index] = true
		}
	}
	return result, err
}

func (c *LoadingCache[K, V]) bulkLoader() BulkLoader[K, V] { return c.cache.bulkLoader }
func (c *LoadingCache[K, V]) invokeBulk(ctx context.Context, keys []K) (entries []Entry[K, V], err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.New("cache: bulk loader panic")
		}
	}()
	return c.bulkLoader()(ctx, keys)
}

func (c *LoadingCache[K, V]) Put(key K, value V)              { c.cache.Put(key, value) }
func (c *LoadingCache[K, V]) PutIfAbsent(key K, value V) bool { return c.cache.PutIfAbsent(key, value) }
func (c *LoadingCache[K, V]) Replace(key K, value V) bool     { return c.cache.Replace(key, value) }
func (c *LoadingCache[K, V]) DeleteValue(key K, value V, equal func(V, V) bool) bool {
	return c.cache.DeleteValue(key, value, equal)
}
func (c *LoadingCache[K, V]) GetIfPresent(key K) (V, bool) { return c.cache.GetIfPresent(key) }
func (c *LoadingCache[K, V]) Peek(key K) (V, bool)         { return c.cache.Peek(key) }
func (c *LoadingCache[K, V]) Invalidate(key K)             { c.cache.Invalidate(key) }
func (c *LoadingCache[K, V]) InvalidateKeys(keys []K)      { c.cache.InvalidateKeys(keys) }
func (c *LoadingCache[K, V]) InvalidateAll()               { c.cache.InvalidateAll() }
func (c *LoadingCache[K, V]) CleanUp()                     { c.cache.CleanUp() }
func (c *LoadingCache[K, V]) Len() int                     { return c.cache.Len() }
func (c *LoadingCache[K, V]) IsEmpty() bool                { return c.cache.IsEmpty() }
func (c *LoadingCache[K, V]) Snapshot() []Entry[K, V]      { return c.cache.Snapshot() }
func (c *LoadingCache[K, V]) All() iter.Seq[Entry[K, V]]   { return c.cache.All() }
func (c *LoadingCache[K, V]) Stats() Stats                 { return c.cache.Stats() }
func (c *LoadingCache[K, V]) ClearStats()                  { c.cache.ClearStats() }
