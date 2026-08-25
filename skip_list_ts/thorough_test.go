package skip_list_ts

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

// TestKeyedNode has a payload separate from the sort key so that duplicate
// replacement can be observed: two items can Compare equal (same Key) while
// carrying different Val data.
type TestKeyedNode struct {
	Key string
	Val int
}

// At compile time verify that this is a correct type/interface setup.
var _ comparable.Comparable = (*TestKeyedNode)(nil)

// Compare implements the Compare function to satisfy the interface
// requirements.  Only Key participates in the ordering.
func (aa TestKeyedNode) Compare(x comparable.Comparable) int {
	bb, ok := x.(TestKeyedNode)
	if !ok {
		panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
	}
	if aa.Key < bb.Key {
		return -1
	} else if aa.Key > bb.Key {
		return 1
	}
	return 0
}

// checkInvariants verifies the internal structural invariants of the list:
// the level-0 chain holds exactly length items in ascending order, and the
// head's forward pointers agree with tt.level.
func checkInvariants[T comparable.Comparable](t *testing.T, tt *SkipList[T], where string) {
	t.Helper()
	if tt.length < 0 {
		t.Fatalf("%s: negative length %d", where, tt.length)
	}
	if tt.length == 0 && tt.head == nil {
		if tt.level != 0 {
			t.Fatalf("%s: empty list with nil head has level %d", where, tt.level)
		}
		return
	}
	if tt.head == nil {
		t.Fatalf("%s: non-empty list (length=%d) has nil head", where, tt.length)
	}
	// Levels above tt.level must be empty; the top used level must not be.
	for i := tt.level; i < maxLevel; i++ {
		if tt.head.forward[i] != nil {
			t.Fatalf("%s: head.forward[%d] non-nil above level %d", where, i, tt.level)
		}
	}
	if tt.level > 0 && tt.head.forward[tt.level-1] == nil {
		t.Fatalf("%s: head.forward[%d] nil at top level %d", where, tt.level-1, tt.level)
	}
	// Walk level 0: count nodes and verify ascending order.
	n := 0
	var prev *T
	for cur := tt.head.forward[0]; cur != nil; cur = cur.forward[0] {
		if prev != nil && (*prev).Compare(*cur.data) >= 0 {
			t.Fatalf("%s: level-0 chain out of order at item %d", where, n)
		}
		cp := *cur.data
		prev = &cp
		n++
	}
	if n != tt.length {
		t.Fatalf("%s: level-0 chain has %d nodes, length says %d", where, n, tt.length)
	}
}

// TestNilReceiverPanics verifies the documented panics on a nil list.
func TestNilReceiverPanics(t *testing.T) {
	var nilList *SkipList[TestSkipListNode]

	mustPanic := func(name string, f func()) {
		t.Helper()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected %s on nil list to panic", name)
			}
		}()
		f()
	}

	mustPanic("Insert", func() { nilList.Insert(TestSkipListNode{S: "00"}) })
	mustPanic("Delete", func() { nilList.Delete(TestSkipListNode{S: "00"}) })
	mustPanic("DeleteAtHead", func() { nilList.DeleteAtHead() })
	mustPanic("DeleteAtTail", func() { nilList.DeleteAtTail() })
}

