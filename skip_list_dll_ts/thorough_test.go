package skip_list_dll_ts

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
)

// mustPanic runs f and reports an error if f does not panic.
func mustPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected %s to panic, it did not", name)
		}
	}()
	f()
}

// TestNilReceiverPanics verifies the documented panics for a nil list
// receiver on Insert, Delete, DeleteAtHead and DeleteAtTail.
func TestNilReceiverPanics(t *testing.T) {
	var nilList *SkipList[TestSkipListNode]

	mustPanic(t, "Insert on nil list", func() {
		nilList.Insert(TestSkipListNode{S: "01"})
	})
	mustPanic(t, "Delete on nil list", func() {
		nilList.Delete(TestSkipListNode{S: "01"})
	})
	mustPanic(t, "DeleteAtHead on nil list", func() {
		nilList.DeleteAtHead()
	})
	mustPanic(t, "DeleteAtTail on nil list", func() {
		nilList.DeleteAtTail()
	})
}

// TestSingleElement exercises every operation on a list holding exactly one
// element.
func TestSingleElement(t *testing.T) {
	var List1 SkipList[TestSkipListNode]

	List1.Insert(TestSkipListNode{S: "42"})

	if List1.Length() != 1 {
		t.Errorf("Expected length of 1, got %d", List1.Length())
	}
	if mn := List1.FindMin(); mn == nil || mn.S != "42" {
		t.Errorf("Expected FindMin of 42, got %+v", mn)
	}
	if mx := List1.FindMax(); mx == nil || mx.S != "42" {
		t.Errorf("Expected FindMax of 42, got %+v", mx)
	}

	// Both iterators yield exactly the single item.
	n := 0
	for v := range List1.All() {
		n++
		if v.S != "42" {
			t.Errorf("All: expected 42, got %s", v.S)
		}
	}
	if n != 1 {
		t.Errorf("All: expected 1 item from single-element list, got %d", n)
	}
	n = 0
	for v := range List1.Backward() {
		n++
		if v.S != "42" {
			t.Errorf("Backward: expected 42, got %s", v.S)
		}
	}
	if n != 1 {
		t.Errorf("Backward: expected 1 item from single-element list, got %d", n)
	}

	// Delete the only element; the list must behave as freshly created.
	if !List1.Delete(TestSkipListNode{S: "42"}) {
		t.Errorf("Expected delete of the single element to return true")
	}
	if !List1.IsEmpty() || List1.Length() != 0 {
		t.Errorf("Expected empty list after deleting the single element")
	}
	if List1.FindMin() != nil || List1.FindMax() != nil {
		t.Errorf("Expected FindMin/FindMax to return nil after deleting the single element")
	}

	// Now drain a single element with DeleteAtHead and DeleteAtTail.
	List1.Insert(TestSkipListNode{S: "07"})
	if !List1.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead of the single element to return true")
	}
	if !List1.IsEmpty() {
		t.Errorf("Expected empty list after DeleteAtHead of the single element")
	}
	List1.Insert(TestSkipListNode{S: "08"})
	if !List1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail of the single element to return true")
	}
	if !List1.IsEmpty() {
		t.Errorf("Expected empty list after DeleteAtTail of the single element")
	}
}

// TestSearchReturnsCopy verifies that Search, FindMin and FindMax hand back
// copies: mutating the returned pointer must not change the stored data.
func TestSearchReturnsCopy(t *testing.T) {
	var List1 SkipList[TestSkipListNode]

	for _, s := range []string{"05", "02", "09"} {
		List1.Insert(TestSkipListNode{S: s})
	}

	if ptr := List1.Search(TestSkipListNode{S: "05"}); ptr != nil {
		ptr.S = "ZZ"
	}
	if ptr := List1.Search(TestSkipListNode{S: "05"}); ptr == nil || ptr.S != "05" {
		t.Errorf("Expected stored value to be unchanged after mutating Search result, got %+v", ptr)
	}

	if mn := List1.FindMin(); mn != nil {
		mn.S = "ZZ"
	}
	if mn := List1.FindMin(); mn == nil || mn.S != "02" {
		t.Errorf("Expected stored min to be unchanged after mutating FindMin result, got %+v", mn)
	}

	if mx := List1.FindMax(); mx != nil {
		mx.S = "ZZ"
	}
	if mx := List1.FindMax(); mx == nil || mx.S != "09" {
		t.Errorf("Expected stored max to be unchanged after mutating FindMax result, got %+v", mx)
	}
}

