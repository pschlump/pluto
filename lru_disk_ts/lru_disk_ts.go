// Package lru_disk_ts — the capacity-bounded LRU cache whose VALUES live
// on disk, one file per entry, behind one sync.RWMutex.
//
// The disk twin of link:../lru_ts/[`lru_ts`]: the same public shape
// (Get/Peek/Put/Delete/PopOldest, the Lock/Unlock + Nl* compound forms),
// but keyed by string and persistence-backed — entries survive the
// process (the cache reindexes its directory on open, recency from file
// mtime, evicting to capacity as it goes), evictions and deletes remove
// the files, and writes are atomic (temp file + rename), so a crash
// mid-write leaves the previous entry intact or nothing, never a torn
// file. Values pass through caller-supplied codecs: encode turns a V
// into the file payload, decode turns the payload back; the key travels
// in the file header, which is what makes restart recovery possible.
//
// Errors: I/O can fail where lru_ts cannot. Put and Delete return the
// error; Get treats an unreadable or undecodable entry as a miss and
// removes it (self-healing — a torn or foreign file cannot poison the
// key). NewLruDisk returns an error instead of panicking when the
// directory cannot be made or scanned.
//
// Which method takes which lock mirrors lru_ts: Get is a WRITE-lock
// (recency), Peek/Len/Capacity are read-lock, Put/Delete/Clear are
// write. File I/O happens under the lock — this is a local-disk cache,
// not a network store.
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

// entry is one cached value: its file path and, once read, the decoded
// value (decoded lazily on the first Get; Peek always decodes fresh so
// it can hold the read lock).
type entry[V any] struct {
	path  string
	value V
	ok    bool
}

// LruDisk is a capacity-bounded, thread-safe LRU cache with its values
// stored in files under dir. Create it with NewLruDisk; the zero value
// reads as an empty cache but cannot be Put into. Do not copy a LruDisk
// (the mutex must not be duplicated) — always use *LruDisk.
type LruDisk[V any] struct {
	inner *lru.Lru[string, entry[V]]
	lock  sync.RWMutex
	dir   string
	enc   func(V) []byte
	dec   func([]byte) (V, error)
}

// NewLruDisk creates a cache holding up to capacity entries as files
// under dir (created if needed). Files written by a previous run of the
// same cache are reindexed — recency from file mtime, oldest evicted
// first back down to capacity — so the cache persists across restarts.
// encode/decode carry values to and from the file payload (nil encode
// is an error; a nil decode treats every read as a miss).
// It panics if capacity < 1 and returns an error if dir cannot be made
// or its existing entries cannot be scanned.
func NewLruDisk[V any](capacity int, dir string, encode func(V) []byte, decode func([]byte) (V, error)) (*LruDisk[V], error) {
	if capacity < 1 {
		panic("lru_disk_ts: NewLruDisk called with capacity < 1: a cache needs room for at least one entry")
	}
	if encode == nil {
		return nil, errors.New("lru_disk_ts: NewLruDisk called with a nil encode")
	}
	c := &LruDisk[V]{
		inner: lru.NewLru[string, entry[V]](capacity),
		dir:   dir,
		enc:   encode,
		dec:   decode,
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
		c.inner.Put(f.key, entry[V]{path: f.path})
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
		tmp.Close()
		return err
	}
	if _, err = tmp.Write(payload); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
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

// Get returns the value stored under key and whether it was found,
// reading and decoding the file on the first access. A hit marks the
// entry most-recently-used — a mutation — so Get takes the WRITE lock,
// not the read lock (Peek is the read-lock form). An unreadable or
// undecodable entry is a miss and is deleted (file included).
// A nil or zero-value cache reports not-found.
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
	e, ok := c.inner.Get(key)
	if !ok {
		var zero V
		return zero, false
	}
	if e.ok {
		return e.value, true
	}
	v, err := c.readEntry(e.path)
	if err != nil {
		c.inner.Delete(key)
		_ = os.Remove(e.path)
		var zero V
		return zero, false
	}
	e.value, e.ok = v, true
	c.inner.Put(key, e)
	return v, true
}

// Peek returns the value stored under key and whether it was found,
// WITHOUT changing the recency order — a true read under the read
// lock. The payload is decoded fresh (a Peek never caches the decode).
// A nil or zero-value cache reports not-found.
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
	e, ok := c.inner.Peek(key)
	if !ok {
		var zero V
		return zero, false
	}
	if e.ok {
		return e.value, true
	}
	v, err := c.readEntry(e.path)
	if err != nil {
		var zero V
		return zero, false
	}
	return v, true
}

// Put stores value under key, writing its file atomically, and returns
// the write error if the file cannot be written (the in-memory entry is
// still updated — a later Get of it will miss-and-self-heal). When the
// cache is at capacity the least-recently-used entry is evicted and its
// file deleted; the eviction file error is NOT swallowed if the Put
// itself succeeded — Put returns the first error.
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
	if c.inner.Len() >= c.inner.Capacity() {
		if _, victim, ok := c.inner.PopOldest(); ok && victim.path != fileFor(c.dir, key) {
			if rerr := os.Remove(victim.path); rerr != nil && !errors.Is(rerr, os.ErrNotExist) && err == nil {
				err = rerr
			}
		}
	}
	path := fileFor(c.dir, key)
	c.inner.Put(key, entry[V]{path: path, value: value, ok: true})
	if werr := c.writeEntry(key, value); werr != nil && err == nil {
		err = werr
	}
	return err
}

// Delete removes key and its file, reporting whether the key was found.
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
	_ = os.Remove(e.path)
	return true
}

// PopOldest removes and returns the least-recently-used entry's key and
// value (decoded from its file), deleting the file. A decode failure
// yields the zero V with ok=true — the entry is gone either way.
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
	_ = os.Remove(e.path)
	if e.ok {
		return key, e.value, true
	}
	v, err := c.readEntry(e.path)
	if err != nil {
		var zero V
		return key, zero, true
	}
	return key, v, true
}

// Len returns the number of entries. A nil cache holds none.
func (c *LruDisk[V]) Len() int {
	if c == nil || c.inner == nil {
		return 0
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.inner.Len()
}

// Capacity returns the maximum number of entries.
func (c *LruDisk[V]) Capacity() int {
	if c == nil || c.inner == nil {
		return 0
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.inner.Capacity()
}

// Clear removes every entry and its file.
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
// caches the decode, so it holds no state beyond the call.
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
