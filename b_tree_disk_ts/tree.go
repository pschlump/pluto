/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The generic B+ tree layer: Tree[K] maps fixed-size encoded keys to
// uint64 values on top of a Store.  Keys travel through the tree as
// their KeySize-byte encodings and are ordered by bytes.Compare of the
// encoding — EncodeKey and Compare MUST agree (encoded byte order is
// the Compare order), the same kind of contract as the eq/hash pair in
// hash_tab_ts.  Compare is kept as the statement of that contract and
// for the caller's own use.
//
// Insert descends top-down, splitting full nodes on the way (so no
// node ever overflows its block): full leaves split by COPYING the
// middle key up (it stays in the right leaf), full internal nodes split
// by PUSHING the middle key up (it stays in neither child), and a root
// split allocates a new root and updates the registry.  Delete removes
// from the leaf and repairs underflow on the way back up — borrow from
// a sibling through the parent, else merge and free the emptied block
// onto the store's free list — and collapses an internal root that
// shrinks to one child.

package b_tree_disk_ts

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"iter"
)

// TreeConfig describes one named tree in a store.  Re-opening an
// existing Name attaches to the existing tree (KeySize must match).
type TreeConfig[K any] struct {
	Name      string                // ≤ 31 bytes
	KeySize   int                   // fixed encoded byte length of a key
	EncodeKey func(k K, buf []byte) // must write exactly KeySize bytes
	DecodeKey func(buf []byte) K    // must copy out of buf if K is a reference type
	Compare   func(a, b K) int      // must order keys exactly as their encodings order byte-wise
}

// Tree is one named B+ tree inside a Store.  Create or attach with
// NewTree; re-attach after reopening the store (a Tree does not survive
// its Store being closed).  Tree methods are safe for concurrent use —
// they all take the store lock; see the package documentation for why
// reads take the same lock as writes.
type Tree[K any] struct {
	store   *Store
	name    string
	slot    int // registry slot in the superblock
	keySize int
	maxLeaf int // max entries in a leaf block
	maxInt  int // max separator keys in an internal block

	encode  func(k K, buf []byte)
	decode  func(buf []byte) K
	compare func(a, b K) int
}

// NewTree creates a named tree in the store, or attaches to the
// existing tree of that name (KeySize must match, else
// ErrKeySizeMismatch).  A new tree allocates one empty leaf block as
// its root.
//
// It panics on programmer errors: a nil store, nil EncodeKey /
// DecodeKey / Compare, KeySize ≤ 0, an empty Name, or a Name longer
// than 31 bytes.  IO failures, a full registry (ErrTooManyTrees) and a
// key too large for the block size (ErrKeyTooLarge) are returned as
// errors.
// Complexity is O(1).
func NewTree[K any](s *Store, cfg TreeConfig[K]) (*Tree[K], error) {
	if s == nil {
		panic("b_tree_disk_ts: NewTree called with a nil store")
	}
	if cfg.EncodeKey == nil {
		panic("b_tree_disk_ts: NewTree called with a nil EncodeKey function")
	}
	if cfg.DecodeKey == nil {
		panic("b_tree_disk_ts: NewTree called with a nil DecodeKey function")
	}
	if cfg.Compare == nil {
		panic("b_tree_disk_ts: NewTree called with a nil Compare function")
	}
	if cfg.KeySize <= 0 {
		panic("b_tree_disk_ts: NewTree called with KeySize <= 0 (KeySize is the fixed encoded byte length of a key)")
	}
	if len(cfg.Name) == 0 {
		panic("b_tree_disk_ts: NewTree called with an empty tree name")
	}
	if len(cfg.Name) > maxTreeNameLen {
		panic(fmt.Sprintf("b_tree_disk_ts: NewTree called with a tree name of %d bytes, the maximum is %d", len(cfg.Name), maxTreeNameLen))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}

	maxLeaf := maxLeafEntries(s.blockSize, cfg.KeySize)
	maxInt := maxInternalKeys(s.blockSize, cfg.KeySize)
	tree := &Tree[K]{
		store:   s,
		name:    cfg.Name,
		keySize: cfg.KeySize,
		maxLeaf: maxLeaf,
		maxInt:  maxInt,
		encode:  cfg.EncodeKey,
		decode:  cfg.DecodeKey,
		compare: cfg.Compare,
	}

	slot, found, err := s.findTreeLocked(cfg.Name)
	if err != nil {
		return nil, err
	}
	if found {
		_, _, ks, err := s.readEntryLocked(slot)
		if err != nil {
			return nil, err
		}
		if int(ks) != cfg.KeySize {
			return nil, fmt.Errorf("%w: tree %q was created with key size %d", ErrKeySizeMismatch, cfg.Name, ks)
		}
		tree.slot = slot
		return tree, nil
	}

	if maxLeaf < 4 || maxInt < 4 {
		return nil, fmt.Errorf("%w: key size %d in %d-byte blocks", ErrKeyTooLarge, cfg.KeySize, s.blockSize)
	}
	slot, err = s.freeSlotLocked()
	if err != nil {
		return nil, err
	}
	if slot < 0 {
		return nil, fmt.Errorf("%w: %d trees in %d-byte blocks", ErrTooManyTrees, s.registryCap(), s.blockSize)
	}
	rootNo, root, err := s.allocBlock()
	if err != nil {
		return nil, err
	}
	leafInit(root.data)
	s.markDirty(root)
	if err := s.writeEntryLocked(slot, cfg.Name, rootNo, 0, uint32(cfg.KeySize)); err != nil {
		return nil, err
	}
	tree.slot = slot
	return tree, nil
}

