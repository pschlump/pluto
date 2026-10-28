package hash_index_disk_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: the extendible-hash structural invariant checker
// (called after structural changes), a fixed-seed randomized property
// test cross-checked against a map[uint64]uint64 reference model with
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

// checkInvariants verifies the structural invariants of the extendible
// hash:
//
//   - the directory occupies exactly dirBlockCount(depth) blocks,
//     inside the file, starting at the registry's dirStart,
//   - every slot holds a primary-bucket block number (never 0),
//   - every primary's local depth is ≤ the global depth, and every
//     slot pointing at a primary shares its top localDepth bits (the
//     bucket pattern),
//   - no block holds more than maxEnt entries, overflow blocks are
//     typed and carry depth 0, and chains are acyclic,
//   - every entry hashes back to a slot that resolves to the primary
//     it actually lives under (the routing invariant), and
//   - no key appears twice, and the total entry count equals the
//     registry length.
//
// It returns the number of entries in the index.  Caller must NOT hold
// the store lock (checkInvariants takes it).
func checkInvariants(t *testing.T, ix *Index[uint64]) int {
	t.Helper()
	s := ix.store
	s.mu.Lock()
	defer s.mu.Unlock()

	dirStart, length, _, depth, err := s.readEntryMeta(ix.slot)
	if err != nil {
		t.Fatalf("checkInvariants: readEntryMeta: %v", err)
	}
	if depth > maxDepth {
		t.Fatalf("global depth %d exceeds the cap %d", depth, maxDepth)
	}
	dirBlocks := s.dirBlockCount(depth)
	if dirStart == 0 || dirStart+dirBlocks > s.numBlocks {
		t.Fatalf("directory at block %d for %d blocks is outside the file (%d blocks)", dirStart, dirBlocks, s.numBlocks)
	}

	ks := ix.keySize
	n := 0
	seenKey := make(map[string]bool)
	seenBucket := make(map[uint64]uint32) // primary block -> its pattern
	slots := uint64(1) << depth
	for slot := uint64(0); slot < slots; slot++ {
		bno, err := ix.dirGet(dirStart, slot)
		if err != nil {
			t.Fatalf("checkInvariants: slot %d: %v", slot, err)
		}
		b, err := s.cacheGet(bno)
		if err != nil {
			t.Fatalf("checkInvariants: bucket %d: %v", bno, err)
		}
		if err := checkBucketType(b.data, bno, blkBucket); err != nil {
			t.Fatalf("checkInvariants: slot %d: %v", slot, err)
		}
		d := bucketDepth(b.data)
		if d > depth {
			t.Fatalf("bucket %d has local depth %d, global depth is %d", bno, d, depth)
		}
		pat := uint32(slot >> (depth - d))
		if first, ok := seenBucket[bno]; ok {
			if first != pat {
				t.Fatalf("bucket %d is reachable from slots of two patterns (%0*b vs %0*b)", bno, depth, first, depth, pat)
			}
			continue // already walked
		}
		seenBucket[bno] = pat

		chainSeen := make(map[uint64]bool)
		curNo := bno
		for {
			if chainSeen[curNo] {
				t.Fatalf("overflow chain of bucket %d cycles at block %d", bno, curNo)
			}
			chainSeen[curNo] = true
			cur, err := s.cacheGet(curNo)
			if err != nil {
				t.Fatalf("checkInvariants: chain block %d: %v", curNo, err)
			}
			if curNo != bno {
				if err := checkBucketType(cur.data, curNo, blkOverflow); err != nil {
					t.Fatalf("checkInvariants: chain block %d: %v", curNo, err)
				}
				if bucketDepth(cur.data) != 0 {
					t.Fatalf("overflow block %d carries local depth %d, want 0", curNo, bucketDepth(cur.data))
				}
			}
			c := bucketCount(cur.data)
			if c > ix.maxEnt {
				t.Fatalf("block %d holds %d entries, max is %d", curNo, c, ix.maxEnt)
			}
			for j := 0; j < c; j++ {
				k := string(bytes.Clone(bucketKey(cur.data, j, ks)))
				if seenKey[k] {
					t.Fatalf("key %x appears in more than one entry", k)
				}
				seenKey[k] = true
				// Routing: the entry's hash must resolve, at the global
				// depth, back to the primary it lives under.
				h := ix.hash([]byte(k))
				want, err := ix.dirGet(dirStart, slotOf(h, depth))
				if err != nil {
					t.Fatalf("checkInvariants: routing key %x: %v", k, err)
				}
				if want != bno {
					t.Fatalf("key %x hashes to slot %d (bucket %d) but lives under bucket %d", k, slotOf(h, depth), want, bno)
				}
				n++
			}
			next := bucketOverflow(cur.data)
			if next == 0 {
				break
			}
			curNo = next
		}
	}

	if uint64(n) != length {
		t.Fatalf("counted %d entries, registry length says %d", n, length)
	}
	return n
}

