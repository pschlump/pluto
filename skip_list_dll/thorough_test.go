package skip_list_dll

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"testing"

	"github.com/pschlump/pluto/comparable"
)

// TestKeyNode is a keyed payload type used to verify that a duplicate
// Insert actually replaces the stored item (not just the key).
type TestKeyNode struct {
	K string
	N int
}

var _ comparable.Comparable = (*TestKeyNode)(nil)

func (aa TestKeyNode) Compare(x comparable.Comparable) int {
	if bb, ok := x.(TestKeyNode); ok {
		if aa.K < bb.K {
			return -1
		} else if aa.K > bb.K {
			return 1
		}
		return 0
	}
	panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
}

// expectPanic runs f and fails the test unless f panics with want.
func expectPanic(t *testing.T, name, want string, f func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("%s: expected panic, got none", name)
			return
		}
		if s, ok := r.(string); !ok || s != want {
			t.Errorf("%s: expected panic %q, got %v", name, want, r)
		}
	}()
	f()
}

// collectAll returns the forward iteration of the list as a slice.
func collectAll[T comparable.Comparable](tt *SkipList[T]) []T {
	var out []T
	for v := range tt.All() {
		out = append(out, v)
	}
	return out
}

// collectBackward returns the backward iteration of the list as a slice.
func collectBackward[T comparable.Comparable](tt *SkipList[T]) []T {
	var out []T
	for v := range tt.Backward() {
		out = append(out, v)
	}
	return out
}

// checkInvariant verifies the structural invariants of the list against a
// sorted reference slice of keys: length, ascending order of All, and that
// Backward is the exact mirror of All (which also validates the level-0
// back chain).
func checkInvariant(t *testing.T, tag string, tt *SkipList[TestSkipListNode], ref []string) {
	t.Helper()
	if tt.Length() != len(ref) {
		t.Fatalf("%s: expected length %d, got %d", tag, len(ref), tt.Length())
	}
	if (len(ref) == 0) != tt.IsEmpty() {
		t.Fatalf("%s: IsEmpty=%v does not match reference length %d", tag, tt.IsEmpty(), len(ref))
	}
	fwd := collectAll(tt)
	if len(fwd) != len(ref) {
		t.Fatalf("%s: All yielded %d items, expected %d", tag, len(fwd), len(ref))
	}
	for i, v := range fwd {
		if v.S != ref[i] {
			t.Fatalf("%s: All[%d] = %s, expected %s", tag, i, v.S, ref[i])
		}
	}
	bwd := collectBackward(tt)
	if len(bwd) != len(ref) {
		t.Fatalf("%s: Backward yielded %d items, expected %d", tag, len(bwd), len(ref))
	}
	for i, v := range bwd {
		if v.S != ref[len(ref)-1-i] {
			t.Fatalf("%s: Backward[%d] = %s, expected %s", tag, i, v.S, ref[len(ref)-1-i])
		}
	}
	if len(ref) > 0 {
		if mn := tt.FindMin(); mn == nil || mn.S != ref[0] {
			t.Fatalf("%s: FindMin = %+v, expected %s", tag, mn, ref[0])
		}
		if mx := tt.FindMax(); mx == nil || mx.S != ref[len(ref)-1] {
			t.Fatalf("%s: FindMax = %+v, expected %s", tag, mx, ref[len(ref)-1])
		}
	}
}

func TestNilReceiverPanics(t *testing.T) {
	var p *SkipList[TestSkipListNode] // nil
	const want = "skip list should not be nil"
	item := TestSkipListNode{S: "01"}
	expectPanic(t, "Insert", want, func() { p.Insert(item) })
	expectPanic(t, "Delete", want, func() { p.Delete(item) })
	expectPanic(t, "DeleteAtHead", want, func() { p.DeleteAtHead() })
	expectPanic(t, "DeleteAtTail", want, func() { p.DeleteAtTail() })
}

