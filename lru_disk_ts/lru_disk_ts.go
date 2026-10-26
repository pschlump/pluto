/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package lru_disk_ts — a two-tier, capacity-bounded LRU cache: an
// optional in-memory tier (the fast path) in front of the disk tier
// (one file per entry under a caller-given directory), behind one
// sync.RWMutex.
//
// The lru_ts-shaped public API (Get/Peek/Put/Delete/PopOldest, the
// Lock/Unlock + Nl* compound forms), keyed by string and
// persistence-backed — entries survive the process (the cache
// reindexes its directory on open, recency from file mtime, evicting
// to capacity as it goes), disk evictions and deletes remove the
// files, and writes are atomic (temp file + rename), so a crash
// mid-write leaves the previous entry intact or nothing, never a torn
// file. Values pass through caller-supplied codecs: encode turns a V
// into the file payload, decode turns the payload back; the key travels
// in the file header, which is what makes restart recovery possible.
//
// # The two tiers
//
// The disk tier holds every live entry, bounded by the constructor's
// capacity (in entries); a disk eviction deletes the file — the entry
// is gone. The memory tier is a cache FOR the disk cache: it holds the
// decoded values of the most recently used entries, bounded by
// WithMemCapacity (also in entries; bytes are entries × value size for
// the caller to budget). A Get that misses the memory tier reads and
// decodes the file and promotes the value into it; when the memory
// tier is full the least-recently-used RESIDENT value is demoted —
// removed from memory but its file stays, so the next Get of it is
// still a hit, just a slower one. Only disk-tier eviction loses data.
//
// The memory tier is OFF by default (mem capacity 0): NewLruDisk
// behaves exactly as it always has — every Get reads and decodes the
// file, no decoded value is retained — which keeps the constructor
// signature and the default contract unchanged. Pass WithMemCapacity(n)
// to turn the tier on.
//
// # Errors
//
// I/O can fail where lru_ts cannot. Put and Delete return the error;
// Get treats an unreadable or undecodable entry as a miss and removes
// it (self-healing — a torn or foreign file cannot poison the key;
// this runs on the disk-read path, so a value still resident in the
// memory tier is served without touching the file). NewLruDisk returns
// an error instead of panicking when the directory cannot be made or
// scanned.
//
// # Locks
//
// Which method takes which lock mirrors lru_ts: Get is a WRITE-lock
// (recency, in both tiers), Peek/Len/Capacity/MemLen/MemCapacity are
// read-lock, Put/Delete/Clear are write. File I/O happens under the
// lock — this is a local-disk cache, not a network store.
//
// Run the tests with -race.
package lru_disk_ts

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/pschlump/pluto/lru"
)

// entryFileSuffix names this package's files inside dir so a shared
// cache directory is safe: foreign files are ignored; files with this
// suffix and an unreadable header are crashed writes and are removed.
const entryFileSuffix = ".ldc"

// entry is one live entry's index record: the file that holds its
// payload. The decoded value is deliberately NOT here — values live in
// the memory tier when it is enabled, and nowhere otherwise (every Get
// decodes fresh), so a disk-only entry costs one path string of RAM.
type entry struct {
	path string
}

// Option configures the optional memory tier; the constructors take
// any number, applied in order.
type Option func(*options)

// options is the collected Option state.
type options struct {
	memCapacity int
}

// WithMemCapacity enables the in-memory tier, bounding it to n
// entries: the decoded values of the n most recently used entries are
// held in RAM and demotion (memory full) keeps the file on disk. n
// less than 1 leaves the tier disabled — the default, the historic
// pure-disk behavior. A memory capacity at or above the disk capacity
// is legal (every entry ends up resident after its first access).
func WithMemCapacity(n int) Option {
	return func(o *options) {
		if n < 1 {
			n = 0
		}
		o.memCapacity = n
	}
}

// LruDisk is a capacity-bounded, thread-safe two-tier LRU cache with
// its entries stored in files under dir. Create it with NewLruDisk;
// the zero value reads as an empty cache but cannot be Put into. Do
// not copy a LruDisk (the mutex must not be duplicated) — always use
// *LruDisk.
type LruDisk[V any] struct {
	inner *lru.Lru[string, entry] // the disk tier's index: every live entry, true recency order
	mem   *lru.Lru[string, V]     // the memory tier: decoded values of the most recently used subset; nil = disabled
	lock  sync.RWMutex
	dir   string
	enc   func(V) []byte
	dec   func([]byte) (V, error)
}

