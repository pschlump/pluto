/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tree_set_ts

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"

	"github.com/pschlump/pluto/tree_set"
)

// expectPanic fails the test if fx returns without panicking.
func expectPanic(t *testing.T, name string, fx func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected a panic, got none", name)
		}
	}()
	fx()
}

// expectWritePanic additionally checks the twin's insert-contract
// message wording.
func expectWritePanic(t *testing.T, name, caller string, fx func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("%s: expected a panic, got none", name)
			return
		}
		want := fmt.Sprintf("tree_set_ts: %s called on a nil or zero-value set (create the set with NewTreeSet or NewTreeSetFunc)", caller)
		if got := fmt.Sprint(r); got != want {
			t.Errorf("%s: panic message = %q, want %q", name, got, want)
		}
	}()
	fx()
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func checkAgainstModel(t *testing.T, name string, set *TreeSet[string], model map[string]bool) {
	t.Helper()
	if got := set.Len(); got != len(model) {
		t.Fatalf("%s: Len = %d, want %d", name, got, len(model))
	}
	want := sortedKeys(model)
	if got := set.Slice(); !slices.Equal(got, want) {
		t.Fatalf("%s: Slice = %v, want %v", name, got, want)
	}
	for _, k := range want {
		if !set.Contains(k) {
			t.Errorf("%s: Contains(%q) = false, want true", name, k)
		}
	}
}

// -------------------------------------------------------------------------------------------------------

// TestTwinModelRandomized: the property battery against a map reference
// model, fixed seeds 42 and 7.
func TestTwinModelRandomized(t *testing.T) {
	for _, seed := range []int64{42, 7} {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			set := NewTreeSet[string]()
			model := map[string]bool{}

			for step := 0; step < 800; step++ {
				switch rng.Intn(4) {
				case 0, 1:
					k := fmt.Sprintf("k%03d", rng.Intn(200))
					if got := set.Insert(k); got != !model[k] {
						t.Fatalf("step %d: Insert(%q) = %v, want %v", step, k, got, !model[k])
					}
					model[k] = true
				case 2:
					k := fmt.Sprintf("k%03d", rng.Intn(200))
					if got := set.Delete(k); got != model[k] {
						t.Fatalf("step %d: Delete(%q) = %v, want %v", step, k, got, model[k])
					}
					delete(model, k)
				case 3:
					k := fmt.Sprintf("k%03d", rng.Intn(200))
					if got := set.Contains(k); got != model[k] {
						t.Fatalf("step %d: Contains(%q) = %v, want %v", step, k, got, model[k])
					}
				}
				if set.Len() != len(model) {
					t.Fatalf("step %d: Len = %d, want %d", step, set.Len(), len(model))
				}
			}
			checkAgainstModel(t, "final", set, model)
		})
	}
}

// TestTwinParityWithPlain: the identical operation sequence run against
// the plain package and the twin must produce the identical observable
// set at every step.
func TestTwinParityWithPlain(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	twin := NewTreeSet[string]()
	plain := tree_set.NewTreeSet[string]()

	for step := 0; step < 500; step++ {
		k := fmt.Sprintf("k%03d", rng.Intn(120))
		switch rng.Intn(2) {
		case 0:
			g, p := twin.Insert(k), plain.Insert(k)
			if g != p {
				t.Fatalf("step %d: twin Insert(%q) = %v, plain = %v", step, k, g, p)
			}
		case 1:
			g, p := twin.Delete(k), plain.Delete(k)
			if g != p {
				t.Fatalf("step %d: twin Delete(%q) = %v, plain = %v", step, k, g, p)
			}
		}
		if twin.Len() != plain.Len() || twin.String() != plain.String() {
			t.Fatalf("step %d: twin = %q (len %d), plain = %q (len %d)",
				step, twin, twin.Len(), plain, plain.Len())
		}
	}
	if !twin.Equal(FromSlice(plain.Slice())) {
		t.Errorf("final twin != plain content")
	}
}

