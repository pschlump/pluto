/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package database_disk_ts implements a disk-backed, thread-safe record
// database on top of the two pluto disk indexes: a hash index for
// by-ID equality lookups (hash_index_disk_ts — one directory block and
// one bucket block, expected) and a B+ tree for by-name ordered
// lookups and range scans (b_tree_disk_ts).  It is the "add a
// database" line of note/database.md: storage for blocks of data, with
// lookup based on those two indexes.
//
// A database is three files under one Path:
//
//	Path.rec  — the record file: fixed-size SLOTS holding the data
//	Path.eq   — the hash-index store (by ID, equality)
//	Path.ord  — the B+ tree store (by name, ordered)
//
// (plus the two stores' ".journal" files).  Both indexes map their key
// to a SLOT NUMBER, and every live slot is SELF-DESCRIBING — it carries
// the encoded ID, the encoded name, then the record bytes:
//
//	[status 1B][len 4B][next 8B][id eqKeySize][name ordKeySize][data len]
//
// The embedded keys are what make cross-index deletes possible without
// a scan: DeleteByID resolves the slot through the hash index, reads
// the name out of the slot, and removes the tree entry too.  They also
// make reads self-verifying (below).
//
// # Keys
//
// The DB is generic over the ID type I and the name type N, fixed at
// OpenDB:
//
//	I needs an injective EncodeKey (the hash-index contract); lookups
//	  by ID are pure equality.
//	N needs an order-preserving EncodeKey that agrees with Compare (the
//	  B+ tree contract); lookups by name get equality AND ordered
//	  range scans.
//
// Both DecodeKey functions must be true inverses of their EncodeKey on
// stored keys (DecodeKey(EncodeKey(k)) == k) — the B+ tree's iteration
// already yields decoded keys, and the DB round-trips them to verify
// slot contents and to drive cross-index deletes.
//
// IDs and names are both UNIQUE: an ID identifies one record (the
// primary key, so Len counts IDs), and a name belongs to at most one
// record — putting a name that is taken by a DIFFERENT ID returns
// ErrNameConflict (delete the other record first).  Renaming a record
// (same ID, new free name) is a normal Put.
//
// # Durability contract
//
// Puts and deletes are applied to the OS page cache when they return
// and become crash-durable as a unit once Sync (or Close) returns nil:
// the record file is fsynced FIRST, then the two index stores, so an
// index entry is never durable while the data it points at is not.
// The two index stores bring their own journals and background
// flushers (see their packages); the record file has no cache of its
// own — reads and writes go straight through the OS page cache.
//
// An operation interrupted by a crash may be PARTIALLY visible: a
// record whose slot was written but whose index entries were not (an
// orphaned slot — leaked until reused... it is NOT leaked: it has no
// index entry, so nothing points at it; it is simply unreachable
// garbage that a later allocation may overwrite only via the free
// list, which it is not on — so it is leaked, deliberately, the honest
// cost of no cross-file atomic commit; a future compact would reclaim
// it), or an index entry whose slot was freed by a concurrent...  no:
// deletes remove the index entries BEFORE freeing the slot, so a
// delete interrupted by a crash leaves at worst a DANGLING entry,
// never a reused slot under a live entry.  Reads are self-verifying:
// GetByID/GetByName check that the slot is live and carries the key
// they looked it up by; a dangling entry is deleted on sight (read
// repair) and reported as not-found.  The other index's stale twin,
// if any, is repaired the same way when it is next read.
//
// # Concurrency
//
// Safe for concurrent use.  One DB mutex serializes compound
// operations (resolve-then-read, put's three files) and the record
// file; the two stores take their own locks underneath — the DB lock
// is always acquired FIRST and nothing under it calls back into the
// DB, so there is no nesting hazard.  Reads take the same lock as
// writes: a read resolves an index entry and then reads the slot, and
// the lock is what keeps that pair consistent.  Iterators are
// SNAPSHOTS: All and RangeByName collect their records under one hold
// of the lock and walk the collected slice with no lock held.
//
// # Panics
//
// Panics are programmer errors caught at construction, and they are
// the two underlying packages' (a nil EncodeKey/DecodeKey/Compare, a
// KeySize ≤ 0, an empty or over-long index name — see
// hash_index_disk_ts and b_tree_disk_ts).  Everything else is an
// error: ErrClosed after Close, ErrRecordTooLarge when data exceeds
// MaxData, ErrNameConflict, and ErrCorrupt for a record file this
// package cannot interpret.
package database_disk_ts

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/pschlump/pluto/b_tree_disk_ts"
	"github.com/pschlump/pluto/hash_index_disk_ts"
)

