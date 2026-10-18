/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tree_set

import (
	"encoding/json"
	"fmt"
	"iter"
	"math/rand"
	"slices"
	"strings"
	"testing"
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

// expectInsertPanic additionally checks the panic message: the insert
// contract's exact wording, naming the constructors.
func expectInsertPanic(t *testing.T, name, caller string, fx func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("%s: expected a panic, got none", name)
			return
		}
		want := fmt.Sprintf("tree_set: %s called on a nil or zero-value set (create the set with NewTreeSet or NewTreeSetFunc)", caller)
		if got := fmt.Sprint(r); got != want {
			t.Errorf("%s: panic message = %q, want %q", name, got, want)
		}
	}()
	fx()
}

// keySpace is the shared randomized-test key universe; small enough that
// duplicates and collisions of every kind occur constantly.
const keySpace = 200

func randKey(rng *rand.Rand) string {
	return fmt.Sprintf("k%03d", rng.Intn(keySpace))
}

// sortedKeys returns the map's keys sorted — the model's ascending
// order, what Slice must equal exactly.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// checkAgainstModel verifies the whole observable surface of set against
// the reference model: length, ascending Slice, and membership of every
// model key (present) and some non-keys (absent).
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
		if _, found := set.Search(k); !found {
			t.Errorf("%s: Search(%q) found = false, want true", name, k)
		}
	}
	for i := 0; i < 20; i++ {
		k := fmt.Sprintf("nope%03d", i)
		if set.Contains(k) {
			t.Errorf("%s: Contains(%q) = true, want false", name, k)
		}
	}
}

// -------------------------------------------------------------------------------------------------------

// TestTreeSetFixedVector drives every mutating operation against a hand
// computed result.
func TestTreeSetFixedVector(t *testing.T) {
	set := NewTreeSet[string]()
	if !set.IsEmpty() {
		t.Errorf("new set: IsEmpty = false, want true")
	}
	if got := set.String(); got != "[]" {
		t.Errorf("new set: String = %q, want \"[]\"", got)
	}

	for _, v := range []string{"pear", "apple", "fig", "date", "banana"} {
		if !set.Insert(v) {
			t.Errorf("Insert(%q) = false, want true (new element)", v)
		}
	}
	if set.Insert("fig") {
		t.Errorf("Insert(duplicate) = true, want false")
	}
	if got, want := set.Len(), 5; got != want {
		t.Errorf("Len = %d, want %d", got, want)
	}
	if got, want := set.String(), "[apple banana date fig pear]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
	if got := set.Slice(); !slices.Equal(got, []string{"apple", "banana", "date", "fig", "pear"}) {
		t.Errorf("Slice = %v, want the ascending five", got)
	}

	if !set.Delete("date") {
		t.Errorf("Delete(present) = false, want true")
	}
	if set.Delete("date") {
		t.Errorf("Delete(absent) = true, want false")
	}
	if got, want := set.String(), "[apple banana fig pear]"; got != want {
		t.Errorf("after Delete: String = %q, want %q", got, want)
	}

	min, found := set.FindMin()
	if !found || min != "apple" {
		t.Errorf("FindMin = (%q, %v), want (\"apple\", true)", min, found)
	}
	max, found := set.FindMax()
	if !found || max != "pear" {
		t.Errorf("FindMax = (%q, %v), want (\"pear\", true)", max, found)
	}

	if !set.DeleteAtHead() {
		t.Errorf("DeleteAtHead = false, want true")
	}
	if !set.DeleteAtTail() {
		t.Errorf("DeleteAtTail = false, want true")
	}
	if got, want := set.String(), "[banana fig]"; got != want {
		t.Errorf("after head/tail deletes: String = %q, want %q", got, want)
	}

	set.Truncate()
	if !set.IsEmpty() || set.Len() != 0 {
		t.Errorf("after Truncate: set not empty")
	}
	// Truncate keeps the comparator: the set is simply refillable.
	if !set.Insert("again") {
		t.Errorf("Insert after Truncate = false, want true")
	}
}

