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
	"sync"
	"testing"
)

// expectPanic runs fn and fails the test unless it panics.
func expectPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
		}
	}()
	fn()
}

// TestNilReceiverPanics verifies the documented panic when Insert is
// called on a nil list — the one operation with no sane answer.
func TestNilReceiverPanics(t *testing.T) {
	var List1 *SkipList[TestSkipListNode]
	key := TestSkipListNode{S: "12"}

	expectPanic(t, "Insert", func() { List1.Insert(key) })

	// Verify the panic message names the method.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected Insert to panic on nil list.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "Insert") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		List1.Insert(key)
	}()
}

// TestNilReceiverTolerated verifies that every operation other than Insert
// treats a nil list as an empty list instead of panicking.
func TestNilReceiverTolerated(t *testing.T) {
	var List1 *SkipList[TestSkipListNode]
	key := TestSkipListNode{S: "12"}

	if !List1.IsEmpty() {
		t.Errorf("Expected nil list to be empty.")
	}
	if List1.Len() != 0 || List1.Length() != 0 {
		t.Errorf("Expected nil list to have length 0.")
	}
	if _, found := List1.Search(key); found {
		t.Errorf("Expected not-found from Search on nil list.")
	}
	if List1.Delete(key) {
		t.Errorf("Expected false from Delete on nil list.")
	}
	if _, found := List1.FindMin(); found {
		t.Errorf("Expected not-found from FindMin on nil list.")
	}
	if _, found := List1.FindMax(); found {
		t.Errorf("Expected not-found from FindMax on nil list.")
	}
	if List1.DeleteAtHead() || List1.DeleteAtTail() {
		t.Errorf("Expected false from DeleteAtHead/DeleteAtTail on nil list.")
	}
	List1.Truncate() // no-op, must not panic

	for range List1.All() {
		t.Errorf("Expected no values from All on nil list.")
	}
	for range List1.Backward() {
		t.Errorf("Expected no values from Backward on nil list.")
	}

	var buf bytes.Buffer
	List1.Dump(&buf)
	if buf.String() != "SkipList (empty)\n" {
		t.Errorf("Expected empty dump message on nil list, got %q", buf.String())
	}
}

// checkInvariant verifies the internal structural invariants of the list:
// the number of level-0 nodes equals Length(), every level is in strictly
// ascending order, head.forward[i] is non-nil exactly for i < level, and no
// node participates in a level >= tt.level.
func checkInvariant(t *testing.T, tt *SkipList[TestSkipListNode], where string) {
	t.Helper()

	if tt.length == 0 {
		if tt.level != 0 {
			t.Errorf("%s: empty list should have level 0, got %d", where, tt.level)
		}
		// The sentinel head may still be allocated after a drain, but it must
		// not link to any node.
		if tt.head != nil {
			for i := range maxLevel {
				if tt.head.forward[i] != nil {
					t.Errorf("%s: empty list head.forward[%d] is non-nil", where, i)
				}
			}
		}
		return
	}
	if tt.head == nil {
		t.Fatalf("%s: non-empty list has nil head", where)
	}
	if tt.level < 1 || tt.level > maxLevel {
		t.Errorf("%s: level %d out of range [1,%d]", where, tt.level, maxLevel)
	}

	// head.forward[i] must be non-nil for i < level and nil for i >= level.
	for i := range maxLevel {
		if i < tt.level {
			if tt.head.forward[i] == nil {
				t.Errorf("%s: head.forward[%d] is nil but level=%d", where, i, tt.level)
			}
		} else {
			if tt.head.forward[i] != nil {
				t.Errorf("%s: head.forward[%d] is non-nil but level=%d", where, i, tt.level)
			}
		}
	}

	// Every level must be strictly ascending and the level-0 chain must have
	// exactly length nodes.
	for i := 0; i < tt.level; i++ {
		n := 0
		var prev *string
		for cur := tt.head.forward[i]; cur != nil; cur = cur.forward[i] {
			if len(cur.forward) > tt.level {
				t.Errorf("%s: node %s has %d forward pointers but list level is %d",
					where, cur.data.S, len(cur.forward), tt.level)
			}
			if len(cur.span) != len(cur.forward) {
				t.Errorf("%s: node %s has %d forward pointers but %d spans",
					where, cur.data.S, len(cur.forward), len(cur.span))
			}
			if prev != nil && *prev >= cur.data.S {
				t.Errorf("%s: level %d not strictly ascending: %s then %s", where, i, *prev, cur.data.S)
			}
			s := cur.data.S
			prev = &s
			n++
		}
		if i == 0 && n != tt.length {
			t.Errorf("%s: level-0 chain has %d nodes but Length()=%d", where, n, tt.length)
		}
	}

	// Span invariant: span[i] on a link counts the level-0 nodes in
	// (node, forward[i]], and when forward[i] is nil it counts the remaining
	// nodes after this one — so the spans along any level, sentinel head
	// included, always sum to Length().  Every link to a real node skips at
	// least that node.
	for i := 0; i < tt.level; i++ {
		sum := 0
		cur := tt.head
		for {
			if cur.forward[i] != nil {
				if cur.span[i] < 1 {
					t.Errorf("%s: level %d: link to %s has span %d < 1",
						where, i, cur.forward[i].data.S, cur.span[i])
				}
			} else if i < len(cur.span) && cur.span[i] < 0 {
				t.Errorf("%s: level %d: nil link from %s has negative span %d",
					where, i, cur.data.S, cur.span[i])
			}
			sum += cur.span[i]
			if cur.forward[i] == nil {
				break
			}
			cur = cur.forward[i]
		}
		if sum != tt.length {
			t.Errorf("%s: level %d spans sum to %d but Length()=%d", where, i, sum, tt.length)
		}
	}
}

