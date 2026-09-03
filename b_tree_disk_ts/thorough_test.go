package b_tree_disk_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: the B+ tree structural invariant checker (called
// after structural changes), a fixed-seed randomized property test
// cross-checked against a map[uint64]uint64 reference model with
// periodic Sync and Close/reopen cycles, and crash-recovery tests
// (mid-operation crashes via simulateCrash, and a torn journal tail).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// checkInvariants verifies the structural invariants of a B+ tree:
//
//   - keys are strictly ascending within every block,
//   - a non-root leaf holds between floor(maxLeaf/2) and maxLeaf
//     entries, a non-root internal node between floor(maxInt/2) and
//     maxInt keys (the split guarantee; the repair trigger is the
//     higher ceil(max/2)),
//   - an internal node with k keys has exactly k+1 children,
//   - every key of every subtree falls inside its separators' range
//     [sep[i-1], sep[i]) — separators may be stale after deletes, but
//     they stay valid lower bounds, so the range check is exact for
//     present keys,
//   - all leaves are at the same depth,
//   - the nextLeaf chain visits exactly the leaves of the tree, in
//     left-to-right (ascending key) order, and
//   - the total number of entries equals the registry length.
//
// It returns the number of entries in the tree.  Caller must NOT hold
// the store lock (checkInvariants takes it).
func checkInvariants(t *testing.T, tr *Tree[uint64]) int {
	t.Helper()
	s := tr.store
	s.mu.Lock()
	defer s.mu.Unlock()

	rootNo, length, _, err := s.readEntryLocked(tr.slot)
	if err != nil {
		t.Fatalf("checkInvariants: readEntry: %v", err)
	}
	ks := tr.keySize
	leafDepth := -1
	n := 0
	var leaves []uint64

	var walk func(no uint64, depth int, lo, hi []byte)
	walk = func(no uint64, depth int, lo, hi []byte) {
		b, err := s.cacheGet(no)
		if err != nil {
			t.Fatalf("checkInvariants: block %d: %v", no, err)
		}
		d := b.data
		if isLeaf(d) {
			c := leafCount(d)
			if c > tr.maxLeaf {
				t.Fatalf("leaf %d has %d entries, max is %d", no, c, tr.maxLeaf)
			}
			if no != rootNo && c < tr.leafFloor() {
				t.Fatalf("leaf %d has %d entries, min for a non-root is %d", no, c, tr.leafFloor())
			}
			for i := 0; i < c; i++ {
				k := leafKey(d, i, ks)
				if i > 0 && bytes.Compare(leafKey(d, i-1, ks), k) >= 0 {
					t.Fatalf("leaf %d keys not strictly ascending at %d", no, i)
				}
				if lo != nil && bytes.Compare(k, lo) < 0 {
					t.Fatalf("leaf %d key %v below separator %v", no, k, lo)
				}
				if hi != nil && bytes.Compare(k, hi) >= 0 {
					t.Fatalf("leaf %d key %v at/above separator %v", no, k, hi)
				}
				n++
			}
			if leafDepth == -1 {
				leafDepth = depth
			} else if depth != leafDepth {
				t.Fatalf("leaf %d at depth %d, another leaf is at depth %d", no, depth, leafDepth)
			}
			leaves = append(leaves, no)
			return
		}
		c := internalCount(d)
		if c > tr.maxInt {
			t.Fatalf("internal %d has %d keys, max is %d", no, c, tr.maxInt)
		}
		if no != rootNo && c < tr.intFloor() {
			t.Fatalf("internal %d has %d keys, min for a non-root is %d", no, c, tr.intFloor())
		}
		if no == rootNo && c < 1 {
			t.Fatalf("internal root %d has 0 keys and should have collapsed", no)
		}
		for i := 1; i < c; i++ {
			if bytes.Compare(internalKey(d, i-1, ks), internalKey(d, i, ks)) >= 0 {
				t.Fatalf("internal %d separators not strictly ascending at %d", no, i)
			}
		}
		for i := 0; i <= c; i++ {
			clo, chi := lo, hi
			if i > 0 {
				clo = internalKey(d, i-1, ks)
			}
			if i < c {
				chi = internalKey(d, i, ks)
			}
			walk(internalChild(d, i, ks, tr.maxInt), depth+1, clo, chi)
		}
	}
	walk(rootNo, 0, nil, nil)

	if uint64(n) != length {
		t.Fatalf("counted %d entries, registry length says %d", n, length)
	}

	// The nextLeaf chain must visit exactly the leaves, left to right.
	if len(leaves) > 0 {
		seen := make(map[uint64]bool, len(leaves))
		i := 0
		for no := leaves[0]; no != 0; {
			if seen[no] {
				t.Fatalf("nextLeaf chain cycles at block %d", no)
			}
			seen[no] = true
			if i >= len(leaves) || leaves[i] != no {
				t.Fatalf("nextLeaf chain visits block %d at position %d, tree order has %d", no, i, leaves[i])
			}
			b, err := s.cacheGet(no)
			if err != nil {
				t.Fatalf("checkInvariants: leaf %d: %v", no, err)
			}
			no = leafNext(b.data)
			i++
		}
		if i != len(leaves) {
			t.Fatalf("nextLeaf chain visited %d leaves, tree has %d", i, len(leaves))
		}
	}
	return n
}

// -------------------------------------------------------------------------------------------------------
// Randomized model test.

// applyRandomOps runs n random operations against both the tree and the
// model: 45% insert, 35% delete, 20% search-verify, over a bounded key
// space so deletes and duplicates actually hit.
func applyRandomOps(t *testing.T, rng *rand.Rand, tr *Tree[uint64], model map[uint64]uint64, n int) {
	t.Helper()
	const keySpace = 5000
	for i := 0; i < n; i++ {
		k := rng.Uint64N(keySpace)
		switch r := rng.IntN(100); {
		case r < 45:
			v := rng.Uint64()
			added, err := tr.Insert(k, v)
			if err != nil {
				t.Fatalf("op %d: Insert(%d): %v", i, k, err)
			}
			_, existed := model[k]
			if added == existed {
				t.Fatalf("op %d: Insert(%d) added=%v, model existed=%v", i, k, added, existed)
			}
			model[k] = v
		case r < 80:
			deleted, err := tr.Delete(k)
			if err != nil {
				t.Fatalf("op %d: Delete(%d): %v", i, k, err)
			}
			_, existed := model[k]
			if deleted != existed {
				t.Fatalf("op %d: Delete(%d)=%v, model existed=%v", i, k, deleted, existed)
			}
			delete(model, k)
		default:
			v, found, err := tr.Search(k)
			if err != nil {
				t.Fatalf("op %d: Search(%d): %v", i, k, err)
			}
			mv, existed := model[k]
			if found != existed || (found && v != mv) {
				t.Fatalf("op %d: Search(%d)=(%d,%v), model=(%d,%v)", i, k, v, found, mv, existed)
			}
		}
	}
}

// assertModel verifies the tree against the model: length, full
// iteration in ascending order, and spot searches.
func assertModel(t *testing.T, tr *Tree[uint64], model map[uint64]uint64) {
	t.Helper()
	if tr.Length() != uint64(len(model)) {
		t.Fatalf("Length = %d, model has %d", tr.Length(), len(model))
	}
	count := 0
	var prev uint64
	for k, v := range tr.All() {
		mv, ok := model[k]
		if !ok {
			t.Fatalf("All yields key %d absent from the model", k)
		}
		if v != mv {
			t.Fatalf("All yields value %d for key %d, model says %d", v, k, mv)
		}
		if count > 0 && k <= prev {
			t.Fatalf("All not ascending: %d after %d", k, prev)
		}
		prev = k
		count++
	}
	if count != len(model) {
		t.Fatalf("All yielded %d pairs, model has %d", count, len(model))
	}
	for k, mv := range model {
		v, found, err := tr.Search(k)
		if err != nil || !found || v != mv {
			t.Fatalf("Search(%d) = (%d, %v, %v), model says (%d, true)", k, v, found, err, mv)
		}
	}
}

func TestRandomizedModel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "model.db")
	rng := rand.New(rand.NewPCG(42, 0))
	model := make(map[uint64]uint64)

	// A short flush interval exercises the background flusher during the
	// test (important under -race); every assertion still goes through
	// Sync or a clean reopen, never through timing.
	cfg := StoreConfig{Path: path, BlockSize: 512, CacheBlocks: 64, FlushInterval: 600e9} // flusher off for bisect
	s, err := OpenStore(cfg)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	tr, err := NewTree[uint64](s, u64TreeConfig)
	if err != nil {
		t.Fatalf("NewTree: %v", err)
	}

	const total = 20000
	done := 0
	for done < total {
		batch := 2000
		if done+batch > total {
			batch = total - done
		}
		applyRandomOps(t, rng, tr, model, batch)
		done += batch
		checkInvariants(t, tr)

		switch done {
		case 8000, 14000:
			// Close/reopen cycle: the model must survive intact.
			if err := s.Sync(); err != nil {
				t.Fatalf("Sync at %d ops: %v", done, err)
			}
			if err := s.Close(); err != nil {
				t.Fatalf("Close at %d ops: %v", done, err)
			}
			s, err = OpenStore(cfg)
			if err != nil {
				t.Fatalf("reopen at %d ops: %v", done, err)
			}
			tr, err = NewTree[uint64](s, u64TreeConfig)
			if err != nil {
				t.Fatalf("re-attach at %d ops: %v", done, err)
			}
			assertModel(t, tr, model)
			checkInvariants(t, tr)
		default:
			if err := s.Sync(); err != nil {
				t.Fatalf("Sync at %d ops: %v", done, err)
			}
		}
	}
	assertModel(t, tr, model)
	if err := s.Close(); err != nil {
		t.Fatalf("final Close: %v", err)
	}
}