// TestTwinRiders covers the sorted-set riders on the twin.
func TestTwinRiders(t *testing.T) {
	set := FromSlice([]string{"ant", "cat", "emu", "fox", "hog"})

	if v, found := set.Floor("dog"); !found || v != "cat" {
		t.Errorf("Floor(dog) = (%q, %v), want (cat, true)", v, found)
	}
	if v, found := set.Ceil("dog"); !found || v != "emu" {
		t.Errorf("Ceil(dog) = (%q, %v), want (emu, true)", v, found)
	}
	if _, found := set.Floor("a"); found {
		t.Errorf("Floor(below all) found, want not found")
	}
	if _, found := set.Ceil("z"); found {
		t.Errorf("Ceil(above all) found, want not found")
	}
	if v, found := set.Index(3); !found || v != "fox" {
		t.Errorf("Index(3) = (%q, %v), want (fox, true)", v, found)
	}
	if _, found := set.Index(-1); found {
		t.Errorf("Index(-1) found")
	}
	if _, found := set.Index(5); found {
		t.Errorf("Index(len) found")
	}
	if !set.DeleteAtHead() || set.String() != "[cat emu fox hog]" {
		t.Errorf("DeleteAtHead did not remove the smallest")
	}
	if !set.DeleteAtTail() || set.String() != "[cat emu fox]" {
		t.Errorf("DeleteAtTail did not remove the largest")
	}
	if v, found := set.FindMin(); !found || v != "cat" {
		t.Errorf("FindMin = (%q, %v)", v, found)
	}
	if v, found := set.FindMax(); !found || v != "fox" {
		t.Errorf("FindMax = (%q, %v)", v, found)
	}
}

// TestTwinRange covers the bounded iterator, including the descending
// comparator.
func TestTwinRange(t *testing.T) {
	set := FromSlice([]string{"ant", "cat", "emu", "fox"})
	var got []string
	for v := range set.Range("b", "fox") {
		got = append(got, v)
	}
	if want := []string{"cat", "emu", "fox"}; !slices.Equal(got, want) {
		t.Errorf("Range(b,fox) = %v, want %v", got, want)
	}
	got = nil
	for range set.Range("fox", "b") { // lo > hi: empty
		got = append(got, "x")
	}
	if got != nil {
		t.Errorf("Range(lo>hi) yielded %v, want empty", got)
	}
	var nilSet *TreeSet[string]
	got = nil
	for v := range nilSet.Range("a", "z") {
		got = append(got, v)
	}
	if got != nil {
		t.Errorf("nil set Range yielded %v", got)
	}

	desc := FromSliceFunc(func(a, b int) int { return b - a }, []int{1, 2, 3, 4, 5})
	var out []int
	for v := range desc.Range(4, 2) {
		out = append(out, v)
	}
	if want := []int{4, 3, 2}; !slices.Equal(out, want) {
		t.Errorf("descending Range = %v, want %v", out, want)
	}
}

// TestTwinBatches covers the batch menu.
func TestTwinBatches(t *testing.T) {
	set := FromSlice([]string{"a", "b", "c"})
	if !set.InsertSlice([]string{"c", "d"}) || set.InsertSlice(nil) {
		t.Errorf("InsertSlice reports wrong")
	}
	if !set.ContainsSlice([]string{"a", "d"}) || set.ContainsSlice([]string{"a", "z"}) {
		t.Errorf("ContainsSlice wrong")
	}
	if !set.ContainsAny([]string{"z", "d"}) || set.ContainsAny([]string{"z"}) {
		t.Errorf("ContainsAny wrong")
	}
	if !set.DeleteSlice([]string{"z", "a"}) || set.DeleteSlice([]string{"z"}) {
		t.Errorf("DeleteSlice wrong")
	}
	other := FromSlice([]string{"e"})
	if !set.InsertIter(other.All()) {
		t.Errorf("InsertIter = false, want true")
	}
	if set.InsertIter(set.All()) {
		t.Errorf("InsertIter(own All) = true, want false (all duplicates)")
	}
	if got, want := set.String(), "[b c d e]"; got != want {
		t.Errorf("after batches: %q, want %q", got, want)
	}
}

