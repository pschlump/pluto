/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset

// table is the private open-addressing hash table behind HashMultiset. One
// slot holds the stored hash, the value, and its count, so a probe touches a
// single slot (one cache line for small values) and a mutation needs exactly
// one hash computation and one probe sequence: find returns the slot index,
// and callers update the count in place or insert at the returned empty slot.
//
// Invariants future editors must keep:
//   - len(slots) is a power of two; shift == 64 - log2(len(slots)).
//   - slot.hash == 0 marks an empty slot; stored hashes are remapped 0 -> 1.
//   - the home slot is the high bits of hash * fibMul (Fibonacci hashing), so
//     weak caller hashes (identity integers) do not cluster under the mask.
//   - linear probing without tombstones: deleteAt shifts the following run
//     back, so a probe can stop at the first empty slot.
//   - length*4 <= len(slots)*3 after every insertion (load factor <= 3/4).
type table[K any, P any] struct {
	slots  []slot[K, P]
	shift  uint
	length int
	eq     func(a, b K) bool
	hash   func(a K) uint64
}

type slot[K any, P any] struct {
	hash uint64
	key  K
	val  P
}

const (
	fibMul         = 0x9E3779B97F4A7C15
	minTableSlots  = 8
	minTableShift  = 64 - 3
	tableLoadNumer = 3
	tableLoadDenom = 4
)

func newTable[K any, P any](eq func(a, b K) bool, hash func(a K) uint64) *table[K, P] {
	return &table[K, P]{
		slots: make([]slot[K, P], minTableSlots),
		shift: minTableShift,
		eq:    eq,
		hash:  hash,
	}
}

func (t *table[K, P]) hashOf(key K) uint64 {
	h := t.hash(key)
	if h == 0 {
		h = 1
	}
	return h
}

func (t *table[K, P]) home(h uint64) int {
	return int((h * fibMul) >> t.shift)
}

// find returns the index of key's slot and true, or the index of the empty
// slot where key belongs and false. h is key's (remapped) hash.
func (t *table[K, P]) find(key K) (index int, h uint64, found bool) {
	h = t.hashOf(key)
	mask := len(t.slots) - 1
	index = t.home(h)
	for {
		s := &t.slots[index]
		if s.hash == 0 {
			return index, h, false
		}
		if s.hash == h && t.eq(s.key, key) {
			return index, h, true
		}
		index = (index + 1) & mask
	}
}

// lookup returns a pointer to key's payload, or nil.
func (t *table[K, P]) lookup(key K) *P {
	if t == nil || t.length == 0 {
		return nil
	}
	index, _, found := t.find(key)
	if !found {
		return nil
	}
	return &t.slots[index].val
}

// insertAt stores a new key at the empty slot index returned by find and
// returns the index where it finally lives (growth moves it).
func (t *table[K, P]) insertAt(index int, h uint64, key K, val P) int {
	if (t.length+1)*tableLoadDenom > len(t.slots)*tableLoadNumer {
		t.grow()
		index = t.emptyFor(h)
	}
	t.slots[index] = slot[K, P]{hash: h, key: key, val: val}
	t.length++
	return index
}

// emptyFor returns the first empty slot on hash h's probe sequence.
func (t *table[K, P]) emptyFor(h uint64) int {
	mask := len(t.slots) - 1
	index := t.home(h)
	for t.slots[index].hash != 0 {
		index = (index + 1) & mask
	}
	return index
}

func (t *table[K, P]) grow() {
	old := t.slots
	t.slots = make([]slot[K, P], 2*len(old))
	t.shift--
	for i := range old {
		if old[i].hash != 0 {
			t.slots[t.emptyFor(old[i].hash)] = old[i]
		}
	}
}

// deleteAt empties slot index and shifts the rest of its probe run back so no
// tombstone is needed.
func (t *table[K, P]) deleteAt(index int) {
	mask := len(t.slots) - 1
	hole := index
	for next := (hole + 1) & mask; t.slots[next].hash != 0; next = (next + 1) & mask {
		home := t.home(t.slots[next].hash)
		// The entry at next may fill the hole only when its home is not in
		// the cyclic interval (hole, next].
		if (next-home)&mask >= (next-hole)&mask {
			t.slots[hole] = t.slots[next]
			hole = next
		}
	}
	t.slots[hole] = slot[K, P]{}
	t.length--
}

func (t *table[K, P]) clear() {
	clear(t.slots)
	t.length = 0
}

func (t *table[K, P]) clone() *table[K, P] {
	if t == nil {
		return nil
	}
	c := *t
	c.slots = append([]slot[K, P](nil), t.slots...)
	return &c
}