// TestTreeSetModelRandomized is the property test against a
// map[string]bool reference model — both fixed seeds, 42 and 7, as the
// plan requires.
func TestTreeSetModelRandomized(t *testing.T) {
	for _, seed := range []int64{42, 7} {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			set := NewTreeSet[string]()
			model := map[string]bool{}

			for step := 0; step < 800; step++ {
				switch rng.Intn(4) {
				case 0, 1: // inserts twice as often as deletes
					k := randKey(rng)
					if got := set.Insert(k); got != !model[k] {
						t.Fatalf("step %d: Insert(%q) = %v, want %v", step, k, got, !model[k])
					}
					model[k] = true
				case 2:
					k := randKey(rng)
					if got := set.Delete(k); got != model[k] {
						t.Fatalf("step %d: Delete(%q) = %v, want %v", step, k, got, model[k])
					}
					delete(model, k)
				case 3:
					k := randKey(rng)
					if got := set.Contains(k); got != model[k] {
						t.Fatalf("step %d: Contains(%q) = %v, want %v", step, k, got, model[k])
					}
				}
				if set.Len() != len(model) {
					t.Fatalf("step %d: Len = %d, want %d", step, set.Len(), len(model))
				}
			}
			checkAgainstModel(t, "final", set, model)

			// Ascending order is Slice's contract; re-verify after a
			// rebuild from the same content in a different order.
			perm := set.Slice()
			rng.Shuffle(len(perm), func(i, j int) { perm[i], perm[j] = perm[j], perm[i] })
			rebuilt := FromSlice(perm)
			checkAgainstModel(t, "rebuilt", rebuilt, model)
			if !set.Equal(rebuilt) {
				t.Errorf("set and its shuffled rebuild are not Equal")
			}
		})
	}
}

// TestTreeSetFromSliceDedup covers the constructors-from-data: duplicates
// collapse exactly as inserting in order would.
func TestTreeSetFromSliceDedup(t *testing.T) {
	set := FromSlice([]string{"b", "a", "b", "c", "a", "b"})
	if got, want := set.Len(), 3; got != want {
		t.Errorf("Len = %d, want %d (duplicates must collapse)", got, want)
	}
	if got := set.String(); got != "[a b c]" {
		t.Errorf("String = %q, want \"[a b c]\"", got)
	}

	empty := FromSlice[string](nil)
	if !empty.IsEmpty() {
		t.Errorf("FromSlice(nil): not empty")
	}
	if got := empty.Slice(); got == nil || len(got) != 0 {
		t.Errorf("FromSlice(nil): Slice = %v, want empty non-nil", got)
	}

	f := FromSliceFunc(func(a, b int) int { return b - a }, []int{1, 2, 3, 2}) // descending order
	if got := f.String(); got != "[3 2 1]" {
		t.Errorf("FromSliceFunc(descending): String = %q, want \"[3 2 1]\"", got)
	}
}

// TestTreeSetCustomComparator covers the NewTreeSetFunc ordering surface:
// structs ordered by one field, and descending order.
type testItem struct {
	Key string
	N   int
}

func cmpTestItem(a, b testItem) int { return strings.Compare(a.Key, b.Key) }

func TestTreeSetCustomComparator(t *testing.T) {
	set := NewTreeSetFunc(cmpTestItem)
	set.Insert(testItem{"m", 1})
	set.Insert(testItem{"a", 1})
	got := set.Insert(testItem{"m", 2}) // duplicate key: replaces the satellite data
	if got {
		t.Errorf("Insert(duplicate key) = true, want false")
	}
	if set.Len() != 2 {
		t.Fatalf("Len = %d, want 2", set.Len())
	}
	item, found := set.Search(testItem{Key: "m"}) // a probe with only the key fields set
	if !found {
		t.Fatalf("Search(key-only probe) not found")
	}
	if item.N != 2 {
		t.Errorf("Search returned N = %d, want 2 (duplicate insert must replace)", item.N)
	}
	if got := set.String(); got != "[{a 1} {m 2}]" {
		t.Errorf("String = %q, want \"[{a 1} {m 2}]\"", got)
	}

	// Descending integers: the built-in ordering replaced by a reversed one.
	// Min and max are by the COMPARATOR's order — under a reversed one
	// FindMin is the largest value (the max-heap-via-reversed-comparator
	// trick).
	desc := NewTreeSetFunc(func(a, b int) int { return b - a })
	desc.InsertSlice([]int{5, 1, 9, 3})
	if got := desc.String(); got != "[9 5 3 1]" {
		t.Errorf("descending: String = %q, want \"[9 5 3 1]\"", got)
	}
	if v, found := desc.FindMin(); !found || v != 9 {
		t.Errorf("descending: FindMin = (%d, %v), want (9, true)", v, found)
	}
}

// TestTreeSetStringDeterminism: same elements inserted in different
// orders render identically — the in-order walk is already sorted, no
// sorting involved anywhere.
func TestTreeSetStringDeterminism(t *testing.T) {
	forward := FromSlice([]string{"a", "b", "c", "d", "e"})
	backward := FromSlice([]string{"e", "d", "c", "b", "a"})
	shuffled := FromSlice([]string{"c", "a", "e", "b", "d"})
	if forward.String() != backward.String() || forward.String() != shuffled.String() {
		t.Errorf("String differs across insertion orders: %q / %q / %q",
			forward.String(), backward.String(), shuffled.String())
	}
	if got := forward.StringFunc(strings.ToUpper); got != "[A B C D E]" {
		t.Errorf("StringFunc = %q, want \"[A B C D E]\"", got)
	}
	if got := forward.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want \"[]\"", got)
	}
}

