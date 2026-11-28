/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"fmt"
	"iter"
)

type linkedAssociation[K any, V any] struct {
	key              K
	associationValue V
	prev, next       *linkedAssociation[K, V]
	keyPrev, keyNext *linkedAssociation[K, V]
}

// linkedKeyEntry is one key's state. The key order (first-insertion order of
// the distinct keys, which KeySet and AsMap follow) is an intrusive list
// through prevKey/nextKey, so the key index itself can be the unordered
// package table. values (unique variant only) indexes the key's associations
// by value; per-key order lives in the association keyPrev/keyNext links.
type linkedKeyEntry[K any, V any] struct {
	key              K
	head, tail       *linkedAssociation[K, V]
	values           *table[V, *linkedAssociation[K, V]]
	count            int
	prevKey, nextKey *linkedKeyEntry[K, V]
}

type linkedCore[K any, V any] struct {
	keys      *table[K, *linkedKeyEntry[K, V]]
	eq        func(K, K) bool
	hash      func(K) uint64
	valueEq   func(V, V) bool
	valueHash func(V) uint64
	unique    bool
	head      *linkedAssociation[K, V]
	tail      *linkedAssociation[K, V]
	keyHead   *linkedKeyEntry[K, V]
	keyTail   *linkedKeyEntry[K, V]
	size      int
}

func newLinkedCore[K any, V any](eq func(K, K) bool, hash func(K) uint64, valueEq func(V, V) bool, valueHash func(V) uint64, unique bool) linkedCore[K, V] {
	if eq == nil || hash == nil || valueEq == nil || (unique && valueHash == nil) {
		panic("multimap: linked multimap requires non-nil equality/hash functions")
	}
	return linkedCore[K, V]{
		keys: newTable[K, *linkedKeyEntry[K, V]](eq, hash),
		eq:   eq, hash: hash, valueEq: valueEq, valueHash: valueHash, unique: unique,
	}
}

func (c *linkedCore[K, V]) writeContract(caller string) {
	if c == nil || c.keys == nil {
		panic(fmt.Sprintf("multimap: %s on a nil or zero-value linked multimap; use a constructor", caller))
	}
}

func (c *linkedCore[K, V]) entry(key K) (*linkedKeyEntry[K, V], bool) {
	if c == nil {
		return nil, false
	}
	if found := c.keys.lookup(key); found != nil {
		return *found, true
	}
	return nil, false
}

func (c *linkedCore[K, V]) Get(key K) ([]V, bool) {
	entry, found := c.entry(key)
	if !found {
		return nil, false
	}
	return entry.snapshot(), true
}

func (entry *linkedKeyEntry[K, V]) snapshot() []V {
	values := make([]V, 0, entry.count)
	for node := entry.head; node != nil; node = node.keyNext {
		values = append(values, node.associationValue)
	}
	return values
}

func (c *linkedCore[K, V]) Put(key K, value V) bool {
	c.writeContract("Put")
	_, added := c.put(key, value)
	return added
}

// put adds one association with a single key probe and (unique variant) a
// single value probe. It returns the key's entry (nil when the key is still
// absent) and whether the association was added. Every caller hash runs
// before anything is linked, so a panicking callback mutates nothing.
func (c *linkedCore[K, V]) put(key K, value V) (*linkedKeyEntry[K, V], bool) {
	keyIndex, keyHash, found := c.keys.find(key)
	var entry *linkedKeyEntry[K, V]
	var valueIndex int
	var valueHash uint64
	if found {
		entry = c.keys.slots[keyIndex].val
		if c.unique {
			var present bool
			if valueIndex, valueHash, present = entry.values.find(value); present {
				return entry, false
			}
		}
	}
	if c.size == maxAssociations {
		panic("multimap: association count overflow")
	}
	if !found {
		entry = &linkedKeyEntry[K, V]{key: key}
		if c.unique {
			entry.values = newTable[V, *linkedAssociation[K, V]](c.valueEq, c.valueHash)
			valueIndex, valueHash, _ = entry.values.find(value)
		}
		c.keys.insertAt(keyIndex, keyHash, key, entry)
		c.linkKey(entry)
	}
	node := c.link(entry, key, value)
	if c.unique {
		entry.values.insertAt(valueIndex, valueHash, value, node)
	}
	return entry, true
}