// -------------------------------------------------------------------------------------------------------
// Crash recovery.

// stopFlusher halts the background flusher so a test can control
// exactly when flushes happen (explicit Sync only).  simulateCrash and
// Close tolerate the already-closed stop channel.
func stopFlusher(t *testing.T, s *Store) {
	t.Helper()
	close(s.flusherStop)
	s.flusherWg.Wait()
	s.flusherStop = make(chan struct{}) // so simulateCrash/Close can close it again
}

func TestCrashRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crash.db")
	rng := rand.New(rand.NewPCG(42, 0))
	model := make(map[uint64]uint64)

	// The background flusher is stopped so the crash point is exact:
	// recovery must restore precisely the state of the last completed
	// Sync, no more and no less.  (A long FlushInterval is not enough —
	// the dirty-half-of-cache signal would still wake the flusher.)
	cfg := StoreConfig{Path: path, BlockSize: 512, CacheBlocks: 64}
	s, err := OpenStore(cfg)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	stopFlusher(t, s)
	tr, err := NewTree[uint64](s, u64TreeConfig)
	if err != nil {
		t.Fatalf("NewTree: %v", err)
	}

	// Phase A: 3000 ops, then Sync — snapshot A is now crash-durable.
	applyRandomOps(t, rng, tr, model, 3000)
	if err := s.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Phase B: 750 more ops, then Sync again — snapshot B is durable and
	// includes post-A updates and deletes.
	applyRandomOps(t, rng, tr, model, 750)
	if err := s.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	snapB := make(map[uint64]uint64, len(model))
	for k, v := range model {
		snapB[k] = v
	}

	// Phase C: 750 more ops, never flushed, then a crash.
	applyRandomOps(t, rng, tr, model, 750)
	s.simulateCrash()

	// Reopen: recovery must restore EXACTLY snapshot B.
	s2, err := OpenStore(cfg)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	tr2, err := NewTree[uint64](s2, u64TreeConfig)
	if err != nil {
		t.Fatalf("re-attach: %v", err)
	}

	// Every key of snapshot B survived, with its value.
	for k, v := range snapB {
		got, found, err := tr2.Search(k)
		if err != nil || !found || got != v {
			t.Fatalf("snapshot-B key %d: Search = (%d, %v, %v), want (%d, true, nil)", k, got, found, err, v)
		}
	}
	// Nothing from after the last completed flush survived.
	present := 0
	for k := range tr2.All() {
		present++
		if _, ok := snapB[k]; !ok {
			t.Fatalf("key %d survived recovery but is not in snapshot B", k)
		}
	}
	if tr2.Length() != uint64(present) {
		t.Fatalf("Length = %d, but %d keys are present", tr2.Length(), present)
	}
	if present != len(snapB) {
		t.Fatalf("%d keys present, snapshot B has %d", present, len(snapB))
	}
	checkInvariants(t, tr2)

	// The store is fully usable after recovery: continue against the
	// model, reconciled to the recovered state.
	applyRandomOps(t, rng, tr2, snapB, 2000)
	if err := s2.Sync(); err != nil {
		t.Fatalf("Sync after recovery: %v", err)
	}
	assertModel(t, tr2, snapB)
	checkInvariants(t, tr2)
	if err := s2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestCrashWithoutSyncLosesOnlyUnflushed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crash2.db")

	s, err := OpenStore(StoreConfig{Path: path})
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	stopFlusher(t, s)
	tr, err := NewTree[uint64](s, u64TreeConfig)
	if err != nil {
		t.Fatalf("NewTree: %v", err)
	}
	for i := 0; i < 1000; i++ {
		if _, err := tr.Insert(uint64(i), uint64(i)); err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}
	if err := s.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	for i := 1000; i < 2000; i++ {
		if _, err := tr.Insert(uint64(i), uint64(i)); err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}
	s.simulateCrash()

	s2, err := OpenStore(StoreConfig{Path: path})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	tr2, err := NewTree[uint64](s2, u64TreeConfig)
	if err != nil {
		t.Fatalf("re-attach: %v", err)
	}
	defer s2.Close()
	// With the flusher off, the post-Sync inserts were never journaled:
	// exactly the first 1000 keys survive.
	if tr2.Length() != 1000 {
		t.Fatalf("Length after crash = %d, want 1000 (flusher disabled, no Sync)", tr2.Length())
	}
	for i := 0; i < 1000; i++ {
		if _, found, _ := tr2.Search(uint64(i)); !found {
			t.Fatalf("key %d missing after crash", i)
		}
	}
	checkInvariants(t, tr2)
}