// TestTwinDeleteFunc: predicate bulk delete, with fx calling back into
// the set (it runs with no lock held).
func TestTwinDeleteFunc(t *testing.T) {
	set := FromSlice([]string{"a1", "b2", "c3", "d4"})
	if !set.DeleteFunc(func(v string) bool {
		// fx may safely call back into the set: it runs on a snapshot,
		// with no lock held.
		return set.Contains(strings.TrimSuffix(v, "0")) && strings.HasSuffix(v, "2")
	}) {
		t.Errorf("DeleteFunc(matching) = false, want true")
	}
	if got, want := set.String(), "[a1 c3 d4]"; got != want {
		t.Errorf("after DeleteFunc: %q, want %q", got, want)
	}
	if set.DeleteFunc(func(string) bool { return false }) {
		t.Errorf("DeleteFunc(no matches) = true")
	}
	if !set.DeleteFunc(func(string) bool { return true }) || !set.IsEmpty() {
		t.Errorf("DeleteFunc(all) did not empty the set")
	}
}

// TestTwinAlgebra: cross-set algebra with the twin's snapshot
// discipline — including every operand aliasing the receiver, the
// no-deadlock opposite direction, and zero-value adoption.
func TestTwinAlgebra(t *testing.T) {
	a := FromSlice([]string{"a", "b", "c"})
	b := FromSlice([]string{"b", "c", "d"})

	dst := NewTreeSet[string]()
	dst.Union(a, b)
	if got, want := dst.String(), "[a b c d]"; got != want {
		t.Errorf("Union: %q, want %q", got, want)
	}

	dst = NewTreeSet[string]()
	dst.Intersect(a, b)
	if got, want := dst.String(), "[b c]"; got != want {
		t.Errorf("Intersect: %q, want %q", got, want)
	}

	dst = NewTreeSet[string]()
	dst.Minus(a, b)
	if got, want := dst.String(), "[a]"; got != want {
		t.Errorf("Minus: %q, want %q", got, want)
	}

	dst = NewTreeSet[string]()
	dst.SymmetricDifference(a, b)
	if got, want := dst.String(), "[a d]"; got != want {
		t.Errorf("SymmetricDifference: %q, want %q", got, want)
	}

	// Alias safety: operands may alias the receiver, and each other.
	self := a.Clone()
	self.Union(self, self)
	if !self.Equal(a) {
		t.Errorf("s.Union(s,s) != s")
	}
	self = a.Clone()
	self.Minus(self, self)
	if !self.IsEmpty() {
		t.Errorf("s.Minus(s,s) not empty")
	}
	self = a.Clone()
	self.Intersect(self, b) // receiver aliases yy
	if got, want := self.String(), "[b c]"; got != want {
		t.Errorf("s.Intersect(s,b): %q, want %q", got, want)
	}

	// Zero-value adoption: the twin adopts an operand's ordering and
	// becomes refillable.
	var zero TreeSet[string]
	zero.Union(a, b)
	if got, want := zero.String(), "[a b c d]"; got != want {
		t.Errorf("adopting Union: %q, want %q", got, want)
	}
	if !zero.Insert("z") {
		t.Errorf("Insert after adoption = false")
	}

	// No ordering anywhere: the result stays a zero value.
	var z1, z2 TreeSet[string]
	z1.Union(&z2, &z2)
	expectWritePanic(t, "no-comparator adoption", "Insert", func() {
		z1.Insert("x")
	})

	// Nil operands contribute nothing — and the algebra is
	// destination-form: Union(nil, nil) REPLACES the receiver with the
	// empty result, exactly as avl_tree's contract reads.
	dst = FromSlice([]string{"a"})
	var nilSet *TreeSet[string]
	dst.Union(nilSet, nilSet)
	if got, want := dst.String(), "[]"; got != want {
		t.Errorf("Union(nil,nil) on a filled destination: %q, want %q (destination-form)", got, want)
	}
	dst = NewTreeSet[string]()
	dst.Minus(a, nilSet)
	if !dst.Equal(a) {
		t.Errorf("Minus(a, nil) != a")
	}
}