// putEntry is put for a key whose entry is already known.
func (c *linkedCore[K, V]) putEntry(entry *linkedKeyEntry[K, V], key K, value V) bool {
	var valueIndex int
	var valueHash uint64
	if c.unique {
		var present bool
		if valueIndex, valueHash, present = entry.values.find(value); present {
			return false
		}
	}
	if c.size == maxAssociations {
		panic("multimap: association count overflow")
	}
	node := c.link(entry, key, value)
	if c.unique {
		entry.values.insertAt(valueIndex, valueHash, value, node)
	}
	return true
}

// link appends a new association to the global and per-key lists.
func (c *linkedCore[K, V]) link(entry *linkedKeyEntry[K, V], key K, value V) *linkedAssociation[K, V] {
	node := &linkedAssociation[K, V]{key: key, associationValue: value, prev: c.tail, keyPrev: entry.tail}
	if c.tail == nil {
		c.head = node
	} else {
		c.tail.next = node
	}
	c.tail = node
	if entry.tail == nil {
		entry.head = node
	} else {
		entry.tail.keyNext = node
	}
	entry.tail = node
	entry.count++
	c.size++
	return node
}

func (c *linkedCore[K, V]) linkKey(entry *linkedKeyEntry[K, V]) {
	entry.prevKey = c.keyTail
	if c.keyTail == nil {
		c.keyHead = entry
	} else {
		c.keyTail.nextKey = entry
	}
	c.keyTail = entry
}

func (c *linkedCore[K, V]) unlinkKey(entry *linkedKeyEntry[K, V]) {
	if entry.prevKey == nil {
		c.keyHead = entry.nextKey
	} else {
		entry.prevKey.nextKey = entry.nextKey
	}
	if entry.nextKey == nil {
		c.keyTail = entry.prevKey
	} else {
		entry.nextKey.prevKey = entry.prevKey
	}
	entry.prevKey, entry.nextKey = nil, nil
}

// addedCount is the exact number of values PutAll would add; it is needed
// only when a batch might overflow the association count.
func (c *linkedCore[K, V]) addedCount(entry *linkedKeyEntry[K, V], values []V) int {
	if !c.unique {
		return len(values)
	}
	seen := newTable[V, struct{}](c.valueEq, c.valueHash)
	added := 0
	for _, value := range values {
		if entry != nil && entry.values.lookup(value) != nil || !seen.put(value, struct{}{}) {
			continue
		}
		added++
	}
	return added
}

func (c *linkedCore[K, V]) PutAll(key K, values []V) int {
	if len(values) == 0 {
		return 0
	}
	c.writeContract("PutAll")
	entry, _ := c.entry(key)
	if c.size > maxAssociations-len(values) {
		checkedSize(c.size, c.addedCount(entry, values))
	}
	added := 0
	for _, value := range values {
		if entry == nil {
			var ok bool
			if entry, ok = c.put(key, value); ok {
				added++
			}
			continue
		}
		if c.putEntry(entry, key, value) {
			added++
		}
	}
	return added
}

func (c *linkedCore[K, V]) unlink(entry *linkedKeyEntry[K, V], node *linkedAssociation[K, V]) {
	if node.prev == nil {
		c.head = node.next
	} else {
		node.prev.next = node.next
	}
	if node.next == nil {
		c.tail = node.prev
	} else {
		node.next.prev = node.prev
	}
	if node.keyPrev == nil {
		entry.head = node.keyNext
	} else {
		node.keyPrev.keyNext = node.keyNext
	}
	if node.keyNext == nil {
		entry.tail = node.keyPrev
	} else {
		node.keyNext.keyPrev = node.keyPrev
	}
	entry.count--
	c.size--
	node.prev, node.next, node.keyPrev, node.keyNext = nil, nil, nil, nil
}

func (c *linkedCore[K, V]) Remove(key K, value V) bool {
	if c == nil || c.keys == nil || c.keys.length == 0 {
		return false
	}
	keyIndex, _, found := c.keys.find(key)
	if !found {
		return false
	}
	entry := c.keys.slots[keyIndex].val
	var node *linkedAssociation[K, V]
	if c.unique {
		valueIndex, _, present := entry.values.find(value)
		if !present {
			return false
		}
		node = entry.values.slots[valueIndex].val
		entry.values.deleteAt(valueIndex)
	} else {
		for candidate := entry.head; candidate != nil; candidate = candidate.keyNext {
			if c.valueEq(candidate.associationValue, value) {
				node = candidate
				break
			}
		}
		if node == nil {
			return false
		}
	}
	c.unlink(entry, node)
	if entry.count == 0 {
		c.keys.deleteAt(keyIndex)
		c.unlinkKey(entry)
	}
	return true
}