// TestSingleElement exercises every operation on a list holding exactly one
// item.
func TestSingleElement(t *testing.T) {
	List1 := newTestList()

	List1.Insert(TestSkipListNode{S: "12"})
	checkInvariant(t, List1, "after single insert")

	mn, mnOK := List1.FindMin()
	mx, mxOK := List1.FindMax()
	if !mnOK || !mxOK || mn.S != "12" || mx.S != "12" {
		t.Errorf("Expected FindMin/FindMax of single-item list to be 12/12, got %+v/%+v", mn, mx)
	}

	// Duplicate insert replaces the data but keeps the length at 1.
	List1.Insert(TestSkipListNode{S: "12"})
	if List1.Length() != 1 {
		t.Errorf("Expected length of 1 after duplicate insert, got %d", List1.Length())
	}
	checkInvariant(t, List1, "after duplicate insert")

	// Iterators yield exactly one item.
	n := 0
	for v := range List1.All() {
		n++
		if v.S != "12" {
			t.Errorf("All: expected 12, got %s", v.S)
		}
	}
	if n != 1 {
		t.Errorf("All: expected 1 item from single-item list, got %d", n)
	}
	n = 0
	for v := range List1.Backward() {
		n++
		if v.S != "12" {
			t.Errorf("Backward: expected 12, got %s", v.S)
		}
	}
	if n != 1 {
		t.Errorf("Backward: expected 1 item from single-item list, got %d", n)
	}

	// Remove the only element via Delete.
	if !List1.Delete(TestSkipListNode{S: "12"}) {
		t.Errorf("Expected delete of single item to return true")
	}
	if !List1.IsEmpty() || List1.Length() != 0 {
		t.Errorf("Expected empty list after deleting only item, length=%d", List1.Length())
	}
	checkInvariant(t, List1, "after deleting only item")
	if _, found := List1.FindMin(); found {
		t.Errorf("Expected FindMin of emptied list to report not-found")
	}
	if _, found := List1.FindMax(); found {
		t.Errorf("Expected FindMax of emptied list to report not-found")
	}

	// Insert again and remove via DeleteAtHead.
	List1.Insert(TestSkipListNode{S: "34"})
	if !List1.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead of single-item list to return true")
	}
	if !List1.IsEmpty() {
		t.Errorf("Expected empty list after DeleteAtHead of only item")
	}
	checkInvariant(t, List1, "after DeleteAtHead of only item")

	// Insert again and remove via DeleteAtTail.
	List1.Insert(TestSkipListNode{S: "56"})
	if !List1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail of single-item list to return true")
	}
	if !List1.IsEmpty() {
		t.Errorf("Expected empty list after DeleteAtTail of only item")
	}
	checkInvariant(t, List1, "after DeleteAtTail of only item")
}