// TestTwinRelations covers the five relation predicates.
func TestTwinRelations(t *testing.T) {
	s := FromSlice([]string{"a", "b"})
	sup := FromSlice([]string{"a", "b", "c"})
	other := FromSlice([]string{"x"})
	var nilSet *TreeSet[string]

	if !s.IsSubsetOf(sup) || sup.IsSubsetOf(s) {
		t.Errorf("IsSubsetOf wrong")
	}
	if s.IsProperSubsetOf(s) || !s.IsProperSubsetOf(sup) {
		t.Errorf("IsProperSubsetOf wrong")
	}
	if !sup.IsSupersetOf(s) || s.IsSupersetOf(sup) {
		t.Errorf("IsSupersetOf wrong")
	}
	if sup.IsProperSupersetOf(sup) || !sup.IsProperSupersetOf(s) {
		t.Errorf("IsProperSupersetOf wrong")
	}
	if !s.IsDisjointWith(other) || s.IsDisjointWith(sup) {
		t.Errorf("IsDisjointWith wrong")
	}
	if !nilSet.IsSubsetOf(other) || !nilSet.IsDisjointWith(other) {
		t.Errorf("nil set relations wrong (empty ⊆ everything, disjoint with everything)")
	}
}

// TestTwinConveniences: String determinism, Equal symmetry, Clone deep
// copy — both directions.
func TestTwinConveniences(t *testing.T) {
	fwd := FromSlice([]string{"a", "b", "c"})
	bwd := FromSlice([]string{"c", "b", "a"})
	if fwd.String() != bwd.String() {
		t.Errorf("String differs across insertion orders")
	}
	if got := fwd.StringFunc(strings.ToUpper); got != "[A B C]" {
		t.Errorf("StringFunc = %q", got)
	}
	if !fwd.Equal(bwd) || !bwd.Equal(fwd) {
		t.Errorf("Equal not symmetric")
	}

	dup := fwd.Clone()
	fwd.Insert("x")
	dup.Delete("a")
	if got, want := fwd.String(), "[a b c x]"; got != want {
		t.Errorf("original: %q, want %q", got, want)
	}
	if got, want := dup.String(), "[b c]"; got != want {
		t.Errorf("clone: %q, want %q", got, want)
	}

	var nilSet *TreeSet[string]
	if nilSet.Clone() != nil {
		t.Errorf("nil clones to non-nil")
	}
	zero := (&TreeSet[string]{}).Clone()
	if zero.Len() != 0 {
		t.Errorf("zero-value clone: Len = %d", zero.Len())
	}
	expectWritePanic(t, "zero-value clone keeps no comparator", "Insert", func() {
		zero.Insert("x")
	})
}

// TestTwinIteratorsSnapshot: All/Backward/Range yield snapshots — the
// set may be mutated from inside the loop.
func TestTwinIteratorsSnapshot(t *testing.T) {
	set := FromSlice([]string{"a", "b", "c", "d"})
	var seen []string
	for v := range set.All() {
		seen = append(seen, v)
		set.Delete(v) // mutation from inside the loop: legal on a snapshot
	}
	if want := []string{"a", "b", "c", "d"}; !slices.Equal(seen, want) {
		t.Errorf("All yielded %v, want %v", seen, want)
	}
	if !set.IsEmpty() {
		t.Errorf("set not empty after in-loop deletes: %q", set)
	}

	set = FromSlice([]string{"a", "b", "c"})
	var back []string
	for v := range set.Backward() {
		back = append(back, v)
	}
	if want := []string{"c", "b", "a"}; !slices.Equal(back, want) {
		t.Errorf("Backward yielded %v, want %v", back, want)
	}

	n := 0
	for range set.Range("a", "z") {
		n++
	}
	if n != 3 {
		t.Errorf("Range visited %d, want 3", n)
	}
}

