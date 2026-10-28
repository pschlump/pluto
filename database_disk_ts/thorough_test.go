package database_disk_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: the structural invariant checker (every live slot is
// reached by exactly one entry in EACH index, both pointing at it; the
// free list is in-bounds, acyclic and disjoint from the live set), a
// fixed-seed randomized property test against a map model with
// Sync and Close/reopen cycles, and the read-repair scenarios (a
// dangling entry heals; a freed slot under a live entry cannot happen
// by construction — deletes free last).

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"sort"
	"testing"
)

// checkInvariants verifies the cross-file invariants of the database:
//
//   - every by-ID entry resolves to a LIVE slot whose embedded id
//     matches, and the by-name entry for that slot's name points at the
//     same slot (the two indexes agree record for record);
//   - every live slot in the record file is reachable from the by-ID
//     index — no orphans, no unreachable records;
//   - both indexes report the same length;
//   - the free list is in bounds, acyclic and disjoint from the live
//     slots.
//
// Caller must NOT hold the DB lock.
func checkInvariants(t *testing.T, db *DB[uint64, [16]byte]) {
	t.Helper()
	db.mu.Lock()
	defer db.mu.Unlock()

	// 1 + 2: walk the by-ID index; verify each slot and cross-check the
	// by-name entry; collect the live slot set.
	idBuf := make([]byte, db.eqKS)
	live := make(map[uint64]bool)
	for id, i := range db.eq.All() {
		db.eqEncode(id, idBuf)
		s, err := db.readSlot(i)
		if err != nil {
			t.Fatalf("checkInvariants: read slot %d: %v", i, err)
		}
		if !s.valid {
			t.Fatalf("checkInvariants: slot %d (id %d) is not interpretable", i, id)
		}
		if !s.live {
			t.Fatalf("checkInvariants: by-ID entry for id %d points at freed slot %d", id, i)
		}
		if !bytes.Equal(s.id, idBuf) {
			t.Fatalf("checkInvariants: slot %d carries id bytes %x, the entry was for id %d (%x)", i, s.id, id, idBuf)
		}
		name := db.ordDecode(s.name)
		j, found, err := db.ord.Search(name)
		if err != nil || !found {
			t.Fatalf("checkInvariants: id %d's name entry is missing (err %v)", id, err)
		}
		if j != i {
			t.Fatalf("checkInvariants: id %d lives in slot %d but its name points at slot %d", id, i, j)
		}
		if live[i] {
			t.Fatalf("checkInvariants: slot %d is reached by two by-ID entries", i)
		}
		live[i] = true
	}

	// 3: every live slot in the file is reachable (no orphans).
	for i := uint64(0); i < db.numSlots; i++ {
		s, err := db.readSlot(i)
		if err != nil {
			t.Fatalf("checkInvariants: read slot %d: %v", i, err)
		}
		if !s.valid {
			t.Fatalf("checkInvariants: slot %d is not interpretable", i)
		}
		if s.live && !live[i] {
			t.Fatalf("checkInvariants: slot %d is live but unreachable from the by-ID index (an orphaned record)", i)
		}
	}

	// 4: the free list is in bounds, acyclic and disjoint from live.
	seenFree := make(map[uint64]bool)
	for p := db.freeHead; p != 0; p = nextFree(t, db, p-1) {
		if p-1 >= db.numSlots {
			t.Fatalf("checkInvariants: free slot %d is outside the file (%d slots)", p-1, db.numSlots)
		}
		if seenFree[p-1] {
			t.Fatalf("checkInvariants: the free list cycles at slot %d", p-1)
		}
		seenFree[p-1] = true
		if live[p-1] {
			t.Fatalf("checkInvariants: slot %d is on the free list AND live", p-1)
		}
	}

	if db.eq.Length() != db.ord.Length() {
		t.Fatalf("checkInvariants: by-ID length %d != by-name length %d", db.eq.Length(), db.ord.Length())
	}
	if db.eq.Length() != uint64(len(live)) {
		t.Fatalf("checkInvariants: by-ID length %d != %d live slots", db.eq.Length(), len(live))
	}
}

// nextFree reads a free slot's next pointer.
func nextFree(t *testing.T, db *DB[uint64, [16]byte], i uint64) uint64 {
	t.Helper()
	s, err := db.readSlot(i)
	if err != nil {
		t.Fatalf("checkInvariants: read free slot %d: %v", i, err)
	}
	return s.next
}

// -------------------------------------------------------------------------------------------------------
// Randomized model test.

type modelRec struct {
	name [16]byte
	data []byte
}