// TestSortedAndReverseInsert verifies the claim that a skip list does not
// degrade on sorted input: inserting in ascending and descending order must
// still produce a correct, sorted list.
func TestSortedAndReverseInsert(t *testing.T) {
	const N = 2000

	asc := newTestList()
	for i := range N {
		asc.Insert(TestSkipListNode{S: fmt.Sprintf("%06d", i)})
	}
	if asc.Length() != N {
		t.Errorf("Expected length %d, got %d", N, asc.Length())
	}
	if mn, found := asc.FindMin(); !found || mn.S != "000000" {
		t.Errorf("Expected min 000000, got %+v", mn)
	}
	if mx, found := asc.FindMax(); !found || mx.S != fmt.Sprintf("%06d", N-1) {
		t.Errorf("Expected max %06d, got %+v", N-1, mx)
	}
	checkInvariant(t, asc, "ascending insert")

	desc := newTestList()
	for i := N - 1; i >= 0; i-- {
		desc.Insert(TestSkipListNode{S: fmt.Sprintf("%06d", i)})
	}
	checkInvariant(t, desc, "descending insert")
	prev := ""
	first := true
	for v := range desc.All() {
		if !first && prev >= v.S {
			t.Fatalf("All after descending inserts not ascending: %s then %s", prev, v.S)
		}
		first = false
		prev = v.S
	}
}

// TestDeleteLevelsDrop verifies that the internal level shrinks when the
// tallest nodes are removed and that the level-0 chain stays correct.
func TestDeleteLevelsDrop(t *testing.T) {
	const N = 500

	List1 := newTestList()
	for i := range N {
		List1.Insert(TestSkipListNode{S: fmt.Sprintf("%06d", i)})
	}
	maxSeen := List1.level
	if maxSeen < 2 {
		t.Fatalf("Test setup: expected level >= 2 with %d nodes, got %d", N, maxSeen)
	}

	// Delete from the tail down; the level must never exceed the max seen and
	// the invariant must hold after every delete.
	for i := N - 1; i >= 0; i-- {
		if !List1.DeleteAtTail() {
			t.Fatalf("DeleteAtTail of %06d failed", i)
		}
		if List1.level > maxSeen {
			t.Errorf("Level grew to %d beyond max %d during deletes", List1.level, maxSeen)
		}
	}
	checkInvariant(t, List1, "after tail drain")
	if List1.level != 0 {
		t.Errorf("Expected level 0 after draining, got %d", List1.level)
	}

	// Same, draining from the head.
	for i := range N {
		List1.Insert(TestSkipListNode{S: fmt.Sprintf("%06d", i)})
	}
	for i := range N {
		if !List1.DeleteAtHead() {
			t.Fatalf("DeleteAtHead %d failed", i)
		}
	}
	checkInvariant(t, List1, "after head drain")
	if !List1.IsEmpty() {
		t.Errorf("Expected empty list after draining from head")
	}
}

// TestDeleteNonMembers verifies that deleting items that sort between,
// before, or after existing members returns false and changes nothing.
func TestDeleteNonMembers(t *testing.T) {
	List1 := newTestList()
	for _, s := range []string{"10", "20", "30"} {
		List1.Insert(TestSkipListNode{S: s})
	}
	before := List1.Length()

	for _, s := range []string{"05", "15", "25", "35"} {
		if List1.Delete(TestSkipListNode{S: s}) {
			t.Errorf("Expected delete of non-member %s to return false", s)
		}
	}
	if List1.Length() != before {
		t.Errorf("Expected length to stay %d, got %d", before, List1.Length())
	}
	for _, s := range []string{"10", "20", "30"} {
		if _, found := List1.Search(TestSkipListNode{S: s}); !found {
			t.Errorf("Expected %s to still be present", s)
		}
	}
	checkInvariant(t, List1, "after deleting non-members")
}

// TestBackwardEarlyBreak verifies that breaking out of the Backward
// iterator stops it immediately.
func TestBackwardEarlyBreak(t *testing.T) {
	List1 := newTestList()
	for _, s := range []string{"01", "02", "03", "04", "05"} {
		List1.Insert(TestSkipListNode{S: s})
	}

	n := 0
	var first string
	for v := range List1.Backward() {
		n++
		first = v.S
		break
	}
	if n != 1 {
		t.Errorf("Expected early break to yield exactly 1 item, got %d", n)
	}
	if first != "05" {
		t.Errorf("Expected first Backward item to be the max (05), got %s", first)
	}

	// Take exactly two items, in descending order.
	var got []string
	for v := range List1.Backward() {
		got = append(got, v.S)
		if len(got) == 2 {
			break
		}
	}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", []string{"05", "04"}) {
		t.Errorf("Expected [05 04] from partial Backward, got %v", got)
	}
}

