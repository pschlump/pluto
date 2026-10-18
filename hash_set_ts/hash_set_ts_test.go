/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_set_ts

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"testing"

	hs "github.com/pschlump/pluto/hash_set"
)

// expectPanic runs fx and fails the test unless it panics with a
// message containing want.
func expectPanic(t *testing.T, want string, fx func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("expected a panic containing %q, got none", want)
			return
		}
		got, ok := r.(string)
		if !ok {
			t.Errorf("expected a string panic containing %q, got %#v", want, r)
			return
		}
		if !strings.Contains(got, want) {
			t.Errorf("expected a panic containing %q, got %q", want, got)
		}
	}()
	fx()
}

// asMap drains a set into the reference model.
func asMap(t *testing.T, ss *HashSet[int]) map[int]bool {
	t.Helper()
	m := make(map[int]bool)
	for _, v := range ss.Slice() {
		if m[v] {
			t.Fatalf("duplicate element %d in set Slice", v)
		}
		m[v] = true
	}
	if len(m) != ss.Len() {
		t.Fatalf("Len = %d, Slice holds %d", ss.Len(), len(m))
	}
	return m
}

func sameContent(t *testing.T, ss *HashSet[int], want map[int]bool) {
	t.Helper()
	got := asMap(t, ss)
	if len(got) != len(want) {
		t.Fatalf("set holds %d elements, want %d", len(got), len(want))
	}
	for v := range want {
		if !got[v] {
			t.Fatalf("element %d missing", v)
		}
	}
}

func TestHashSetTSBasic(t *testing.T) {
	ss := NewHashSet[int]()
	if !ss.IsEmpty() || ss.Len() != 0 {
		t.Errorf("new set not empty")
	}
	if !ss.Insert(1) || !ss.Insert(2) || ss.Insert(1) {
		t.Errorf("insert return values wrong")
	}
	if got, found := ss.Search(2); !found || got != 2 {
		t.Errorf("Search = (%d, %v)", got, found)
	}
	if !ss.Contains(1) || ss.Contains(9) {
		t.Errorf("Contains wrong")
	}
	if !ss.Delete(1) || ss.Delete(1) {
		t.Errorf("Delete return values wrong")
	}
	if ss.Len() != 1 {
		t.Errorf("Len = %d, want 1", ss.Len())
	}
	ss.Truncate()
	if !ss.IsEmpty() {
		t.Errorf("Truncate left the set non-empty")
	}
	if !ss.Insert(3) {
		t.Errorf("set unusable after Truncate")
	}

	// Eq/Hash parity with the plain package.
	if ss.Eq() == nil || ss.Hash() == nil {
		t.Errorf("constructor-built set must expose its functions")
	}
}

// TestHashSetTSParityWithPlain drives BOTH the plain and the
// thread-safe set through the same operation script; the results must
// agree (the twin is the same semantics behind a mutex).
func TestHashSetTSParityWithPlain(t *testing.T) {
	plain := hs.NewHashSet[int]()
	twin := NewHashSet[int]()

	rng := rand.New(rand.NewSource(42))
	for step := 0; step < 500; step++ {
		k := rng.Intn(100)
		switch rng.Intn(4) {
		case 0:
			if a, b := plain.Insert(k), twin.Insert(k); a != b {
				t.Fatalf("step %d: Insert(%d) plain=%v twin=%v", step, k, a, b)
			}
		case 1:
			if a, b := plain.Delete(k), twin.Delete(k); a != b {
				t.Fatalf("step %d: Delete(%d) plain=%v twin=%v", step, k, a, b)
			}
		case 2:
			if a, b := plain.Contains(k), twin.Contains(k); a != b {
				t.Fatalf("step %d: Contains(%d) plain=%v twin=%v", step, k, a, b)
			}
		}
		if plain.Len() != twin.Len() {
			t.Fatalf("step %d: Len plain=%d twin=%d", step, plain.Len(), twin.Len())
		}
	}
	// Content and rendering agree (String is deterministic on both).
	if a, b := plain.String(), twin.String(); a != b {
		t.Errorf("String plain=%q twin=%q", a, b)
	}
	if plain.Len() != twin.Len() {
		t.Errorf("final Len mismatch")
	}
	for _, v := range plain.Slice() {
		if !twin.Contains(v) {
			t.Errorf("twin missing %d", v)
		}
	}
}