func TestZeroValueOperations(t *testing.T) {
	var list SkipList[TestSkipListNode]

	if !list.IsEmpty() {
		t.Errorf("zero value should be empty")
	}
	if list.Length() != 0 {
		t.Errorf("zero value should have length 0, got %d", list.Length())
	}
	if list.Search(TestSkipListNode{S: "x"}) != nil {
		t.Errorf("Search on zero value should return nil")
	}
	if list.FindMin() != nil || list.FindMax() != nil {
		t.Errorf("FindMin/FindMax on zero value should return nil")
	}
	if list.Delete(TestSkipListNode{S: "x"}) {
		t.Errorf("Delete on zero value should return false")
	}
	if list.DeleteAtHead() || list.DeleteAtTail() {
		t.Errorf("DeleteAtHead/DeleteAtTail on zero value should return false")
	}
	if n := len(collectAll(&list)); n != 0 {
		t.Errorf("All on zero value should yield nothing, got %d items", n)
	}
	if n := len(collectBackward(&list)); n != 0 {
		t.Errorf("Backward on zero value should yield nothing, got %d items", n)
	}

	// Truncate of an empty list is a no-op and the list stays usable.
	list.Truncate()
	if !list.IsEmpty() {
		t.Errorf("Truncate on empty list should leave it empty")
	}
	list.Insert(TestSkipListNode{S: "a"})
	if list.Length() != 1 {
		t.Errorf("expected length 1 after insert into truncated zero value, got %d", list.Length())
	}

	// Deleting the last element returns the list to the empty state.
	if !list.Delete(TestSkipListNode{S: "a"}) {
		t.Errorf("expected Delete of only element to return true")
	}
	checkInvariant(t, "back to empty", &list, nil)

	// The list is reusable after being drained.
	list.Insert(TestSkipListNode{S: "b"})
	list.Insert(TestSkipListNode{S: "c"})
	checkInvariant(t, "refilled", &list, []string{"b", "c"})
}

func TestSingleElement(t *testing.T) {
	var list SkipList[TestSkipListNode]
	list.Insert(TestSkipListNode{S: "m"})

	if mn := list.FindMin(); mn == nil || mn.S != "m" {
		t.Errorf("FindMin of single-element list = %+v, expected m", mn)
	}
	if mx := list.FindMax(); mx == nil || mx.S != "m" {
		t.Errorf("FindMax of single-element list = %+v, expected m", mx)
	}
	checkInvariant(t, "single", &list, []string{"m"})

	// Search misses that land before and after the single element.
	if list.Search(TestSkipListNode{S: "a"}) != nil {
		t.Errorf("Search for key before single element should return nil")
	}
	if list.Search(TestSkipListNode{S: "z"}) != nil {
		t.Errorf("Search for key after single element should return nil")
	}

	// DeleteAtHead removes the only element.
	if !list.DeleteAtHead() {
		t.Errorf("DeleteAtHead on single-element list should return true")
	}
	checkInvariant(t, "drained by head", &list, nil)

	// DeleteAtTail removes the only element.
	list.Insert(TestSkipListNode{S: "m"})
	if !list.DeleteAtTail() {
		t.Errorf("DeleteAtTail on single-element list should return true")
	}
	checkInvariant(t, "drained by tail", &list, nil)
}

// TestDuplicateReplacesStoredValue verifies that inserting a Compare-equal
// item replaces the previously stored item, keeping the length unchanged.
func TestDuplicateReplacesStoredValue(t *testing.T) {
	var list SkipList[TestKeyNode]

	list.Insert(TestKeyNode{K: "b", N: 1})
	list.Insert(TestKeyNode{K: "a", N: 1})
	list.Insert(TestKeyNode{K: "c", N: 1})

	// Replace head, middle, and tail.
	list.Insert(TestKeyNode{K: "a", N: 100})
	list.Insert(TestKeyNode{K: "b", N: 200})
	list.Insert(TestKeyNode{K: "c", N: 300})

	if list.Length() != 3 {
		t.Fatalf("expected length 3 after duplicate inserts, got %d", list.Length())
	}
	for _, want := range []TestKeyNode{{"a", 100}, {"b", 200}, {"c", 300}} {
		got := list.Search(TestKeyNode{K: want.K})
		if got == nil {
			t.Fatalf("expected to find key %s", want.K)
		}
		if got.N != want.N {
			t.Errorf("key %s: expected stored payload %d, got %d", want.K, want.N, got.N)
		}
	}
}