// TestTreeSetClone covers the deep copy: independent afterwards, in both
// directions, and the zero-value clones to a fresh zero value.
func TestTreeSetClone(t *testing.T) {
	set := FromSlice([]string{"a", "b", "c"})
	dup := set.Clone()
	if !set.Equal(dup) {
		t.Fatalf("clone not Equal to the original")
	}

	set.Insert("x") // mutate the original
	dup.Delete("a") // and the clone, in the other direction
	if got, want := set.String(), "[a b c x]"; got != want {
		t.Errorf("original after clone mutations: %q, want %q", got, want)
	}
	if got, want := dup.String(), "[b c]"; got != want {
		t.Errorf("clone after clone mutations: %q, want %q", got, want)
	}

	var nilSet *TreeSet[string]
	if nilSet.Clone() != nil {
		t.Errorf("nil set clones to non-nil")
	}

	var zero TreeSet[string]
	zeroClone := zero.Clone()
	if zeroClone.Len() != 0 {
		t.Errorf("zero-value clone: Len = %d, want 0", zeroClone.Len())
	}
	expectInsertPanic(t, "zero-value clone keeps no comparator", "Insert", func() {
		zeroClone.Insert("x")
	})
}

// TestTreeSetEqual covers symmetry, reflexivity and the nil/empty cases.
func TestTreeSetEqual(t *testing.T) {
	a := FromSlice([]string{"a", "b", "c"})
	b := FromSlice([]string{"c", "b", "a", "a"}) // same set, inserted differently
	c := FromSlice([]string{"a", "b"})

	if !a.Equal(a) {
		t.Errorf("Equal is not reflexive")
	}
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("Equal is not symmetric on equal sets")
	}
	if a.Equal(c) || c.Equal(a) {
		t.Errorf("Equal true on different sizes")
	}

	var nilSet *TreeSet[string]
	if !nilSet.Equal(nilSet) {
		t.Errorf("nil vs nil: not equal")
	}
	if !nilSet.Equal(NewTreeSet[string]()) { // a fresh empty, still-built set
		t.Errorf("nil vs empty: not equal")
	}
	if a.Equal(nilSet) {
		t.Errorf("non-empty vs nil: equal")
	}
}

// TestTreeSetBatches covers the batch menu on one fixture.
func TestTreeSetBatches(t *testing.T) {
	set := FromSlice([]string{"a", "b", "c"})

	if !set.InsertSlice([]string{"c", "d", "d"}) {
		t.Errorf("InsertSlice with a new element = false, want true")
	}
	if set.InsertSlice([]string{"a"}) != false {
		t.Errorf("InsertSlice of only duplicates = true, want false")
	}
	if set.InsertSlice(nil) != false {
		t.Errorf("InsertSlice(nil) = true, want false")
	}
	if got, want := set.String(), "[a b c d]"; got != want {
		t.Errorf("after InsertSlice: %q, want %q", got, want)
	}

	if !set.ContainsSlice([]string{"b", "d"}) {
		t.Errorf("ContainsSlice(present pair) = false, want true")
	}
	if set.ContainsSlice([]string{"b", "zz"}) {
		t.Errorf("ContainsSlice(one missing) = true, want false")
	}
	if !set.ContainsSlice(nil) {
		t.Errorf("ContainsSlice(nil) = false, want true (vacuous)")
	}
	if !set.ContainsAny([]string{"zz", "c", "qq"}) {
		t.Errorf("ContainsAny(one present) = false, want true")
	}
	if set.ContainsAny([]string{"zz", "qq"}) {
		t.Errorf("ContainsAny(none present) = true, want false")
	}
	if set.ContainsAny(nil) {
		t.Errorf("ContainsAny(nil) = true, want false")
	}

	if !set.DeleteSlice([]string{"zz", "a"}) {
		t.Errorf("DeleteSlice(one present) = false, want true")
	}
	if set.DeleteSlice([]string{"zz"}) {
		t.Errorf("DeleteSlice(none present) = true, want false")
	}
	if got, want := set.String(), "[b c d]"; got != want {
		t.Errorf("after DeleteSlice: %q, want %q", got, want)
	}

	// InsertIter: fed from another set's All(), no interface between.
	other := FromSlice([]string{"d", "e"})
	if !set.InsertIter(other.All()) {
		t.Errorf("InsertIter with a new element = false, want true")
	}
	if set.InsertIter(set.All()) {
		t.Errorf("InsertIter(own All()) = true, want false (all duplicates)")
	}
	if set.InsertIter(nil) {
		t.Errorf("InsertIter(nil) = true, want false")
	}
	if got, want := set.String(), "[b c d e]"; got != want {
		t.Errorf("after InsertIter: %q, want %q", got, want)
	}
}

