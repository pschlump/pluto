/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package cache

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

var (
	ErrNoLoader      = errors.New("cache: no loader configured")
	ErrRecursiveLoad = errors.New("cache: recursive load for the same key")
)

type loadCall[V any] struct {
	ready   chan struct{}
	value   V
	err     error
	refresh bool
}

type loadContextKey struct{}

func invokeLoader[K any, V any](ctx context.Context, loader Loader[K, V], key K) (value V, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("cache: loader panic: %v", recovered)
		}
	}()
	return loader(ctx, key)
}

// Get loads key with the configured loader, coalescing concurrent misses.
func (c *Cache[K, V]) Get(ctx context.Context, key K) (V, error) {
	if c.loader == nil {
		return *new(V), ErrNoLoader
	}
	return c.GetWith(ctx, key, c.loader)
}

// GetWith loads key with loader. The initiating caller's context controls the
// shared load; other waiters may cancel independently without stopping it.
func (c *Cache[K, V]) GetWith(ctx context.Context, key K, loader Loader[K, V]) (V, error) {
	if loader == nil {
		return *new(V), ErrNoLoader
	}
	if ctx == nil {
		ctx = context.Background()
	}
	c.lock.Lock()
	now := c.now()
	events := c.cleanExpiredLocked(now, nil)
	entry, found := c.tab.Search(&cacheEntry[K, V]{key: key})
	if found && c.expired(entry, now) {
		events = c.removeLocked(entry, Expired, events)
		found = false
	}
	if found && entry.call != nil {
		call := entry.call
		if parent, ok := ctx.Value(loadContextKey{}).(*loadCall[V]); ok && parent == call {
			c.lock.Unlock()
			c.notify(events)
			return *new(V), ErrRecursiveLoad
		}
		if call.refresh {
			c.stats.Hits++
			c.touchInline(entry, now)
			value := entry.value
			c.lock.Unlock()
			c.notify(events)
			return value, nil
		}
		c.stats.Misses++
		c.lock.Unlock()
		c.notify(events)
		select {
		case <-call.ready:
			return call.value, call.err
		case <-ctx.Done():
			return *new(V), ctx.Err()
		}
	}
	if found {
		c.stats.Hits++
		c.touchInline(entry, now)
		value := entry.value
		var call *loadCall[V]
		if c.refreshWrite > 0 && now.Sub(entry.writeTime) >= c.refreshWrite {
			call = &loadCall[V]{ready: make(chan struct{}), refresh: true}
			entry.call = call
			c.stats.Loads++
		}
		c.lock.Unlock()
		c.notify(events)
		if call != nil {
			c.executor(func() { _, _ = c.executeLoad(context.WithoutCancel(ctx), entry, call, key, loader, c.now()) })
		}
		return value, nil
	}

	call := &loadCall[V]{ready: make(chan struct{})}
	entry = &cacheEntry[K, V]{key: key, call: call}
	c.tab.Insert(entry)
	c.stats.Misses++
	c.stats.Loads++
	c.lock.Unlock()
	c.notify(events)
	return c.executeLoad(ctx, entry, call, key, loader, now)
}

func (c *Cache[K, V]) executeLoad(ctx context.Context, entry *cacheEntry[K, V], call *loadCall[V], key K, loader Loader[K, V], started time.Time) (V, error) {
	call.value, call.err = invokeLoader(context.WithValue(ctx, loadContextKey{}, call), loader, key)
	finished := c.now()
	weight := int64(0)
	if call.err == nil {
		weight = c.weight(key, call.value)
	}
	c.lock.Lock()
	events := []Notification[K, V]{}
	c.stats.TotalLoadTime += finished.Sub(started)
	if call.err != nil {
		c.stats.LoadFailures++
	} else {
		c.stats.LoadSuccesses++
	}
	current, present := c.tab.Search(&cacheEntry[K, V]{key: key})
	if present && current == entry {
		if call.refresh {
			entry.call = nil
			if call.err == nil {
				old := entry.value
				if weight > 0 && c.totalWeight-entry.weight > math.MaxInt64-weight {
					c.lock.Unlock()
					close(call.ready)
					panic("cache: total weight overflow")
				}
				entry.value = call.value
				c.totalWeight += weight - entry.weight
				entry.weight = weight
				entry.writeTime = finished
				entry.accessTime = finished
				if c.listener != nil {
					events = append(events, Notification[K, V]{Key: key, Value: old, Cause: Replaced})
				}
				events = c.evictLocked(events)
			}
		} else {
			entry.call = nil
			if call.err != nil {
				events = c.removeLocked(entry, LoaderError, events)
			} else {
				entry.value = call.value
				entry.weight = weight
				entry.writeTime, entry.accessTime = finished, finished
				if entry.weight > 0 && c.totalWeight > math.MaxInt64-entry.weight {
					c.lock.Unlock()
					close(call.ready)
					panic("cache: total weight overflow")
				}
				c.link(entry)
				events = c.evictLocked(events)
			}
		}
	}
	c.lock.Unlock()
	close(call.ready)
	c.notify(events)
	return call.value, call.err
}