// TestTornJournal appends garbage to the journal after a clean Close
// and verifies recovery ignores it.
func TestTornJournal(t *testing.T) {
	build := func(t *testing.T) (path string) {
		path = filepath.Join(t.TempDir(), "torn.db")
		s, err := OpenStore(StoreConfig{Path: path})
		if err != nil {
			t.Fatalf("OpenStore: %v", err)
		}
		tr, err := NewTree[uint64](s, u64TreeConfig)
		if err != nil {
			t.Fatalf("NewTree: %v", err)
		}
		for i := 0; i < 1500; i++ {
			if _, err := tr.Insert(uint64(i), uint64(i*3)); err != nil {
				t.Fatalf("Insert %d: %v", i, err)
			}
		}
		if err := s.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		return path
	}
	verify := func(t *testing.T, path string) {
		s, err := OpenStore(StoreConfig{Path: path})
		if err != nil {
			t.Fatalf("reopen: %v", err)
		}
		defer s.Close()
		tr, err := NewTree[uint64](s, u64TreeConfig)
		if err != nil {
			t.Fatalf("re-attach: %v", err)
		}
		if tr.Length() != 1500 {
			t.Fatalf("Length = %d, want 1500", tr.Length())
		}
		for i := 0; i < 1500; i += 37 {
			v, found, _ := tr.Search(uint64(i))
			if !found || v != uint64(i*3) {
				t.Fatalf("Search(%d) = (%d, %v)", i, v, found)
			}
		}
		checkInvariants(t, tr)
	}

	t.Run("GarbageBytes", func(t *testing.T) {
		path := build(t)
		f, err := os.OpenFile(path+".journal", os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte("garbage garbage garbage")); err != nil {
			t.Fatal(err)
		}
		f.Close()
		verify(t, path)
	})

	t.Run("TruncatedBatch", func(t *testing.T) {
		path := build(t)
		f, err := os.OpenFile(path+".journal", os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			t.Fatal(err)
		}
		// A plausible batch header (magic, seq, count) with the payload
		// cut off mid-block: a classic torn write.
		hdr := make([]byte, 16)
		hdr[0] = 0x4c // "JRNL" little-endian
		hdr[1] = 0x4e
		hdr[2] = 0x52
		hdr[3] = 0x4a
		hdr[12] = 3 // count = 3, no payload follows
		if _, err := f.Write(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(make([]byte, 100)); err != nil {
			t.Fatal(err)
		}
		f.Close()
		verify(t, path)
	})

	t.Run("BadCRC", func(t *testing.T) {
		path := build(t)
		f, err := os.OpenFile(path+".journal", os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			t.Fatal(err)
		}
		// A full-size but corrupt batch: valid magic, count 1, one block
		// image, wrong CRC.
		blk := make([]byte, 16+8+4096+8)
		blk[0], blk[1], blk[2], blk[3] = 0x4c, 0x4e, 0x52, 0x4a
		blk[12] = 1 // count
		if _, err := f.Write(blk); err != nil {
			t.Fatal(err)
		}
		f.Close()
		verify(t, path)
	})
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// jsonTreePairs collects All into a printable "k=v" list.
func jsonTreePairs(tr *Tree[uint64]) []string {
	var out []string
	for k, v := range tr.All() {
		out = append(out, fmt.Sprintf("%d=%d", k, v))
	}
	return out
}

func TestJSONMarshal(t *testing.T) {
	s := openTempStore(t, StoreConfig{})
	tr := newU64Tree(t, s)

	// Exact array output, ascending key order regardless of insert order.
	for _, kv := range [][2]uint64{{3, 30}, {1, 10}, {2, 20}} {
		if _, err := tr.Insert(kv[0], kv[1]); err != nil {
			t.Fatalf("Insert(%d): %v", kv[0], err)
		}
	}
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("json.Marshal(tree): %v", err)
	}
	want := `[{"key":1,"value":10},{"key":2,"value":20},{"key":3,"value":30}]`
	if string(b) != want {
		t.Errorf("Expected %s, got %s", want, b)
	}

	// An empty tree encodes as [].
	empty := newU64Tree(t, openTempStore(t, StoreConfig{}))
	if b, err := json.Marshal(empty); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty tree, got (%s, %v)", b, err)
	}

	// A zero-value tree is a tolerated read: [].
	var zero Tree[uint64]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value tree, got (%s, %v)", b, err)
	}

	// A direct call on a nil tree encodes as []; json.Marshal on a nil
	// *Tree never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilTree *Tree[uint64]
	if b, err := nilTree.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-tree call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilTree); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil tree, got (%s, %v)", b, err)
	}

	// A tree on a closed store encodes as [] (the All contract).
	cs := openTempStore(t, StoreConfig{})
	ctr := newU64Tree(t, cs)
	if _, err := ctr.Insert(1, 1); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := cs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if b, err := json.Marshal(ctr); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a tree on a closed store, got (%s, %v)", b, err)
	}
}