// TestZeroValueOperations exercises every read-only operation on a freshly
// declared (zero value) list.
func TestZeroValueOperations(t *testing.T) {
	var List1 SkipList[TestSkipListNode]
	checkInvariants(t, &List1, "zero value")

	if !List1.IsEmpty() {
		t.Errorf("Expected zero value list to be empty")
	}
	if List1.Length() != 0 {
		t.Errorf("Expected zero value length of 0, got %d", List1.Length())
	}
	if p := List1.Search(TestSkipListNode{S: "x"}); p != nil {
		t.Errorf("Expected Search on zero value list to return nil, got %+v", *p)
	}
	if p := List1.FindMin(); p != nil {
		t.Errorf("Expected FindMin on zero value list to return nil, got %+v", *p)
	}
	if p := List1.FindMax(); p != nil {
		t.Errorf("Expected FindMax on zero value list to return nil, got %+v", *p)
	}

	// Truncate on an empty list must be a no-op and leave it usable.
	List1.Truncate()
	checkInvariants(t, &List1, "truncate of empty")
	if !List1.IsEmpty() {
		t.Errorf("Expected list to be empty after truncating an empty list")
	}
	List1.Insert(TestSkipListNode{S: "01"})
	if List1.Length() != 1 {
		t.Errorf("Expected length 1 after insert post-truncate, got %d", List1.Length())
	}
}

// TestSingleElement covers the one-element edge cases.
func TestSingleElement(t *testing.T) {
	var List1 SkipList[TestSkipListNode]
	List1.Insert(TestSkipListNode{S: "42"})
	checkInvariants(t, &List1, "single element")

	if mn := List1.FindMin(); mn == nil || mn.S != "42" {
		t.Errorf("Expected min of 42, got %+v", mn)
	}
	if mx := List1.FindMax(); mx == nil || mx.S != "42" {
		t.Errorf("Expected max of 42, got %+v", mx)
	}

	// Elements on either side of the single item are absent.
	if p := List1.Search(TestSkipListNode{S: "41"}); p != nil {
		t.Errorf("Expected Search(41) to return nil, got %+v", *p)
	}
	if p := List1.Search(TestSkipListNode{S: "43"}); p != nil {
		t.Errorf("Expected Search(43) to return nil, got %+v", *p)
	}

	// DeleteAtTail on a single-element list must empty it (head == tail).
	if !List1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail on single-element list to return true")
	}
	checkInvariants(t, &List1, "after DeleteAtTail of single element")
	if !List1.IsEmpty() {
		t.Errorf("Expected empty list after DeleteAtTail of single element")
	}

	// Same for DeleteAtHead.
	List1.Insert(TestSkipListNode{S: "42"})
	if !List1.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead on single-element list to return true")
	}
	checkInvariants(t, &List1, "after DeleteAtHead of single element")
	if !List1.IsEmpty() {
		t.Errorf("Expected empty list after DeleteAtHead of single element")
	}
}

// TestDuplicateReplacesData verifies that re-inserting a Compare-equal item
// replaces the stored data and does not change the length.
func TestDuplicateReplacesData(t *testing.T) {
	var List1 SkipList[TestKeyedNode]

	List1.Insert(TestKeyedNode{Key: "k1", Val: 1})
	List1.Insert(TestKeyedNode{Key: "k2", Val: 2})
	List1.Insert(TestKeyedNode{Key: "k1", Val: 100})

	if List1.Length() != 2 {
		t.Errorf("Expected length of 2 after duplicate insert, got %d", List1.Length())
	}
	p := List1.Search(TestKeyedNode{Key: "k1"})
	if p == nil {
		t.Fatalf("Expected to find k1 in list")
	}
	if p.Val != 100 {
		t.Errorf("Expected duplicate insert to replace data (Val=100), got Val=%d", p.Val)
	}

	// The duplicate of the current head and tail must also be replaced.
	List1.Insert(TestKeyedNode{Key: "k2", Val: 200})
	if mx := List1.FindMax(); mx == nil || mx.Val != 200 {
		t.Errorf("Expected max Val of 200 after replace, got %+v", mx)
	}
	List1.Insert(TestKeyedNode{Key: "k1", Val: 300})
	if mn := List1.FindMin(); mn == nil || mn.Val != 300 {
		t.Errorf("Expected min Val of 300 after replace, got %+v", mn)
	}
	if List1.Length() != 2 {
		t.Errorf("Expected length of 2, got %d", List1.Length())
	}
	checkInvariants(t, &List1, "duplicate replace")
}