// DefaultRecordSize is the record slot size used when
// DBConfig.RecordSize is 0.  A slot holds MaxData() record bytes —
// DefaultRecordSize minus 13 bytes of header minus the two key
// encodings.
const DefaultRecordSize = 4096

// recHeaderSize is the record file's header block; slots start after it.
const recHeaderSize = 512

// Slot header layout: status(1) + length(4) + next(8), then the id and
// name encodings, then the data.
const slotHeaderSize = 13

// Slot status bytes.
const (
	slotFree byte = 0
	slotLive byte = 1
)

// Sentinel errors returned by this package; check them with errors.Is.
var (
	// ErrClosed is returned by every DB operation after Close.
	ErrClosed = errors.New("database_disk_ts: database is closed")
	// ErrRecordTooLarge is returned by Put when the data exceeds
	// MaxData — a record must fit in one slot.
	ErrRecordTooLarge = errors.New("database_disk_ts: record too large for the slot size (see MaxData)")
	// ErrNameConflict is returned by Put when the name is already taken
	// by a different ID; names are a unique secondary index.
	ErrNameConflict = errors.New("database_disk_ts: name is already used by another record")
	// ErrCorrupt is returned when the record file cannot be interpreted.
	ErrCorrupt = errors.New("database_disk_ts: record file is corrupt")
)

// DBConfig configures OpenDB.  BlockSize/CacheBlocks/FlushInterval
// configure BOTH index stores (the hash index and the B+ tree share
// the settings); RecordSize is the record slot size.
type DBConfig struct {
	Path           string        // database path prefix; .rec/.eq/.ord/.journal files appear beside it
	RecordSize     int           // 0 => DefaultRecordSize; must hold 13+eqKeySize+ordKeySize+1 bytes
	IndexBlockSize int           // 0 => each store's default (4096)
	CacheBlocks    int           // 0 => each store's default
	FlushInterval  time.Duration // 0 => each store's default
}

// DB is a record database with a by-ID equality index (hash index) and
// a by-name ordered index (B+ tree).  Create or open with OpenDB; the
// generic parameters fix the ID type (equality) and the name type
// (ordered).  All methods are safe for concurrent use; see the package
// documentation for the durability and locking contracts.
type DB[I, N any] struct {
	mu   sync.Mutex // guards everything below and serializes compound operations
	path string

	rec *os.File // the record file

	eqStore  *hash_index_disk_ts.Store
	ordStore *b_tree_disk_ts.Store
	eq       *hash_index_disk_ts.Index[I]
	ord      *b_tree_disk_ts.Tree[N]

	slotSize int
	eqKS     int
	ordKS    int
	maxData  int

	numSlots uint64 // slots in the record file
	freeHead uint64 // free-list head, slot index + 1; 0 = none

	eqName  string // the two index names, re-written into the header on
	ordName string // every header update (the header is one unit)

	eqEncode  func(I, []byte)
	eqDecode  func([]byte) I
	ordEncode func(N, []byte)
	ordDecode func([]byte) N
	compare   func(a, b N) int // the DB keeps its own copy (the priority_queue precedent)

	closed bool
}