// TestAllEarlyBreakFirstItem verifies All stops after a break on the first
// (smallest) item.
func TestAllEarlyBreakFirstItem(t *testing.T) {
	List1 := newTestList()
	for _, s := range []string{"01", "02", "03"} {
		List1.Insert(TestSkipListNode{S: s})
	}
	var first string
	n := 0
	for v := range List1.All() {
		first = v.S
		n++
		break
	}
	if n != 1 || first != "01" {
		t.Errorf("Expected first All item to be 01 (n=1), got %s (n=%d)", first, n)
	}
}

// TestDump verifies the debugging output for both an empty and a populated
// list.  It uses a plain string list so each node prints as a single
// whitespace token.
func TestDump(t *testing.T) {
	List1 := NewSkipList[string]()

	var buf bytes.Buffer
	List1.Dump(&buf)
	if got := buf.String(); got != "SkipList (empty)\n" {
		t.Errorf("Dump of empty list: expected %q, got %q", "SkipList (empty)\n", got)
	}

	const N = 100
	for i := range N {
		List1.Insert(fmt.Sprintf("%06d", i))
	}

	buf.Reset()
	List1.Dump(&buf)
	out := buf.String()

	head := fmt.Sprintf("SkipList length=%d level=%d\n", N, List1.level)
	if !strings.HasPrefix(out, head) {
		t.Errorf("Dump header: expected prefix %q, got %q", head, out)
	}

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 1+List1.level {
		t.Fatalf("Expected 1 header + %d level lines, got %d lines", List1.level, len(lines))
	}
	for i := 1; i < len(lines); i++ {
		wantPrefix := fmt.Sprintf("L%d: ", List1.level-i)
		if !strings.HasPrefix(lines[i], wantPrefix) {
			t.Errorf("Dump line %d: expected prefix %q, got %q", i, wantPrefix, lines[i])
		}
	}

	// The bottom level (L0) must list every node in ascending order.
	l0 := lines[len(lines)-1]
	if !strings.HasPrefix(l0, "L0: ") {
		t.Fatalf("Expected last line to be L0, got %q", l0)
	}
	fields := strings.Fields(strings.TrimPrefix(l0, "L0: "))
	if len(fields) != N {
		t.Errorf("L0: expected %d items, got %d", N, len(fields))
	}
	if !sort.StringsAreSorted(fields) {
		t.Errorf("L0: items not in ascending order")
	}

	// Higher levels must be subsequences of level 0.
	inL0 := make(map[string]bool, len(fields))
	for _, f := range fields {
		inL0[f] = true
	}
	for i := 1; i < len(lines)-1; i++ {
		lvlFields := strings.Fields(lines[i])
		if len(lvlFields) < 1 {
			t.Errorf("Dump line %d is empty: %q", i, lines[i])
			continue
		}
		for _, f := range lvlFields[1:] { // skip the "Ln:" label itself
			if !inL0[f] {
				t.Errorf("%s: item %s not present in L0", lvlFields[0], f)
			}
		}
	}
}