// TestTwinWalks: the walks traverse live under the read lock (callback
// must not call back).
func TestTwinWalks(t *testing.T) {
	set := FromSlice([]string{"a", "b", "c"})
	var order []string
	set.WalkInOrder(func(pos, depth int, data string) bool {
		order = append(order, data)
		return true
	})
	if want := []string{"a", "b", "c"}; !slices.Equal(order, want) {
		t.Errorf("WalkInOrder = %v, want %v", order, want)
	}
	stopped := false
	set.WalkPreOrder(func(pos, depth int, data string) bool {
		if data == "b" {
			stopped = true
			return false
		}
		return true
	})
	if !stopped {
		t.Errorf("WalkPreOrder early stop did not trigger")
	}
	count := 0
	set.WalkPostOrder(func(pos, depth int, data string) bool {
		count++
		return true
	})
	if count != 3 {
		t.Errorf("WalkPostOrder visited %d, want 3", count)
	}
}

// TestTwinJSON covers the round trip under the twin's lock discipline.
func TestTwinJSON(t *testing.T) {
	set := NewTreeSetFunc(func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	set.InsertSlice([]string{"Bravo", "alpha"})

	data, err := set.MarshalJSON()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got, want := string(data), `["alpha","Bravo"]`; got != want {
		t.Errorf("Marshal = %s, want %s", got, want)
	}

	into := NewTreeSetFunc(func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	into.Insert("junk")
	if err := into.UnmarshalJSON(data); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !into.Contains("ALPHA") {
		t.Errorf("comparator not kept across Unmarshal")
	}
	if err := into.UnmarshalJSON([]byte(`{"nope":1}`)); err == nil {
		t.Errorf("non-array decoded without error")
	}
	if got, want := into.String(), "[alpha Bravo]"; got != want {
		t.Errorf("after failed Unmarshal: %q, want %q (must be untouched)", got, want)
	}
	if err := into.UnmarshalJSON([]byte(`[]`)); err != nil || !into.IsEmpty() {
		t.Errorf("Unmarshal([]) = %v, empty = %v", err, into.IsEmpty())
	}

	nilData, err := (*TreeSet[string])(nil).MarshalJSON()
	if err != nil || string(nilData) != "[]" {
		t.Errorf("nil MarshalJSON = (%s, %v), want ([], nil)", nilData, err)
	}
	if err := (*TreeSet[string])(nil).UnmarshalJSON([]byte(`[]`)); err != nil {
		t.Errorf("nil UnmarshalJSON([]): %v", err)
	}
	expectWritePanic(t, "nil UnmarshalJSON storing", "UnmarshalJSON", func() {
		_ = (*TreeSet[string])(nil).UnmarshalJSON([]byte(`["x"]`))
	})
}

// TestTwinNilTolerated: nil set and zero value read as empty.
func TestTwinNilTolerated(t *testing.T) {
	var nilSet *TreeSet[string]
	var zero TreeSet[string]

	for name, set := range map[string]*TreeSet[string]{"nil": nilSet, "zero": &zero} {
		if set.Len() != 0 || !set.IsEmpty() {
			t.Errorf("%s: size wrong", name)
		}
		if set.Contains("x") {
			t.Errorf("%s: Contains true", name)
		}
		if _, found := set.Search("x"); found {
			t.Errorf("%s: Search found", name)
		}
		if set.Delete("x") || set.DeleteAtHead() || set.DeleteAtTail() {
			t.Errorf("%s: delete on empty true", name)
		}
		if _, found := set.FindMin(); found {
			t.Errorf("%s: FindMin found", name)
		}
		if _, found := set.Floor("x"); found {
			t.Errorf("%s: Floor found", name)
		}
		if _, found := set.Index(0); found {
			t.Errorf("%s: Index found", name)
		}
		if set.String() != "[]" {
			t.Errorf("%s: String = %q", name, set.String())
		}
		if set.InsertSlice(nil) || set.InsertIter(nil) {
			t.Errorf("%s: empty insert true", name)
		}
		if !set.ContainsSlice(nil) {
			t.Errorf("%s: ContainsSlice(nil) false", name)
		}
		if set.DeleteSlice([]string{"x"}) || set.DeleteFunc(func(string) bool { return true }) {
			t.Errorf("%s: deletes true", name)
		}
		set.Truncate()
		n := 0
		for range set.All() {
			n++
		}
		for range set.Range("a", "z") {
			n++
		}
		if n != 0 {
			t.Errorf("%s: iterators visited %d", name, n)
		}
	}
	if got := nilSet.Slice(); got != nil {
		t.Errorf("nil Slice = %v, want nil", got)
	}
	if !nilSet.Equal(nilSet) || !nilSet.Equal(NewTreeSet[string]()) {
		t.Errorf("nil Equal wrong")
	}
}

// TestTwinPanics: the exact panic surface, twin-named messages.
func TestTwinPanics(t *testing.T) {
	expectPanic(t, "NewTreeSetFunc(nil)", func() {
		NewTreeSetFunc[string](nil)
	})
	expectPanic(t, "FromSliceFunc(nil, …)", func() {
		FromSliceFunc[string](nil, []string{"x"})
	})

	var nilSet *TreeSet[string]
	expectWritePanic(t, "Insert on nil set", "Insert", func() {
		nilSet.Insert("x")
	})
	var zero TreeSet[string]
	expectWritePanic(t, "Insert on zero value", "Insert", func() {
		zero.Insert("x")
	})
	expectWritePanic(t, "InsertSlice storing on zero value", "InsertSlice", func() {
		zero.InsertSlice([]string{"x"})
	})
	expectWritePanic(t, "InsertIter storing on zero value", "InsertIter", func() {
		zero.InsertIter(slices.Values([]string{"x"}))
	})

	// Empty batches are tolerated on the zero value.
	zero.InsertSlice(nil)
	zero.InsertIter(nil)
}

// TestTwinLockNlCompound: the race-free compound pattern — Lock held
// across a multi-step sequence of Nl* calls.
func TestTwinLockNlCompound(t *testing.T) {
	set := NewTreeSet[string]()
	set.Lock()
	if set.NlLen() != 0 || !set.NlIsEmpty() {
		t.Errorf("Nl size wrong on the empty set")
	}
	set.NlInsert("b")
	set.NlInsert("a")
	if v, found := set.NlSearch("a"); !found || v != "a" {
		t.Errorf("NlSearch = (%q, %v)", v, found)
	}
	if !set.NlContains("b") {
		t.Errorf("NlContains = false")
	}
	if !set.NlDelete("b") {
		t.Errorf("NlDelete = false")
	}
	set.Unlock()
	if got, want := set.String(), "[a]"; got != want {
		t.Errorf("after compound: %q, want %q", got, want)
	}

	// The nil-tolerant Lock/Unlock on a nil set.
	nilSet := (*TreeSet[string])(nil)
	nilSet.Lock()
	nilSet.Unlock()

	// Read-side sequences do not need the compound pattern: the plain
	// read methods are self-locking, and concurrent readers never block
	// each other under the RWMutex.
	if n := set.NlLen(); n != 1 {
		t.Errorf("after unlock: Len = %d, want 1", n)
	}
}

// TestTwinConcurrent: the race battery — concurrent writers, readers
// and mixed access from several goroutines (run under -race).
func TestTwinConcurrent(t *testing.T) {
	set := NewTreeSet[string]()
	const writers = 4
	const readers = 4
	const items = 300

	done := make(chan struct{})

	for w := 0; w < writers; w++ {
		go func(w int) {
			defer func() { done <- struct{}{} }()
			for i := 0; i < items; i++ {
				k := fmt.Sprintf("w%d-%03d", w, i)
				set.Insert(k)
				if i%3 == 0 {
					set.Delete(k)
				}
			}
		}(w)
	}
	for r := 0; r < readers; r++ {
		go func(r int) {
			defer func() { done <- struct{}{} }()
			for i := 0; i < items; i++ {
				switch i % 5 {
				case 0:
					set.Contains(fmt.Sprintf("w%d-%03d", i%writers, i))
				case 1:
					_ = set.Slice()
				case 2:
					for range set.All() {
						break
					}
				case 3:
					set.Len()
				case 4:
					_ = set.Range("w", "w0")
				}
			}
		}(r)
	}
	for i := 0; i < writers+readers; i++ {
		<-done
	}

	// Every surviving element is a live insert: w-i not deleted (i%3 != 0).
	want := map[string]bool{}
	for w := 0; w < writers; w++ {
		for i := 0; i < items; i++ {
			if i%3 != 0 {
				want[fmt.Sprintf("w%d-%03d", w, i)] = true
			}
		}
	}
	if got := set.Len(); got != len(want) {
		t.Errorf("Len = %d, want %d", got, len(want))
	}
	for k := range want {
		if !set.Contains(k) {
			t.Errorf("Contains(%q) = false after the race battery", k)
		}
	}
}

// TestTwinBatchAtomicity: an InsertSlice batch is atomic — a concurrent
// reader never observes a partial batch.
func TestTwinBatchAtomicity(t *testing.T) {
	set := NewTreeSet[string]()
	items := make([]string, 500)
	for i := range items {
		items[i] = fmt.Sprintf("b%03d", i)
	}

	stop := make(chan struct{})
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			select {
			case <-stop:
				return
			default:
			}
			// Every observation is all-or-nothing: 0 or the full 500.
			if n := set.Len(); n != 0 && n != len(items) {
				t.Errorf("partial batch observed: Len = %d", n)
				return
			}
		}
	}()

	set.InsertSlice(items) // one atomic write batch
	close(stop)
	<-readerDone
	if set.Len() != len(items) {
		t.Errorf("final Len = %d, want %d", set.Len(), len(items))
	}
}