// Record-file header layout (offsets in the 512-byte header block).
const (
	recMagic    = 0  // uint64 "pluto-db"
	recVersion  = 8  // uint32
	recSlotSize = 12 // uint32
	recEqKS     = 16 // uint32
	recOrdKS    = 20 // uint32
	recNumSlots = 24 // uint64
	recFreeHead = 32 // uint64, slot index + 1, 0 = none
	recEqName   = 40 // 32 bytes, NUL-padded
	recOrdName  = 72 // 32 bytes, NUL-padded
)

const dbMagic uint64 = 0x706c75746f2d6462 // "pluto-db"
const dbVersion uint32 = 1

// OpenDB creates the database at cfg.Path (three files: .rec, .eq,
// .ord) or opens the existing one; eq and ord name the two indexes
// inside their stores and their key encodings fix the slot layout.
// Reopening attaches to the existing indexes (key sizes and index
// names must match, else ErrCorrupt).
//
// Program errors panic as the two underlying packages do (nil
// EncodeKey/DecodeKey/Compare, KeySize ≤ 0, empty or over-long index
// names); a RecordSize too small for the two key encodings returns
// ErrRecordTooLarge.
// Complexity is O(1) plus each store's open (journal recovery).
func OpenDB[I, N any](cfg DBConfig, eq hash_index_disk_ts.IndexConfig[I], ord b_tree_disk_ts.TreeConfig[N]) (*DB[I, N], error) {
	if cfg.Path == "" {
		return nil, errors.New("database_disk_ts: OpenDB requires a path")
	}
	slotSize := cfg.RecordSize
	if slotSize == 0 {
		slotSize = DefaultRecordSize
	}
	minSlot := slotHeaderSize + eq.KeySize + ord.KeySize + 1
	if slotSize < 64 || slotSize < minSlot {
		return nil, fmt.Errorf("%w: record size %d cannot hold id (%d bytes) + name (%d bytes) + data; need at least %d",
			ErrRecordTooLarge, slotSize, eq.KeySize, ord.KeySize, minSlot)
	}

	// The two index stores first (their constructors run the panic
	// checks on the caller's functions); a failure to open either one
	// closes what did open.
	eqStore, err := hash_index_disk_ts.OpenStore(hash_index_disk_ts.StoreConfig{
		Path:          cfg.Path + ".eq",
		BlockSize:     cfg.IndexBlockSize,
		CacheBlocks:   cfg.CacheBlocks,
		FlushInterval: cfg.FlushInterval,
	})
	if err != nil {
		return nil, err
	}
	ordStore, err := b_tree_disk_ts.OpenStore(b_tree_disk_ts.StoreConfig{
		Path:          cfg.Path + ".ord",
		BlockSize:     cfg.IndexBlockSize,
		CacheBlocks:   cfg.CacheBlocks,
		FlushInterval: cfg.FlushInterval,
	})
	if err != nil {
		_ = eqStore.Close()
		return nil, err
	}

	db := &DB[I, N]{
		path:     cfg.Path,
		slotSize: slotSize,
		eqKS:     eq.KeySize,
		ordKS:    ord.KeySize,
		maxData:  slotSize - slotHeaderSize - eq.KeySize - ord.KeySize,
		eqName:   eq.Name,
		ordName:  ord.Name,
	}

	rec, numSlots, freeHead, err := openRecFile(cfg.Path+".rec", slotSize, eq.KeySize, ord.KeySize, eq.Name, ord.Name)
	if err != nil {
		_ = eqStore.Close()
		_ = ordStore.Close()
		return nil, err
	}
	db.rec = rec
	db.numSlots = numSlots
	db.freeHead = freeHead

	eqIndex, err := hash_index_disk_ts.NewIndex(eqStore, eq)
	if err != nil {
		_ = rec.Close()
		_ = eqStore.Close()
		_ = ordStore.Close()
		return nil, err
	}
	ordTree, err := b_tree_disk_ts.NewTree(ordStore, ord)
	if err != nil {
		_ = rec.Close()
		_ = eqStore.Close()
		_ = ordStore.Close()
		return nil, err
	}
	db.eqStore = eqStore
	db.ordStore = ordStore
	db.eq = eqIndex
	db.ord = ordTree
	db.eqEncode = eq.EncodeKey
	db.eqDecode = eq.DecodeKey
	db.ordEncode = ord.EncodeKey
	db.ordDecode = ord.DecodeKey
	db.compare = ord.Compare
	return db, nil
}