// TestTreeSetDeleteFunc covers the predicate bulk delete: half-delete
// with a full model re-verify, then delete-everything.
func TestTreeSetDeleteFunc(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	var items []string
	model := map[string]bool{}
	for len(model) < 60 {
		k := fmt.Sprintf("k%03d", rng.Intn(1000))
		if !model[k] {
			model[k] = true
			items = append(items, k)
		}
	}
	set := FromSlice(items)
	checkAgainstModel(t, "before DeleteFunc", set, model)

	if !set.DeleteFunc(func(v string) bool { return strings.HasSuffix(v, "7") }) {
		t.Errorf("DeleteFunc(matching) = false, want true")
	}
	for k := range model {
		if strings.HasSuffix(k, "7") {
			delete(model, k)
		}
	}
	checkAgainstModel(t, "after half DeleteFunc", set, model)

	if set.DeleteFunc(nil) {
		t.Errorf("DeleteFunc(nil) = true, want false")
	}
	if set.DeleteFunc(func(string) bool { return false }) {
		t.Errorf("DeleteFunc(no matches) = true, want false")
	}
	if !set.DeleteFunc(func(string) bool { return true }) {
		t.Errorf("DeleteFunc(all) = false, want true")
	}
	if !set.IsEmpty() {
		t.Errorf("after DeleteFunc(all): set not empty")
	}
}

// TestTreeSetAlgebraProperties runs the four algebra operations against
// map-computed references and the set-lattice laws.
func TestTreeSetAlgebraProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	mk := func(prefix string, n int) (map[string]bool, *TreeSet[string]) {
		m := map[string]bool{}
		for len(m) < n {
			m[fmt.Sprintf("%s%03d", prefix, rng.Intn(40))] = true // heavy overlap between a and b
		}
		return m, FromSlice(sortedKeys(m))
	}
	ma, a := mk("a", 25)
	mb, b := mk("b", 20)
	union, inter, minusAB, minusBA, symdiff := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for k := range ma {
		union[k] = true
		if mb[k] {
			inter[k] = true
		} else {
			minusAB[k] = true
		}
	}
	for k := range mb {
		union[k] = true
		if !ma[k] {
			minusBA[k] = true
		}
	}
	for k := range union {
		if !inter[k] {
			symdiff[k] = true
		}
	}

	run := func(op string, fx func(dst, yy, zz *TreeSet[string]), want map[string]bool) {
		t.Helper()
		dst := NewTreeSet[string]()
		fx(dst, a, b)
		if got := dst.Slice(); !slices.Equal(got, sortedKeys(want)) {
			t.Errorf("%s: Slice = %v, want %v", op, got, sortedKeys(want))
		}
		dst2 := NewTreeSet[string]()
		fx(dst2, b, a) // operands swapped: the commutative law
		if !dst.Equal(dst2) {
			t.Errorf("%s: not commutative", op)
		}
	}

	run("Union", (*TreeSet[string]).Union, union)
	run("Intersect", (*TreeSet[string]).Intersect, inter)
	run("SymmetricDifference", (*TreeSet[string]).SymmetricDifference, symdiff)

	// Minus is not commutative — check both directions explicitly.
	dst := NewTreeSet[string]()
	dst.Minus(a, b)
	if got := dst.Slice(); !slices.Equal(got, sortedKeys(minusAB)) {
		t.Errorf("Minus(a,b): Slice = %v, want %v", got, sortedKeys(minusAB))
	}
	if !dst.IsDisjointWith(b) {
		t.Errorf("Minus(a,b) shares an element with b: a-b and b must be disjoint")
	}
	dst = NewTreeSet[string]()
	dst.Minus(b, a)
	if got := dst.Slice(); !slices.Equal(got, sortedKeys(minusBA)) {
		t.Errorf("Minus(b,a): Slice = %v, want %v", got, sortedKeys(minusBA))
	}

	// a-b together with b-a is the symmetric difference.
	both := NewTreeSet[string]()
	both.Union(dst, func() *TreeSet[string] { d := NewTreeSet[string](); d.Minus(a, b); return d }())
	if !both.Equal(func() *TreeSet[string] { d := NewTreeSet[string](); d.SymmetricDifference(a, b); return d }()) {
		t.Errorf("(a-b) ∪ (b-a) != a △ b")
	}

	// Alias safety: every operand may alias the receiver.
	selfUnion := a.Clone()
	selfUnion.Union(selfUnion, selfUnion)
	if !selfUnion.Equal(a) {
		t.Errorf("s.Union(s,s) != s")
	}
	selfInter := a.Clone()
	selfInter.Intersect(selfInter, selfInter)
	if !selfInter.Equal(a) {
		t.Errorf("s.Intersect(s,s) != s")
	}
	selfMinus := a.Clone()
	selfMinus.Minus(selfMinus, selfMinus)
	if !selfMinus.IsEmpty() {
		t.Errorf("s.Minus(s,s) is not empty")
	}
	aThenB := a.Clone()
	aThenB.Union(aThenB, b) // receiver aliases the first operand
	if !aThenB.Equal(func() *TreeSet[string] { d := NewTreeSet[string](); d.Union(a, b); return d }()) {
		t.Errorf("s.Union(s,b) != Union(a,b): receiver-alias is broken")
	}

	// Union keeps zz's element when both hold an equal-by-cmp element
	// with different satellite data (the avl contract).
	sa := NewTreeSetFunc(cmpTestItem)
	sa.InsertSlice([]testItem{{"k", 1}, {"j", 0}})
	sb := NewTreeSetFunc(cmpTestItem)
	sb.InsertSlice([]testItem{{"k", 2}})
	rs := NewTreeSetFunc(cmpTestItem)
	rs.Union(sa, sb)
	if item, _ := rs.Search(testItem{Key: "k"}); item.N != 2 {
		t.Errorf("Union kept yy's element (N=%d), want zz's (N=2)", item.N)
	}
}