// -------------------------------------------------------------------------------------------------------
// Occupancy thresholds: repair triggers below ceil(max/2); a sibling
// can spare an entry when it holds more than floor(max/2).  See the
// occupancy note at the top of node.go.

func (t *Tree[K]) leafTrigger() int { return (t.maxLeaf + 1) / 2 }
func (t *Tree[K]) leafFloor() int   { return t.maxLeaf / 2 }
func (t *Tree[K]) intTrigger() int  { return (t.maxInt + 1) / 2 }
func (t *Tree[K]) intFloor() int    { return t.maxInt / 2 }

// Insert adds key → value to the tree.  It returns true when the key is
// new and false when an existing entry's value was replaced.  The
// change is in the cache when Insert returns; it is crash-durable after
// the next completed flush (see Sync and the package durability
// contract).  Insert on a closed store returns ErrClosed.
// Complexity is O(log n) block accesses plus O(maxEntries) byte moves
// per visited block.
func (t *Tree[K]) Insert(key K, value uint64) (bool, error) {
	buf := make([]byte, t.keySize)
	t.encode(key, buf)
	s := t.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false, ErrClosed
	}
	return t.insertNoLock(buf, value)
}

// insertNoLock is Insert without locking.  Caller holds s.mu.
func (t *Tree[K]) insertNoLock(key []byte, value uint64) (bool, error) {
	s := t.store
	ks := t.keySize

	var pinned []*cacheBlock
	pin := func(b *cacheBlock) { b.pins++; pinned = append(pinned, b) }
	defer func() {
		for _, b := range pinned {
			b.pins--
		}
	}()

	rootNo, _, _, err := s.readEntryLocked(t.slot)
	if err != nil {
		return false, err
	}
	root, err := s.cacheGet(rootNo)
	if err != nil {
		return false, err
	}
	pin(root)

	// Split a full root first: the tree grows one level.
	if nodeFull(root.data, t.maxLeaf, t.maxInt) {
		rightNo, right, err := s.allocBlock()
		if err != nil {
			return false, err
		}
		var sep []byte
		if isLeaf(root.data) {
			sep = leafSplit(root.data, right.data, rightNo, ks, t.maxLeaf)
		} else {
			sep = internalSplit(root.data, right.data, ks, t.maxInt)
		}
		s.markDirty(root)
		s.markDirty(right)
		newRootNo, newRoot, err := s.allocBlock()
		if err != nil {
			return false, err
		}
		internalInit(newRoot.data)
		internalInsertAt(newRoot.data, 0, sep, rightNo, ks, t.maxInt)
		internalSetChild(newRoot.data, 0, rootNo, ks, t.maxInt)
		s.markDirty(newRoot)
		if err := s.setEntryRootLocked(t.slot, newRootNo); err != nil {
			return false, err
		}
		root = newRoot // dirty, hence unevictable; no pin needed
	}

	// Descend, splitting full children before stepping into them so the
	// current node always has room for a separator pushed up from below.
	// cur is pinned across each child load: an eviction of a clean,
	// unpinned cur would orphan the separator a split writes into it.
	cur := root
	for !isLeaf(cur.data) {
		pin(cur)
		i := internalFind(cur.data, key, ks)
		child, err := s.cacheGet(internalChild(cur.data, i, ks, t.maxInt))
		if err != nil {
			return false, err
		}
		if nodeFull(child.data, t.maxLeaf, t.maxInt) {
			pin(child) // held across allocBlock; released by the deferred unpin
			rightNo, right, err := s.allocBlock()
			if err != nil {
				return false, err
			}
			var sep []byte
			if isLeaf(child.data) {
				sep = leafSplit(child.data, right.data, rightNo, ks, t.maxLeaf)
			} else {
				sep = internalSplit(child.data, right.data, ks, t.maxInt)
			}
			internalInsertAt(cur.data, i, sep, rightNo, ks, t.maxInt)
			s.markDirty(child)
			s.markDirty(right)
			s.markDirty(cur)
			if bytes.Compare(key, sep) >= 0 {
				child = right
			}
		}
		cur = child
	}

	i, found := leafFind(cur.data, key, ks)
	if found {
		off := leafEntryOff(i, ks)
		binary.LittleEndian.PutUint64(cur.data[off+ks:], value)
		s.markDirty(cur)
		return false, nil
	}
	leafInsertAt(cur.data, i, key, value, ks)
	s.markDirty(cur)
	if err := s.addEntryLengthLocked(t.slot, 1); err != nil {
		return false, err
	}
	return true, nil
}