// openRecFile opens (creating on demand) the record file and returns
// the handle with the numSlots/freeHead mirrors from its header.  On
// creation it writes the header with the given layout; on reopen it
// validates the layout, the key sizes and the two index names — a
// mismatch is ErrCorrupt (this database's record file belongs to
// exactly one pair of indexes).
func openRecFile(path string, slotSize, eqKS, ordKS int, eqName, ordName string) (*os.File, uint64, uint64, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("database_disk_ts: open %s: %w", path, err)
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, 0, fmt.Errorf("database_disk_ts: stat %s: %w", path, err)
	}

	hdr := make([]byte, recHeaderSize)
	binary.LittleEndian.PutUint64(hdr[recMagic:], dbMagic)
	binary.LittleEndian.PutUint32(hdr[recVersion:], dbVersion)
	binary.LittleEndian.PutUint32(hdr[recSlotSize:], uint32(slotSize))
	binary.LittleEndian.PutUint32(hdr[recEqKS:], uint32(eqKS))
	binary.LittleEndian.PutUint32(hdr[recOrdKS:], uint32(ordKS))
	copy(hdr[recEqName:], eqName)
	copy(hdr[recOrdName:], ordName)

	if st.Size() == 0 { // fresh file: lay down the header
		if _, err := f.WriteAt(hdr, 0); err != nil {
			_ = f.Close()
			return nil, 0, 0, fmt.Errorf("database_disk_ts: write record-file header: %w", err)
		}
		return f, 0, 0, nil
	}

	if st.Size() < recHeaderSize {
		_ = f.Close()
		return nil, 0, 0, fmt.Errorf("%w: %s is too short to be a record file (%d bytes)", ErrCorrupt, path, st.Size())
	}
	got := make([]byte, recHeaderSize)
	if _, err := f.ReadAt(got, 0); err != nil {
		_ = f.Close()
		return nil, 0, 0, fmt.Errorf("database_disk_ts: read record-file header: %w", err)
	}
	bad := func(what string, gotVal, wantVal any) error {
		_ = f.Close()
		return fmt.Errorf("%w: record file %s is %v, this database uses %v", ErrCorrupt, what, gotVal, wantVal)
	}
	if binary.LittleEndian.Uint64(got[recMagic:]) != dbMagic {
		_ = f.Close()
		return nil, 0, 0, fmt.Errorf("%w: %s is not a database_disk_ts record file (bad magic)", ErrCorrupt, path)
	}
	if v := binary.LittleEndian.Uint32(got[recVersion:]); v != dbVersion {
		_ = f.Close()
		return nil, 0, 0, fmt.Errorf("%w: record file has format version %d, this build understands %d", ErrCorrupt, v, dbVersion)
	}
	if v := int(binary.LittleEndian.Uint32(got[recSlotSize:])); v != slotSize {
		return nil, 0, 0, bad("slot size", v, slotSize)
	}
	if v := int(binary.LittleEndian.Uint32(got[recEqKS:])); v != eqKS {
		return nil, 0, 0, bad("by-ID key size", v, eqKS)
	}
	if v := int(binary.LittleEndian.Uint32(got[recOrdKS:])); v != ordKS {
		return nil, 0, 0, bad("by-name key size", v, ordKS)
	}
	name := func(off int) string {
		nb := got[off : off+32]
		for z, c := range nb {
			if c == 0 {
				nb = nb[:z]
				break
			}
		}
		return string(nb)
	}
	if n := name(recEqName); n != eqName {
		return nil, 0, 0, bad("by-ID index name", n, eqName)
	}
	if n := name(recOrdName); n != ordName {
		return nil, 0, 0, bad("by-name index name", n, ordName)
	}

	numSlots := binary.LittleEndian.Uint64(got[recNumSlots:])
	freeHead := binary.LittleEndian.Uint64(got[recFreeHead:])
	// The header may lag the file size by a crash between a slot append
	// and the header write: slots the header does not know about are
	// free (nothing durable ever pointed at them).
	if sizeSlots := (uint64(st.Size()) - recHeaderSize) / uint64(slotSize); sizeSlots > numSlots {
		numSlots = sizeSlots
	}
	if numSlots > 0 && freeHead > numSlots {
		_ = f.Close()
		return nil, 0, 0, fmt.Errorf("%w: record file free-list head %d is outside the file (%d slots)", ErrCorrupt, freeHead, numSlots)
	}
	return f, numSlots, freeHead, nil
}

