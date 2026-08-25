package dll_ts

/*
Copyright (C) Philip Schlump, 2012-2024.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
)

// checkModel verifies the list against a reference slice model: length,
// forward order, backward order, peek/peekTail, and Index consistency.
func checkModel(t *testing.T, ns *Dll[TestDemo], model []string) {
	t.Helper()

	if ns.Length() != len(model) {
		t.Fatalf("Length: expected %d got %d", len(model), ns.Length())
	}
	if ns.IsEmpty() != (len(model) == 0) {
		t.Fatalf("IsEmpty: expected %v, Length is %d", len(model) == 0, ns.Length())
	}

	// Forward order via All, with matching indexes.
	i := 0
	for idx, v := range ns.All() {
		if idx != i {
			t.Fatalf("All: index mismatch, expected %d got %d", i, idx)
		}
		if i >= len(model) || v.S != model[i] {
			t.Fatalf("All: value mismatch at %d, got %q model %v", i, v.S, model)
		}
		i++
	}
	if i != len(model) {
		t.Fatalf("All: iterated %d elements, model has %d", i, len(model))
	}

	// Backward order via Backward, with matching indexes.
	j := len(model) - 1
	for idx, v := range ns.Backward() {
		if idx != j {
			t.Fatalf("Backward: index mismatch, expected %d got %d", j, idx)
		}
		if j < 0 || v.S != model[j] {
			t.Fatalf("Backward: value mismatch at %d, got %q model %v", j, v.S, model)
		}
		j--
	}
	if j != -1 {
		t.Fatalf("Backward: iterated wrong number of elements, stopped at %d", j)
	}

	// Peek / PeekTail.
	if len(model) == 0 {
		if _, err := ns.Peek(); err != ErrEmptyDll {
			t.Fatalf("Peek on empty: expected ErrEmptyDll, got %v", err)
		}
		if _, err := ns.PeekTail(); err != ErrEmptyDll {
			t.Fatalf("PeekTail on empty: expected ErrEmptyDll, got %v", err)
		}
		return
	}
	if v, err := ns.Peek(); err != nil || v.S != model[0] {
		t.Fatalf("Peek: expected %q got %v err %v", model[0], v, err)
	}
	if v, err := ns.PeekTail(); err != nil || v.S != model[len(model)-1] {
		t.Fatalf("PeekTail: expected %q got %v err %v", model[len(model)-1], v, err)
	}

	// Index consistency at the edges and middle.
	for _, sub := range []int{0, len(model) / 2, len(model) - 1} {
		el, err := ns.Index(sub)
		if err != nil {
			t.Fatalf("Index(%d): unexpected error %v", sub, err)
		}
		if el.Data.S != model[sub] {
			t.Fatalf("Index(%d): expected %q got %q", sub, model[sub], el.Data.S)
		}
		el, err = ns.IndexFromTail(len(model) - 1 - sub)
		if err != nil {
			t.Fatalf("IndexFromTail(%d): unexpected error %v", len(model)-1-sub, err)
		}
		if el.Data.S != model[sub] {
			t.Fatalf("IndexFromTail(%d): expected %q got %q", len(model)-1-sub, model[sub], el.Data.S)
		}
	}
}

func TestNewDllAndElementAccessors(t *testing.T) {

	Dll1 := NewDll[TestDemo]()
	if Dll1 == nil {
		t.Fatalf("NewDll returned nil")
	}
	if !Dll1.IsEmpty() || Dll1.Length() != 0 {
		t.Errorf("NewDll: expected an empty list")
	}

	// GetData/SetData on an element of the list.
	v := TestDemo{S: "01"}
	Dll1.AppendAtTail(&v)
	el, err := Dll1.Index(0)
	if err != nil {
		t.Fatalf("Index(0): unexpected error %v", err)
	}
	if el.GetData().S != "01" {
		t.Errorf("GetData: expected 01, got %s", el.GetData().S)
	}
	w := TestDemo{S: "02"}
	el.SetData(&w)
	if el.GetData().S != "02" {
		t.Errorf("SetData: expected 02, got %s", el.GetData().S)
	}
	if pv, _ := Dll1.Peek(); pv.S != "02" {
		t.Errorf("SetData did not change the list element, got %s", pv.S)
	}
}

func TestInsertReturnValues(t *testing.T) {

	var Dll1 Dll[TestDemo]
	if !Dll1.InsertBeforeHead(&TestDemo{S: "01"}) {
		t.Errorf("InsertBeforeHead: expected true")
	}
	if !Dll1.AppendAtTail(&TestDemo{S: "02"}) {
		t.Errorf("AppendAtTail: expected true")
	}
	if Dll1.Length() != 2 {
		t.Errorf("Expected length 2, got %d", Dll1.Length())
	}
}

func TestEmptyListOperations(t *testing.T) {

	var Dll1 Dll[TestDemo]

	if _, err := Dll1.Pop(); err != ErrEmptyDll {
		t.Errorf("Pop on empty: expected ErrEmptyDll, got %v", err)
	}
	if _, err := Dll1.PopTail(); err != ErrEmptyDll {
		t.Errorf("PopTail on empty: expected ErrEmptyDll, got %v", err)
	}
	if err := Dll1.DeleteAtHead(); err != ErrEmptyDll {
		t.Errorf("DeleteAtHead on empty: expected ErrEmptyDll, got %v", err)
	}
	if err := Dll1.DeleteAtTail(); err != ErrEmptyDll {
		t.Errorf("DeleteAtTail on empty: expected ErrEmptyDll, got %v", err)
	}
	if err := Dll1.Delete(&TestDemo{S: "x"}); err != ErrNotFound {
		t.Errorf("Delete on empty: expected ErrNotFound, got %v", err)
	}
	if err := Dll1.DeleteSearch(&TestDemo{S: "x"}); err != ErrNotFound {
		t.Errorf("DeleteSearch on empty: expected ErrNotFound, got %v", err)
	}
	if el, pos := Dll1.Search(&TestDemo{S: "x"}); el != nil || pos != -1 {
		t.Errorf("Search on empty: expected nil, -1 got %v, %d", el, pos)
	}
	if el, pos := Dll1.ReverseSearch(&TestDemo{S: "x"}); el != nil || pos != -1 {
		t.Errorf("ReverseSearch on empty: expected nil, -1 got %v, %d", el, pos)
	}
	if _, err := Dll1.Index(0); err != ErrOutOfRange {
		t.Errorf("Index on empty: expected ErrOutOfRange, got %v", err)
	}
	if _, err := Dll1.IndexFromTail(0); err != ErrOutOfRange {
		t.Errorf("IndexFromTail on empty: expected ErrOutOfRange, got %v", err)
	}

	// Walk / ReverseWalk on an empty list run no callbacks and return nil, -1.
	called := false
	fx := func(pos int, data TestDemo, userData interface{}) bool {
		called = true
		return false
	}
	if el, pos := Dll1.Walk(fx, nil); el != nil || pos != -1 {
		t.Errorf("Walk on empty: expected nil, -1 got %v, %d", el, pos)
	}
	if el, pos := Dll1.ReverseWalk(fx, nil); el != nil || pos != -1 {
		t.Errorf("ReverseWalk on empty: expected nil, -1 got %v, %d", el, pos)
	}
	if called {
		t.Errorf("Walk/ReverseWalk on empty list invoked the callback")
	}

	// Truncate and Reverse on an empty list are harmless no-ops.
	Dll1.Truncate()
	Dll1.Reverse()
	Dll1.ReverseList()
	if !Dll1.IsEmpty() {
		t.Errorf("Expected empty list after no-ops on empty list")
	}

	// Legacy iterators on an empty list are immediately done.
	if ii := Dll1.Front(); !ii.Done() {
		t.Errorf("Front on empty: expected Done")
	}
	if ii := Dll1.Rear(); !ii.Done() {
		t.Errorf("Rear on empty: expected Done")
	}
}

func TestOutOfRangeIndexes(t *testing.T) {

	Dll1 := buildList("01", "02", "03")

	if _, err := Dll1.Index(-1); err != ErrOutOfRange {
		t.Errorf("Index(-1): expected ErrOutOfRange, got %v", err)
	}
	if _, err := Dll1.Index(3); err != ErrOutOfRange {
		t.Errorf("Index(3): expected ErrOutOfRange, got %v", err)
	}
	if _, err := Dll1.IndexFromTail(-1); err != ErrOutOfRange {
		t.Errorf("IndexFromTail(-1): expected ErrOutOfRange, got %v", err)
	}
	if _, err := Dll1.IndexFromTail(3); err != ErrOutOfRange {
		t.Errorf("IndexFromTail(3): expected ErrOutOfRange, got %v", err)
	}

	// Index walks from the head for the first half and from the tail for the
	// second half; check every position on a larger list both ways.
	Dll2 := buildList("a0", "a1", "a2", "a3", "a4", "a5", "a6")
	for i := 0; i < 7; i++ {
		el, err := Dll2.Index(i)
		if err != nil || el.Data.S != fmt.Sprintf("a%d", i) {
			t.Errorf("Index(%d): expected a%d, got %v, %v", i, i, el, err)
		}
		el, err = Dll2.IndexFromTail(6 - i)
		if err != nil || el.Data.S != fmt.Sprintf("a%d", i) {
			t.Errorf("IndexFromTail(%d): expected a%d, got %v, %v", 6-i, i, el, err)
		}
	}
}

func TestDeleteFoundPaths(t *testing.T) {

	// Delete the head of a multi-element list.
	Dll1 := buildList("01", "02", "03")
	el, _ := Dll1.Search(&TestDemo{S: "01"})
	if err := Dll1.DeleteFound(el); err != nil {
		t.Errorf("DeleteFound(head): unexpected error %v", err)
	}
	checkModel(t, &Dll1, []string{"02", "03"})

	// Delete the tail of a multi-element list.
	el, _ = Dll1.Search(&TestDemo{S: "03"})
	if err := Dll1.DeleteFound(el); err != nil {
		t.Errorf("DeleteFound(tail): unexpected error %v", err)
	}
	checkModel(t, &Dll1, []string{"02"})

	// Delete the middle element of a 3-element list (the length > 2 path).
	Dll2 := buildList("01", "02", "03")
	el, _ = Dll2.Search(&TestDemo{S: "02"})
	if err := Dll2.DeleteFound(el); err != nil {
		t.Errorf("DeleteFound(middle): unexpected error %v", err)
	}
	checkModel(t, &Dll2, []string{"01", "03"})

	// DeleteFound with an element that is not in the list: on a 2-element
	// list the node is neither head nor tail, so the internal-error path is
	// taken and the list is left unchanged.
	Dll3 := buildList("01", "02")
	foreign := &DllElement[TestDemo]{Data: &TestDemo{S: "zz"}}
	if err := Dll3.DeleteFound(foreign); err != ErrInternalDll {
		t.Errorf("DeleteFound(foreign): expected ErrInternalDll, got %v", err)
	}
	checkModel(t, &Dll3, []string{"01", "02"})
}

func TestDuplicateValues(t *testing.T) {

	Dll1 := buildList("01", "02", "02", "03", "02")

	// Delete removes only the first matching element.
	if err := Dll1.Delete(&TestDemo{S: "02"}); err != nil {
		t.Errorf("Delete: unexpected error %v", err)
	}
	checkModel(t, &Dll1, []string{"01", "02", "03", "02"})

	// Search finds the first match from the head.
	if _, pos := Dll1.Search(&TestDemo{S: "02"}); pos != 1 {
		t.Errorf("Search: expected first match at pos 1, got %d", pos)
	}
	// ReverseSearch finds the first match from the tail.
	if _, pos := Dll1.ReverseSearch(&TestDemo{S: "02"}); pos != 3 {
		t.Errorf("ReverseSearch: expected last match at pos 3, got %d", pos)
	}
}

func TestWalkEarlyStopAndComplete(t *testing.T) {

	Dll1 := buildList("01", "02", "03")

	// Walk that never stops returns nil, -1.
	fx := func(pos int, data TestDemo, userData interface{}) bool { return false }
	if el, pos := Dll1.Walk(fx, nil); el != nil || pos != -1 {
		t.Errorf("Walk full: expected nil, -1 got %v, %d", el, pos)
	}
	if el, pos := Dll1.ReverseWalk(fx, nil); el != nil || pos != -1 {
		t.Errorf("ReverseWalk full: expected nil, -1 got %v, %d", el, pos)
	}

	// Walk that stops returns the element and its position.
	stop := func(pos int, data TestDemo, userData interface{}) bool {
		return data.S == userData.(string)
	}
	el, pos := Dll1.Walk(stop, "02")
	if el == nil || pos != 1 || el.Data.S != "02" {
		t.Errorf("Walk stop: expected element 02 at pos 1, got %v, %d", el, pos)
	}
	el, pos = Dll1.ReverseWalk(stop, "02")
	if el == nil || pos != 1 || el.Data.S != "02" {
		t.Errorf("ReverseWalk stop: expected element 02 at pos 1, got %v, %d", el, pos)
	}
}

func TestLegacyIteratorEdges(t *testing.T) {

	Dll1 := buildList("01", "02")

	// Front iteration runs off the end; Value on a finished iterator is nil
	// and extra Next calls are harmless.
	ii := Dll1.Front()
	ii.Next()
	ii.Next()
	if !ii.Done() {
		t.Errorf("Expected Done after walking off the end")
	}
	if v := ii.Value(); v != nil {
		t.Errorf("Value after Done: expected nil, got %v", v)
	}
	ii.Next() // must not panic or move
	if !ii.Done() {
		t.Errorf("Expected Done to remain true after extra Next")
	}
	if ii.Pos() != 2 {
		t.Errorf("Pos after walking off the end: expected 2, got %d", ii.Pos())
	}

	// Rear iteration runs off the front; extra Prev calls are harmless.
	ij := Dll1.Rear()
	if ij.Pos() != 1 {
		t.Errorf("Rear Pos: expected 1, got %d", ij.Pos())
	}
	ij.Prev()
	ij.Prev()
	if !ij.Done() {
		t.Errorf("Expected Done after walking off the front")
	}
	if v := ij.Value(); v != nil {
		t.Errorf("Value after Done: expected nil, got %v", v)
	}
	ij.Prev() // must not panic or move
	if ij.Pos() != -1 {
		t.Errorf("Pos after walking off the front: expected -1, got %d", ij.Pos())
	}

	// Current starts an iteration from a found element.
	el, pos := Dll1.Search(&TestDemo{S: "02"})
	ik := Dll1.Current(el, pos)
	if ik.Done() {
		t.Errorf("Current iterator should not be Done")
	}
	if ik.Pos() != 1 || ik.Value().S != "02" {
		t.Errorf("Current iterator: expected pos 1 value 02, got %d %v", ik.Pos(), ik.Value())
	}
	ik.Prev()
	if ik.Pos() != 0 || ik.Value().S != "01" {
		t.Errorf("Current iterator after Prev: expected pos 0 value 01, got %d %v", ik.Pos(), ik.Value())
	}
}

func TestRangeIteratorsEarlyBreak(t *testing.T) {

	Dll1 := buildList("01", "02", "03")

	// Backward with early break.
	count := 0
	for _, v := range Dll1.Backward() {
		if v.S != "03" {
			t.Errorf("Backward first value: expected 03, got %s", v.S)
		}
		count++
		break
	}
	if count != 1 {
		t.Errorf("Backward with break: expected 1 iteration, got %d", count)
	}

	// IteratePtr with early break; pointers alias the stored data.
	count = 0
	for _, v := range Dll1.IteratePtr() {
		if v == nil || v.S != "01" {
			t.Errorf("IteratePtr first value: expected 01, got %v", v)
		}
		count++
		break
	}
	if count != 1 {
		t.Errorf("IteratePtr with break: expected 1 iteration, got %d", count)
	}

	// IterateOver with early break.
	count = 0
	for range Dll1.IterateOver() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("IterateOver with break: expected 1 iteration, got %d", count)
	}

	// Empty list: no iterations for the ptr iterator either.
	var Dll2 Dll[TestDemo]
	for range Dll2.IteratePtr() {
		t.Errorf("IteratePtr on empty list iterated")
	}
	for range Dll2.IterateOver() {
		t.Errorf("IterateOver on empty list iterated")
	}
}

func TestDump(t *testing.T) {

	var buf bytes.Buffer
	Dll1 := buildList("01", "02")
	Dll1.Dump(&buf)
	out := buf.String()
	if !strings.Contains(out, "0: {S:01}") || !strings.Contains(out, "1: {S:02}") {
		t.Errorf("Dump: unexpected output %q", out)
	}

	// Dump of an empty list produces no output.
	buf.Reset()
	var Dll2 Dll[TestDemo]
	Dll2.Dump(&buf)
	if buf.String() != "" {
		t.Errorf("Dump of empty list: expected empty output, got %q", buf.String())
	}
}

func TestLockUnlock(t *testing.T) {

	// Lock/Unlock exist so code written against the thread-safe API can take
	// the list lock explicitly.  Verify they are usable and balanced.
	Dll1 := buildList("01", "02")
	Dll1.Lock()
	Dll1.Unlock()
	if Dll1.Length() != 2 {
		t.Errorf("Expected length 2 after Lock/Unlock, got %d", Dll1.Length())
	}
}

func TestSingleElementList(t *testing.T) {

	var Dll1 Dll[TestDemo]
	Dll1.Push(&TestDemo{S: "01"})
	checkModel(t, &Dll1, []string{"01"})

	// Reverse a single-element list is a no-op.
	Dll1.Reverse()
	checkModel(t, &Dll1, []string{"01"})

	// Search/Index hit the only element.
	if _, pos := Dll1.Search(&TestDemo{S: "01"}); pos != 0 {
		t.Errorf("Search single: expected pos 0, got %d", pos)
	}
	if _, pos := Dll1.ReverseSearch(&TestDemo{S: "01"}); pos != 0 {
		t.Errorf("ReverseSearch single: expected pos 0, got %d", pos)
	}

	// PopTail removes the only element and resets head.
	if v, err := Dll1.PopTail(); err != nil || v.S != "01" {
		t.Errorf("PopTail single: expected 01, got %v, %v", v, err)
	}
	checkModel(t, &Dll1, []string{})

	// The list is fully reusable after being emptied.
	Dll1.AppendAtTail(&TestDemo{S: "02"})
	checkModel(t, &Dll1, []string{"02"})
}

func TestConcatEmpty(t *testing.T) {

	// Concat an empty list onto a non-empty list: no change.
	Dll1 := buildList("01", "02")
	var Dll2 Dll[TestDemo]
	Dll1.Concat(&Dll2)
	checkModel(t, &Dll1, []string{"01", "02"})

	// Concat onto an empty list.
	Dll2.Concat(&Dll1)
	checkModel(t, &Dll2, []string{"01", "02"})

	// Self-concat of an empty list stays empty.
	var Dll3 Dll[TestDemo]
	Dll3.Concat(&Dll3)
	if !Dll3.IsEmpty() {
		t.Errorf("Self-concat of empty list should stay empty")
	}
}

// TestPropertyRandomOps cross-checks the DLL against a plain slice reference
// model over hundreds of mixed operations with a fixed seed.
func TestPropertyRandomOps(t *testing.T) {

	rng := rand.New(rand.NewSource(20240817))
	var Dll1 Dll[TestDemo]
	var model []string

	value := func() string { return fmt.Sprintf("v%d", rng.Intn(10)) }

	for step := 0; step < 2000; step++ {
		switch rng.Intn(12) {
		case 0: // Push (InsertBeforeHead)
			v := value()
			Dll1.Push(&TestDemo{S: v})
			model = append([]string{v}, model...)
		case 1, 2, 3: // AppendAtTail / Enqueue
			v := value()
			if rng.Intn(2) == 0 {
				Dll1.AppendAtTail(&TestDemo{S: v})
			} else {
				Dll1.Enqueue(&TestDemo{S: v})
			}
			model = append(model, v)
		case 4: // Pop
			v, err := Dll1.Pop()
			if len(model) == 0 {
				if err != ErrEmptyDll {
					t.Fatalf("step %d: Pop on empty: expected ErrEmptyDll, got %v", step, err)
				}
			} else {
				if err != nil || v.S != model[0] {
					t.Fatalf("step %d: Pop: expected %q, got %v, %v", step, model[0], v, err)
				}
				model = model[1:]
			}
		case 5: // PopTail
			v, err := Dll1.PopTail()
			if len(model) == 0 {
				if err != ErrEmptyDll {
					t.Fatalf("step %d: PopTail on empty: expected ErrEmptyDll, got %v", step, err)
				}
			} else {
				if err != nil || v.S != model[len(model)-1] {
					t.Fatalf("step %d: PopTail: expected %q, got %v, %v", step, model[len(model)-1], v, err)
				}
				model = model[:len(model)-1]
			}
		case 6: // Delete a (possibly absent) value: removes first match.
			v := value()
			err := Dll1.Delete(&TestDemo{S: v})
			found := -1
			for i, s := range model {
				if s == v {
					found = i
					break
				}
			}
			if found == -1 {
				if err != ErrNotFound {
					t.Fatalf("step %d: Delete(%q): expected ErrNotFound, got %v", step, v, err)
				}
			} else {
				if err != nil {
					t.Fatalf("step %d: Delete(%q): unexpected error %v", step, v, err)
				}
				model = append(model[:found], model[found+1:]...)
			}
		case 7: // Reverse
			Dll1.Reverse()
			for i, j := 0, len(model)-1; i < j; i, j = i+1, j-1 {
				model[i], model[j] = model[j], model[i]
			}
		case 8: // Trim
			n := rng.Intn(len(model) + 3) - 1 // -1 .. len+1
			err := Dll1.Trim(n)
			if len(model) == 0 {
				if err != ErrEmptyDll {
					t.Fatalf("step %d: Trim on empty: expected ErrEmptyDll, got %v", step, err)
				}
			} else {
				if err != nil {
					t.Fatalf("step %d: Trim(%d): unexpected error %v", step, n, err)
				}
				if n <= 0 {
					model = nil
				} else if n < len(model) {
					model = model[:n]
				}
			}
		case 9: // TrimTail
			n := rng.Intn(len(model) + 3) - 1
			err := Dll1.TrimTail(n)
			if len(model) == 0 {
				if err != ErrEmptyDll {
					t.Fatalf("step %d: TrimTail on empty: expected ErrEmptyDll, got %v", step, err)
				}
			} else {
				if err != nil {
					t.Fatalf("step %d: TrimTail(%d): unexpected error %v", step, n, err)
				}
				if n <= 0 {
					model = nil
				} else if n < len(model) {
					model = model[len(model)-n:]
				}
			}
		case 10: // Search / Index on a random value and position.
			v := value()
			_, pos := Dll1.Search(&TestDemo{S: v})
			expect := -1
			for i, s := range model {
				if s == v {
					expect = i
					break
				}
			}
			if pos != expect {
				t.Fatalf("step %d: Search(%q): expected pos %d, got %d", step, v, expect, pos)
			}
			if len(model) > 0 {
				sub := rng.Intn(len(model))
				el, err := Dll1.Index(sub)
				if err != nil || el.Data.S != model[sub] {
					t.Fatalf("step %d: Index(%d): expected %q, got %v, %v", step, sub, model[sub], el, err)
				}
			}
		case 11: // Truncate
			Dll1.Truncate()
			model = nil
		}

		// Verify full structural state after every operation.
		if step%7 == 0 {
			checkModel(t, &Dll1, model)
		} else if Dll1.Length() != len(model) {
			t.Fatalf("step %d: Length: expected %d got %d", step, len(model), Dll1.Length())
		}
	}

	checkModel(t, &Dll1, model)
}

// TestConcurrentReadersWriters exercises concurrent writers, readers, and
// range-over-func iterators.  Run with -race to validate the locking.
func TestConcurrentReadersWriters(t *testing.T) {

	var Dll1 Dll[TestDemo]
	const writers = 8
	const perWriter = 2000
	const readers = 4

	var wg sync.WaitGroup
	var wgWriters sync.WaitGroup
	stop := make(chan struct{})

	// Readers: iterate, walk, search, and check length until told to stop.
	for g := 0; g < readers; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				n := 0
				for idx := range Dll1.All() {
					if idx != n {
						t.Errorf("All: non-consecutive index %d after %d", idx, n)
						return
					}
					n++
				}
				m := 0
				prev := -1
				for idx := range Dll1.Backward() {
					if m > 0 && idx != prev-1 {
						t.Errorf("Backward: non-consecutive index %d after %d", idx, prev)
						return
					}
					prev = idx
					m++
				}
				if m > 0 && prev != 0 {
					t.Errorf("Backward: indexes did not end at 0, ended at %d", prev)
					return
				}
				_, _ = Dll1.Search(&TestDemo{S: "00000042"})
				_, _ = Dll1.Peek()
				_, _ = Dll1.PeekTail()
				_, _ = Dll1.Index(0)
			}
		}()
	}

	// Writers: push disjoint sets of elements.
	for w := 0; w < writers; w++ {
		wgWriters.Add(1)
		go func(w int) {
			defer wgWriters.Done()
			for i := 0; i < perWriter; i++ {
				v := TestDemo{S: fmt.Sprintf("%08d", w*perWriter+i)}
				if i%2 == 0 {
					Dll1.Push(&v)
				} else {
					Dll1.AppendAtTail(&v)
				}
			}
		}(w)
	}

	// Wait for writers to finish, then stop the readers and wait for them.
	wgWriters.Wait()
	close(stop)
	wg.Wait()

	if got, expect := Dll1.Length(), writers*perWriter; got != expect {
		t.Errorf("Expected length %d after concurrent writes, got %d", expect, got)
	}

	// Every pushed element must be present exactly once.
	seen := make(map[string]int)
	for _, v := range Dll1.All() {
		seen[v.S]++
	}
	if len(seen) != writers*perWriter {
		t.Errorf("Expected %d distinct elements, got %d", writers*perWriter, len(seen))
	}
	for k, c := range seen {
		if c != 1 {
			t.Errorf("Element %s seen %d times", k, c)
		}
	}
}

func TestIteratorSnapshotSemantics(t *testing.T) {

	Dll1 := buildList("01", "02", "03")

	// The range-over-func iterators walk a snapshot taken under a read lock
	// before the loop starts, so the loop body may safely call back into the
	// list (no deadlock) without disturbing the iteration in progress.
	var got []string
	first := true
	for _, v := range Dll1.All() {
		got = append(got, v.S)
		if first {
			first = false
			Dll1.Push(&TestDemo{S: "00"}) // mutate while iterating
			if _, err := Dll1.PopTail(); err != nil {
				t.Fatalf("PopTail during iteration: unexpected error %v", err)
			}
		}
	}
	if len(got) != 3 || got[0] != "01" || got[1] != "02" || got[2] != "03" {
		t.Errorf("All snapshot: expected [01 02 03], got %v", got)
	}
	checkModel(t, &Dll1, []string{"00", "01", "02"})

	// Same for Backward: delete the head during the first iteration.
	got = nil
	first = true
	for _, v := range Dll1.Backward() {
		got = append(got, v.S)
		if first {
			first = false
			if err := Dll1.DeleteAtHead(); err != nil {
				t.Fatalf("DeleteAtHead during iteration: unexpected error %v", err)
			}
		}
	}
	if len(got) != 3 || got[0] != "02" || got[1] != "01" || got[2] != "00" {
		t.Errorf("Backward snapshot: expected [02 01 00], got %v", got)
	}
	checkModel(t, &Dll1, []string{"01", "02"})
}

func TestIndexWithDeleteFound(t *testing.T) {

	// Index/IndexFromTail return elements in a form usable with DeleteFound.
	Dll1 := buildList("01", "02", "03", "04")

	// Delete a middle element found via Index.
	el, err := Dll1.Index(1)
	if err != nil {
		t.Fatalf("Index(1): unexpected error %v", err)
	}
	if err := Dll1.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(Index(1)): unexpected error %v", err)
	}
	checkModel(t, &Dll1, []string{"01", "03", "04"})

	// Delete the tail found via IndexFromTail(0).
	el, err = Dll1.IndexFromTail(0)
	if err != nil {
		t.Fatalf("IndexFromTail(0): unexpected error %v", err)
	}
	if err := Dll1.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(IndexFromTail(0)): unexpected error %v", err)
	}
	checkModel(t, &Dll1, []string{"01", "03"})

	// Delete the head found via Index(0).
	el, err = Dll1.Index(0)
	if err != nil {
		t.Fatalf("Index(0): unexpected error %v", err)
	}
	if err := Dll1.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(Index(0)): unexpected error %v", err)
	}
	checkModel(t, &Dll1, []string{"03"})
}

func TestIteratePtrAliasing(t *testing.T) {

	Dll1 := buildList("01", "02")

	// IteratePtr yields pointers that alias the stored data; a write through
	// the pointer is visible in the list.
	for _, p := range Dll1.IteratePtr() {
		if p.S == "02" {
			p.S = "zz"
		}
	}
	checkModel(t, &Dll1, []string{"01", "zz"})
}

// TestConcurrentPushPopIterators exercises concurrent pushers, poppers, and
// iterators with a deterministic final length.  Run with -race to validate.
func TestConcurrentPushPopIterators(t *testing.T) {

	var Dll1 Dll[TestDemo]
	const writers = 4
	const perWriter = 500
	const poppers = 2
	const perPopper = 500

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Readers: iterate both directions while mutations are in flight.
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				for range Dll1.All() {
				}
				for range Dll1.Backward() {
				}
			}
		}()
	}

	var wgMut sync.WaitGroup

	// Writers push disjoint sets of elements.
	for w := 0; w < writers; w++ {
		wgMut.Add(1)
		go func(w int) {
			defer wgMut.Done()
			for i := 0; i < perWriter; i++ {
				v := TestDemo{S: fmt.Sprintf("w%02d-%04d", w, i)}
				Dll1.Push(&v)
			}
		}(w)
	}

	// Poppers each remove exactly perPopper elements, retrying while the
	// list is momentarily empty.  Total pops < total pushes, so they always
	// terminate.
	for p := 0; p < poppers; p++ {
		wgMut.Add(1)
		go func() {
			defer wgMut.Done()
			popped := 0
			for popped < perPopper {
				if _, err := Dll1.PopTail(); err == nil {
					popped++
				}
			}
		}()
	}

	wgMut.Wait()
	close(stop)
	wg.Wait()

	expect := writers*perWriter - poppers*perPopper
	if got := Dll1.Length(); got != expect {
		t.Errorf("Expected length %d after concurrent push/pop, got %d", expect, got)
	}
}