// TestTwinNoDeadlockOppositeDirections: two goroutines touch the same
// pair of sets in opposite directions — Equal and the algebra both —
// without ever holding two locks at once.  A deadlock hangs the test.
func TestTwinNoDeadlockOppositeDirections(t *testing.T) {
	a := FromSlice([]string{"a", "b", "c"})
	b := FromSlice([]string{"b", "c", "d"})
	done := make(chan struct{}, 2)

	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < 200; i++ {
			a.Equal(b)
			dst := NewTreeSet[string]()
			dst.Union(a, b)
			a.IsSubsetOf(b)
			a.IsDisjointWith(b)
		}
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < 200; i++ {
			b.Equal(a)
			dst := NewTreeSet[string]()
			dst.Intersect(b, a)
			b.IsSupersetOf(a)
			b.IsDisjointWith(a)
		}
	}()
	<-done
	<-done
}

// TestTwinConcurrentAlgebraAdoption: algebra adopting an ordering while
// other goroutines probe the sets (the cmp write is under the write
// lock; every read of it is under a lock — run under -race).
func TestTwinConcurrentAlgebraAdoption(t *testing.T) {
	built := FromSlice([]string{"a", "b"})
	var zero TreeSet[string]

	done := make(chan struct{}, 3)
	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < 100; i++ {
			zero.Union(built, built) // adoption writes zero.cmp under the write lock
			zero.Truncate()
		}
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < 100; i++ {
			_ = zero.Slice()
			zero.Len()
		}
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < 100; i++ {
			dst := NewTreeSet[string]()
			dst.Union(&zero, built) // reads zero.cmp under zero's read lock
		}
	}()
	<-done
	<-done
	<-done
}
