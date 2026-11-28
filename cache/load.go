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
	s, h := c.locate(key)
	// Lock-free hit: a live, unexpired entry with no refresh due.
	if entry := s.find(key, h); entry != nil && entry.state.Load() == entryLive {
		now := c.stamp()
		if !c.expiredAt(entry, now) && (c.refreshWrite <= 0 || now-entry.writeTime < int64(c.refreshWrite)) {
			value := entry.value
			s.recordRead(entry, now)
			return value, nil
		}
	}
	s.lock.Lock()
	events := s.cleanLocked(nil)
	now := c.stamp()
	entry := s.find(key, h)
	found := entry != nil
	if found && c.expired(entry, now) {
		events = s.removeLocked(entry, Expired, events)
		found = false
	}
	if found && entry.call != nil {
		call := entry.call
		if parent, ok := ctx.Value(loadContextKey{}).(*loadCall[V]); ok && parent == call {
			s.lock.Unlock()
			c.notify(events)
			return *new(V), ErrRecursiveLoad
		}
		if call.refresh {
			value := entry.value
			s.applyRead(entry, now)
			s.lock.Unlock()
			c.notify(events)
			return value, nil
		}
		s.stats.Misses++
		s.lock.Unlock()
		c.notify(events)
		select {
		case <-call.ready:
			return call.value, call.err
		case <-ctx.Done():
			return *new(V), ctx.Err()
		}
	}
	if found {
		value := entry.value
		s.applyRead(entry, now)
		var call *loadCall[V]
		if c.refreshWrite > 0 && now-entry.writeTime >= int64(c.refreshWrite) {
			call = &loadCall[V]{ready: make(chan struct{}), refresh: true}
			entry.call = call
			s.stats.Loads++
		}
		s.lock.Unlock()
		c.notify(events)
		if call != nil {
			started := c.nanos()
			c.executor(func() { _, _ = c.executeLoad(context.WithoutCancel(ctx), s, entry, call, key, loader, started) })
		}
		return value, nil
	}

	call := &loadCall[V]{ready: make(chan struct{})}
	entry = &cacheEntry[K, V]{key: key, hash: h, call: call}
	s.insertLocked(entry)
	s.stats.Misses++
	s.stats.Loads++
	s.lock.Unlock()
	c.notify(events)
	if !c.timed {
		now = c.nanos()
	}
	return c.executeLoad(ctx, s, entry, call, key, loader, now)
}

func (c *Cache[K, V]) executeLoad(ctx context.Context, s *cacheShard[K, V], entry *cacheEntry[K, V], call *loadCall[V], key K, loader Loader[K, V], started int64) (V, error) {
	call.value, call.err = invokeLoader(context.WithValue(ctx, loadContextKey{}, call), loader, key)
	weight := int64(0)
	if call.err == nil {
		weight = c.weight(key, call.value)
	}
	s.lock.Lock()
	s.drainReadsLocked()
	// The completion time is read under the shard lock so the shard's
	// time-ordered lists stay sorted for the front-of-queue expiry drain.
	finished := c.nanos()
	var events []Notification[K, V]
	s.stats.TotalLoadTime += time.Duration(finished - started)
	if call.err != nil {
		s.stats.LoadFailures++
	} else {
		s.stats.LoadSuccesses++
	}
	if s.find(key, entry.hash) == entry {
		entry.call = nil
		if call.refresh {
			if call.err == nil {
				if weight > 0 && s.totalWeight-entry.weight > math.MaxInt64-weight {
					s.lock.Unlock()
					close(call.ready)
					panic("cache: total weight overflow")
				}
				replacement := &cacheEntry[K, V]{key: entry.key, value: call.value, hash: entry.hash, weight: weight, writeTime: finished}
				replacement.accessTime.Store(finished)
				s.replaceLocked(entry, replacement)
				if c.listener != nil {
					events = append(events, Notification[K, V]{Key: key, Value: entry.value, Cause: Replaced})
				}
				events = s.evictLocked(events)
			}
		} else if call.err != nil {
			events = s.removeLocked(entry, LoaderError, events)
		} else {
			if weight > 0 && s.totalWeight > math.MaxInt64-weight {
				s.lock.Unlock()
				close(call.ready)
				panic("cache: total weight overflow")
			}
			entry.value = call.value
			entry.weight = weight
			entry.writeTime = finished
			entry.accessTime.Store(finished)
			s.link(entry) // publishes the placeholder to lock-free readers
			events = s.evictLocked(events)
		}
	}
	s.lock.Unlock()
	close(call.ready)
	c.notify(events)
	return call.value, call.err
}