func TestHashSetTSFromSlice(t *testing.T) {
	ss := FromSlice([]int{1, 2, 2, 3, 1})
	if ss.Len() != 3 {
		t.Errorf("Len = %d, want 3", ss.Len())
	}
	sameContent(t, ss, map[int]bool{1: true, 2: true, 3: true})

	eq := func(a, b string) bool { return strings.EqualFold(a, b) }
	hash := func(a string) uint64 {
		var h uint64 = 14695981039346656037
		for _, c := range strings.ToLower(a) {
			h ^= uint64(c)
			h *= 1099511628211
		}
		return h
	}
	f := FromSliceFunc(eq, hash, []string{"a", "A", "b"})
	if f.Len() != 2 || !f.Contains("B") {
		t.Errorf("FromSliceFunc case-insensitive collapse wrong: Len=%d", f.Len())
	}
}

func TestHashSetTSBatchesAndCapabilities(t *testing.T) {
	ss := NewHashSet[int]()
	if !ss.InsertSlice([]int{5, 6, 5}) || ss.InsertSlice([]int{5}) {
		t.Errorf("InsertSlice return values wrong")
	}
	if !ss.ContainsSlice([]int{5, 6}) || ss.ContainsSlice([]int{5, 9}) {
		t.Errorf("ContainsSlice wrong")
	}
	if !ss.ContainsAny([]int{9, 5}) || ss.ContainsAny(nil) {
		t.Errorf("ContainsAny wrong")
	}
	if !ss.InsertIter(slices.Values([]int{7})) {
		t.Errorf("InsertIter wrong")
	}
	if ss.InsertIter(ss.All()) {
		t.Errorf("self-feeding InsertIter must report false")
	}
	if !ss.DeleteSlice([]int{6, 99}) || ss.DeleteSlice([]int{99}) {
		t.Errorf("DeleteSlice return values wrong")
	}
	if !ss.DeleteFunc(func(v int) bool { return v > 6 }) {
		t.Errorf("DeleteFunc wrong")
	}
	sameContent(t, ss, map[int]bool{5: true})

	// DeleteFunc's callback runs with NO lock held: calling back into
	// the set from fx is legal and must not deadlock.
	ss2 := FromSlice([]int{1, 2, 3, 4})
	lens := make([]int, 0)
	ss2.DeleteFunc(func(v int) bool {
		lens = append(lens, ss2.Len()) // call back in: safe, no lock held
		return v%2 == 0
	})
	sameContent(t, ss2, map[int]bool{1: true, 3: true})

	// StringFunc's format callback runs unlocked too.
	up := ss2.StringFunc(func(v int) string {
		_ = ss2.Len() // safe call back in
		return fmt.Sprintf("<%d>", v)
	})
	if up != "[<1> <3>]" {
		t.Errorf("StringFunc = %q", up)
	}

	// Slice is a copy under the read lock.
	items := ss2.Slice()
	items[0] = 99
	if ss2.Contains(99) {
		t.Errorf("Slice aliases storage")
	}

	// Equal/Clone.
	a := FromSlice([]int{1, 2, 3})
	b := FromSlice([]int{3, 2, 1})
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("Equal must ignore order")
	}
	c := a.Clone()
	a.Insert(4)
	b.Delete(1)
	sameContent(t, c, map[int]bool{1: true, 2: true, 3: true})
}