// TestSearchReturnsCopy verifies that Search, FindMin and FindMax hand back
// copies: mutating the returned value must not corrupt the list.
func TestSearchReturnsCopy(t *testing.T) {
	var List1 SkipList[TestKeyedNode]
	List1.Insert(TestKeyedNode{Key: "a", Val: 1})
	List1.Insert(TestKeyedNode{Key: "b", Val: 2})

	p := List1.Search(TestKeyedNode{Key: "a"})
	if p == nil {
		t.Fatalf("Expected to find a in list")
	}
	p.Val = 999
	if q := List1.Search(TestKeyedNode{Key: "a"}); q == nil || q.Val != 1 {
		t.Errorf("Mutating Search result changed the list: %+v", q)
	}

	if mn := List1.FindMin(); mn != nil {
		mn.Val = 888
	}
	if q := List1.FindMin(); q == nil || q.Val != 1 {
		t.Errorf("Mutating FindMin result changed the list: %+v", q)
	}

	if mx := List1.FindMax(); mx != nil {
		mx.Val = 777
	}
	if q := List1.FindMax(); q == nil || q.Val != 2 {
		t.Errorf("Mutating FindMax result changed the list: %+v", q)
	}
}

// TestDeleteBoundariesAndGaps deletes the min, the max, and probes for keys
// that fall between stored keys.
func TestDeleteBoundariesAndGaps(t *testing.T) {
	var List1 SkipList[TestSkipListNode]
	for _, s := range []string{"10", "20", "30", "40", "50"} {
		List1.Insert(TestSkipListNode{S: s})
	}

	// Gap probes: below min, between keys, above max.
	for _, s := range []string{"05", "15", "25", "35", "45", "55"} {
		if List1.Delete(TestSkipListNode{S: s}) {
			t.Errorf("Expected delete of absent %s to return false", s)
		}
		if List1.Search(TestSkipListNode{S: s}) != nil {
			t.Errorf("Expected search of absent %s to return nil", s)
		}
	}
	if List1.Length() != 5 {
		t.Fatalf("Expected length of 5, got %d", List1.Length())
	}

	// Delete min then max.
	if !List1.Delete(TestSkipListNode{S: "10"}) {
		t.Errorf("Expected delete of min (10) to return true")
	}
	checkInvariants(t, &List1, "after deleting min")
	if mn := List1.FindMin(); mn == nil || mn.S != "20" {
		t.Errorf("Expected min of 20 after deleting 10, got %+v", mn)
	}
	if !List1.Delete(TestSkipListNode{S: "50"}) {
		t.Errorf("Expected delete of max (50) to return true")
	}
	checkInvariants(t, &List1, "after deleting max")
	if mx := List1.FindMax(); mx == nil || mx.S != "40" {
		t.Errorf("Expected max of 40 after deleting 50, got %+v", mx)
	}
	if List1.Length() != 3 {
		t.Errorf("Expected length of 3, got %d", List1.Length())
	}
}

// TestBackwardEarlyBreak covers the early-exit path of the Backward
// iterator, and checks that only the expected prefix is produced.
func TestBackwardEarlyBreak(t *testing.T) {
	var List1 SkipList[TestSkipListNode]
	for _, s := range []string{"01", "02", "03", "04", "05"} {
		List1.Insert(TestSkipListNode{S: s})
	}

	var got []string
	for v := range List1.Backward() {
		got = append(got, v.S)
		if len(got) == 2 {
			break
		}
	}
	want := []string{"05", "04"}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Errorf("Backward with early break: expected %v, got %v", want, got)
	}

	// Early break in All must also stop immediately after the first item.
	got = nil
	for v := range List1.All() {
		got = append(got, v.S)
		break
	}
	want = []string{"01"}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Errorf("All with early break: expected %v, got %v", want, got)
	}
}