// -------------------------------------------------------------------------------------------------------
// Randomized model test.

// applyRandomOps runs n random operations against both the index and
// the model: 45% insert, 35% delete, 20% search-verify, over a bounded
// key space so deletes and duplicates actually hit.
func applyRandomOps(t *testing.T, rng *rand.Rand, ix *Index[uint64], model map[uint64]uint64, n int) {
	t.Helper()
	const keySpace = 5000
	for i := 0; i < n; i++ {
		k := rng.Uint64N(keySpace)
		switch r := rng.IntN(100); {
		case r < 45:
			v := rng.Uint64()
			added, err := ix.Insert(k, v)
			if err != nil {
				t.Fatalf("op %d: Insert(%d): %v", i, k, err)
			}
			_, existed := model[k]
			if added == existed {
				t.Fatalf("op %d: Insert(%d) added=%v, model existed=%v", i, k, added, existed)
			}
			model[k] = v
		case r < 80:
			deleted, err := ix.Delete(k)
			if err != nil {
				t.Fatalf("op %d: Delete(%d): %v", i, k, err)
			}
			_, existed := model[k]
			if deleted != existed {
				t.Fatalf("op %d: Delete(%d)=%v, model existed=%v", i, k, deleted, existed)
			}
			delete(model, k)
		default:
			v, found, err := ix.Search(k)
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

// assertModel verifies the index against the model: length, full
// iteration as a SET (bucket order is not key order) and spot
// searches.
func assertModel(t *testing.T, ix *Index[uint64], model map[uint64]uint64) {
	t.Helper()
	if ix.Length() != uint64(len(model)) {
		t.Fatalf("Length = %d, model has %d", ix.Length(), len(model))
	}
	got := make(map[uint64]uint64)
	for k, v := range ix.All() {
		mv, ok := model[k]
		if !ok {
			t.Fatalf("All yields key %d absent from the model", k)
		}
		if v != mv {
			t.Fatalf("All yields value %d for key %d, model says %d", v, k, mv)
		}
		if _, dup := got[k]; dup {
			t.Fatalf("All yields key %d twice", k)
		}
		got[k] = v
	}
	if len(got) != len(model) {
		t.Fatalf("All yielded %d pairs, model has %d", len(got), len(model))
	}
	for k, mv := range model {
		v, found, err := ix.Search(k)
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

	// The flusher TICK is off (bisect-friendly) but the half-dirty
	// signal still wakes the flusher mid-test, which exercises the
	// concurrent-flush paths under -race; every assertion still goes
	// through Sync or a clean reopen, never through timing.
	cfg := StoreConfig{Path: path, BlockSize: 512, CacheBlocks: 64, FlushInterval: 600e9}
	s, err := OpenStore(cfg)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	ix, err := NewIndex[uint64](s, u64IndexConfig)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}

	const total = 20000
	done := 0
	for done < total {
		batch := 2000
		if done+batch > total {
			batch = total - done
		}
		applyRandomOps(t, rng, ix, model, batch)
		done += batch
		checkInvariants(t, ix)

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
			ix, err = NewIndex[uint64](s, u64IndexConfig)
			if err != nil {
				t.Fatalf("re-attach at %d ops: %v", done, err)
			}
			assertModel(t, ix, model)
			checkInvariants(t, ix)
		default:
			if err := s.Sync(); err != nil {
				t.Fatalf("Sync at %d ops: %v", done, err)
			}
		}
	}
	assertModel(t, ix, model)
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
	ix, err := NewIndex[uint64](s, u64IndexConfig)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}

	// Phase A: 3000 ops, then Sync — snapshot A is now crash-durable.
	applyRandomOps(t, rng, ix, model, 3000)
	if err := s.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Phase B: 750 more ops, then Sync again — snapshot B is durable and
	// includes post-A updates and deletes.
	applyRandomOps(t, rng, ix, model, 750)
	if err := s.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	snapB := make(map[uint64]uint64, len(model))
	for k, v := range model {
		snapB[k] = v
	}

	// Phase C: 750 more ops, never flushed, then a crash.
	applyRandomOps(t, rng, ix, model, 750)
	s.simulateCrash()

	// Reopen: recovery must restore EXACTLY snapshot B.
	s2, err := OpenStore(cfg)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	ix2, err := NewIndex[uint64](s2, u64IndexConfig)
	if err != nil {
		t.Fatalf("re-attach: %v", err)
	}

	// Every key of snapshot B survived, with its value.
	for k, v := range snapB {
		got, found, err := ix2.Search(k)
		if err != nil || !found || got != v {
			t.Fatalf("snapshot-B key %d: Search = (%d, %v, %v), want (%d, true, nil)", k, got, found, err, v)
		}
	}
	// Nothing from after the last completed flush survived.
	present := 0
	for k := range ix2.All() {
		present++
		if _, ok := snapB[k]; !ok {
			t.Fatalf("key %d survived recovery but is not in snapshot B", k)
		}
	}
	if ix2.Length() != uint64(present) {
		t.Fatalf("Length = %d, but %d keys are present", ix2.Length(), present)
	}
	if present != len(snapB) {
		t.Fatalf("%d keys present, snapshot B has %d", present, len(snapB))
	}
	checkInvariants(t, ix2)

	// The store is fully usable after recovery: continue against the
	// model, reconciled to the recovered state.
	applyRandomOps(t, rng, ix2, snapB, 2000)
	if err := s2.Sync(); err != nil {
		t.Fatalf("Sync after recovery: %v", err)
	}
	assertModel(t, ix2, snapB)
	checkInvariants(t, ix2)
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
	ix, err := NewIndex[uint64](s, u64IndexConfig)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	for i := 0; i < 1000; i++ {
		if _, err := ix.Insert(uint64(i), uint64(i)); err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}
	if err := s.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	for i := 1000; i < 2000; i++ {
		if _, err := ix.Insert(uint64(i), uint64(i)); err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}
	s.simulateCrash()

	s2, err := OpenStore(StoreConfig{Path: path})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	ix2, err := NewIndex[uint64](s2, u64IndexConfig)
	if err != nil {
		t.Fatalf("re-attach: %v", err)
	}
	defer s2.Close()
	// With the flusher off, the post-Sync inserts were never journaled:
	// exactly the first 1000 keys survive.
	if ix2.Length() != 1000 {
		t.Fatalf("Length after crash = %d, want 1000 (flusher disabled, no Sync)", ix2.Length())
	}
	for i := 0; i < 1000; i++ {
		if _, found, _ := ix2.Search(uint64(i)); !found {
			t.Fatalf("key %d missing after crash", i)
		}
	}
	checkInvariants(t, ix2)
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
		ix, err := NewIndex[uint64](s, u64IndexConfig)
		if err != nil {
			t.Fatalf("NewIndex: %v", err)
		}
		for i := 0; i < 1500; i++ {
			if _, err := ix.Insert(uint64(i), uint64(i*3)); err != nil {
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
		ix, err := NewIndex[uint64](s, u64IndexConfig)
		if err != nil {
			t.Fatalf("re-attach: %v", err)
		}
		if ix.Length() != 1500 {
			t.Fatalf("Length = %d, want 1500", ix.Length())
		}
		for i := 0; i < 1500; i += 37 {
			v, found, _ := ix.Search(uint64(i))
			if !found || v != uint64(i*3) {
				t.Fatalf("Search(%d) = (%d, %v)", i, v, found)
			}
		}
		checkInvariants(t, ix)
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
			f.Close()
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
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		verify(t, path)
	})
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// jsonIndexPairs collects All into a numerically sorted "k=v" list
// (iteration order is bucket order, so tests sort before comparing).
func jsonIndexPairs(ix *Index[uint64]) []string {
	var out []string
	for k, v := range ix.All() {
		out = append(out, fmt.Sprintf("%d=%d", k, v))
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func TestJSONMarshal(t *testing.T) {
	s := openTempStore(t, StoreConfig{})
	ix := newU64Index(t, s)

	// Exact array output, ascending key order regardless of bucket
	// order: the wire form sorts by the encoded key bytes.
	for _, kv := range [][2]uint64{{3, 30}, {1, 10}, {2, 20}} {
		if _, err := ix.Insert(kv[0], kv[1]); err != nil {
			t.Fatalf("Insert(%d): %v", kv[0], err)
		}
	}
	b, err := json.Marshal(ix)
	if err != nil {
		t.Fatalf("json.Marshal(index): %v", err)
	}
	want := `[{"key":1,"value":10},{"key":2,"value":20},{"key":3,"value":30}]`
	if string(b) != want {
		t.Errorf("Expected %s, got %s", want, b)
	}

	// An empty index encodes as [].
	empty := newU64Index(t, openTempStore(t, StoreConfig{}))
	if b, err := json.Marshal(empty); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty index, got (%s, %v)", b, err)
	}

	// A zero-value index is a tolerated read: [].
	var zero Index[uint64]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value index, got (%s, %v)", b, err)
	}

	// A direct call on a nil index encodes as []; json.Marshal on a nil
	// *Index never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilIndex *Index[uint64]
	if b, err := nilIndex.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-index call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilIndex); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil index, got (%s, %v)", b, err)
	}

	// An index on a closed store encodes as [] (the All contract).
	cs := openTempStore(t, StoreConfig{})
	cix := newU64Index(t, cs)
	if _, err := cix.Insert(1, 1); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := cs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if b, err := json.Marshal(cix); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an index on a closed store, got (%s, %v)", b, err)
	}
}