// TestMixedRandomized runs hundreds of mixed operations against a reference
// model (a sorted-set built from a map) using a fixed seed, and
// cross-checks the list after every step.
func TestMixedRandomized(t *testing.T) {
	const Ops = 4000
	const KeySpace = 300 // small key space forces plenty of duplicates

	rng := rand.New(rand.NewPCG(20260825, 99))

	List1 := newTestList()
	ref := make(map[string]bool)

	key := func(n int) string { return fmt.Sprintf("%06d", n) }

	// sortedRef returns the reference keys in ascending order.
	sortedRef := func() []string {
		keys := make([]string, 0, len(ref))
		for k := range ref {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	}

	check := func(step int) {
		t.Helper()
		if List1.Length() != len(ref) {
			t.Fatalf("step %d: expected length %d, got %d", step, len(ref), List1.Length())
		}
		keys := sortedRef()
		if len(keys) == 0 {
			if !List1.IsEmpty() {
				t.Errorf("step %d: expected IsEmpty, length=%d", step, List1.Length())
			}
			if _, found := List1.FindMin(); found {
				t.Errorf("step %d: expected not-found FindMin on empty list", step)
			}
			if _, found := List1.FindMax(); found {
				t.Errorf("step %d: expected not-found FindMax on empty list", step)
			}
		} else {
			if mn, found := List1.FindMin(); !found || mn.S != keys[0] {
				t.Errorf("step %d: expected min %s, got %+v", step, keys[0], mn)
			}
			if mx, found := List1.FindMax(); !found || mx.S != keys[len(keys)-1] {
				t.Errorf("step %d: expected max %s, got %+v", step, keys[len(keys)-1], mx)
			}
		}
	}

	for i := range Ops {
		k := key(rng.IntN(KeySpace))
		switch rng.IntN(6) {
		case 0, 1, 2: // Insert (duplicates replace)
			added := List1.Insert(TestSkipListNode{S: k})
			if added == ref[k] {
				t.Fatalf("step %d: Insert(%s)=%v, model said present=%v", i, k, added, ref[k])
			}
			ref[k] = true
		case 3: // Delete
			got := List1.Delete(TestSkipListNode{S: k})
			want := ref[k]
			if got != want {
				t.Fatalf("step %d: Delete(%s) returned %v, model says %v", i, k, got, want)
			}
			delete(ref, k)
		case 4: // Search
			_, found := List1.Search(TestSkipListNode{S: k})
			want := ref[k]
			if found != want {
				t.Fatalf("step %d: Search(%s) found=%v, model says %v", i, k, found, want)
			}
		case 5: // DeleteAtHead / DeleteAtTail
			keys := sortedRef()
			if rng.IntN(2) == 0 {
				got := List1.DeleteAtHead()
				if len(keys) == 0 {
					if got {
						t.Fatalf("step %d: DeleteAtHead on empty list returned true", i)
					}
				} else {
					if !got {
						t.Fatalf("step %d: DeleteAtHead returned false on non-empty list", i)
					}
					delete(ref, keys[0])
				}
			} else {
				got := List1.DeleteAtTail()
				if len(keys) == 0 {
					if got {
						t.Fatalf("step %d: DeleteAtTail on empty list returned true", i)
					}
				} else {
					if !got {
						t.Fatalf("step %d: DeleteAtTail returned false on non-empty list", i)
					}
					delete(ref, keys[len(keys)-1])
				}
			}
		}
		if i%100 == 0 {
			checkInvariant(t, List1, fmt.Sprintf("step %d", i))
		}
	}
	check(Ops)

	// Final full-iteration cross-check in both directions.
	keys := sortedRef()
	var fwd []string
	for v := range List1.All() {
		fwd = append(fwd, v.S)
	}
	if fmt.Sprintf("%v", fwd) != fmt.Sprintf("%v", keys) {
		t.Errorf("Final All: expected %v, got %v", keys, fwd)
	}
	var bwd []string
	for v := range List1.Backward() {
		bwd = append(bwd, v.S)
	}
	for i := range keys {
		want := keys[len(keys)-1-i]
		if i >= len(bwd) || bwd[i] != want {
			t.Fatalf("Final Backward: position %d expected %s, got %v", i, want, bwd)
		}
	}
	if len(bwd) != len(keys) {
		t.Errorf("Final Backward: expected %d items, got %d", len(keys), len(bwd))
	}

	// Truncate mid-stream and rebuild, verifying the list stays correct.
	List1.Truncate()
	ref = make(map[string]bool)
	check(Ops)
	for range 100 {
		k := key(rng.IntN(KeySpace))
		List1.Insert(TestSkipListNode{S: k})
		ref[k] = true
	}
	checkInvariant(t, List1, "after rebuild")
}

// TestZeroValueOperations exercises read-only operations on a freshly
// declared zero-value list before any insert, and verifies that Insert
// fails loudly because no comparison function has been set.
func TestZeroValueOperations(t *testing.T) {
	var List1 SkipList[TestSkipListNode]

	if !List1.IsEmpty() {
		t.Errorf("Expected zero-value list to be empty")
	}
	if List1.Len() != 0 || List1.Length() != 0 {
		t.Errorf("Expected zero-value list length 0, got %d", List1.Length())
	}
	if _, found := List1.Search(TestSkipListNode{S: "12"}); found {
		t.Errorf("Expected Search on zero-value list to report not-found")
	}
	if _, found := List1.FindMin(); found {
		t.Errorf("Expected FindMin on zero-value list to report not-found")
	}
	if _, found := List1.FindMax(); found {
		t.Errorf("Expected FindMax on zero-value list to report not-found")
	}
	if List1.Delete(TestSkipListNode{S: "12"}) {
		t.Errorf("Expected Delete on zero-value list to return false")
	}
	if List1.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead on zero-value list to return false")
	}
	if List1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail on zero-value list to return false")
	}

	n := 0
	for range List1.All() {
		n++
	}
	for range List1.Backward() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected zero items from iterators on zero-value list, got %d", n)
	}

	// Truncate on a zero-value list is a no-op.
	List1.Truncate()
	if !List1.IsEmpty() {
		t.Errorf("Expected zero-value list to be empty after Truncate")
	}

	// Insert without a comparison function panics with a clear message.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("Expected Insert on zero-value list to panic.")
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewSkipList") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		List1.Insert(TestSkipListNode{S: "12"})
	}()

	// A list from the constructor remains fully usable after the same drain.
	live := newTestList()
	live.Truncate()
	live.Insert(TestSkipListNode{S: "12"})
	if live.Length() != 1 {
		t.Errorf("Expected length 1 after insert into truncated list, got %d", live.Length())
	}
}

