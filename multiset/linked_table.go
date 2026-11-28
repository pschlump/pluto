/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset

// linkedTable is table plus an intrusive doubly linked insertion-order list
// threaded through the slots by index. It backs LinkedHashMultiset without a
// per-entry allocation: a lookup is one hash and one probe run over inline
// slots, and an update changes the count in place without touching the list.
//
// Invariants future editors must keep (in addition to table's):
//   - head/tail and every prev/next are slot indexes, or -1 for none.
//   - the list visits every occupied slot exactly once, in first-insertion
//     order; updates never move an entry in the list.
//   - deleteAt unlinks first, then every backward-shift move of a slot from
//     j to i rewrites the moved entry's neighbours (or head/tail) to i.
//   - grow re-inserts by walking the list, so order survives resizing.
type linkedTable[K any, P any] struct {
	slots      []linkedSlot[K, P]
	shift      uint
	length     int
	head, tail int
	eq         func(a, b K) bool
	hash       func(a K) uint64
}

type linkedSlot[K any, P any] struct {
	hash       uint64
	key        K
	val        P
	prev, next int
}

func newLinkedTable[K any, P any](eq func(a, b K) bool, hash func(a K) uint64) *linkedTable[K, P] {
	return &linkedTable[K, P]{
		slots: make([]linkedSlot[K, P], minTableSlots),
		shift: minTableShift,
		head:  -1,
		tail:  -1,
		eq:    eq,
		hash:  hash,
	}
}

func (t *linkedTable[K, P]) home(h uint64) int {
	return int((h * fibMul) >> t.shift)
}

// find returns the index of key's slot and true, or the index of the empty
// slot where key belongs and false. h is key's (remapped) hash.
func (t *linkedTable[K, P]) find(key K) (index int, h uint64, found bool) {
	h = t.hash(key)
	if h == 0 {
		h = 1
	}
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
func (t *linkedTable[K, P]) lookup(key K) *P {
	if t == nil || t.length == 0 {
		return nil
	}
	index, _, found := t.find(key)
	if !found {
		return nil
	}
	return &t.slots[index].val
}

func (t *linkedTable[K, P]) emptyFor(h uint64) int {
	mask := len(t.slots) - 1
	index := t.home(h)
	for t.slots[index].hash != 0 {
		index = (index + 1) & mask
	}
	return index
}

// insertAt stores a new key at the empty slot index returned by find and
// appends it to the insertion-order list.
func (t *linkedTable[K, P]) insertAt(index int, h uint64, key K, val P) {
	if (t.length+1)*tableLoadDenom > len(t.slots)*tableLoadNumer {
		t.grow()
		index = t.emptyFor(h)
	}
	t.slots[index] = linkedSlot[K, P]{hash: h, key: key, val: val, prev: t.tail, next: -1}
	if t.tail < 0 {
		t.head = index
	} else {
		t.slots[t.tail].next = index
	}
	t.tail = index
	t.length++
}

func (t *linkedTable[K, P]) grow() {
	old := t.slots
	t.slots = make([]linkedSlot[K, P], 2*len(old))
	t.shift--
	prev := -1
	for current := t.head; current >= 0; current = old[current].next {
		index := t.emptyFor(old[current].hash)
		t.slots[index] = old[current]
		t.slots[index].prev = prev
		t.slots[index].next = -1
		if prev < 0 {
			t.head = index
		} else {
			t.slots[prev].next = index
		}
		prev = index
	}
	t.tail = prev
}

// relink points the neighbours of the entry now stored at index back to it.
func (t *linkedTable[K, P]) relink(index int) {
	s := &t.slots[index]
	if s.prev < 0 {
		t.head = index
	} else {
		t.slots[s.prev].next = index
	}
	if s.next < 0 {
		t.tail = index
	} else {
		t.slots[s.next].prev = index
	}
}

// deleteAt unlinks slot index, empties it, and shifts the rest of its probe
// run back so no tombstone is needed.
func (t *linkedTable[K, P]) deleteAt(index int) {
	s := &t.slots[index]
	if s.prev < 0 {
		t.head = s.next
	} else {
		t.slots[s.prev].next = s.next
	}
	if s.next < 0 {
		t.tail = s.prev
	} else {
		t.slots[s.next].prev = s.prev
	}
	mask := len(t.slots) - 1
	hole := index
	for next := (hole + 1) & mask; t.slots[next].hash != 0; next = (next + 1) & mask {
		home := t.home(t.slots[next].hash)
		if (next-home)&mask >= (next-hole)&mask {
			t.slots[hole] = t.slots[next]
			t.relink(hole)
			hole = next
		}
	}
	t.slots[hole] = linkedSlot[K, P]{}
	t.length--
}

func (t *linkedTable[K, P]) clear() {
	clear(t.slots)
	t.length = 0
	t.head, t.tail = -1, -1
}

func (t *linkedTable[K, P]) clone() *linkedTable[K, P] {
	if t == nil {
		return nil
	}
	c := *t
	c.slots = append([]linkedSlot[K, P](nil), t.slots...)
	return &c
}