// NewLruDisk creates a cache holding up to capacity entries as files
// under dir (created if needed). Files written by a previous run of
// the same cache are reindexed — recency from file mtime, oldest
// evicted first back down to capacity — so the cache persists across
// restarts (the memory tier starts empty; values re-promote lazily on
// their first Get). encode/decode carry values to and from the file
// payload (nil encode is an error; a nil decode treats every read as
// a miss). opts enable the in-memory tier — see WithMemCapacity;
// without it the cache is pure disk, the historic behavior.
// It panics if capacity < 1 and returns an error if dir cannot be made
// or its existing entries cannot be scanned.
func NewLruDisk[V any](capacity int, dir string, encode func(V) []byte, decode func([]byte) (V, error), opts ...Option) (*LruDisk[V], error) {
	if capacity < 1 {
		panic("lru_disk_ts: NewLruDisk called with capacity < 1: a cache needs room for at least one entry")
	}
	if encode == nil {
		return nil, errors.New("lru_disk_ts: NewLruDisk called with a nil encode")
	}
	cfg := options{}
	for _, o := range opts {
		o(&cfg)
	}
	c := &LruDisk[V]{
		inner: lru.NewLru[string, entry](capacity),
		dir:   dir,
		enc:   encode,
		dec:   decode,
	}
	if cfg.memCapacity > 0 {
		c.mem = lru.NewLru[string, V](cfg.memCapacity)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("lru_disk_ts: create %s: %w", dir, err)
	}
	if err := c.reindex(capacity); err != nil {
		return nil, err
	}
	return c, nil
}

// found is one reindexed file: its recovered key, path, and mtime.
type found struct {
	key  string
	path string
	mod  int64
}

// reindex rebuilds the in-memory index from the files on disk, oldest
// mtime first, evicting (deleting files) back down to capacity.
func (c *LruDisk[V]) reindex(capacity int) error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return fmt.Errorf("lru_disk_ts: scan %s: %w", c.dir, err)
	}
	var files []found
	for _, de := range entries {
		if de.IsDir() || filepath.Ext(de.Name()) != entryFileSuffix {
			continue
		}
		path := filepath.Join(c.dir, de.Name())
		key, kerr := readKey(path)
		if kerr != nil {
			// a *.ldc file with an unreadable header is a crashed
			// write — this package owns the suffix, so remove it
			// (self-healing; torn files cannot accumulate).
			_ = os.Remove(path)
			continue
		}
		info, ierr := de.Info()
		if ierr != nil {
			continue
		}
		files = append(files, found{key: key, path: path, mod: info.ModTime().UnixNano()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod < files[j].mod })
	// oldest mtime first: the entries beyond the newest `capacity` files
	// (the surplus at the front) are dropped and deleted; the rest are
	// indexed, so the restart keeps the most recently used entries.
	drop := len(files) - capacity
	if drop < 0 {
		drop = 0
	}
	for i, f := range files {
		if i < drop {
			if err := os.Remove(f.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("lru_disk_ts: evict %s: %w", f.path, err)
			}
			continue
		}
		c.inner.Put(f.key, entry{path: f.path})
	}
	return nil
}

// fileFor returns the fixed file path for key (sha256 of the key bytes,
// so any string key maps to a safe filename).
func fileFor(dir, key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(dir, hex.EncodeToString(sum[:])+entryFileSuffix)
}

// writeEntry atomically writes key/value: temp file in the same
// directory, rename over the target, so readers never see a torn file.
func (c *LruDisk[V]) writeEntry(key string, value V) error {
	payload := c.enc(value)
	header := make([]byte, 4, 4+len(key)+len(payload))
	binary.LittleEndian.PutUint32(header, uint32(len(key)))
	header = append(header, key...)
	tmp, err := os.CreateTemp(c.dir, "tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	rm := true
	defer func() {
		if rm {
			_ = os.Remove(name)
		}
	}()
	if _, err = tmp.Write(header); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err = tmp.Write(payload); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, fileFor(c.dir, key)); err != nil {
		return err
	}
	rm = false
	return nil
}

// readEntry reads and decodes the payload at path.
func (c *LruDisk[V]) readEntry(path string) (V, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		var zero V
		return zero, err
	}
	if len(raw) < 4 {
		var zero V
		return zero, errors.New("lru_disk_ts: short file")
	}
	kl := int(binary.LittleEndian.Uint32(raw))
	if kl < 0 || 4+kl > len(raw) {
		var zero V
		return zero, errors.New("lru_disk_ts: bad key length")
	}
	return c.dec(raw[4+kl:])
}

// readKey reads just the key from an entry file (the reindex path).
func readKey(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(raw) < 4 {
		return "", errors.New("lru_disk_ts: short file")
	}
	kl := int(binary.LittleEndian.Uint32(raw))
	if kl < 0 || 4+kl > len(raw) || kl == 0 {
		return "", errors.New("lru_disk_ts: bad key length")
	}
	return string(raw[4 : 4+kl]), nil
}