// Search returns the value stored for key and whether it was found.
// Search on a closed store returns ErrClosed.
// Complexity is O(log n) block accesses.
func (t *Tree[K]) Search(key K) (uint64, bool, error) {
	buf := make([]byte, t.keySize)
	t.encode(key, buf)
	s := t.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, false, ErrClosed
	}
	return t.searchNoLock(buf)
}

// searchNoLock is Search without locking.  Caller holds s.mu.
func (t *Tree[K]) searchNoLock(key []byte) (uint64, bool, error) {
	s := t.store
	ks := t.keySize
	rootNo, _, _, err := s.readEntryLocked(t.slot)
	if err != nil {
		return 0, false, err
	}
	cur, err := s.cacheGet(rootNo)
	if err != nil {
		return 0, false, err
	}
	for !isLeaf(cur.data) {
		i := internalFind(cur.data, key, ks)
		if cur, err = s.cacheGet(internalChild(cur.data, i, ks, t.maxInt)); err != nil {
			return 0, false, err
		}
	}
	i, found := leafFind(cur.data, key, ks)
	if !found {
		return 0, false, nil
	}
	return leafValue(cur.data, i, ks), true, nil
}

// Delete removes key from the tree, returning true if it was present.
// Underfull nodes are repaired by borrowing from a sibling or by
// merging (which frees a block onto the store's free list); separators
// left behind by plain deletions may stay stale — they remain valid
// lower bounds, see the node.go note.  Delete on a closed store returns
// ErrClosed.
// Complexity is O(log n) block accesses plus O(maxEntries) byte moves
// per visited block.
func (t *Tree[K]) Delete(key K) (bool, error) {
	buf := make([]byte, t.keySize)
	t.encode(key, buf)
	s := t.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false, ErrClosed
	}
	return t.deleteNoLock(buf)
}