// TestTreeSetZeroValueAdoption: a zero-value destination adopts an
// operand's ordering and becomes a fully working set.
func TestTreeSetZeroValueAdoption(t *testing.T) {
	a := FromSlice([]string{"a", "c"})
	b := FromSlice([]string{"b", "c"})

	var zero TreeSet[string]
	zero.Union(a, b) // zero-value destination: adopts a's comparator
	if got, want := zero.String(), "[a b c]"; got != want {
		t.Errorf("adopting Union: %q, want %q", got, want)
	}
	if !zero.Insert("d") { // and is refillable afterwards
		t.Errorf("Insert after adoption = false, want true")
	}

	var zero2 TreeSet[string]
	zero2.Minus(a, b)
	if got, want := zero2.String(), "[a]"; got != want {
		t.Errorf("adopting Minus: %q, want %q", got, want)
	}

	// With no ordering anywhere the result is the empty zero value.
	var zero3, zero4 TreeSet[string]
	zero3.Union(&zero4, &zero4)
	if zero3.Len() != 0 {
		t.Errorf("no-comparator Union: Len = %d, want 0", zero3.Len())
	}
	expectInsertPanic(t, "no-comparator adoption leaves no comparator", "Insert", func() {
		zero3.Insert("x")
	})
}

// TestTreeSetRelations covers the five relation predicates against hand
// computed answers.
func TestTreeSetRelations(t *testing.T) {
	s := FromSlice([]string{"a", "b"})
	two := FromSlice([]string{"a", "b"})
	sup := FromSlice([]string{"a", "b", "c"})
	other := FromSlice([]string{"x", "y"})
	var nilSet *TreeSet[string]

	if !s.IsSubsetOf(s) || !s.IsSubsetOf(two) || !s.IsSubsetOf(sup) {
		t.Errorf("IsSubsetOf failed on a true case")
	}
	if s.IsSubsetOf(other) || sup.IsSubsetOf(s) {
		t.Errorf("IsSubsetOf true on a false case")
	}
	if s.IsProperSubsetOf(two) {
		t.Errorf("equal sets: IsProperSubsetOf = true, want false")
	}
	if !s.IsProperSubsetOf(sup) {
		t.Errorf("s ⊂ sup: IsProperSubsetOf = false, want true")
	}
	if !sup.IsSupersetOf(s) || s.IsSupersetOf(sup) {
		t.Errorf("IsSupersetOf wrong")
	}
	if !sup.IsProperSupersetOf(s) || s.IsProperSupersetOf(two) {
		t.Errorf("IsProperSupersetOf wrong")
	}
	if !s.IsDisjointWith(other) {
		t.Errorf("s and other share nothing: IsDisjointWith = false, want true")
	}
	if s.IsDisjointWith(two) || s.IsDisjointWith(sup) {
		t.Errorf("overlapping sets: IsDisjointWith = true, want false")
	}
	if !nilSet.IsDisjointWith(other) || !other.IsDisjointWith(nilSet) {
		t.Errorf("nil set is disjoint with everything")
	}
	if !nilSet.IsSubsetOf(other) || !nilSet.IsSubsetOf(nilSet) {
		t.Errorf("the empty set is a subset of every set")
	}
}