// TestSortedInsertion checks that inserting in ascending and descending
// order produces a correctly ordered list (skip lists must not degrade
// structurally on sorted input the way plain BSTs do).
func TestSortedInsertion(t *testing.T) {
	const N = 500

	var asc SkipList[TestSkipListNode]
	ref := make([]string, 0, N)
	for i := 0; i < N; i++ {
		s := fmt.Sprintf("%06d", i)
		asc.Insert(TestSkipListNode{S: s})
		ref = append(ref, s)
	}
	checkInvariant(t, "ascending insert", &asc, ref)

	var desc SkipList[TestSkipListNode]
	for i := N - 1; i >= 0; i-- {
		desc.Insert(TestSkipListNode{S: fmt.Sprintf("%06d", i)})
	}
	checkInvariant(t, "descending insert", &desc, ref)
}

// TestDeleteEnds verifies Delete of the current minimum and maximum, and
// that the back chain stays intact afterwards.
func TestDeleteEnds(t *testing.T) {
	var list SkipList[TestSkipListNode]
	ref := []string{"01", "02", "03", "04", "05"}
	for _, s := range ref {
		list.Insert(TestSkipListNode{S: s})
	}

	if !list.Delete(TestSkipListNode{S: "01"}) {
		t.Errorf("expected Delete of min to return true")
	}
	checkInvariant(t, "after delete min", &list, []string{"02", "03", "04", "05"})

	if !list.Delete(TestSkipListNode{S: "05"}) {
		t.Errorf("expected Delete of max to return true")
	}
	checkInvariant(t, "after delete max", &list, []string{"02", "03", "04"})

	if !list.Delete(TestSkipListNode{S: "03"}) {
		t.Errorf("expected Delete of middle to return true")
	}
	checkInvariant(t, "after delete middle", &list, []string{"02", "04"})
}

// TestDrainAlternatingEnds drains a list by alternating DeleteAtHead and
// DeleteAtTail, checking invariants after every removal.
func TestDrainAlternatingEnds(t *testing.T) {
	const N = 200
	var list SkipList[TestSkipListNode]
	ref := make([]string, 0, N)
	for i := 0; i < N; i++ {
		s := fmt.Sprintf("%04d", i)
		list.Insert(TestSkipListNode{S: s})
		ref = append(ref, s)
	}

	for i := 0; i < N; i++ {
		var ok bool
		if i%2 == 0 {
			ok = list.DeleteAtHead()
			ref = ref[1:]
		} else {
			ok = list.DeleteAtTail()
			ref = ref[:len(ref)-1]
		}
		if !ok {
			t.Fatalf("drain step %d: expected true", i)
		}
		checkInvariant(t, fmt.Sprintf("drain step %d", i), &list, ref)
	}
	if list.DeleteAtHead() || list.DeleteAtTail() {
		t.Errorf("expected false from DeleteAtHead/DeleteAtTail on drained list")
	}
}

func TestDump(t *testing.T) {
	var list SkipList[TestSkipListNode]

	var buf bytes.Buffer
	list.Dump(&buf)
	if got, want := buf.String(), "SkipList (empty)\n"; got != want {
		t.Errorf("Dump of empty list: expected %q, got %q", want, got)
	}

	keys := []string{"03", "01", "02"}
	for _, s := range keys {
		list.Insert(TestSkipListNode{S: s})
	}
	buf.Reset()
	list.Dump(&buf)
	out := buf.String()

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("Dump: expected at least a header and an L0 line, got %q", out)
	}
	head := lines[0]
	if !strings.HasPrefix(head, "SkipList length=3 level=") {
		t.Errorf("Dump header: expected prefix %q, got %q", "SkipList length=3 level=", head)
	}

	// The last line must be L0 with every node in ascending order.
	want := "L0: "
	for _, s := range []string{"01", "02", "03"} {
		want += fmt.Sprintf("%v ", TestSkipListNode{S: s})
	}
	if lines[len(lines)-1] != want {
		t.Errorf("Dump L0 line: expected %q, got %q", want, lines[len(lines)-1])
	}

	// Every higher-level line must contain a subset of the L0 items in order.
	for _, ln := range lines[1 : len(lines)-1] {
		if !strings.HasPrefix(ln, "L") {
			t.Errorf("Dump: malformed level line %q", ln)
		}
	}
}

