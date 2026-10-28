/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The record-slot layer and the DB operations: Put/Get/Delete through
// either index, ordered RangeByName over the B+ tree, and All through
// the hash index.  Everything here runs under the DB lock; the two
// index stores take their own locks underneath.

package database_disk_ts

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"iter"
)

// -------------------------------------------------------------------------------------------------------
// Record slots.

// slot is one decoded record slot.  id and name alias the read buffer.
type slot struct {
	live  bool
	id    []byte
	name  []byte
	data  []byte
	next  uint64 // free-list next (slot index + 1) of a FREE slot
	valid bool   // the on-disk status byte was interpretable
}

// readSlot reads and decodes slot i.
// Caller holds db.mu.
func (db *DB[I, N]) readSlot(i uint64) (slot, error) {
	buf := make([]byte, db.slotSize)
	if _, err := db.rec.ReadAt(buf, db.slotOff(i)); err != nil {
		return slot{}, fmt.Errorf("database_disk_ts: read slot %d: %w", i, err)
	}
	n := int(binary.LittleEndian.Uint32(buf[1:5]))
	s := slot{
		next:  binary.LittleEndian.Uint64(buf[5:13]),
		valid: buf[0] == slotFree || buf[0] == slotLive,
		live:  buf[0] == slotLive,
	}
	if s.valid && s.live && n >= 0 && n <= db.maxData {
		s.id = buf[slotHeaderSize : slotHeaderSize+db.eqKS]
		s.name = buf[slotHeaderSize+db.eqKS : slotHeaderSize+db.eqKS+db.ordKS]
		s.data = bytes.Clone(buf[slotHeaderSize+db.eqKS+db.ordKS : slotHeaderSize+db.eqKS+db.ordKS+n])
	}
	if s.live && (n < 0 || n > db.maxData) {
		s.valid = false // a live slot with an impossible length is corrupt
	}
	return s, nil
}

// writeSlot lays down a live slot: status, length, the two key
// encodings, the data.  Caller holds db.mu.
func (db *DB[I, N]) writeSlot(i uint64, id, name, data []byte) error {
	buf := make([]byte, db.slotSize)
	buf[0] = slotLive
	binary.LittleEndian.PutUint32(buf[1:5], uint32(len(data)))
	copy(buf[slotHeaderSize:], id)
	copy(buf[slotHeaderSize+db.eqKS:], name)
	copy(buf[slotHeaderSize+db.eqKS+db.ordKS:], data)
	if _, err := db.rec.WriteAt(buf, db.slotOff(i)); err != nil {
		return fmt.Errorf("database_disk_ts: write slot %d: %w", i, err)
	}
	return nil
}

// allocSlot returns a free slot number, popping the free list or
// extending the file; the header is written before returning.  The
// caller follows up with writeSlot.
// Caller holds db.mu.
func (db *DB[I, N]) allocSlot() (uint64, error) {
	if db.freeHead != 0 {
		i := db.freeHead - 1
		if i >= db.numSlots {
			return 0, fmt.Errorf("%w: free-list head %d is outside the file (%d slots)", ErrCorrupt, i+1, db.numSlots)
		}
		s, err := db.readSlot(i)
		if err != nil {
			return 0, err
		}
		db.freeHead = s.next // the popped slot's next, +1-encoded
		return i, nil
	}
	i := db.numSlots
	db.numSlots++
	if err := db.writeHeaderLocked(); err != nil {
		return 0, err
	}
	return i, nil
}

// freeSlot marks slot i free and pushes it onto the free list.  Callers
// delete the index entries FIRST — a delete interrupted by a crash
// leaves a dangling entry (read-repaired on sight), never a live entry
// over a reused slot.
// Caller holds db.mu.
func (db *DB[I, N]) freeSlot(i uint64) error {
	buf := make([]byte, db.slotSize)
	buf[0] = slotFree
	binary.LittleEndian.PutUint64(buf[5:13], db.freeHead)
	if _, err := db.rec.WriteAt(buf, db.slotOff(i)); err != nil {
		return fmt.Errorf("database_disk_ts: free slot %d: %w", i, err)
	}
	db.freeHead = i + 1
	return db.writeHeaderLocked()
}

// -------------------------------------------------------------------------------------------------------
// Put.