// TestTreeSetRiders covers the sorted-set riders: Floor/Ceil at, between
// and past the ends, Index bounds, and the head/tail drain order.
func TestTreeSetRiders(t *testing.T) {
	set := FromSlice([]string{"ant", "cat", "emu", "fox", "hog"})

	// Floor: largest element <= key.
	for _, tc := range []struct{ key, want string }{
		{"cat", "cat"},   // at an element
		{"cow", "cat"},   // between: rounds down
		{"aardvark", ""}, // below every element: not found
		{"zebra", "hog"}, // above every element: the max
	} {
		v, found := set.Floor(tc.key)
		if tc.want == "" {
			if found {
				t.Errorf("Floor(%q) found %q, want not found", tc.key, v)
			}
		} else if !found || v != tc.want {
			t.Errorf("Floor(%q) = (%q, %v), want (%q, true)", tc.key, v, found, tc.want)
		}
	}

	// Ceil: smallest element >= key.
	for _, tc := range []struct{ key, want string }{
		{"emu", "emu"},
		{"dog", "emu"}, // between: rounds up
		{"igloo", ""},  // above every element: not found
		{"aardvark", "ant"},
	} {
		v, found := set.Ceil(tc.key)
		if tc.want == "" {
			if found {
				t.Errorf("Ceil(%q) found %q, want not found", tc.key, v)
			}
		} else if !found || v != tc.want {
			t.Errorf("Ceil(%q) = (%q, %v), want (%q, true)", tc.key, v, found, tc.want)
		}
	}

	// Index: the pos-th element ascending, and the bounds.
	for pos, want := range []string{"ant", "cat", "emu", "fox", "hog"} {
		v, found := set.Index(pos)
		if !found || v != want {
			t.Errorf("Index(%d) = (%q, %v), want (%q, true)", pos, v, found, want)
		}
	}
	if _, found := set.Index(-1); found {
		t.Errorf("Index(-1) found, want not found")
	}
	if _, found := set.Index(5); found {
		t.Errorf("Index(len) found, want not found")
	}

	// DeleteAtHead/DeleteAtTail drain smallest-first / largest-first:
	// the element each delete removes is the pre-delete Index(0) /
	// Index(Len-1) — ascending head drain, descending tail drain.
	asc := FromSlice([]string{"d", "b", "e", "a", "c"})
	var headOrder []string
	for !asc.IsEmpty() {
		v, _ := asc.Index(0)
		headOrder = append(headOrder, v)
		if !asc.DeleteAtHead() {
			t.Fatalf("DeleteAtHead = false on a non-empty set")
		}
	}
	if want := []string{"a", "b", "c", "d", "e"}; !slices.Equal(headOrder, want) {
		t.Errorf("DeleteAtHead drain = %v, want %v", headOrder, want)
	}
	desc := FromSlice([]string{"d", "b", "e", "a", "c"})
	var tailOrder []string
	for !desc.IsEmpty() {
		v, _ := desc.Index(desc.Len() - 1)
		tailOrder = append(tailOrder, v)
		if !desc.DeleteAtTail() {
			t.Fatalf("DeleteAtTail = false on a non-empty set")
		}
	}
	if want := []string{"e", "d", "c", "b", "a"}; !slices.Equal(tailOrder, want) {
		t.Errorf("DeleteAtTail drain = %v, want %v", tailOrder, want)
	}
}

// TestTreeSetRange covers the bounded iterator: inclusive ends, lo > hi,
// empty sets, early break and a custom comparator.
func TestTreeSetRange(t *testing.T) {
	set := FromSlice([]string{"ant", "cat", "emu", "fox", "hog"})

	collect := func(seq iter.Seq[string]) []string {
		var out []string
		for v := range seq {
			out = append(out, v)
		}
		return out
	}

	if got := collect(set.Range("cat", "fox")); !slices.Equal(got, []string{"cat", "emu", "fox"}) {
		t.Errorf("Range(cat,fox) = %v, want [cat emu fox] (both ends inclusive)", got)
	}
	if got := collect(set.Range("cow", "gib")); !slices.Equal(got, []string{"emu", "fox"}) {
		t.Errorf("Range between elements = %v, want [emu fox]", got)
	}
	if got := collect(set.Range("cow", "fish")); !slices.Equal(got, []string{"emu"}) {
		t.Errorf("Range with hi between fox and emu = %v, want [emu] (\"fish\" < \"fox\")", got)
	}
	if got := collect(set.Range("emu", "cat")); got != nil {
		t.Errorf("Range with lo > hi = %v, want empty", got)
	}
	if got := collect(set.Range("aardvark", "zebra")); !slices.Equal(got, set.Slice()) {
		t.Errorf("full Range != Slice: %v vs %v", got, set.Slice())
	}
	if got := collect(set.Range("1", "9")); got != nil {
		t.Errorf("Range missing everything = %v, want empty", got)
	}

	broke := false
	for v := range set.Range("ant", "hog") {
		if v == "cat" {
			broke = true
			break // early exit must be safe
		}
	}
	if !broke {
		t.Errorf("early break did not happen")
	}

	var empty *TreeSet[string]
	if got := collect(empty.Range("a", "z")); got != nil {
		t.Errorf("Range on nil set = %v, want empty", got)
	}
	var zero TreeSet[string]
	if got := collect(zero.Range("a", "z")); got != nil {
		t.Errorf("Range on zero value = %v, want empty", got)
	}

	desc := NewTreeSetFunc(func(a, b int) int { return b - a })
	desc.InsertSlice([]int{1, 2, 3, 4, 5})
	var out []int
	for v := range desc.Range(4, 2) { // descending comparator: bounds are in ITS order
		out = append(out, v)
	}
	if want := []int{4, 3, 2}; !slices.Equal(out, want) {
		t.Errorf("descending Range = %v, want %v", out, want)
	}
}