func (c *linkedCore[K, V]) RemoveAll(key K) []V {
	if c == nil || c.keys == nil || c.keys.length == 0 {
		return nil
	}
	keyIndex, _, found := c.keys.find(key)
	if !found {
		return nil
	}
	entry := c.keys.slots[keyIndex].val
	values := make([]V, 0, entry.count)
	for node := entry.head; node != nil; {
		next := node.keyNext
		values = append(values, node.associationValue)
		c.unlink(entry, node)
		node = next
	}
	c.keys.deleteAt(keyIndex)
	c.unlinkKey(entry)
	return values
}

func (c *linkedCore[K, V]) ReplaceValues(key K, values []V) []V {
	c.writeContract("ReplaceValues")
	entry, _ := c.entry(key)
	oldCount := 0
	if entry != nil {
		oldCount = entry.count
	}
	if c.size-oldCount > maxAssociations-len(values) {
		checkedSize(c.size-oldCount, c.addedCount(nil, values))
	}
	old := c.RemoveAll(key)
	c.PutAll(key, values)
	return old
}

func (c *linkedCore[K, V]) ContainsKey(key K) bool {
	_, found := c.entry(key)
	return found
}

func (c *linkedCore[K, V]) ContainsEntry(key K, value V) bool {
	entry, found := c.entry(key)
	if !found {
		return false
	}
	if c.unique {
		return entry.values.lookup(value) != nil
	}
	for node := entry.head; node != nil; node = node.keyNext {
		if c.valueEq(node.associationValue, value) {
			return true
		}
	}
	return false
}

func (c *linkedCore[K, V]) ContainsValue(value V) bool {
	if c == nil || c.keys == nil {
		return false
	}
	for node := c.head; node != nil; node = node.next {
		if c.valueEq(node.associationValue, value) {
			return true
		}
	}
	return false
}

func (c *linkedCore[K, V]) Len() int {
	if c == nil {
		return 0
	}
	return c.size
}

func (c *linkedCore[K, V]) KeyLen() int {
	if c == nil || c.keys == nil {
		return 0
	}
	return c.keys.length
}

func (c *linkedCore[K, V]) IsEmpty() bool { return c.Len() == 0 }

func (c *linkedCore[K, V]) Clear() {
	if c == nil || c.keys == nil {
		return
	}
	c.keys.clear()
	c.head, c.tail, c.keyHead, c.keyTail, c.size = nil, nil, nil, nil, 0
}

func (c *linkedCore[K, V]) Entries() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if c == nil || c.keys == nil {
			return
		}
		for node := c.head; node != nil; node = node.next {
			if !yield(node.key, node.associationValue) {
				return
			}
		}
	}
}

func (c *linkedCore[K, V]) EntriesSlice() []Association[K, V] {
	if c == nil {
		return nil
	}
	entries := make([]Association[K, V], 0, c.size)
	for key, value := range c.Entries() {
		entries = append(entries, Association[K, V]{Key: key, Value: value})
	}
	return entries
}

func (c *linkedCore[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for key := range c.Entries() {
			if !yield(key) {
				return
			}
		}
	}
}

func (c *linkedCore[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, value := range c.Entries() {
			if !yield(value) {
				return
			}
		}
	}
}

// KeySet yields the distinct keys in first-insertion order.
func (c *linkedCore[K, V]) KeySet() iter.Seq[K] {
	return func(yield func(K) bool) {
		if c == nil || c.keys == nil {
			return
		}
		for entry := c.keyHead; entry != nil; entry = entry.nextKey {
			if !yield(entry.key) {
				return
			}
		}
	}
}

func (c *linkedCore[K, V]) AsMap() MapSnapshot[K, V] {
	if c == nil || c.keys == nil {
		return MapSnapshot[K, V]{}
	}
	result := MapSnapshot[K, V]{eq: c.eq, size: c.size}
	for entry := c.keyHead; entry != nil; entry = entry.nextKey {
		result.entries = append(result.entries, MapSnapshotEntry[K, V]{Key: entry.key, Values: entry.snapshot()})
	}
	return result
}