// TestInsertAfterFullDrain verifies that a list drained to empty (so all
// levels are dropped) is fully usable again.
func TestInsertAfterFullDrain(t *testing.T) {
	var List1 SkipList[TestSkipListNode]

	for i := 0; i < 200; i++ {
		List1.Insert(TestSkipListNode{S: fmt.Sprintf("%06d", i)})
	}
	// Drain via Delete so the internal level counter collapses to 0.
	for i := 0; i < 200; i++ {
		if !List1.Delete(TestSkipListNode{S: fmt.Sprintf("%06d", i)}) {
			t.Fatalf("Delete of %06d failed", i)
		}
	}
	if !List1.IsEmpty() {
		t.Fatalf("Expected empty list after full drain, length=%d", List1.Length())
	}

	// Rebuild and check ordering in both directions.
	for _, s := range []string{"09", "01", "05"} {
		List1.Insert(TestSkipListNode{S: s})
	}
	var fwd []string
	for v := range List1.All() {
		fwd = append(fwd, v.S)
	}
	want := []string{"01", "05", "09"}
	if fmt.Sprintf("%v", fwd) != fmt.Sprintf("%v", want) {
		t.Errorf("Expected %v after reinsert into drained list, got %v", want, fwd)
	}
	if mx := List1.FindMax(); mx == nil || mx.S != "09" {
		t.Errorf("Expected max of 09 after reinsert into drained list, got %+v", mx)
	}
}

// TestIteratorsAfterTruncate verifies that both iterators yield nothing on a
// truncated list (head sentinel is gone).
func TestIteratorsAfterTruncate(t *testing.T) {
	var List1 SkipList[TestSkipListNode]

	for _, s := range []string{"05", "02", "09"} {
		List1.Insert(TestSkipListNode{S: s})
	}
	List1.Truncate()

	for range List1.All() {
		t.Errorf("Expected no items from All after truncate")
	}
	for range List1.Backward() {
		t.Errorf("Expected no items from Backward after truncate")
	}
}

// TestDump verifies the human-readable Dump output on an empty and a
// populated list.
func TestDump(t *testing.T) {
	var Empty SkipList[TestSkipListNode]
	var buf bytes.Buffer
	Empty.Dump(&buf)
	if out := buf.String(); !strings.Contains(out, "empty") {
		t.Errorf("Expected Dump of empty list to mention empty, got %q", out)
	}

	var List1 SkipList[TestSkipListNode]
	for i := 0; i < 100; i++ {
		List1.Insert(TestSkipListNode{S: fmt.Sprintf("%06d", i)})
	}
	buf.Reset()
	List1.Dump(&buf)
	out := buf.String()
	if !strings.Contains(out, "length=100") {
		t.Errorf("Expected Dump to report length=100, got %q", out)
	}
	if !strings.Contains(out, "L0: ") {
		t.Errorf("Expected Dump to print the L0 level, got %q", out)
	}
	// Level 0 must list every element in ascending order.
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "L0: ") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, "L0: "))
		if len(fields) != 100 {
			t.Errorf("Expected L0 to list 100 items, got %d", len(fields))
			break
		}
		if !sort.StringsAreSorted(fields) {
			t.Errorf("Expected L0 dump to be in ascending order")
		}
	}
}