func applyRandomOps(t *testing.T, rng *rand.Rand, db *DB[uint64, [16]byte], model map[uint64]modelRec, n int) {
	t.Helper()
	const keySpace = 1500
	for i := 0; i < n; i++ {
		id := rng.Uint64N(keySpace)
		switch r := rng.IntN(100); {
		case r < 50: // put (new record or data rewrite; names derive from ids so no conflicts)
			data := []byte(fmt.Sprintf("data-%d-%d", id, rng.Uint64N(1000)))
			_, err := db.Put(id, nameOf(id), data)
			if err != nil {
				t.Fatalf("op %d: Put(%d): %v", i, id, err)
			}
			model[id] = modelRec{nameOf(id), data}
		case r < 70: // delete by ID
			ok, err := db.DeleteByID(id)
			if err != nil {
				t.Fatalf("op %d: DeleteByID(%d): %v", i, id, err)
			}
			if ok != (model[id].data != nil) {
				_, had := model[id]
				if ok != had {
					t.Fatalf("op %d: DeleteByID(%d) = %v, model had %v", i, id, ok, had)
				}
			}
			delete(model, id)
		case r < 80: // delete by name
			ok, err := db.DeleteByName(nameOf(id))
			if err != nil {
				t.Fatalf("op %d: DeleteByName(%d): %v", i, id, err)
			}
			if _, had := model[id]; ok != had {
				t.Fatalf("op %d: DeleteByName(%d) = %v, model had %v", i, id, ok, had)
			}
			delete(model, id)
		case r < 90: // point read by ID
			data, found, err := db.GetByID(id)
			if err != nil {
				t.Fatalf("op %d: GetByID(%d): %v", i, id, err)
			}
			rec, had := model[id]
			if found != had || (found && string(data) != string(rec.data)) {
				t.Fatalf("op %d: GetByID(%d) = (%q, %v), model had %v", i, id, data, found, had)
			}
		default: // point read by name
			data, found, err := db.GetByName(nameOf(id))
			if err != nil {
				t.Fatalf("op %d: GetByName(%d): %v", i, id, err)
			}
			rec, had := model[id]
			if found != had || (found && string(data) != string(rec.data)) {
				t.Fatalf("op %d: GetByName(%d) = (%q, %v), model had %v", i, id, data, found, had)
			}
		}
	}
}

func assertModel(t *testing.T, db *DB[uint64, [16]byte], model map[uint64]modelRec) {
	t.Helper()
	if db.Len() != uint64(len(model)) {
		t.Fatalf("Len = %d, model has %d", db.Len(), len(model))
	}
	for id, rec := range model {
		data, found, err := db.GetByID(id)
		if err != nil || !found || string(data) != string(rec.data) {
			t.Fatalf("GetByID(%d) = (%q, %v, %v), want the model's data", id, data, found, err)
		}
		if data, found, _ := db.GetByName(rec.name); !found || string(data) != string(rec.data) {
			t.Fatalf("GetByName(%d's name) = (%q, %v)", id, data, found)
		}
	}
	// All yields exactly the model's ids.
	seen := make(map[uint64]bool)
	for id, data := range db.All() {
		rec, ok := model[id]
		if !ok {
			t.Fatalf("All yielded id %d absent from the model", id)
		}
		if string(data) != string(rec.data) {
			t.Fatalf("All yielded the wrong data for id %d", id)
		}
		if seen[id] {
			t.Fatalf("All yielded id %d twice", id)
		}
		seen[id] = true
	}
	if len(seen) != len(model) {
		t.Fatalf("All yielded %d records, model has %d", len(seen), len(model))
	}
	// A spot-checked range: the model's names in [lo, hi], ascending.
	if len(model) > 0 {
		ids := make([]uint64, 0, len(model))
		for id := range model {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
		lo, hi := nameOf(ids[len(ids)/3]), nameOf(ids[2*len(ids)/3])
		var got []string
		for name := range db.RangeByName(lo, hi) {
			got = append(got, string(name[:]))
		}
		var want []string
		for _, id := range ids {
			n := nameOf(id)
			if bytes.Compare(n[:], lo[:]) >= 0 && bytes.Compare(n[:], hi[:]) <= 0 {
				want = append(want, string(n[:]))
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("RangeByName spot check = %v, want %v", got, want)
		}
	}
}

func TestRandomizedModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-db")
	cfg := DBConfig{Path: path, RecordSize: 128, IndexBlockSize: 512, CacheBlocks: 64}
	rng := rand.New(rand.NewPCG(42, 0))
	model := make(map[uint64]modelRec)

	eq, ord := testConfigs()
	db, err := OpenDB[uint64, [16]byte](cfg, eq, ord)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}

	const total = 10000
	done := 0
	for done < total {
		batch := 2000
		if done+batch > total {
			batch = total - done
		}
		applyRandomOps(t, rng, db, model, batch)
		done += batch
		checkInvariants(t, db)

		if done == 4000 || done == 8000 {
			// Close/reopen cycle: the model must survive intact.
			if err := db.Close(); err != nil {
				t.Fatalf("Close at %d ops: %v", done, err)
			}
			db, err = OpenDB[uint64, [16]byte](cfg, eq, ord)
			if err != nil {
				t.Fatalf("reopen at %d ops: %v", done, err)
			}
			assertModel(t, db, model)
			checkInvariants(t, db)
		} else if err := db.Sync(); err != nil {
			t.Fatalf("Sync at %d ops: %v", done, err)
		}
	}
	assertModel(t, db, model)
	checkInvariants(t, db)
	if err := db.Close(); err != nil {
		t.Fatalf("final Close: %v", err)
	}
}