// -------------------------------------------------------------------------------------------------------
// Thread-safety: snapshot iterators and concurrent access
// -------------------------------------------------------------------------------------------------------

// TestListIterateSnapshot verifies that the All/Backward iterators operate
// on a snapshot taken when they are called: later modifications — even
// truncating the whole list — are not observed, and mutating the list from
// inside the loop is safe.
func TestListIterateSnapshot(t *testing.T) {
	list := newTestList()
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		list.Insert(TestSkipListNode{S: s})
	}

	all := list.All()
	backward := list.Backward()

	list.Truncate() // the iterators above must not observe this

	expect := []string{"00", "02", "03", "05", "09"}

	var got []string
	for v := range all {
		got = append(got, v.S)
	}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", expect) {
		t.Errorf("All after Truncate error, expected %v got %v", expect, got)
	}

	var gotB []string
	for v := range backward {
		gotB = append(gotB, v.S)
	}
	for i := range expect {
		if gotB[i] != expect[len(expect)-1-i] {
			t.Fatalf("Backward after Truncate error, expected reverse of %v got %v", expect, gotB)
		}
	}

	// Mutating from inside the loop is safe: the loop sees the snapshot.
	list = newTestList()
	for _, s := range []string{"05", "02", "09"} {
		list.Insert(TestSkipListNode{S: s})
	}
	visited := 0
	for v := range list.All() {
		visited++
		list.Delete(v)
	}
	if visited != 3 {
		t.Errorf("Expected 3 visits while deleting during iteration, got %d", visited)
	}
	if !list.IsEmpty() {
		t.Errorf("Expected empty list after deleting every visited element.")
	}
}

// TestListConcurrent runs writers (each owning a disjoint key range)
// against a reader that iterates snapshots and queries metadata in a
// tight loop.  It is primarily a test for the race detector (`make race`);
// it also verifies that every operation reports success and that the list
// ends up empty and consistent.
func TestListConcurrent(t *testing.T) {
	list := NewSkipListFunc(cmpTestSkipListNode)

	// Fixed probes for the reader's new positional/range reads.
	kLo := TestSkipListNode{S: "00-0000"}
	kHi := TestSkipListNode{S: "99-9999"}
	kMid := TestSkipListNode{S: "42"}

	const workers = 8
	const perWorker = 200

	stop := make(chan struct{})
	var writers sync.WaitGroup

	// Reader: iterate snapshots and query metadata while the writers work.
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			for range list.All() {
			}
			for range list.Backward() {
			}
			for i, v := range list.Range(kLo, kHi) {
				_, _ = i, v
			}
			for i, v := range list.RangeBackward(kLo, kHi) {
				_, _ = i, v
			}
			_ = list.Len()
			_ = list.IsEmpty()
			_, _ = list.FindMin()
			_, _ = list.FindMax()
			_, _ = list.Rank(kMid)
			_, _ = list.AtIndex(0)
			_, _ = list.Ceil(kMid)
			_, _ = list.Floor(kMid)
			_ = list.CountRange(kLo, kHi)
		}
	}()

	for w := range workers {
		writers.Add(1)
		go func(w int) {
			defer writers.Done()
			for i := range perWorker {
				k := TestSkipListNode{S: fmt.Sprintf("%02d-%04d", w, i)}
				if !list.Insert(k) {
					t.Errorf("worker %d: Insert(%s) returned false", w, k.S)
					return
				}
			}
			for i := range perWorker {
				k := TestSkipListNode{S: fmt.Sprintf("%02d-%04d", w, i)}
				if _, found := list.Search(k); !found {
					t.Errorf("worker %d: Search(%s) not found", w, k.S)
				}
				if !list.Delete(k) {
					t.Errorf("worker %d: Delete(%s) returned false", w, k.S)
				}
			}
		}(w)
	}

	writers.Wait()
	close(stop)

	if !list.IsEmpty() || list.Length() != 0 {
		t.Errorf("Expected empty list after concurrent insert/delete, got length %d", list.Length())
	}
}