func TestHashSetTSAlgebraAndRelations(t *testing.T) {
	a := FromSlice([]int{1, 2, 3, 4})
	b := FromSlice([]int{3, 4, 5, 6})
	ma, mb := asMap(t, a), asMap(t, b)

	r := NewHashSet[int]()
	r.Union(a, b)
	if len(r.Slice()) != len(ma)+2 {
		t.Errorf("Union size = %d, want %d", r.Len(), len(ma)+2)
	}
	u := make(map[int]bool)
	for v := range ma {
		u[v] = true
	}
	for v := range mb {
		u[v] = true
	}
	sameContent(t, r, u)

	r = NewHashSet[int]()
	r.Intersect(a, b)
	sameContent(t, r, map[int]bool{3: true, 4: true})

	r = NewHashSet[int]()
	r.Minus(a, b)
	sameContent(t, r, map[int]bool{1: true, 2: true})

	r = NewHashSet[int]()
	r.SymmetricDifference(a, b)
	sameContent(t, r, map[int]bool{1: true, 2: true, 5: true, 6: true})

	// Aliases: the destination may be an operand.
	s := a.Clone()
	s.Union(s, s)
	sameContent(t, s, ma)
	s = a.Clone()
	s.Minus(s, s)
	if s.Len() != 0 {
		t.Errorf("s.Minus(s, s) must empty the set")
	}

	// Zero-value adoption and nil receiver no-op.
	var zero HashSet[int]
	zero.Union(a, nil)
	sameContent(t, &zero, ma)
	if zero.Eq() == nil {
		t.Errorf("zero-value receiver must adopt an operand's pair")
	}
	var np *HashSet[int]
	np.Union(a, b) // no-op, must not panic
	np.Minus(a, b)
	if np != nil {
		t.Errorf("nil receiver must stay nil")
	}

	// Relations.
	sub := FromSlice([]int{2, 4})
	if !sub.IsSubsetOf(a) || !sub.IsProperSubsetOf(a) {
		t.Errorf("sub must be a proper subset of a")
	}
	if !a.IsSupersetOf(sub) || !a.IsProperSupersetOf(sub) {
		t.Errorf("a must be a proper superset of sub")
	}
	if a.IsDisjointWith(sub) {
		t.Errorf("a and sub share elements")
	}
	if !a.IsDisjointWith(nil) {
		t.Errorf("everything is disjoint from empty")
	}
	var nilSet *HashSet[int]
	if !nilSet.IsSubsetOf(a) || !nilSet.IsProperSubsetOf(a) {
		t.Errorf("nil (the empty set) is a subset AND a proper subset of non-empty a")
	}
}

func TestHashSetTSIteration(t *testing.T) {
	ss := FromSlice([]int{3, 1, 2})
	n := 0
	for v := range ss.All() {
		if !ss.Contains(v) {
			t.Errorf("All yielded %d, not a member", v)
		}
		n++
	}
	if n != 3 {
		t.Errorf("All visited %d, want 3", n)
	}
	seen := make(map[int]bool)
	if !ss.Walk(func(_ int, v int) bool {
		seen[v] = true
		return true
	}) {
		t.Errorf("Walk must report complete")
	}
	if len(seen) != 3 {
		t.Errorf("Walk visited %d", len(seen))
	}
}

func TestHashSetTSJSON(t *testing.T) {
	a := FromSlice([]int{2, 1})
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	b := NewHashSet[int]()
	if err := b.UnmarshalJSON(data); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	sameContent(t, b, map[int]bool{1: true, 2: true})

	// Unmarshal REPLACES contents and keeps the pair.
	eq := func(x, y string) bool { return strings.EqualFold(x, y) }
	hash := func(x string) uint64 {
		var h uint64 = 14695981039346656037
		for _, c := range strings.ToLower(x) {
			h ^= uint64(c)
			h *= 1099511628211
		}
		return h
	}
	f := NewHashSetFunc(eq, hash)
	if err := f.UnmarshalJSON([]byte(`["Alpha","beta"]`)); err != nil {
		t.Fatalf("Unmarshal func set: %v", err)
	}
	if f.Len() != 2 || !f.Contains("ALPHA") {
		t.Errorf("unmarshal must keep the functions")
	}

	if err := b.UnmarshalJSON([]byte(`[]`)); err != nil || b.Len() != 0 {
		t.Errorf("empty array must empty the set: %v", err)
	}
	if err := b.UnmarshalJSON([]byte(`nope`)); err == nil {
		t.Errorf("malformed JSON must error")
	}
}