// TestIteratorsUseSnapshot verifies that All/Backward iterate over a
// snapshot: mutating the list from inside the loop body is safe (no
// deadlock, no race) and does not change what the iteration yields.
func TestIteratorsUseSnapshot(t *testing.T) {
	var List1 SkipList[TestSkipListNode]
	for _, s := range []string{"01", "02", "03"} {
		List1.Insert(TestSkipListNode{S: s})
	}

	var got []string
	for v := range List1.All() {
		got = append(got, v.S)
		// Mutations during iteration must not affect the snapshot.
		List1.Insert(TestSkipListNode{S: "9" + v.S})
		List1.Delete(TestSkipListNode{S: "02"})
	}
	want := []string{"01", "02", "03"}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Errorf("All snapshot: expected %v, got %v", want, got)
	}

	got = nil
	for v := range List1.Backward() {
		got = append(got, v.S)
		List1.Truncate()
	}
	if len(got) == 0 {
		t.Errorf("Backward snapshot: expected items even though list was truncated mid-iteration")
	}
	// The snapshot must have been in descending order.
	sorted := sort.SliceIsSorted(got, func(i, j int) bool { return got[i] > got[j] })
	if !sorted {
		t.Errorf("Backward snapshot: items not in descending order: %v", got)
	}
}

// TestDump verifies the debugging dump of both an empty and a populated
// list.
func TestDump(t *testing.T) {
	var List1 SkipList[TestSkipListNode]

	var buf bytes.Buffer
	List1.Dump(&buf)
	if got := buf.String(); got != "SkipList (empty)\n" {
		t.Errorf("Dump of empty list: expected %q, got %q", "SkipList (empty)\n", got)
	}

	items := []string{"05", "02", "09", "00", "03", "07"}
	for _, s := range items {
		List1.Insert(TestSkipListNode{S: s})
	}

	buf.Reset()
	List1.Dump(&buf)
	out := buf.String()

	header := fmt.Sprintf("SkipList length=%d level=", len(items))
	if !strings.HasPrefix(out, header) {
		t.Errorf("Dump: expected output to start with %q, got:\n%s", header, out)
	}

	// The L0 line must list every item in ascending order.  Items print in
	// their %v form, e.g. "{05}" for TestSkipListNode{S: "05"}.
	sorted := append([]string(nil), items...)
	sort.Strings(sorted)
	rendered := make([]string, len(sorted))
	for i, s := range sorted {
		rendered[i] = fmt.Sprintf("%v", TestSkipListNode{S: s})
	}
	wantL0 := "L0: " + strings.Join(rendered, " ") + " \n"
	foundL0 := false
	for _, line := range strings.SplitAfter(out, "\n") {
		if strings.HasPrefix(line, "L0: ") {
			foundL0 = true
			if line != wantL0 {
				t.Errorf("Dump: expected L0 line %q, got %q", wantL0, line)
			}
		}
	}
	if !foundL0 {
		t.Errorf("Dump: no L0 line in output:\n%s", out)
	}

	// Higher levels list a subset of L0, in order.
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if !strings.HasPrefix(line, "L") || strings.HasPrefix(line, "L0: ") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, strings.SplitN(line, ": ", 2)[0]+": "))
		pos := 0
		for _, f := range fields {
			for pos < len(rendered) && rendered[pos] != f {
				pos++
			}
			if pos == len(rendered) {
				t.Errorf("Dump: item %q on line %q not in ascending L0 sequence", f, line)
				break
			}
			pos++
		}
	}
}