func TestJSONUnmarshal(t *testing.T) {
	s := openTempStore(t, StoreConfig{})
	ix := newU64Index(t, s)

	// Decoded entries are stored; searches find them.
	if err := json.Unmarshal([]byte(`[{"key":3,"value":30},{"key":1,"value":10}]`), ix); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(jsonIndexPairs(ix)), "[1=10 3=30]"; got != want {
		t.Errorf("Expected %s, got %s", want, got)
	}
	if v, found, err := ix.Search(3); err != nil || !found || v != 30 {
		t.Errorf("Expected Search(3) = (30, true), got (%d, %v, %v)", v, found, err)
	}

	// A round trip through a second named index in the same store
	// rebuilds the contents and keeps the functions (Search works).
	if _, err := ix.Insert(2, 20); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	b, err := json.Marshal(ix)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	copyCfg := u64IndexConfig
	copyCfg.Name = "json-copy"
	again, err := NewIndex[uint64](s, copyCfg)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(jsonIndexPairs(again)), "[1=10 2=20 3=30]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}
	if v, found, err := again.Search(2); err != nil || !found || v != 20 {
		t.Errorf("Expected Search(2) = (20, true) after unmarshal, got (%d, %v, %v)", v, found, err)
	}
	checkInvariants(t, ix)
	checkInvariants(t, again)

	// Unmarshaling replaces the contents; it does not merge.
	if err := json.Unmarshal([]byte(`[{"key":7,"value":70}]`), ix); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := ix.Length(); got != 1 {
		t.Errorf("Expected replacement, got length %d, want 1", got)
	}
	if got, want := fmt.Sprint(jsonIndexPairs(ix)), "[7=70]"; got != want {
		t.Errorf("Expected %s after replacement, got %s", want, got)
	}
	checkInvariants(t, ix)

	// A duplicate key in the array keeps the last value, as Insert does.
	if err := json.Unmarshal([]byte(`[{"key":5,"value":1},{"key":5,"value":2}]`), ix); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, found, _ := ix.Search(5); !found || v != 2 || ix.Length() != 1 {
		t.Errorf("Expected duplicate key to keep the last value, got (%d, %v), length %d", v, found, ix.Length())
	}

	// An empty array and null clear the index.
	for _, data := range []string{"[]", "null"} {
		if err := json.Unmarshal([]byte(data), ix); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", data, err)
		}
		if ix.Length() != 0 {
			t.Errorf("Expected %s to clear the index, length %d", data, ix.Length())
		}
	}
	checkInvariants(t, ix)

	// Decode errors are returned and leave the index untouched.
	if _, err := ix.Insert(9, 90); err != nil {
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
		if err := json.Unmarshal([]byte(badData), ix); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got, want := fmt.Sprint(jsonIndexPairs(ix)), "[9=90]"; got != want {
			t.Errorf("Index changed after the error on %s: %s", badData, got)
		}
	}
	checkInvariants(t, ix)
}