func TestHashSetTSNilAndZeroValue(t *testing.T) {
	var nilSet *HashSet[int]
	var zero HashSet[int]

	// Reads are total functions.
	for _, ss := range []*HashSet[int]{nilSet, &zero} {
		if ss.Len() != 0 || !ss.IsEmpty() {
			t.Errorf("Len/IsEmpty wrong")
		}
		if _, found := ss.Search(1); found || ss.Contains(1) {
			t.Errorf("nil/zero found something")
		}
		if ss.Delete(1) || ss.DeleteSlice([]int{1}) || ss.DeleteFunc(func(int) bool { return true }) {
			t.Errorf("nil/zero deletes wrong")
		}
		if ss.ContainsAny([]int{1}) || !ss.ContainsSlice(nil) {
			t.Errorf("nil/zero batch membership wrong")
		}
		if ss.Slice() != nil && len(ss.Slice()) != 0 {
			t.Errorf("nil/zero Slice wrong")
		}
		if got := ss.String(); got != "[]" {
			t.Errorf("String = %q", got)
		}
		n := 0
		for range ss.All() {
			n++
		}
		if n != 0 {
			t.Errorf("All visited %d", n)
		}
		if !ss.Walk(func(int, int) bool { return true }) {
			t.Errorf("Walk must report complete")
		}
		if !ss.Equal(NewHashSet[int]()) {
			t.Errorf("nil/zero equals empty")
		}
		ss.Truncate()
		ss.Lock()
		ss.Unlock() // nil no-ops, must not panic
	}
	if nilSet.Clone() != nil {
		t.Errorf("nil clones to nil")
	}
	if (&zero).Clone().Len() != 0 {
		t.Errorf("zero clones to a fresh zero value")
	}
	if zero.InsertSlice(nil) || nilSet.InsertSlice(nil) {
		t.Errorf("empty InsertSlice must report false")
	}
	if zero.InsertIter(nil) {
		t.Errorf("nil InsertIter must report false")
	}
}

func TestHashSetTSPanics(t *testing.T) {
	var nilSet *HashSet[int]
	var zero HashSet[int]

	expectPanic(t, "hash_set_ts: NewHashSetFunc called with a nil equality function",
		func() { NewHashSetFunc[int](nil, func(a int) uint64 { return 0 }) })
	expectPanic(t, "hash_set_ts: NewHashSetFunc called with a nil hash function",
		func() { NewHashSetFunc[int](func(a, b int) bool { return false }, nil) })
	expectPanic(t, "hash_set_ts: FromSliceFunc called with a nil equality function",
		func() { FromSliceFunc[int](nil, func(a int) uint64 { return 0 }, []int{1}) })
	expectPanic(t, "hash_set_ts: FromSliceFunc called with a nil hash function",
		func() { FromSliceFunc[int](func(a, b int) bool { return false }, nil, []int{1}) })

	expectPanic(t, "hash_set_ts: Insert called on a nil set", func() { nilSet.Insert(1) })
	expectPanic(t, "hash_set_ts: InsertSlice called on a nil set", func() { nilSet.InsertSlice([]int{1}) })
	expectPanic(t, "hash_set_ts: InsertIter called on a nil set", func() { nilSet.InsertIter(slices.Values([]int{1})) })
	expectPanic(t, "hash_set_ts: UnmarshalJSON called on a nil set", func() { _ = nilSet.UnmarshalJSON([]byte(`[1]`)) })

	expectPanic(t, "hash_set_ts: Insert called on a zero-value set", func() { zero.Insert(1) })
	expectPanic(t, "hash_set_ts: InsertSlice called on a zero-value set", func() { zero.InsertSlice([]int{1}) })
	expectPanic(t, "hash_set_ts: UnmarshalJSON called on a zero-value set", func() { _ = zero.UnmarshalJSON([]byte(`[1]`)) })

	// The Nl insert path is a direct inner call: the contract panic
	// surfaces from the wrapped plain set, with its message.
	expectPanic(t, "hash_set: Insert called on a set with no equality/hash functions", func() {
		zero.Lock()
		defer zero.Unlock()
		zero.NlInsert(1)
	})
}

func TestHashSetTSNlCompound(t *testing.T) {
	ss := FromSlice([]int{1, 2, 3})
	ss.Lock() // caller-built read-modify-write compound
	if _, found := ss.NlSearch(2); !found {
		ss.NlInsert(2)
	}
	if !ss.NlDelete(1) {
		t.Errorf("NlDelete must find 1")
	}
	if ss.NlLen() != 2 || ss.NlIsEmpty() {
		t.Errorf("NlLen/NlIsEmpty wrong")
	}
	ss.Unlock()
	sameContent(t, ss, map[int]bool{2: true, 3: true})
}