// TestTreeSetIterators covers All, Backward and the walks.
func TestTreeSetIterators(t *testing.T) {
	set := FromSlice([]string{"c", "a", "b"})

	var asc []string
	for v := range set.All() {
		asc = append(asc, v)
	}
	if want := []string{"a", "b", "c"}; !slices.Equal(asc, want) {
		t.Errorf("All = %v, want %v", asc, want)
	}

	var desc []string
	for v := range set.Backward() {
		desc = append(desc, v)
	}
	if want := []string{"c", "b", "a"}; !slices.Equal(desc, want) {
		t.Errorf("Backward = %v, want %v", desc, want)
	}

	var pre []string
	set.WalkPreOrder(func(pos, depth int, data string) bool {
		pre = append(pre, data)
		return true
	})
	if len(pre) != 3 || !slices.Equal(slices.Sorted(slices.Values(pre)), []string{"a", "b", "c"}) {
		t.Errorf("WalkPreOrder visited %v, want all three elements", pre)
	}

	stopped := false
	set.WalkInOrder(func(pos, depth int, data string) bool {
		if data == "b" {
			stopped = true
			return false // stop the walk
		}
		return true
	})
	if !stopped {
		t.Errorf("WalkInOrder never reached the stop element")
	}

	var nilSet *TreeSet[string]
	count := 0
	for range nilSet.All() {
		count++
	}
	for range nilSet.Backward() {
		count++
	}
	nilSet.WalkInOrder(func(int, int, string) bool { count++; return true })
	if count != 0 {
		t.Errorf("nil set iterators visited %d elements, want 0", count)
	}
}

// TestTreeSetJSON covers the round trip: ascending canonical marshal,
// replace-on-unmarshal, comparator kept, error cases tolerated.
func TestTreeSetJSON(t *testing.T) {
	set := NewTreeSetFunc(func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	set.InsertSlice([]string{"Bravo", "alpha", "Charlie"})

	data, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got, want := string(data), `["alpha","Bravo","Charlie"]`; got != want {
		t.Errorf("Marshal = %s, want %s (ascending by the set's own comparator)", got, want)
	}

	// Unmarshal into a set with the same comparator: contents replaced,
	// comparator kept — the case-insensitive probe still finds.
	into := NewTreeSetFunc(func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	into.Insert("junk")
	if err := json.Unmarshal(data, &into); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got, want := into.Len(), 3; got != want {
		t.Errorf("after Unmarshal: Len = %d, want %d", got, want)
	}
	if !into.Contains("ALPHA") {
		t.Errorf("after Unmarshal: case-insensitive Contains lost — comparator not kept")
	}

	// A decode error leaves the set untouched.
	bad := NewTreeSet[string]()
	bad.Insert("keep")
	if err := json.Unmarshal([]byte(`{"nope":1}`), &bad); err == nil {
		t.Errorf("Unmarshal of a non-array: no error")
	}
	if got, want := bad.String(), "[keep]"; got != want {
		t.Errorf("after failed Unmarshal: %q, want %q (must be untouched)", got, want)
	}

	// An empty array clears, everywhere.
	if err := json.Unmarshal([]byte(`[]`), &bad); err != nil {
		t.Fatalf("Unmarshal([]): %v", err)
	}
	if !bad.IsEmpty() {
		t.Errorf("Unmarshal([]) did not clear the set")
	}
	var zero TreeSet[string]
	if err := json.Unmarshal([]byte(`[]`), &zero); err != nil {
		t.Errorf("Unmarshal([]) on a zero value: %v", err)
	}
	var nilSet *TreeSet[string]
	if err := json.Unmarshal([]byte(`[]`), &nilSet); err != nil {
		t.Errorf("Unmarshal([]) on nil: %v", err)
	}

	// nil / zero-value sets marshal as an empty array when called
	// directly — json.Marshal on a nil *TreeSet writes `null` before
	// MarshalJSON is ever called (the documented caveat, shared with
	// every other pluto structure).
	nilData, err := (*TreeSet[string])(nil).MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON on nil set: %v", err)
	}
	if got := string(nilData); got != "[]" {
		t.Errorf("MarshalJSON on nil set = %s, want []", got)
	}
	zeroData, err := (&TreeSet[string]{}).MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON on zero value: %v", err)
	}
	if got := string(zeroData); got != "[]" {
		t.Errorf("MarshalJSON on zero value = %s, want []", got)
	}
}