// -------------------------------------------------------------------------------------------------------
// Read repair.

// TestReadRepairDanglingEntry simulates the crash window a delete
// leaves (index entry present, slot already freed — here forced
// directly) and verifies the reads heal it.
func TestReadRepairDanglingEntry(t *testing.T) {
	db := openTempDB(t, DBConfig{})
	defer db.Close()

	if _, err := db.Put(1, nk("ada"), []byte("data")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Free the slot behind both indexes' backs (a delete interrupted
	// after the free would leave exactly this on disk).
	slotNo, found, err := db.eq.Search(1)
	if err != nil || !found {
		t.Fatalf("Search: (%v, %v)", found, err)
	}
	db.mu.Lock()
	buf := make([]byte, db.slotSize)
	buf[0] = slotFree
	if _, err := db.rec.WriteAt(buf, db.slotOff(slotNo)); err != nil {
		db.mu.Unlock()
		t.Fatalf("scribble: %v", err)
	}
	db.mu.Unlock()

	// GetByID reports not-found and removes the dangling entry.
	if data, found, err := db.GetByID(1); err != nil || found || data != nil {
		t.Fatalf("GetByID after the scribble = (%v, %v, %v), want (nil, false, nil)", data, found, err)
	}
	if _, found, _ := db.GetByID(1); found {
		t.Fatal("the dangling by-ID entry survived the read repair")
	}
	// The name twin heals the same way.
	if _, found, _ := db.GetByName(nk("ada")); found {
		t.Fatal("the dangling by-name entry still resolves")
	}
	if _, found, _ := db.GetByName(nk("ada")); found {
		t.Fatal("the dangling by-name entry survived the read repair")
	}
	if db.Len() != 0 {
		t.Fatalf("Len = %d after both repairs, want 0", db.Len())
	}

	// The database keeps working; the freed slot is reusable.
	if _, err := db.Put(2, nk("grace"), []byte("fresh")); err != nil {
		t.Fatalf("Put after repair: %v", err)
	}
	if data, found, _ := db.GetByID(2); !found || string(data) != "fresh" {
		t.Fatalf("GetByID after repair = (%q, %v)", data, found)
	}
	checkInvariants(t, db)
}

// TestReadRepairMismatchedSlot simulates a stale entry pointing at a
// slot that now holds a DIFFERENT record (cannot happen by
// construction — deletes free last — but the reads must stay correct if
// it ever did): the entry is dropped, the live record is untouched.
func TestReadRepairMismatchedSlot(t *testing.T) {
	db := openTempDB(t, DBConfig{})
	defer db.Close()

	if _, err := db.Put(1, nk("ada"), []byte("old")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := db.Put(2, nk("grace"), []byte("new")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Scribble: point ada's by-name entry at grace's slot.
	graceSlot, found, err := db.ord.Search(nk("grace"))
	if err != nil || !found {
		t.Fatalf("Search grace: (%v, %v)", found, err)
	}
	if _, err := db.ord.Delete(nk("ada")); err != nil {
		t.Fatalf("Delete ada: %v", err)
	}
	if _, err := db.ord.Insert(nk("ada"), graceSlot); err != nil {
		t.Fatalf("Insert ada at grace's slot: %v", err)
	}

	// GetByName(ada) lands on a live slot carrying grace's name: a
	// mismatch, healed by dropping the entry.
	if data, found, err := db.GetByName(nk("ada")); err != nil || found || data != nil {
		t.Fatalf("GetByName(ada) = (%v, %v, %v), want (nil, false, nil)", data, found, err)
	}
	// Grace is untouched through both of her keys.
	if data, found, _ := db.GetByID(2); !found || string(data) != "new" {
		t.Fatalf("grace was disturbed: GetByID(2) = (%q, %v)", data, found)
	}
	if data, found, _ := db.GetByName(nk("grace")); !found || string(data) != "new" {
		t.Fatalf("GetByName(grace) = (%q, %v)", data, found)
	}
	// Re-putting ada works normally.
	if _, err := db.Put(1, nk("ada"), []byte("reborn")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if data, found, _ := db.GetByID(1); !found || string(data) != "reborn" {
		t.Fatalf("GetByID(1) = (%q, %v)", data, found)
	}
	checkInvariants(t, db)
}