// -------------------------------------------------------------------------------------------------------
// Concurrency battery.

// TestHashSetTSRaceTrio: concurrent mutators and readers hammer one set;
// correct lock discipline means -race stays silent and the final length
// is exactly the surviving population.
func TestHashSetTSRaceTrio(t *testing.T) {
	ss := NewHashSet[int]()
	const writers = 4
	const perWriter = 2000
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				k := i*writers + w // disjoint key ranges per writer
				ss.Insert(k)
				ss.Contains(k)
				ss.Search(k)
				if i%3 == 0 {
					ss.Delete(k)
					ss.Insert(k)
				}
				_ = ss.String()
			}
		}(w)
	}
	// Readers on the same set.
	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				ss.Len()
				ss.IsEmpty()
				_ = ss.Slice()
				ss.ContainsAny([]int{0, 1, 2})
			}
		}()
	}
	wg.Wait()
	if ss.Len() != writers*perWriter {
		t.Errorf("final Len = %d, want %d", ss.Len(), writers*perWriter)
	}
}

// TestHashSetTSBatchAtomicity: InsertSlice runs under ONE write lock, so
// a compound reader (Lock + Nl* + Unlock) sees the batch all-or-nothing
// — never a half-populated set.
func TestHashSetTSBatchAtomicity(t *testing.T) {
	ss := NewHashSet[int]()
	const n = 1000
	all := make([]int, n)
	for i := range all {
		all[i] = i
	}
	ss.InsertSlice(all)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // writer: drain and refill the whole batch, over and over
		defer wg.Done()
		for i := 0; i < 3000; i++ {
			ss.Truncate()
			ss.InsertSlice(all)
		}
		close(stop)
	}()
	wg.Add(1)
	go func() { // compound reader: the invariant must hold every time
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			ss.Lock()
			l := ss.NlLen()
			_, found := ss.NlSearch(n / 2)
			ss.Unlock()
			if l != 0 && l != n {
				t.Errorf("atomicity broken: Len = %d (want 0 or %d)", l, n)
				return
			}
			if found != (l == n) {
				t.Errorf("atomicity broken: found=%v with Len=%d", found, l)
				return
			}
		}
	}()
	wg.Wait()
	if ss.Len() != n {
		t.Errorf("final Len = %d, want %d", ss.Len(), n)
	}
}

// TestHashSetTSNoDeadlockOppositeDirections: Equal and the algebra
// never hold two locks at once, so two goroutines working the same pair
// of sets in opposite directions must complete.
func TestHashSetTSNoDeadlockOppositeDirections(t *testing.T) {
	a := FromSlice([]int{1, 2, 3})
	b := FromSlice([]int{3, 4, 5})
	x := NewHashSet[int]()
	y := NewHashSet[int]()

	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			a.Equal(b)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			b.Equal(a)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			x.Union(a, b) // snapshots a then b
			x.Minus(a, b)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			y.Union(b, a) // snapshots b then a — opposite order
			y.Intersect(b, a)
		}
	}()
	wg.Wait() // a deadlock hangs the test; completing is the assertion

	// The loops interleaved Union with Minus/Intersect, so settle both
	// with the same union and compare the results — opposite snapshot
	// orders must produce the same set.
	x.Union(a, b)
	y.Union(b, a)
	if !x.Equal(y) {
		t.Errorf("opposite-order unions must agree")
	}
	// Relations in opposite directions concurrently.
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			a.IsSubsetOf(b)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			b.IsSubsetOf(a)
		}
	}()
	wg.Wait()
}

// TestHashSetTSConcurrentSnapshotters: iterators and renderers are
// snapshots, safe to run concurrently with mutation.
func TestHashSetTSConcurrentSnapshotters(t *testing.T) {
	ss := NewHashSet[int]()
	for i := 0; i < 500; i++ {
		ss.Insert(i)
	}
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_ = ss.String()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			n := 0
			for range ss.All() {
				n++
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			ss.Insert(1000 + i)
			ss.Delete(i)
		}
	}()
	wg.Wait()
}