// writeHeaderLocked flushes the in-memory numSlots/freeHead mirrors
// into the record-file header.  Caller holds db.mu.
func (db *DB[I, N]) writeHeaderLocked() error {
	hdr := make([]byte, recHeaderSize)
	binary.LittleEndian.PutUint64(hdr[recMagic:], dbMagic)
	binary.LittleEndian.PutUint32(hdr[recVersion:], dbVersion)
	binary.LittleEndian.PutUint32(hdr[recSlotSize:], uint32(db.slotSize))
	binary.LittleEndian.PutUint32(hdr[recEqKS:], uint32(db.eqKS))
	binary.LittleEndian.PutUint32(hdr[recOrdKS:], uint32(db.ordKS))
	binary.LittleEndian.PutUint64(hdr[recNumSlots:], db.numSlots)
	binary.LittleEndian.PutUint64(hdr[recFreeHead:], db.freeHead)
	copy(hdr[recEqName:], db.eqName)
	copy(hdr[recOrdName:], db.ordName)
	if _, err := db.rec.WriteAt(hdr, 0); err != nil {
		return fmt.Errorf("database_disk_ts: write record-file header: %w", err)
	}
	return nil
}

// slotOff returns the file offset of slot i.
func (db *DB[I, N]) slotOff(i uint64) int64 {
	return int64(recHeaderSize) + int64(i)*int64(db.slotSize)
}

// MaxData returns the largest record Put accepts: the slot size minus
// 13 header bytes minus the id and name encodings.
func (db *DB[I, N]) MaxData() int {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.maxData
}

// Len returns the number of records — the length of the by-ID index
// (the ID is the primary key).  On a closed database it returns 0.
// Complexity is O(1).
func (db *DB[I, N]) Len() uint64 {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return 0
	}
	return db.eq.Length()
}

// Sync makes every change that returned before Sync crash-durable:
// the record file is fsynced first, then both index stores flush (each
// with its own journal discipline), so an index entry is never durable
// while the record it points at is not.  Sync returns ErrClosed on a
// closed database.
// Complexity is O(dirty index blocks) plus two fsyncs.
func (db *DB[I, N]) Sync() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return ErrClosed
	}
	return db.syncNoLock()
}

// syncNoLock is Sync without locking.  Caller holds db.mu.
func (db *DB[I, N]) syncNoLock() error {
	if err := db.rec.Sync(); err != nil {
		return fmt.Errorf("database_disk_ts: fsync record file: %w", err)
	}
	if err := db.eqStore.Sync(); err != nil {
		return err
	}
	return db.ordStore.Sync()
}

// Close syncs and closes the record file and both index stores.  Every
// change that returned before Close is crash-durable once Close
// returns nil.  Later operations return ErrClosed; Close itself
// returns ErrClosed when called twice.
// Complexity is O(dirty index blocks) plus two fsyncs.
func (db *DB[I, N]) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return ErrClosed
	}
	db.closed = true

	err := db.syncNoLock()
	if cerr := db.rec.Close(); err == nil {
		err = cerr
	}
	if cerr := db.eqStore.Close(); err == nil {
		err = cerr
	}
	if cerr := db.ordStore.Close(); err == nil {
		err = cerr
	}
	return err
}