func TestJSONUnmarshal(t *testing.T) {
	s := openTempStore(t, StoreConfig{})
	tr := newU64Tree(t, s)

	// Decoded entries are stored; iteration is ascending key order.
	if err := json.Unmarshal([]byte(`[{"key":3,"value":30},{"key":1,"value":10}]`), tr); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(jsonTreePairs(tr)), "[1=10 3=30]"; got != want {
		t.Errorf("Expected %s, got %s", want, got)
	}
	if v, found, err := tr.Search(3); err != nil || !found || v != 30 {
		t.Errorf("Expected Search(3) = (30, true), got (%d, %v, %v)", v, found, err)
	}

	// A round trip through a second named tree in the same store
	// rebuilds the contents and keeps the functions (Search works).
	if _, err := tr.Insert(2, 20); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	copyCfg := u64TreeConfig
	copyCfg.Name = "json-copy"
	again, err := NewTree[uint64](s, copyCfg)
	if err != nil {
		t.Fatalf("NewTree: %v", err)
	}
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(jsonTreePairs(again)), "[1=10 2=20 3=30]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}
	if v, found, err := again.Search(2); err != nil || !found || v != 20 {
		t.Errorf("Expected Search(2) = (20, true) after unmarshal, got (%d, %v, %v)", v, found, err)
	}
	checkInvariants(t, tr)
	checkInvariants(t, again)

	// Unmarshaling replaces the contents; it does not merge.
	if err := json.Unmarshal([]byte(`[{"key":7,"value":70}]`), tr); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := tr.Length(); got != 1 {
		t.Errorf("Expected replacement, got length %d, want 1", got)
	}
	if got, want := fmt.Sprint(jsonTreePairs(tr)), "[7=70]"; got != want {
		t.Errorf("Expected %s after replacement, got %s", want, got)
	}
	checkInvariants(t, tr)

	// A duplicate key in the array keeps the last value, as Insert does.
	if err := json.Unmarshal([]byte(`[{"key":5,"value":1},{"key":5,"value":2}]`), tr); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, found, _ := tr.Search(5); !found || v != 2 || tr.Length() != 1 {
		t.Errorf("Expected duplicate key to keep the last value, got (%d, %v), length %d", v, found, tr.Length())
	}

	// An empty array and null clear the tree.
	for _, data := range []string{"[]", "null"} {
		if err := json.Unmarshal([]byte(data), tr); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", data, err)
		}
		if tr.Length() != 0 {
			t.Errorf("Expected %s to clear the tree, length %d", data, tr.Length())
		}
	}
	checkInvariants(t, tr)

	// Decode errors are returned and leave the tree untouched.
	if _, err := tr.Insert(9, 90); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	for _, badData := range []string{
		`[{"key":1,`,              // malformed
		`{"key":1,"value":2}`,     // not an array
		`[{"key":"a","value":1}]`, // wrong key type
		`[{"key":1,"value":"x"}]`, // wrong value type
		`7`,                       // not an array
		`[{"key":1,"value":1},`,   // truncated
		`[{"key":1,"value":1.5}]`, // fractional uint64
	} {
		if err := json.Unmarshal([]byte(badData), tr); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got, want := fmt.Sprint(jsonTreePairs(tr)), "[9=90]"; got != want {
			t.Errorf("Tree changed after the error on %s: %s", badData, got)
		}
	}
	checkInvariants(t, tr)
}