// deleteNoLock is Delete without locking.  Caller holds s.mu.
func (t *Tree[K]) deleteNoLock(key []byte) (bool, error) {
	s := t.store
	ks := t.keySize

	var pinned []*cacheBlock
	pin := func(b *cacheBlock) { b.pins++; pinned = append(pinned, b) }
	defer func() {
		for _, b := range pinned {
			b.pins--
		}
	}()

	rootNo, _, _, err := s.readEntryLocked(t.slot)
	if err != nil {
		return false, err
	}

	// Descend to the leaf, remembering the path for the repair pass.
	type step struct {
		no  uint64
		idx int // child index taken in the parent
	}
	var path []step
	curNo := rootNo
	cur, err := s.cacheGet(curNo)
	if err != nil {
		return false, err
	}
	pin(cur)
	for !isLeaf(cur.data) {
		i := internalFind(cur.data, key, ks)
		path = append(path, step{curNo, i})
		childNo := internalChild(cur.data, i, ks, t.maxInt)
		if cur, err = s.cacheGet(childNo); err != nil {
			return false, err
		}
		pin(cur)
		curNo = childNo
	}

	i, found := leafFind(cur.data, key, ks)
	if !found {
		return false, nil
	}
	leafRemoveAt(cur.data, i, ks)
	s.markDirty(cur)
	if err := s.addEntryLengthLocked(t.slot, -1); err != nil {
		return false, err
	}

	// Repair underflow from the leaf up to (but not including) the root.
	childNo, child := curNo, cur
	for level := len(path) - 1; level >= 0; level-- {
		trigger, floor := t.intTrigger(), t.intFloor()
		if isLeaf(child.data) {
			trigger, floor = t.leafTrigger(), t.leafFloor()
		}
		if nodeCount(child.data) >= trigger {
			break
		}
		parentNo, pidx := path[level].no, path[level].idx
		parent, err := s.cacheGet(parentNo) // already pinned
		if err != nil {
			return false, err
		}
		pn := internalCount(parent.data)
		var left, right *cacheBlock
		var leftNo, rightNo uint64
		if pidx > 0 {
			leftNo = internalChild(parent.data, pidx-1, ks, t.maxInt)
			if left, err = s.cacheGet(leftNo); err != nil {
				return false, err
			}
			pin(left)
		}
		if pidx < pn {
			rightNo = internalChild(parent.data, pidx+1, ks, t.maxInt)
			if right, err = s.cacheGet(rightNo); err != nil {
				return false, err
			}
			pin(right)
		}

		if isLeaf(child.data) {
			switch {
			case left != nil && leafCount(left.data) > floor:
				leafBorrowFromLeft(parent.data, pidx, left.data, child.data, ks)
				s.markDirty(parent)
				s.markDirty(left)
				s.markDirty(child)
			case right != nil && leafCount(right.data) > floor:
				leafBorrowFromRight(parent.data, pidx, child.data, right.data, ks)
				s.markDirty(parent)
				s.markDirty(child)
				s.markDirty(right)
			case left != nil:
				leafMergeInto(left.data, child.data, ks)
				internalRemoveAt(parent.data, pidx-1, ks, t.maxInt)
				s.markDirty(parent)
				s.markDirty(left)
				if err := s.freeBlock(childNo); err != nil {
					return false, err
				}
				childNo, child = parentNo, parent
				continue
			default:
				leafMergeInto(child.data, right.data, ks)
				internalRemoveAt(parent.data, pidx, ks, t.maxInt)
				s.markDirty(parent)
				s.markDirty(child)
				if err := s.freeBlock(rightNo); err != nil {
					return false, err
				}
				childNo, child = parentNo, parent
				continue
			}
			break // a borrow restores occupancy
		}

		switch {
		case left != nil && internalCount(left.data) > floor:
			internalBorrowFromLeft(parent.data, pidx, left.data, child.data, ks, t.maxInt)
			s.markDirty(parent)
			s.markDirty(left)
			s.markDirty(child)
		case right != nil && internalCount(right.data) > floor:
			internalBorrowFromRight(parent.data, pidx, child.data, right.data, ks, t.maxInt)
			s.markDirty(parent)
			s.markDirty(child)
			s.markDirty(right)
		case left != nil:
			internalMergeInto(left.data, child.data, internalKey(parent.data, pidx-1, ks), ks, t.maxInt)
			internalRemoveAt(parent.data, pidx-1, ks, t.maxInt)
			s.markDirty(parent)
			s.markDirty(left)
			if err := s.freeBlock(childNo); err != nil {
				return false, err
			}
			childNo, child = parentNo, parent
			continue
		default:
			internalMergeInto(child.data, right.data, internalKey(parent.data, pidx, ks), ks, t.maxInt)
			internalRemoveAt(parent.data, pidx, ks, t.maxInt)
			s.markDirty(parent)
			s.markDirty(child)
			if err := s.freeBlock(rightNo); err != nil {
				return false, err
			}
			childNo, child = parentNo, parent
			continue
		}
		break // a borrow restores occupancy
	}

	// Collapse an internal root that lost its last separator.
	root, err := s.cacheGet(rootNo)
	if err != nil {
		return false, err
	}
	if !isLeaf(root.data) && internalCount(root.data) == 0 {
		newRoot := internalChild(root.data, 0, ks, t.maxInt)
		if err := s.freeBlock(rootNo); err != nil {
			return false, err
		}
		if err := s.setEntryRootLocked(t.slot, newRoot); err != nil {
			return false, err
		}
	}
	return true, nil
}