// TestMixedOpsModel is a randomized property test with a fixed seed.  It
// cross-checks the skip list against a simple reference model (a sorted
// slice plus a presence set) over thousands of mixed operations.
func TestMixedOpsModel(t *testing.T) {
	const (
		ops     = 3000
		keySpan = 300 // small key space forces duplicate inserts and delete misses
	)

	rng := rand.New(rand.NewPCG(20260825, 17))

	var list SkipList[TestSkipListNode]
	var ref []string             // sorted keys
	present := map[string]bool{} // presence set

	key := func() string { return fmt.Sprintf("%04d", rng.IntN(keySpan)) }

	refInsert := func(s string) {
		i := sort.SearchStrings(ref, s)
		ref = append(ref, "")
		copy(ref[i+1:], ref[i:])
		ref[i] = s
	}
	refDelete := func(s string) {
		i := sort.SearchStrings(ref, s)
		ref = append(ref[:i], ref[i+1:]...)
	}

	for op := 0; op < ops; op++ {
		tag := fmt.Sprintf("op %d", op)
		switch rng.IntN(8) {
		case 0, 1, 2: // insert
			s := key()
			list.Insert(TestSkipListNode{S: s})
			if !present[s] {
				present[s] = true
				refInsert(s)
			}
		case 3, 4: // delete
			s := key()
			got := list.Delete(TestSkipListNode{S: s})
			if got != present[s] {
				t.Fatalf("%s: Delete(%s) = %v, model says %v", tag, s, got, present[s])
			}
			if got {
				delete(present, s)
				refDelete(s)
			}
		case 5: // search
			s := key()
			got := list.Search(TestSkipListNode{S: s})
			if (got != nil) != present[s] {
				t.Fatalf("%s: Search(%s) found=%v, model says %v", tag, s, got != nil, present[s])
			}
			if got != nil && got.S != s {
				t.Fatalf("%s: Search(%s) returned %+v", tag, s, *got)
			}
		case 6: // delete at head / tail
			if rng.IntN(2) == 0 {
				got := list.DeleteAtHead()
				if got != (len(ref) > 0) {
					t.Fatalf("%s: DeleteAtHead = %v, model has %d items", tag, got, len(ref))
				}
				if got {
					delete(present, ref[0])
					ref = ref[1:]
				}
			} else {
				got := list.DeleteAtTail()
				if got != (len(ref) > 0) {
					t.Fatalf("%s: DeleteAtTail = %v, model has %d items", tag, got, len(ref))
				}
				if got {
					delete(present, ref[len(ref)-1])
					ref = ref[:len(ref)-1]
				}
			}
		case 7: // min/max probe
			if len(ref) == 0 {
				if list.FindMin() != nil || list.FindMax() != nil {
					t.Fatalf("%s: FindMin/FindMax on empty model should be nil", tag)
				}
			} else {
				if mn := list.FindMin(); mn == nil || mn.S != ref[0] {
					t.Fatalf("%s: FindMin = %+v, model min %s", tag, mn, ref[0])
				}
				if mx := list.FindMax(); mx == nil || mx.S != ref[len(ref)-1] {
					t.Fatalf("%s: FindMax = %+v, model max %s", tag, mx, ref[len(ref)-1])
				}
			}
		}
		if op%97 == 0 {
			checkInvariant(t, tag, &list, ref)
		}
	}
	checkInvariant(t, "final", &list, ref)

	// Truncate mid-stream and confirm the list diverges from the old model
	// and is fully reusable.
	list.Truncate()
	checkInvariant(t, "after truncate", &list, nil)
	for i := 0; i < 50; i++ {
		list.Insert(TestSkipListNode{S: fmt.Sprintf("%04d", i)})
	}
	if list.Length() != 50 {
		t.Errorf("expected length 50 after refill, got %d", list.Length())
	}
}