// Put stores a record: id → name → data, maintaining both indexes and
// the record slot.  It returns true when an existing ID was replaced.
//
// Names are a unique secondary index: a name already held by a
// DIFFERENT id returns ErrNameConflict (delete that record first).
// Putting an existing id with a NEW, free name renames the record (the
// old name entry is removed); putting an existing id with its own name
// simply rewrites the data.
//
// The change is in the OS page cache when Put returns and is
// crash-durable once Sync or Close returns nil (see the package
// durability contract).  Put on a closed database returns ErrClosed.
// Complexity is O(1) expected index block accesses plus one record
// slot write (the hash index side; the B+ tree side is O(log n)).
func (db *DB[I, N]) Put(id I, name N, data []byte) (bool, error) {
	idBuf := make([]byte, db.eqKS)
	db.eqEncode(id, idBuf)
	nameBuf := make([]byte, db.ordKS)
	db.ordEncode(name, nameBuf)

	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return false, ErrClosed
	}
	if len(data) > db.maxData {
		return false, fmt.Errorf("%w: %d bytes, max is %d", ErrRecordTooLarge, len(data), db.maxData)
	}

	// Resolve both keys, read-repairing dangling entries on sight.
	eqSlot, eqFound, err := db.eq.Search(id)
	if err != nil {
		return false, err
	}
	if eqFound {
		s, err := db.readSlot(eqSlot)
		if err != nil {
			return false, err
		}
		if !s.valid || !s.live || !bytes.Equal(s.id, idBuf) {
			if _, err := db.eq.Delete(id); err != nil { // read repair
				return false, err
			}
			eqFound = false
		}
	}
	ordSlot, ordFound, err := db.ord.Search(name)
	if err != nil {
		return false, err
	}
	if ordFound {
		s, err := db.readSlot(ordSlot)
		if err != nil {
			return false, err
		}
		if !s.valid || !s.live || !bytes.Equal(s.name, nameBuf) {
			if _, err := db.ord.Delete(name); err != nil { // read repair
				return false, err
			}
			ordFound = false
		}
	}

	switch {
	case eqFound && ordFound && eqSlot != ordSlot:
		// The name belongs to a different record.
		return false, fmt.Errorf("%w: %v is held by another record", ErrNameConflict, name)
	case eqFound && ordFound: // same slot: a plain data rewrite
		if err := db.writeSlot(eqSlot, idBuf, nameBuf, data); err != nil {
			return false, err
		}
		return true, nil
	case eqFound: // existing record under a new name: rename + rewrite
		old, err := db.readSlot(eqSlot)
		if err != nil {
			return false, err
		}
		if _, err := db.ord.Delete(db.ordDecode(old.name)); err != nil {
			return false, err
		}
		if err := db.writeSlot(eqSlot, idBuf, nameBuf, data); err != nil {
			return false, err
		}
		if _, err := db.ord.Insert(name, eqSlot); err != nil {
			return false, err
		}
		return true, nil
	default: // fresh record (ordFound here means the name is taken)
		if ordFound {
			return false, fmt.Errorf("%w: %v is held by another record", ErrNameConflict, name)
		}
		i, err := db.allocSlot()
		if err != nil {
			return false, err
		}
		if err := db.writeSlot(i, idBuf, nameBuf, data); err != nil {
			return false, err
		}
		if _, err := db.eq.Insert(id, i); err != nil {
			return false, err
		}
		if _, err := db.ord.Insert(name, i); err != nil {
			return false, err
		}
		return false, nil
	}
}

// -------------------------------------------------------------------------------------------------------
// Point reads.

// GetByID returns the record stored under id.  The lookup is the hash
// index's O(1)-expected equality path.  The slot read verifies that the
// entry still resolves to this id (read repair — see the package
// durability contract).  GetByID on a closed database returns ErrClosed.
// Complexity is O(1) expected index block accesses plus one slot read.
func (db *DB[I, N]) GetByID(id I) ([]byte, bool, error) {
	idBuf := make([]byte, db.eqKS)
	db.eqEncode(id, idBuf)

	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return nil, false, ErrClosed
	}
	i, found, err := db.eq.Search(id)
	if err != nil || !found {
		return nil, false, err
	}
	s, err := db.readSlot(i)
	if err != nil {
		return nil, false, err
	}
	if !s.valid || !s.live || !bytes.Equal(s.id, idBuf) {
		if _, err := db.eq.Delete(id); err != nil { // read repair
			return nil, false, err
		}
		return nil, false, nil
	}
	return s.data, true, nil
}

// GetByName returns the record stored under name.  The lookup rides
// the B+ tree's O(log n) descent (equality this time).  The slot read
// verifies that the entry still resolves to this name.  GetByName on a
// closed database returns ErrClosed.
// Complexity is O(log n) index block accesses plus one slot read.
func (db *DB[I, N]) GetByName(name N) ([]byte, bool, error) {
	nameBuf := make([]byte, db.ordKS)
	db.ordEncode(name, nameBuf)

	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return nil, false, ErrClosed
	}
	i, found, err := db.ord.Search(name)
	if err != nil || !found {
		return nil, false, err
	}
	s, err := db.readSlot(i)
	if err != nil {
		return nil, false, err
	}
	if !s.valid || !s.live || !bytes.Equal(s.name, nameBuf) {
		if _, err := db.ord.Delete(name); err != nil { // read repair
			return nil, false, err
		}
		return nil, false, nil
	}
	return s.data, true, nil
}

// -------------------------------------------------------------------------------------------------------
// Deletes.

// deleteSlot removes both index entries of a live slot and frees it.
// The index deletes run BEFORE the free (a crash in between leaves a
// dangling entry — read-repaired — never a reused slot under a live
// entry).  Caller holds db.mu.
func (db *DB[I, N]) deleteSlot(i uint64, id I, name N) error {
	if _, err := db.eq.Delete(id); err != nil {
		return err
	}
	if _, err := db.ord.Delete(name); err != nil {
		return err
	}
	return db.freeSlot(i)
}