// Length returns the number of entries in the tree, read from the
// registry entry under the store lock.  On a closed store it returns 0.
// Complexity is O(1).
func (t *Tree[K]) Length() uint64 {
	s := t.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0
	}
	_, n, _, err := s.readEntryLocked(t.slot)
	if err != nil {
		return 0
	}
	return n
}

// All returns an iterator over every (key, value) pair in ascending key
// order.  The pairs are a SNAPSHOT: All collects them under one hold of
// the store lock (costing O(n) memory) and the returned iterator walks
// that slice with no lock held, so it is unaffected by — and does not
// block — concurrent inserts and deletes.  On a closed store (or an IO
// error) the iterator is empty.
// Complexity is O(n) to build the snapshot, O(1) per yield.
func (t *Tree[K]) All() iter.Seq2[K, uint64] {
	pairs, err := t.collect(nil, false)
	return pairSeq(pairs, err)
}

// Range returns an iterator like All, starting at the first key ≥ lo.
// Complexity is O(log n + k) for k yielded pairs, plus an O(n) worst
// case snapshot memory cost.
func (t *Tree[K]) Range(lo K) iter.Seq2[K, uint64] {
	buf := make([]byte, t.keySize)
	t.encode(lo, buf)
	pairs, err := t.collect(buf, true)
	return pairSeq(pairs, err)
}

type kvPair[K any] struct {
	k K
	v uint64
}

// pairSeq turns a snapshot (or an error, which yields nothing) into an
// iterator.
func pairSeq[K any](pairs []kvPair[K], err error) iter.Seq2[K, uint64] {
	return func(yield func(K, uint64) bool) {
		if err != nil {
			return
		}
		for _, p := range pairs {
			if !yield(p.k, p.v) {
				return
			}
		}
	}
}

// collect snapshots the tree's pairs under the store lock by walking
// the nextLeaf chain, optionally starting at the first key ≥ lo.
func (t *Tree[K]) collect(lo []byte, useLo bool) ([]kvPair[K], error) {
	s := t.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	ks := t.keySize
	rootNo, _, _, err := s.readEntryLocked(t.slot)
	if err != nil {
		return nil, err
	}
	cur, err := s.cacheGet(rootNo)
	if err != nil {
		return nil, err
	}
	for !isLeaf(cur.data) {
		idx := 0
		if useLo {
			idx = internalFind(cur.data, lo, ks)
		}
		if cur, err = s.cacheGet(internalChild(cur.data, idx, ks, t.maxInt)); err != nil {
			return nil, err
		}
	}
	var out []kvPair[K]
	for {
		start := 0
		if useLo {
			start, _ = leafFind(cur.data, lo, ks)
			useLo = false
		}
		for i := start; i < leafCount(cur.data); i++ {
			out = append(out, kvPair[K]{t.decode(leafKey(cur.data, i, ks)), leafValue(cur.data, i, ks)})
		}
		next := leafNext(cur.data)
		if next == 0 {
			break
		}
		if cur, err = s.cacheGet(next); err != nil {
			return nil, err
		}
	}
	return out, nil
}