// TestTreeSetNilTolerated: every read on a nil set and on a zero-value
// set has a sane empty answer.
func TestTreeSetNilTolerated(t *testing.T) {
	var nilSet *TreeSet[string]
	var zero TreeSet[string]

	for name, set := range map[string]*TreeSet[string]{"nil": nilSet, "zero": &zero} {
		if set.Len() != 0 {
			t.Errorf("%s: Len = %d, want 0", name, set.Len())
		}
		if !set.IsEmpty() {
			t.Errorf("%s: IsEmpty = false, want true", name)
		}
		if set.Contains("x") {
			t.Errorf("%s: Contains = true", name)
		}
		if _, found := set.Search("x"); found {
			t.Errorf("%s: Search found", name)
		}
		if set.Delete("x") {
			t.Errorf("%s: Delete = true", name)
		}
		if set.DeleteAtHead() || set.DeleteAtTail() {
			t.Errorf("%s: DeleteAtHead/Tail = true", name)
		}
		if _, found := set.FindMin(); found {
			t.Errorf("%s: FindMin found", name)
		}
		if _, found := set.FindMax(); found {
			t.Errorf("%s: FindMax found", name)
		}
		if _, found := set.Floor("x"); found {
			t.Errorf("%s: Floor found", name)
		}
		if _, found := set.Ceil("x"); found {
			t.Errorf("%s: Ceil found", name)
		}
		if _, found := set.Index(0); found {
			t.Errorf("%s: Index(0) found", name)
		}
		if set.String() != "[]" {
			t.Errorf("%s: String = %q, want []", name, set.String())
		}
		if set.InsertSlice(nil) || set.InsertIter(nil) {
			t.Errorf("%s: empty batch insert = true", name)
		}
		if set.DeleteSlice([]string{"x"}) {
			t.Errorf("%s: DeleteSlice = true", name)
		}
		if set.DeleteFunc(func(string) bool { return true }) {
			t.Errorf("%s: DeleteFunc = true", name)
		}
		if !set.ContainsSlice(nil) {
			t.Errorf("%s: ContainsSlice(nil) = false, want vacuous true", name)
		}
		if set.ContainsAny([]string{"x"}) {
			t.Errorf("%s: ContainsAny = true", name)
		}
		set.Truncate() // tolerated, changes nothing
		n := 0
		for range set.All() {
			n++
		}
		for range set.Range("a", "z") {
			n++
		}
		set.WalkPostOrder(func(int, int, string) bool { n++; return true })
		if n != 0 {
			t.Errorf("%s: iterators visited %d elements", name, n)
		}
		dst := NewTreeSet[string]()
		dst.Union(set, set) // nil/zero operand: empty contribution
		if !dst.IsEmpty() {
			t.Errorf("%s: as algebra operand it contributed elements", name)
		}
	}
	if got := nilSet.Slice(); got != nil {
		t.Errorf("nil set: Slice = %v, want nil", got)
	}
	if got := zero.Slice(); got == nil || len(got) != 0 {
		t.Errorf("zero value: Slice = %v, want empty non-nil", got)
	}
}

// TestTreeSetPanics: the exact panic surface, with message checks.
func TestTreeSetPanics(t *testing.T) {
	expectPanic(t, "NewTreeSetFunc(nil)", func() {
		NewTreeSetFunc[string](nil)
	})
	expectPanic(t, "FromSliceFunc(nil, …)", func() {
		FromSliceFunc[string](nil, []string{"x"})
	})

	var nilSet *TreeSet[string]
	expectInsertPanic(t, "Insert on nil set", "Insert", func() {
		nilSet.Insert("x")
	})
	var zero TreeSet[string]
	expectInsertPanic(t, "Insert on zero value", "Insert", func() {
		zero.Insert("x")
	})
	expectInsertPanic(t, "InsertSlice on zero value", "InsertSlice", func() {
		zero.InsertSlice([]string{"x"})
	})
	expectInsertPanic(t, "InsertIter on zero value", "InsertIter", func() {
		zero.InsertIter(slices.Values([]string{"x"}))
	})
	expectInsertPanic(t, "UnmarshalJSON storing on zero value", "UnmarshalJSON", func() {
		_ = zero.UnmarshalJSON([]byte(`["x"]`))
	})

	// Empty batches are tolerated on the zero value — no panic.
	zero.InsertSlice(nil)
	zero.InsertIter(nil)
	if err := zero.UnmarshalJSON([]byte(`[]`)); err != nil {
		t.Errorf("UnmarshalJSON([]) on zero value: %v", err)
	}
}