// TestTruncateRestoresZeroState checks that truncate fully resets the list
// and that it behaves like new afterwards.
func TestTruncateRestoresZeroState(t *testing.T) {
	var List1 SkipList[TestSkipListNode]
	for i := 0; i < 500; i++ {
		List1.Insert(TestSkipListNode{S: fmt.Sprintf("%04d", i)})
	}
	List1.Truncate()
	checkInvariants(t, &List1, "after truncate")

	if List1.Delete(TestSkipListNode{S: "0001"}) {
		t.Errorf("Expected delete after truncate to return false")
	}
	if List1.DeleteAtHead() || List1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtHead/DeleteAtTail after truncate to return false")
	}
	if List1.FindMin() != nil || List1.FindMax() != nil {
		t.Errorf("Expected FindMin/FindMax after truncate to return nil")
	}
	for range List1.All() {
		t.Errorf("Expected no items from All after truncate")
	}
}

// TestMixedOpsAgainstModel is a randomized property test with a fixed seed.
// It performs hundreds of mixed operations and cross-checks the list against
// a simple reference model (a sorted-set map) after each step.
func TestMixedOpsAgainstModel(t *testing.T) {
	const ops = 3000
	const keySpace = 200 // small key space forces duplicates and delete misses

	rng := rand.New(rand.NewPCG(12345, 67890))

	var List1 SkipList[TestKeyedNode]
	model := make(map[string]int) // key -> Val

	key := func(n int) string { return fmt.Sprintf("%03d", n) }

	sortedKeys := func() []string {
		keys := make([]string, 0, len(model))
		for k := range model {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	}

	verify := func(step int) {
		t.Helper()
		if List1.Length() != len(model) {
			t.Fatalf("step %d: length mismatch: list=%d model=%d", step, List1.Length(), len(model))
		}
		keys := sortedKeys()
		if len(keys) == 0 {
			if !List1.IsEmpty() {
				t.Fatalf("step %d: list not empty but model is", step)
			}
			if List1.FindMin() != nil || List1.FindMax() != nil {
				t.Fatalf("step %d: FindMin/FindMax non-nil on empty model", step)
			}
			return
		}
		if List1.IsEmpty() {
			t.Fatalf("step %d: list empty but model has %d keys", step, len(model))
		}
		if mn := List1.FindMin(); mn == nil || mn.Key != keys[0] || mn.Val != model[keys[0]] {
			t.Fatalf("step %d: min mismatch: got %+v want key=%s", step, mn, keys[0])
		}
		if mx := List1.FindMax(); mx == nil || mx.Key != keys[len(keys)-1] || mx.Val != model[keys[len(keys)-1]] {
			t.Fatalf("step %d: max mismatch: got %+v want key=%s", step, mx, keys[len(keys)-1])
		}
		// Full iteration must match the model exactly, in order.
		i := 0
		for v := range List1.All() {
			if i >= len(keys) {
				t.Fatalf("step %d: All yielded more than %d items", step, len(keys))
			}
			if v.Key != keys[i] || v.Val != model[keys[i]] {
				t.Fatalf("step %d: All[%d] = %+v, want key=%s val=%d", step, i, v, keys[i], model[keys[i]])
			}
			i++
		}
		if i != len(keys) {
			t.Fatalf("step %d: All yielded %d items, want %d", step, i, len(keys))
		}
		// Backward must be the exact reverse.
		i = len(keys) - 1
		for v := range List1.Backward() {
			if i < 0 {
				t.Fatalf("step %d: Backward yielded too many items", step)
			}
			if v.Key != keys[i] || v.Val != model[keys[i]] {
				t.Fatalf("step %d: Backward mismatch at %d: %+v want key=%s", step, i, v, keys[i])
			}
			i--
		}
		if i != -1 {
			t.Fatalf("step %d: Backward yielded too few items", step)
		}
	}

	for step := 0; step < ops; step++ {
		k := key(rng.IntN(keySpace))
		switch rng.IntN(10) {
		case 0, 1, 2, 3: // insert (40%)
			val := rng.IntN(10000)
			List1.Insert(TestKeyedNode{Key: k, Val: val})
			model[k] = val
		case 4, 5, 6: // delete (30%)
			_, inModel := model[k]
			if got := List1.Delete(TestKeyedNode{Key: k}); got != inModel {
				t.Fatalf("step %d: Delete(%s) = %v, model says present=%v", step, k, got, inModel)
			}
			delete(model, k)
		case 7: // search hit/miss (10%)
			val, inModel := model[k]
			p := List1.Search(TestKeyedNode{Key: k})
			if inModel {
				if p == nil || p.Val != val {
					t.Fatalf("step %d: Search(%s) = %+v, model has val=%d", step, k, p, val)
				}
			} else if p != nil {
				t.Fatalf("step %d: Search(%s) = %+v, model says absent", step, k, *p)
			}
		case 8: // DeleteAtHead (10%)
			keys := sortedKeys()
			if got := List1.DeleteAtHead(); got != (len(keys) > 0) {
				t.Fatalf("step %d: DeleteAtHead = %v, model size=%d", step, got, len(keys))
			}
			if len(keys) > 0 {
				delete(model, keys[0])
			}
		case 9: // DeleteAtTail (10%)
			keys := sortedKeys()
			if got := List1.DeleteAtTail(); got != (len(keys) > 0) {
				t.Fatalf("step %d: DeleteAtTail = %v, model size=%d", step, got, len(keys))
			}
			if len(keys) > 0 {
				delete(model, keys[len(keys)-1])
			}
		}
		if step%50 == 0 {
			verify(step)
			checkInvariants(t, &List1, fmt.Sprintf("step %d", step))
		}
	}
	verify(ops)
	checkInvariants(t, &List1, "final")
}

// TestSortedAndReverseSortedInsert covers the degenerate insertion orders.
func TestSortedAndReverseSortedInsert(t *testing.T) {
	const N = 500

	var asc SkipList[TestSkipListNode]
	for i := 0; i < N; i++ {
		asc.Insert(TestSkipListNode{S: fmt.Sprintf("%06d", i)})
	}
	checkInvariants(t, &asc, "ascending insert")
	if asc.Length() != N {
		t.Errorf("Expected length %d, got %d", N, asc.Length())
	}
	if mn := asc.FindMin(); mn == nil || mn.S != "000000" {
		t.Errorf("Expected min 000000, got %+v", mn)
	}
	if mx := asc.FindMax(); mx == nil || mx.S != fmt.Sprintf("%06d", N-1) {
		t.Errorf("Expected max %06d, got %+v", N-1, mx)
	}

	var desc SkipList[TestSkipListNode]
	for i := N - 1; i >= 0; i-- {
		desc.Insert(TestSkipListNode{S: fmt.Sprintf("%06d", i)})
	}
	checkInvariants(t, &desc, "descending insert")
	if desc.Length() != N {
		t.Errorf("Expected length %d, got %d", N, desc.Length())
	}

	// Drain the ascending list from the head: every DeleteAtHead must remove
	// the current minimum, in order.
	for i := 0; i < N; i++ {
		if !asc.DeleteAtHead() {
			t.Fatalf("DeleteAtHead %d failed", i)
		}
		if asc.Length() > 0 {
			if mn := asc.FindMin(); mn == nil || mn.S != fmt.Sprintf("%06d", i+1) {
				t.Fatalf("After DeleteAtHead %d, expected min %06d, got %+v", i, i+1, mn)
			}
		}
	}
	checkInvariants(t, &asc, "drained from head")

	// Drain the descending list from the tail: every DeleteAtTail must
	// remove the current maximum, in order.
	for i := N - 1; i >= 0; i-- {
		if !desc.DeleteAtTail() {
			t.Fatalf("DeleteAtTail %d failed", i)
		}
		if desc.Length() > 0 {
			if mx := desc.FindMax(); mx == nil || mx.S != fmt.Sprintf("%06d", i-1) {
				t.Fatalf("After DeleteAtTail %d, expected max %06d, got %+v", i, i-1, mx)
			}
		}
	}
	checkInvariants(t, &desc, "drained from tail")
}