// DeleteByID removes the record stored under id (and its name entry —
// the name comes out of the slot, no scan).  It returns true when a
// record was removed.  DeleteByID on a closed database returns
// ErrClosed.
// Complexity is O(1) expected (hash index) plus O(log n) (tree).
func (db *DB[I, N]) DeleteByID(id I) (bool, error) {
	idBuf := make([]byte, db.eqKS)
	db.eqEncode(id, idBuf)

	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return false, ErrClosed
	}
	i, found, err := db.eq.Search(id)
	if err != nil || !found {
		return false, err
	}
	s, err := db.readSlot(i)
	if err != nil {
		return false, err
	}
	if !s.valid || !s.live || !bytes.Equal(s.id, idBuf) {
		if _, err := db.eq.Delete(id); err != nil { // read repair
			return false, err
		}
		return false, nil
	}
	if err := db.deleteSlot(i, id, db.ordDecode(s.name)); err != nil {
		return false, err
	}
	return true, nil
}

// DeleteByName removes the record stored under name (and its by-ID
// entry — the id comes out of the slot, no scan).  It returns true
// when a record was removed.  DeleteByName on a closed database
// returns ErrClosed.
// Complexity is O(log n) (tree) plus O(1) expected (hash index).
func (db *DB[I, N]) DeleteByName(name N) (bool, error) {
	nameBuf := make([]byte, db.ordKS)
	db.ordEncode(name, nameBuf)

	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return false, ErrClosed
	}
	i, found, err := db.ord.Search(name)
	if err != nil || !found {
		return false, err
	}
	s, err := db.readSlot(i)
	if err != nil {
		return false, err
	}
	if !s.valid || !s.live || !bytes.Equal(s.name, nameBuf) {
		if _, err := db.ord.Delete(name); err != nil { // read repair
			return false, err
		}
		return false, nil
	}
	if err := db.deleteSlot(i, db.eqDecode(s.id), name); err != nil {
		return false, err
	}
	return true, nil
}

// -------------------------------------------------------------------------------------------------------
// Iteration.

type recPair[T any] struct {
	key  T
	data []byte
}

// seq yields a collected snapshot (the All/RangeByName convention).
func recSeq[T any](pairs []recPair[T], err error) iter.Seq2[T, []byte] {
	return func(yield func(T, []byte) bool) {
		if err != nil {
			return
		}
		for _, p := range pairs {
			if !yield(p.key, p.data) {
				return
			}
		}
	}
}

// All iterates every record by ID.  The order is the hash index's
// BUCKET order — deterministic for a given database but NOT ID order;
// collect and sort when an order matters.  The records are a SNAPSHOT:
// All collects them under one hold of the DB lock and the returned
// iterator walks that slice with no lock held.  On a closed database
// (or an IO error) the iterator is empty.
// Complexity is O(n) to build the snapshot, O(1) per yield.
func (db *DB[I, N]) All() iter.Seq2[I, []byte] {
	db.mu.Lock()
	var pairs []recPair[I]
	var err error
	if db.closed {
		err = ErrClosed
	} else {
		for id, i := range db.eq.All() {
			idBuf := make([]byte, db.eqKS)
			db.eqEncode(id, idBuf)
			s, rerr := db.readSlot(i)
			if rerr != nil {
				err = rerr
				break
			}
			if !s.valid || !s.live || !bytes.Equal(s.id, idBuf) {
				continue // a dangling entry; repaired by the point reads
			}
			pairs = append(pairs, recPair[I]{id, s.data})
		}
	}
	db.mu.Unlock()
	return recSeq(pairs, err)
}

// RangeByName iterates the records whose names fall in [lo, hi], in
// ascending name order — the B+ tree's ordered path, the capability
// the hash index cannot supply.  The records are a SNAPSHOT, like All.
// On a closed database (or an IO error) the iterator is empty.
// Complexity is O(log n + k) index block accesses for k results, plus
// one slot read per record.
func (db *DB[I, N]) RangeByName(lo, hi N) iter.Seq2[N, []byte] {
	db.mu.Lock()
	var pairs []recPair[N]
	var err error
	if db.closed {
		err = ErrClosed
	} else {
		for name, i := range db.ord.Range(lo) {
			if db.compare(name, hi) > 0 {
				break // past the upper bound; the tree walks ascending
			}
			nameBuf := make([]byte, db.ordKS)
			db.ordEncode(name, nameBuf)
			s, rerr := db.readSlot(i)
			if rerr != nil {
				err = rerr
				break
			}
			if !s.valid || !s.live || !bytes.Equal(s.name, nameBuf) {
				continue // a dangling entry; repaired by the point reads
			}
			pairs = append(pairs, recPair[N]{name, s.data})
		}
	}
	db.mu.Unlock()
	return recSeq(pairs, err)
}