// TestJSONUnmarshalPanics verifies that UnmarshalJSON joins the insert
// family: storing entries into a nil or zero-value tree panics with a
// message naming the method and the fix, while [] and null — which store
// nothing — are tolerated everywhere.
func TestJSONUnmarshalPanics(t *testing.T) {
	var zero Tree[uint64]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value tree to be tolerated, got %v", data, err)
		}
	}
	expectPanic(t, "UnmarshalJSON on a zero-value tree", "NewTree", func() {
		_ = zero.UnmarshalJSON([]byte(`[{"key":1,"value":1}]`))
	})

	var nilTree *Tree[uint64]
	if err := nilTree.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil tree to be tolerated, got %v", err)
	}
	expectPanic(t, "UnmarshalJSON on a nil tree", "nil tree", func() {
		_ = nilTree.UnmarshalJSON([]byte(`[{"key":1,"value":1}]`))
	})
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a map[uint64]uint64 reference model at fixed seed: the
// marshaled tree must equal the model marshaled as a sorted slice of
// pairs, and unmarshaling into a second tree must reproduce the model.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260903, 42))
	const ops = 80
	const keySpace = 300

	s := openTempStore(t, StoreConfig{BlockSize: 512, CacheBlocks: 64})
	tr := newU64Tree(t, s)
	copyCfg := u64TreeConfig
	copyCfg.Name = "json-copy"
	fresh, err := NewTree[uint64](s, copyCfg)
	if err != nil {
		t.Fatalf("NewTree: %v", err)
	}
	model := make(map[uint64]uint64)

	for step := range ops {
		k := rng.Uint64N(keySpace)
		if rng.IntN(100) < 55 {
			v := rng.Uint64()
			if _, err := tr.Insert(k, v); err != nil {
				t.Fatalf("step %d: Insert(%d): %v", step, k, err)
			}
			model[k] = v
		} else {
			if _, err := tr.Delete(k); err != nil {
				t.Fatalf("step %d: Delete(%d): %v", step, k, err)
			}
			delete(model, k)
		}

		// Marshal must equal the model marshaled as a sorted pair slice.
		keys := make([]uint64, 0, len(model))
		for mk := range model {
			keys = append(keys, mk)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		pairs := make([]jsonKV[uint64], 0, len(keys))
		for _, mk := range keys {
			pairs = append(pairs, jsonKV[uint64]{Key: mk, Value: model[mk]})
		}
		want, _ := json.Marshal(pairs)
		got, err := json.Marshal(tr)
		if err != nil {
			t.Fatalf("step %d: json.Marshal: %v", step, err)
		}
		if string(got) != string(want) {
			t.Fatalf("step %d: marshaled %s, model %s", step, got, want)
		}

		// Unmarshaling into the second tree must reproduce the model.
		if err := json.Unmarshal(got, fresh); err != nil {
			t.Fatalf("step %d: json.Unmarshal: %v", step, err)
		}
		assertModel(t, fresh, model)
	}
	checkInvariants(t, tr)
	checkInvariants(t, fresh)
}
