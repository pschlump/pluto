package dll

/*
Copyright (C) Philip Schlump, 2012-2024.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

// checkDll verifies the structural invariants of the list: length, head/tail
// pointers, forward and backward chains, and prev/next symmetry.  want is the
// expected list contents from head to tail.
func checkDll(t *testing.T, ns *Dll[TestDemo], want []string) {
	t.Helper()

	if got := ns.Length(); got != len(want) {
		t.Fatalf("Length: got %d, want %d", got, len(want))
	}
	if ns.IsEmpty() != (len(want) == 0) {
		t.Fatalf("IsEmpty: got %v, want %v", ns.IsEmpty(), len(want) == 0)
	}

	if len(want) == 0 {
		if ns.head != nil || ns.tail != nil {
			t.Fatalf("empty list with non-nil head/tail: head=%v tail=%v", ns.head, ns.tail)
		}
		return
	}
	if ns.head == nil || ns.tail == nil {
		t.Fatalf("non-empty list with nil head/tail: head=%v tail=%v", ns.head, ns.tail)
	}
	if ns.head.prev != nil {
		t.Errorf("head.prev is not nil")
	}
	if ns.tail.next != nil {
		t.Errorf("tail.next is not nil")
	}

	// Forward chain must match want exactly.
	n := 0
	for p := ns.head; p != nil; p = p.next {
		if n >= len(want) {
			t.Fatalf("forward chain longer than %d elements", len(want))
		}
		if p.Data.S != want[n] {
			t.Errorf("forward chain [%d]: got %s, want %s", n, p.Data.S, want[n])
		}
		// prev/next symmetry.
		if p.next != nil && p.next.prev != p {
			t.Errorf("node %d: next.prev does not point back", n)
		}
		if p.prev != nil && p.prev.next != p {
			t.Errorf("node %d: prev.next does not point forward", n)
		}
		n++
	}
	if n != len(want) {
		t.Fatalf("forward chain has %d elements, want %d", n, len(want))
	}

	// Backward chain must match the reverse of want.
	m := 0
	for p := ns.tail; p != nil; p = p.prev {
		if m >= len(want) {
			t.Fatalf("backward chain longer than %d elements", len(want))
		}
		if p.Data.S != want[len(want)-1-m] {
			t.Errorf("backward chain [%d]: got %s, want %s", m, p.Data.S, want[len(want)-1-m])
		}
		m++
	}
	if m != len(want) {
		t.Fatalf("backward chain has %d elements, want %d", m, len(want))
	}
}

func TestNewDllAndElementAccessors(t *testing.T) {

	ns := NewDll[TestDemo]()
	if ns == nil {
		t.Fatalf("NewDll returned nil")
	}
	if !ns.IsEmpty() {
		t.Errorf("NewDll: expected empty list")
	}
	checkDll(t, ns, nil)

	ns.AppendAtTail(&TestDemo{S: "01"})
	el, pos := ns.Search(&TestDemo{S: "01"})
	if el == nil || pos != 0 {
		t.Fatalf("Search: got %v, %d", el, pos)
	}

	// GetData returns the stored data.
	if got := el.GetData(); got == nil || got.S != "01" {
		t.Errorf("GetData: got %v", got)
	}

	// SetData replaces the stored data.
	el.SetData(&TestDemo{S: "02"})
	if got := el.GetData(); got.S != "02" {
		t.Errorf("GetData after SetData: got %v", got)
	}
	if v, err := ns.Peek(); err != nil || v.S != "02" {
		t.Errorf("Peek after SetData: got %v, %v", v, err)
	}
}

func TestPushAndLockUnlock(t *testing.T) {

	var ns Dll[TestDemo]

	// Lock/Unlock are no-ops kept for API compatibility with dll_ts; they
	// must not disturb the list.
	ns.Lock()
	ns.Push(&TestDemo{S: "01"})
	ns.Push(&TestDemo{S: "02"})
	ns.Unlock()

	checkDll(t, &ns, []string{"02", "01"})

	if v, err := ns.Peek(); err != nil || v.S != "02" {
		t.Errorf("Peek: got %v, %v", v, err)
	}
}

func TestLegacyIterEdgeCases(t *testing.T) {

	// Iterators on an empty list are done immediately and safe to advance.
	var ns Dll[TestDemo]
	front := ns.Front()
	if !front.Done() {
		t.Errorf("Front on empty list: not Done")
	}
	if got := front.Value(); got != nil {
		t.Errorf("Value on empty iterator: got %v", got)
	}
	front.Next() // must be a no-op, not a panic
	if !front.Done() {
		t.Errorf("Front after Next on empty list: not Done")
	}

	rear := ns.Rear()
	if !rear.Done() {
		t.Errorf("Rear on empty list: not Done")
	}
	if got := rear.Value(); got != nil {
		t.Errorf("Value on empty rear iterator: got %v", got)
	}
	rear.Prev() // must be a no-op, not a panic
	if !rear.Done() {
		t.Errorf("Rear after Prev on empty list: not Done")
	}

	// Current starts an iteration from a found element.
	ns2 := buildList("01", "02", "03")
	el, pos := ns2.Search(&TestDemo{S: "02"})
	if el == nil || pos != 1 {
		t.Fatalf("Search: got %v, %d", el, pos)
	}
	it := ns2.Current(el, pos)
	if it.Pos() != 1 {
		t.Errorf("Current: expected pos 1, got %d", it.Pos())
	}
	var got []string
	var idx []int
	for ; !it.Done(); it.Next() {
		got = append(got, it.Value().S)
		idx = append(idx, it.Pos())
	}
	if len(got) != 2 || got[0] != "02" || got[1] != "03" {
		t.Errorf("Current iteration: unexpected values %v", got)
	}
	if len(idx) != 2 || idx[0] != 1 || idx[1] != 2 {
		t.Errorf("Current iteration: unexpected positions %v", idx)
	}

	// Prev from the head stays done-adjacent: stepping back past the head
	// makes the iterator Done.
	it2 := ns2.Front()
	it2.Prev()
	if !it2.Done() {
		t.Errorf("Prev past head: expected Done")
	}
	if got := it2.Value(); got != nil {
		t.Errorf("Value of done iterator: got %v", got)
	}

	// Single element list: Front == Rear.
	ns3 := buildList("x")
	f3 := ns3.Front()
	if f3.Done() || f3.Value().S != "x" || f3.Pos() != 0 {
		t.Errorf("Front on single-element list: done=%v pos=%d", f3.Done(), f3.Pos())
	}
	f3.Next()
	if !f3.Done() {
		t.Errorf("single-element list: not Done after Next")
	}
	r3 := ns3.Rear()
	if r3.Done() || r3.Value().S != "x" || r3.Pos() != 0 {
		t.Errorf("Rear on single-element list: done=%v pos=%d", r3.Done(), r3.Pos())
	}
}

func TestSearchWalkOnEmpty(t *testing.T) {

	var ns Dll[TestDemo]

	if rv, pos := ns.Search(&TestDemo{S: "x"}); rv != nil || pos != -1 {
		t.Errorf("Search on empty: got %v, %d", rv, pos)
	}
	if rv, pos := ns.ReverseSearch(&TestDemo{S: "x"}); rv != nil || pos != -1 {
		t.Errorf("ReverseSearch on empty: got %v, %d", rv, pos)
	}
	called := false
	if rv, pos := ns.Walk(func(pos int, data TestDemo, userData interface{}) bool {
		called = true
		return true
	}, nil); rv != nil || pos != -1 || called {
		t.Errorf("Walk on empty: got %v, %d, called=%v", rv, pos, called)
	}
	if rv, pos := ns.ReverseWalk(func(pos int, data TestDemo, userData interface{}) bool {
		called = true
		return true
	}, nil); rv != nil || pos != -1 || called {
		t.Errorf("ReverseWalk on empty: got %v, %d, called=%v", rv, pos, called)
	}
}

func TestWalkEarlyStopReturnsElement(t *testing.T) {

	ns := buildList("01", "02", "03")

	// Walk that never stops returns nil, -1.
	rv, pos := ns.Walk(func(pos int, data TestDemo, userData interface{}) bool {
		return false
	}, nil)
	if rv != nil || pos != -1 {
		t.Errorf("Walk full pass: got %v, %d", rv, pos)
	}

	// Walk that stops returns the element and its position.
	rv, pos = ns.Walk(func(pos int, data TestDemo, userData interface{}) bool {
		return data.S == userData.(string)
	}, "03")
	if rv == nil || pos != 2 || rv.Data.S != "03" {
		t.Errorf("Walk stop: got %v, %d", rv, pos)
	}

	rv, pos = ns.ReverseWalk(func(pos int, data TestDemo, userData interface{}) bool {
		return data.S == userData.(string)
	}, "01")
	if rv == nil || pos != 0 || rv.Data.S != "01" {
		t.Errorf("ReverseWalk stop: got %v, %d", rv, pos)
	}

	rv, pos = ns.ReverseWalk(func(pos int, data TestDemo, userData interface{}) bool {
		return false
	}, nil)
	if rv != nil || pos != -1 {
		t.Errorf("ReverseWalk full pass: got %v, %d", rv, pos)
	}
}

func TestSingleElementEdgeCases(t *testing.T) {

	// Pop the only element, then reuse the list.
	ns := buildList("one")
	if v, err := ns.Pop(); err != nil || v.S != "one" {
		t.Errorf("Pop single: got %v, %v", v, err)
	}
	checkDll(t, &ns, nil)

	// PopTail the only element.
	ns = buildList("one")
	if v, err := ns.PopTail(); err != nil || v.S != "one" {
		t.Errorf("PopTail single: got %v, %v", v, err)
	}
	checkDll(t, &ns, nil)

	// DeleteAtHead / DeleteAtTail on a single element.
	ns = buildList("one")
	if err := ns.DeleteAtHead(); err != nil {
		t.Errorf("DeleteAtHead single: %v", err)
	}
	checkDll(t, &ns, nil)

	ns = buildList("one")
	if err := ns.DeleteAtTail(); err != nil {
		t.Errorf("DeleteAtTail single: %v", err)
	}
	checkDll(t, &ns, nil)

	// DeleteAtHead / DeleteAtTail on an empty list return ErrEmptyDll.
	if err := ns.DeleteAtHead(); err != ErrEmptyDll {
		t.Errorf("DeleteAtHead empty: got %v", err)
	}
	if err := ns.DeleteAtTail(); err != ErrEmptyDll {
		t.Errorf("DeleteAtTail empty: got %v", err)
	}

	// Reverse of empty and single-element lists is a no-op.
	ns.Reverse()
	checkDll(t, &ns, nil)
	ns = buildList("one")
	ns.Reverse()
	checkDll(t, &ns, []string{"one"})
}

func TestDuplicateValues(t *testing.T) {

	ns := buildList("dup", "mid", "dup", "end", "dup")

	// Search finds the first occurrence from the head.
	_, pos := ns.Search(&TestDemo{S: "dup"})
	if pos != 0 {
		t.Errorf("Search duplicate: expected pos 0, got %d", pos)
	}
	// ReverseSearch finds the last occurrence.
	_, pos = ns.ReverseSearch(&TestDemo{S: "dup"})
	if pos != 4 {
		t.Errorf("ReverseSearch duplicate: expected pos 4, got %d", pos)
	}

	// Delete removes the first occurrence.
	if err := ns.Delete(&TestDemo{S: "dup"}); err != nil {
		t.Errorf("Delete duplicate: %v", err)
	}
	checkDll(t, &ns, []string{"mid", "dup", "end", "dup"})

	if err := ns.Delete(&TestDemo{S: "dup"}); err != nil {
		t.Errorf("Delete duplicate: %v", err)
	}
	checkDll(t, &ns, []string{"mid", "end", "dup"})

	if err := ns.Delete(&TestDemo{S: "dup"}); err != nil {
		t.Errorf("Delete duplicate: %v", err)
	}
	checkDll(t, &ns, []string{"mid", "end"})

	if err := ns.Delete(&TestDemo{S: "dup"}); err != ErrNotFound {
		t.Errorf("Delete with no remaining duplicates: got %v", err)
	}
}

func TestDeleteFoundAllBranches(t *testing.T) {

	// Delete the head of a multi-element list via DeleteFound.
	ns := buildList("01", "02", "03")
	el, _ := ns.Search(&TestDemo{S: "01"})
	if err := ns.DeleteFound(el); err != nil {
		t.Errorf("DeleteFound head: %v", err)
	}
	checkDll(t, &ns, []string{"02", "03"})

	// Delete the tail of a multi-element list via DeleteFound.
	el, _ = ns.Search(&TestDemo{S: "03"})
	if err := ns.DeleteFound(el); err != nil {
		t.Errorf("DeleteFound tail: %v", err)
	}
	checkDll(t, &ns, []string{"02"})

	// Delete a middle element of a list with length > 2.
	ns = buildList("01", "02", "03", "04", "05")
	el, pos := ns.Search(&TestDemo{S: "03"})
	if pos != 2 {
		t.Fatalf("Search: expected pos 2, got %d", pos)
	}
	if err := ns.DeleteFound(el); err != nil {
		t.Errorf("DeleteFound middle: %v", err)
	}
	checkDll(t, &ns, []string{"01", "02", "04", "05"})

	// The removed element is fully unlinked.
	if el.next != nil || el.prev != nil {
		t.Errorf("DeleteFound: removed element still linked: next=%v prev=%v", el.next, el.prev)
	}

	// DeleteFound on an element that is not in this list (and the list is
	// too short for the middle-delete path) reports an internal error and
	// leaves the list unchanged.
	other := buildList("zz")
	otherEl, _ := other.Search(&TestDemo{S: "zz"})
	short := buildList("01", "02")
	if err := short.DeleteFound(otherEl); err != ErrInternalDll {
		t.Errorf("DeleteFound foreign element: got %v", err)
	}
	checkDll(t, &short, []string{"01", "02"})
}

func TestIndexEdgeCases(t *testing.T) {

	var ns Dll[TestDemo]
	if _, err := ns.Index(0); err != ErrOutOfRange {
		t.Errorf("Index on empty: got %v", err)
	}

	ns = buildList("01", "02", "03", "04", "05")
	if _, err := ns.Index(-1); err != ErrOutOfRange {
		t.Errorf("Index(-1): got %v", err)
	}
	if _, err := ns.Index(5); err != ErrOutOfRange {
		t.Errorf("Index(5): got %v", err)
	}

	// All positions, exercising both the head-walk and tail-walk halves.
	want := []string{"01", "02", "03", "04", "05"}
	for i, w := range want {
		rv, err := ns.Index(i)
		if err != nil || rv.Data.S != w {
			t.Errorf("Index(%d): got %v, %v", i, rv, err)
		}
	}

	// Even-length list boundary at length/2.
	ns2 := buildList("a", "b", "c", "d")
	for i, w := range []string{"a", "b", "c", "d"} {
		rv, err := ns2.Index(i)
		if err != nil || rv.Data.S != w {
			t.Errorf("Index(%d) on 4-list: got %v, %v", i, rv, err)
		}
		rv, err = ns2.IndexFromTail(3 - i)
		if err != nil || rv.Data.S != w {
			t.Errorf("IndexFromTail(%d) on 4-list: got %v, %v", 3-i, rv, err)
		}
	}

	// Index and IndexFromTail agree on a single-element list.
	ns3 := buildList("x")
	if rv, err := ns3.Index(0); err != nil || rv.Data.S != "x" {
		t.Errorf("Index(0) single: got %v, %v", rv, err)
	}
	if rv, err := ns3.IndexFromTail(0); err != nil || rv.Data.S != "x" {
		t.Errorf("IndexFromTail(0) single: got %v, %v", rv, err)
	}
}

func TestTrimNoOpAndBoundaries(t *testing.T) {

	// Trim with n >= length leaves the list unchanged.
	ns := buildList("01", "02", "03")
	if err := ns.Trim(3); err != nil {
		t.Errorf("Trim(3) on 3-list: %v", err)
	}
	checkDll(t, &ns, []string{"01", "02", "03"})
	if err := ns.Trim(10); err != nil {
		t.Errorf("Trim(10) on 3-list: %v", err)
	}
	checkDll(t, &ns, []string{"01", "02", "03"})

	// TrimTail with n >= length leaves the list unchanged.
	if err := ns.TrimTail(3); err != nil {
		t.Errorf("TrimTail(3) on 3-list: %v", err)
	}
	checkDll(t, &ns, []string{"01", "02", "03"})
	if err := ns.TrimTail(10); err != nil {
		t.Errorf("TrimTail(10) on 3-list: %v", err)
	}
	checkDll(t, &ns, []string{"01", "02", "03"})

	// Trim to exactly one element.
	ns = buildList("01", "02", "03")
	if err := ns.Trim(1); err != nil {
		t.Errorf("Trim(1): %v", err)
	}
	checkDll(t, &ns, []string{"01"})

	// TrimTail to exactly one element.
	ns = buildList("01", "02", "03")
	if err := ns.TrimTail(1); err != nil {
		t.Errorf("TrimTail(1): %v", err)
	}
	checkDll(t, &ns, []string{"03"})

	// Negative n empties the list.
	ns = buildList("01", "02")
	if err := ns.Trim(-5); err != nil {
		t.Errorf("Trim(-5): %v", err)
	}
	checkDll(t, &ns, nil)
	ns = buildList("01", "02")
	if err := ns.TrimTail(-5); err != nil {
		t.Errorf("TrimTail(-5): %v", err)
	}
	checkDll(t, &ns, nil)

	// Trim/TrimTail on an empty list report ErrEmptyDll.
	if err := ns.Trim(0); err != ErrEmptyDll {
		t.Errorf("Trim on empty: got %v", err)
	}
	if err := ns.TrimTail(0); err != ErrEmptyDll {
		t.Errorf("TrimTail on empty: got %v", err)
	}
}

func TestConcatEdgeCases(t *testing.T) {

	// Concat of an empty source changes nothing.
	ns := buildList("01", "02")
	var empty Dll[TestDemo]
	ns.Concat(&empty)
	checkDll(t, &ns, []string{"01", "02"})

	// Concat onto an empty destination.
	empty.Concat(&ns)
	checkDll(t, &empty, []string{"01", "02"})

	// Order is preserved across a full concat.
	ns2 := buildList("a", "b")
	ns3 := buildList("c", "d", "e")
	ns2.Concat(&ns3)
	checkDll(t, &ns2, []string{"a", "b", "c", "d", "e"})
	checkDll(t, &ns3, []string{"c", "d", "e"})
}

func TestDump(t *testing.T) {

	var buf bytes.Buffer
	var ns Dll[TestDemo]
	ns.Dump(&buf)
	if buf.String() != "" {
		t.Errorf("Dump of empty list: got %q", buf.String())
	}

	buf.Reset()
	ns = buildList("01", "02")
	ns.Dump(&buf)
	want := "0: {S:01}\n1: {S:02}\n"
	if buf.String() != want {
		t.Errorf("Dump: got %q, want %q", buf.String(), want)
	}
}

func TestIteratorEarlyBreakAndAliasing(t *testing.T) {

	ns := buildList("01", "02", "03")

	// Early break in Backward.
	count := 0
	var first string
	for i, v := range ns.Backward() {
		if i != 2 {
			t.Errorf("Backward first index: got %d", i)
		}
		first = v.S
		count++
		break
	}
	if count != 1 || first != "03" {
		t.Errorf("Backward break: count=%d first=%s", count, first)
	}

	// Early break in IteratePtr.
	count = 0
	for range ns.IteratePtr() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("IteratePtr break: expected 1 iteration, got %d", count)
	}

	// IteratePtr yields live pointers; mutation through them is visible in
	// the list.
	for i, p := range ns.IteratePtr() {
		p.S = fmt.Sprintf("m%d", i)
	}
	checkDll(t, &ns, []string{"m0", "m1", "m2"})

	// IterateOver/IteratePtr on an empty list produce nothing.
	var empty Dll[TestDemo]
	for range empty.IterateOver() {
		t.Errorf("IterateOver on empty list iterated")
	}
	for range empty.IteratePtr() {
		t.Errorf("IteratePtr on empty list iterated")
	}
}

// TestRandomizedAgainstModel runs a fixed-seed sequence of mixed operations,
// cross-checking the list against a plain slice model after every step.
func TestRandomizedAgainstModel(t *testing.T) {

	rng := rand.New(rand.NewSource(42))
	var ns Dll[TestDemo]
	var model []string
	counter := 0

	newVal := func() string {
		v := fmt.Sprintf("v%03d", counter)
		counter++
		return v
	}

	for step := 0; step < 800; step++ {
		op := rng.Intn(100)
		switch {
		case op < 20: // insert at head
			v := newVal()
			ns.InsertBeforeHead(&TestDemo{S: v})
			model = append([]string{v}, model...)
		case op < 40: // append at tail
			v := newVal()
			ns.AppendAtTail(&TestDemo{S: v})
			model = append(model, v)
		case op < 50: // pop head
			got, err := ns.Pop()
			if len(model) == 0 {
				if err != ErrEmptyDll {
					t.Fatalf("step %d: Pop on empty: got %v", step, err)
				}
			} else {
				if err != nil || got.S != model[0] {
					t.Fatalf("step %d: Pop: got %v, %v; want %s", step, got, err, model[0])
				}
				model = model[1:]
			}
		case op < 60: // pop tail
			got, err := ns.PopTail()
			if len(model) == 0 {
				if err != ErrEmptyDll {
					t.Fatalf("step %d: PopTail on empty: got %v", step, err)
				}
			} else {
				if err != nil || got.S != model[len(model)-1] {
					t.Fatalf("step %d: PopTail: got %v, %v; want %s", step, got, err, model[len(model)-1])
				}
				model = model[:len(model)-1]
			}
		case op < 70: // delete by value (first occurrence)
			if len(model) == 0 {
				if err := ns.Delete(&TestDemo{S: "absent"}); err != ErrNotFound {
					t.Fatalf("step %d: Delete on empty: got %v", step, err)
				}
			} else {
				target := model[rng.Intn(len(model))]
				if err := ns.Delete(&TestDemo{S: target}); err != nil {
					t.Fatalf("step %d: Delete(%s): %v", step, target, err)
				}
				for i, v := range model {
					if v == target {
						model = append(model[:i], model[i+1:]...)
						break
					}
				}
				// Deleting an absent value fails and changes nothing.
				if err := ns.Delete(&TestDemo{S: "absent"}); err != ErrNotFound {
					t.Fatalf("step %d: Delete absent: got %v", step, err)
				}
			}
		case op < 75: // reverse
			ns.Reverse()
			for i, j := 0, len(model)-1; i < j; i, j = i+1, j-1 {
				model[i], model[j] = model[j], model[i]
			}
		case op < 80: // trim (keep head)
			n := rng.Intn(len(model) + 3)
			err := ns.Trim(n)
			if len(model) == 0 {
				if err != ErrEmptyDll {
					t.Fatalf("step %d: Trim on empty: got %v", step, err)
				}
			} else {
				if err != nil {
					t.Fatalf("step %d: Trim(%d): %v", step, n, err)
				}
				if n <= 0 {
					model = nil
				} else if len(model) > n {
					model = model[:n]
				}
			}
		case op < 85: // trim tail (keep tail)
			n := rng.Intn(len(model) + 3)
			err := ns.TrimTail(n)
			if len(model) == 0 {
				if err != ErrEmptyDll {
					t.Fatalf("step %d: TrimTail on empty: got %v", step, err)
				}
			} else {
				if err != nil {
					t.Fatalf("step %d: TrimTail(%d): %v", step, n, err)
				}
				if n <= 0 {
					model = nil
				} else if len(model) > n {
					model = model[len(model)-n:]
				}
			}
		case op < 92: // random Index / Search checks
			if len(model) > 0 {
				i := rng.Intn(len(model))
				rv, err := ns.Index(i)
				if err != nil || rv.Data.S != model[i] {
					t.Fatalf("step %d: Index(%d): got %v, %v; want %s", step, i, rv, err, model[i])
				}
				rv, err = ns.IndexFromTail(len(model) - 1 - i)
				if err != nil || rv.Data.S != model[i] {
					t.Fatalf("step %d: IndexFromTail(%d): got %v, %v; want %s", step, len(model)-1-i, rv, err, model[i])
				}
				// Search must find the first occurrence.
				el, pos := ns.Search(&TestDemo{S: model[i]})
				first := 0
				for j, v := range model {
					if v == model[i] {
						first = j
						break
					}
				}
				if el == nil || pos != first {
					t.Fatalf("step %d: Search(%s): got pos %d; want %d", step, model[i], pos, first)
				}
			}
		default: // truncate and rebuild occasionally
			ns.Truncate()
			model = nil
			k := rng.Intn(5)
			for i := 0; i < k; i++ {
				v := newVal()
				ns.AppendAtTail(&TestDemo{S: v})
				model = append(model, v)
			}
		}

		// Full structural cross-check after every operation.
		if got := ns.Length(); got != len(model) {
			t.Fatalf("step %d: Length: got %d, want %d", step, got, len(model))
		}
		var fwd []string
		for _, v := range ns.All() {
			fwd = append(fwd, v.S)
		}
		if len(fwd) != len(model) {
			t.Fatalf("step %d: All yielded %d items, want %d", step, len(fwd), len(model))
		}
		for i := range model {
			if fwd[i] != model[i] {
				t.Fatalf("step %d: All[%d]: got %s, want %s", step, i, fwd[i], model[i])
			}
		}
		var bwd []string
		for _, v := range ns.Backward() {
			bwd = append(bwd, v.S)
		}
		if len(bwd) != len(model) {
			t.Fatalf("step %d: Backward yielded %d items, want %d", step, len(bwd), len(model))
		}
		for i := range model {
			if bwd[i] != model[len(model)-1-i] {
				t.Fatalf("step %d: Backward[%d]: got %s, want %s", step, i, bwd[i], model[len(model)-1-i])
			}
		}
		if len(model) > 0 {
			if v, err := ns.Peek(); err != nil || v.S != model[0] {
				t.Fatalf("step %d: Peek: got %v, %v; want %s", step, v, err, model[0])
			}
			if v, err := ns.PeekTail(); err != nil || v.S != model[len(model)-1] {
				t.Fatalf("step %d: PeekTail: got %v, %v; want %s", step, v, err, model[len(model)-1])
			}
		}
		checkDll(t, &ns, model)
	}
}