// TestJSONUnmarshalPanics verifies that UnmarshalJSON joins the insert
// family: storing entries into a nil or zero-value index panics with a
// message naming the method and the fix, while [] and null — which
// store nothing — are tolerated everywhere.
func TestJSONUnmarshalPanics(t *testing.T) {
	var zero Index[uint64]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value index to be tolerated, got %v", data, err)
		}
	}
	expectPanic(t, "UnmarshalJSON on a zero-value index", "NewIndex", func() {
		_ = zero.UnmarshalJSON([]byte(`[{"key":1,"value":1}]`))
	})

	var nilIndex *Index[uint64]
	if err := nilIndex.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil index to be tolerated, got %v", err)
	}
	expectPanic(t, "UnmarshalJSON on a nil index", "nil index", func() {
		_ = nilIndex.UnmarshalJSON([]byte(`[{"key":1,"value":1}]`))
	})
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a map[uint64]uint64 reference model at fixed seed: the
// marshaled index must equal the model marshaled as a sorted slice of
// pairs, and unmarshaling into a second index must reproduce the model.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260903, 42))
	const ops = 80
	const keySpace = 300

	s := openTempStore(t, StoreConfig{BlockSize: 512, CacheBlocks: 64})
	ix := newU64Index(t, s)
	copyCfg := u64IndexConfig
	copyCfg.Name = "json-copy"
	fresh, err := NewIndex[uint64](s, copyCfg)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	model := make(map[uint64]uint64)

	for step := range ops {
		k := rng.Uint64N(keySpace)
		if rng.IntN(100) < 55 {
			v := rng.Uint64()
			if _, err := ix.Insert(k, v); err != nil {
				t.Fatalf("step %d: Insert(%d): %v", step, k, err)
			}
			model[k] = v
		} else {
			if _, err := ix.Delete(k); err != nil {
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
		got, err := json.Marshal(ix)
		if err != nil {
			t.Fatalf("step %d: json.Marshal: %v", step, err)
		}
		if string(got) != string(want) {
			t.Fatalf("step %d: marshaled %s, model %s", step, got, want)
		}

		// Unmarshaling into the second index must reproduce the model.
		if err := json.Unmarshal(got, fresh); err != nil {
			t.Fatalf("step %d: json.Unmarshal: %v", step, err)
		}
		assertModel(t, fresh, model)
	}
	checkInvariants(t, ix)
	checkInvariants(t, fresh)
}