// TestRandomizedAgainstModel runs a fixed-seed mix of operations against the
// list and cross-checks it against a simple sorted-slice reference model
// after every step.
func TestRandomizedAgainstModel(t *testing.T) {
	const ops = 3000
	const keySpace = 400 // small key space forces duplicate inserts and deletes of absent keys

	rng := rand.New(rand.NewPCG(12345, 6789))

	var List1 SkipList[TestSkipListNode]
	var model []string // sorted reference of keys present

	key := func(n int) string { return fmt.Sprintf("%06d", n) }

	modelSearch := func(k string) (int, bool) {
		// sort.Find wants cmp(i) > 0 while model[i] < k, so compare k to model[i].
		return sort.Find(len(model), func(i int) int {
			return strings.Compare(k, model[i])
		})
	}
	modelMin := func() string { return model[0] }
	modelMax := func() string { return model[len(model)-1] }

	check := func(tag string) {
		t.Helper()
		if List1.Length() != len(model) {
			t.Fatalf("%s: length mismatch, list=%d model=%d", tag, List1.Length(), len(model))
		}
		if List1.IsEmpty() != (len(model) == 0) {
			t.Fatalf("%s: IsEmpty mismatch, list=%v model len=%d", tag, List1.IsEmpty(), len(model))
		}
		mn := List1.FindMin()
		mx := List1.FindMax()
		if len(model) == 0 {
			if mn != nil || mx != nil {
				t.Fatalf("%s: expected nil min/max on empty model, got %v/%v", tag, mn, mx)
			}
		} else {
			if mn == nil || mn.S != modelMin() {
				t.Fatalf("%s: min mismatch, list=%v model=%s", tag, mn, modelMin())
			}
			if mx == nil || mx.S != modelMax() {
				t.Fatalf("%s: max mismatch, list=%v model=%s", tag, mx, modelMax())
			}
		}
	}

	for i := 0; i < ops; i++ {
		k := key(rng.IntN(keySpace))
		switch rng.IntN(6) {
		case 0, 1, 2: // Insert
			List1.Insert(TestSkipListNode{S: k})
			if idx, found := modelSearch(k); !found {
				model = append(model, "")
				copy(model[idx+1:], model[idx:])
				model[idx] = k
			}
		case 3: // Delete
			got := List1.Delete(TestSkipListNode{S: k})
			idx, found := modelSearch(k)
			if got != found {
				t.Fatalf("op %d: Delete(%s) returned %v, model says %v", i, k, got, found)
			}
			if found {
				model = append(model[:idx], model[idx+1:]...)
			}
		case 4: // Search
			ptr := List1.Search(TestSkipListNode{S: k})
			_, found := modelSearch(k)
			if (ptr != nil) != found {
				t.Fatalf("op %d: Search(%s) found=%v, model says %v", i, k, ptr != nil, found)
			}
			if found && ptr.S != k {
				t.Fatalf("op %d: Search(%s) returned %s", i, k, ptr.S)
			}
		case 5: // DeleteAtHead or DeleteAtTail
			if rng.IntN(2) == 0 {
				got := List1.DeleteAtHead()
				if got != (len(model) > 0) {
					t.Fatalf("op %d: DeleteAtHead returned %v, model len=%d", i, got, len(model))
				}
				if got {
					model = model[1:]
				}
			} else {
				got := List1.DeleteAtTail()
				if got != (len(model) > 0) {
					t.Fatalf("op %d: DeleteAtTail returned %v, model len=%d", i, got, len(model))
				}
				if got {
					model = model[:len(model)-1]
				}
			}
		}
		check(fmt.Sprintf("op %d", i))

		// Periodically verify the full contents in both directions.
		if i%500 == 499 {
			fwd := make([]string, 0, len(model))
			for v := range List1.All() {
				fwd = append(fwd, v.S)
			}
			if fmt.Sprintf("%v", fwd) != fmt.Sprintf("%v", model) {
				t.Fatalf("op %d: All mismatch, list=%v model=%v", i, fwd, model)
			}
			bwd := make([]string, 0, len(model))
			for v := range List1.Backward() {
				bwd = append(bwd, v.S)
			}
			if len(bwd) != len(model) {
				t.Fatalf("op %d: Backward yielded %d items, model has %d", i, len(bwd), len(model))
			}
			for j := range model {
				if bwd[len(bwd)-1-j] != model[j] {
					t.Fatalf("op %d: Backward does not mirror model at position %d", i, j)
				}
			}
		}
	}

	// Final full-content check in both directions.
	fwd := make([]string, 0, len(model))
	for v := range List1.All() {
		fwd = append(fwd, v.S)
	}
	if fmt.Sprintf("%v", fwd) != fmt.Sprintf("%v", model) {
		t.Errorf("final: All mismatch, list=%v model=%v", fwd, model)
	}
}