// Get returns the value stored under key and whether it was found. A
// memory-tier hit serves the decoded value with no file I/O; otherwise
// the file is read and decoded, and — when the tier is enabled — the
// value is promoted into it (demoting the least-recently-used resident
// value, whose file stays). Either way the entry is marked
// most-recently-used, a mutation, so Get takes the WRITE lock, not the
// read lock (Peek is the read-lock form). An unreadable or
// undecodable entry is a miss and is deleted (file included).
// A nil or zero-value cache reports not-found.
// Complexity is O(1) plus at most one file read+decode when the value
// is not memory-resident.
func (c *LruDisk[V]) Get(key string) (V, bool) {
	if c == nil || c.inner == nil {
		var zero V
		return zero, false
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.nlGet(key)
}

// nlGet is Get without the lock.
func (c *LruDisk[V]) nlGet(key string) (V, bool) {
	if c.mem != nil {
		if v, ok := c.mem.Get(key); ok {
			// keep the disk tier's order equal to the true recency, so
			// a disk eviction still picks the globally coldest entry
			c.inner.Get(key)
			return v, true
		}
	}
	e, ok := c.inner.Get(key)
	if !ok {
		var zero V
		return zero, false
	}
	v, err := c.readEntry(e.path)
	if err != nil {
		c.inner.Delete(key)
		_ = os.Remove(e.path)
		var zero V
		return zero, false
	}
	if c.mem != nil {
		// promote; the demoted victim (if any) stays on disk
		c.mem.Put(key, v)
	}
	return v, true
}

// Peek returns the value stored under key and whether it was found,
// WITHOUT changing the recency order — a true read under the read
// lock. A memory-tier hit serves the resident value; otherwise the
// payload is decoded fresh from the file. A Peek never promotes into
// the memory tier and never caches a decode.
// A nil or zero-value cache reports not-found.
// Complexity is O(1) plus one file read+decode when the value is not
// memory-resident.
func (c *LruDisk[V]) Peek(key string) (V, bool) {
	if c == nil || c.inner == nil {
		var zero V
		return zero, false
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.nlPeek(key)
}

// nlPeek is Peek without the lock.
func (c *LruDisk[V]) nlPeek(key string) (V, bool) {
	if c.mem != nil {
		if v, ok := c.mem.Peek(key); ok {
			return v, true
		}
	}
	e, ok := c.inner.Peek(key)
	if !ok {
		var zero V
		return zero, false
	}
	v, err := c.readEntry(e.path)
	if err != nil {
		var zero V
		return zero, false
	}
	return v, true
}

// Put stores value under key, writing its file atomically, and returns
// the write error if the file cannot be written (the in-memory state
// is still updated — without the memory tier a later Get of it will
// miss-and-self-heal; with it the resident value is served until it
// demotes). When the disk tier is at capacity and the key is NEW, the
// globally least-recently-used entry is evicted and its file deleted
// (an overwrite replaces in place and never evicts, like lru.Put);
// the eviction file error is NOT swallowed if the Put itself
// succeeded — Put returns the first error. With the memory tier
// enabled the new value is also resident (demoting a victim whose
// file stays).
// Complexity is O(1) plus the file write.
func (c *LruDisk[V]) Put(key string, value V) error {
	if c == nil || c.inner == nil {
		return errors.New("lru_disk_ts: Put on a zero-value cache")
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.nlPut(key, value)
}

// nlPut is Put without the lock.
func (c *LruDisk[V]) nlPut(key string, value V) error {
	var err error
	// make room exactly like lru.Put: only a NEW key needs space — an
	// overwrite replaces in place and never evicts
	if _, exists := c.inner.Peek(key); !exists && c.inner.Len() >= c.inner.Capacity() {
		if victim, e, ok := c.inner.PopOldest(); ok {
			// the victim cannot be key (key does not exist yet); it can
			// still be memory-resident when the tiers overlap fully
			// (mem capacity >= disk capacity)
			c.mem.Delete(victim)
			if rerr := os.Remove(e.path); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
				err = rerr
			}
		}
	}
	c.inner.Put(key, entry{path: fileFor(c.dir, key)})
	if c.mem != nil {
		c.mem.Put(key, value)
	}
	if werr := c.writeEntry(key, value); werr != nil && err == nil {
		err = werr
	}
	return err
}

// Delete removes key and its file, reporting whether the key was found.
// Complexity is O(1) plus the file remove.
func (c *LruDisk[V]) Delete(key string) bool {
	if c == nil || c.inner == nil {
		return false
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.nlDelete(key)
}

// nlDelete is Delete without the lock.
func (c *LruDisk[V]) nlDelete(key string) bool {
	e, ok := c.inner.Peek(key)
	if !ok {
		return false
	}
	c.inner.Delete(key)
	c.mem.Delete(key)
	_ = os.Remove(e.path)
	return true
}

// PopOldest removes and returns the globally least-recently-used
// entry's key and value — from the memory tier when it is resident,
// else decoded from its file BEFORE the file is removed (a decode
// failure yields the zero V with ok=true — the entry is gone either
// way).
// Complexity is O(1) plus at most one file read+decode.
func (c *LruDisk[V]) PopOldest() (string, V, bool) {
	if c == nil || c.inner == nil {
		var zero V
		return "", zero, false
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.nlPopOldest()
}

// nlPopOldest is PopOldest without the lock.
func (c *LruDisk[V]) nlPopOldest() (string, V, bool) {
	key, e, ok := c.inner.PopOldest()
	if !ok {
		var zero V
		return "", zero, false
	}
	var v V
	haveVal := false
	if c.mem != nil {
		if mv, mok := c.mem.Peek(key); mok {
			v, haveVal = mv, true
		}
		c.mem.Delete(key)
	}
	if !haveVal {
		// decode BEFORE the remove — the file is the only copy left
		if dv, derr := c.readEntry(e.path); derr == nil {
			v = dv
		}
	}
	_ = os.Remove(e.path)
	return key, v, true
}

// Len returns the number of entries (in both tiers together — a
// memory-resident entry is one entry, not two). A nil cache holds none.
// Complexity is O(1).
func (c *LruDisk[V]) Len() int {
	if c == nil || c.inner == nil {
		return 0
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.inner.Len()
}

// Capacity returns the maximum number of entries (the disk-tier
// capacity). Complexity is O(1).
func (c *LruDisk[V]) Capacity() int {
	if c == nil || c.inner == nil {
		return 0
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.inner.Capacity()
}

// MemLen returns the number of decoded values currently held in the
// memory tier — 0 when the tier is disabled (the default).
// Complexity is O(1).
func (c *LruDisk[V]) MemLen() int {
	if c == nil || c.mem == nil {
		return 0
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.mem.Len()
}

// MemCapacity returns the memory tier's entry bound — 0 when the tier
// is disabled (the default). Complexity is O(1).
func (c *LruDisk[V]) MemCapacity() int {
	if c == nil || c.mem == nil {
		return 0
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.mem.Capacity()
}

// Clear removes every entry and its file, and empties the memory tier.
func (c *LruDisk[V]) Clear() error {
	if c == nil || c.inner == nil {
		return nil
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	var err error
	for {
		_, e, ok := c.inner.PopOldest()
		if !ok {
			break
		}
		if rerr := os.Remove(e.path); rerr != nil && !errors.Is(rerr, os.ErrNotExist) && err == nil {
			err = rerr
		}
	}
	c.mem.Clear()
	return err
}

// Lock locks the cache for a compound operation. Inside it, use the
// Nl* forms — calling a regular method while the lock is held
// deadlocks. Nil receivers no-op, matching lru_ts.
func (c *LruDisk[V]) Lock() {
	if c == nil {
		return
	}
	c.lock.Lock()
}

// Unlock releases the compound-operation lock.
func (c *LruDisk[V]) Unlock() {
	if c == nil {
		return
	}
	c.lock.Unlock()
}

// NlGet is Get without the lock (compound operations only).
func (c *LruDisk[V]) NlGet(key string) (V, bool) { return c.nlGet(key) }

// NlPeek is Peek without the lock (compound operations only); it never
// promotes or caches a decode, so it holds no state beyond the call.
func (c *LruDisk[V]) NlPeek(key string) (V, bool) { return c.nlPeek(key) }

// NlPut is Put without the lock (compound operations only).
func (c *LruDisk[V]) NlPut(key string, value V) error { return c.nlPut(key, value) }

// NlDelete is Delete without the lock (compound operations only).
func (c *LruDisk[V]) NlDelete(key string) bool { return c.nlDelete(key) }

// NlPopOldest is PopOldest without the lock (compound operations only).
func (c *LruDisk[V]) NlPopOldest() (string, V, bool) { return c.nlPopOldest() }

// NlLen is Len without the lock (compound operations only).
func (c *LruDisk[V]) NlLen() int { return c.inner.Len() }

// NlCapacity is Capacity without the lock (compound operations only).
func (c *LruDisk[V]) NlCapacity() int { return c.inner.Capacity() }

// NlMemLen is MemLen without the lock (compound operations only).
func (c *LruDisk[V]) NlMemLen() int { return c.mem.Len() }

// NlMemCapacity is MemCapacity without the lock (compound operations
// only).
func (c *LruDisk[V]) NlMemCapacity() int { return c.mem.Capacity() }
