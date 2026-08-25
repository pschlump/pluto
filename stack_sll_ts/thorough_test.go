package stack

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"errors"
	"math/rand"
	"slices"
	"sync"
	"testing"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/sll_ts"
)

// checkInvariant verifies the stack against a reference model where
// model[0] is the bottom and model[len-1] is the top.
func checkInvariant[T comparable.Equality](t *testing.T, stk *Stack[T], model []T) {
	t.Helper()
	if got, want := stk.Length(), len(model); got != want {
		t.Fatalf("Length: expected %d, got %d", want, got)
	}
	if got, want := stk.IsEmpty(), len(model) == 0; got != want {
		t.Errorf("IsEmpty: expected %v, got %v", want, got)
	}
	// All must walk top to bottom.
	var topDown []T
	for i, v := range stk.All() {
		if i != len(topDown) {
			t.Errorf("All: expected index %d, got %d", len(topDown), i)
		}
		topDown = append(topDown, *v)
	}
	want := make([]T, len(model))
	for i, v := range model {
		want[len(model)-1-i] = v
	}
	eq := func(a, b T) bool { return a.IsEqual(b) }
	if !slices.EqualFunc(topDown, want, eq) {
		t.Errorf("All: expected %v, got %v", want, topDown)
	}
	// Backward must walk bottom to top.
	var bottomUp []T
	for i, v := range stk.Backward() {
		if i != len(bottomUp) {
			t.Errorf("Backward: expected index %d, got %d", len(bottomUp), i)
		}
		bottomUp = append(bottomUp, *v)
	}
	if !slices.EqualFunc(bottomUp, model, eq) {
		t.Errorf("Backward: expected %v, got %v", model, bottomUp)
	}
}

func TestZeroValue(t *testing.T) {
	var stk Stack[Int]

	if !stk.IsEmpty() {
		t.Errorf("Expected zero-value stack to be empty")
	}
	if got := stk.Length(); got != 0 {
		t.Errorf("Expected length 0 on zero-value stack, got %d", got)
	}
	if _, err := stk.Peek(); !errors.Is(err, sll_ts.ErrEmptySll) {
		t.Errorf("Expected sll_ts.ErrEmptySll from Peek on empty stack, got %v", err)
	}
	if _, err := stk.Pop(); !errors.Is(err, sll_ts.ErrEmptySll) {
		t.Errorf("Expected sll_ts.ErrEmptySll from Pop on empty stack, got %v", err)
	}
	n := 0
	for range stk.All() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected All on empty stack to yield nothing, got %d elements", n)
	}
	n = 0
	for range stk.Backward() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected Backward on empty stack to yield nothing, got %d elements", n)
	}
	// Truncate on an empty stack must be a no-op, not a panic.
	stk.Truncate()
	if !stk.IsEmpty() {
		t.Errorf("Expected stack to remain empty after Truncate on empty stack")
	}
}

func TestSingleElement(t *testing.T) {
	var stk Stack[Int]
	v := Int(42)
	stk.Push(&v)

	if stk.IsEmpty() {
		t.Errorf("Expected non-empty stack after one Push")
	}
	if got := stk.Length(); got != 1 {
		t.Errorf("Expected length 1, got %d", got)
	}

	p, err := stk.Peek()
	if err != nil {
		t.Fatalf("Unexpected error from Peek: %v", err)
	}
	if *p != 42 {
		t.Errorf("Expected Peek of 42, got %v", *p)
	}
	// Peek must not remove the element.
	if got := stk.Length(); got != 1 {
		t.Errorf("Expected length 1 after Peek, got %d", got)
	}

	got, err := stk.Pop()
	if err != nil {
		t.Fatalf("Unexpected error from Pop: %v", err)
	}
	if *got != 42 {
		t.Errorf("Expected Pop of 42, got %v", *got)
	}
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after popping the only element")
	}
	if _, err := stk.Pop(); !errors.Is(err, sll_ts.ErrEmptySll) {
		t.Errorf("Expected sll_ts.ErrEmptySll after draining stack, got %v", err)
	}
}

func TestDuplicateValues(t *testing.T) {
	var stk Stack[Int]
	v := Int(7)
	stk.Push(&v)
	stk.Push(&v)
	stk.Push(&v)

	if got := stk.Length(); got != 3 {
		t.Fatalf("Expected length 3, got %d", got)
	}
	for i := 0; i < 3; i++ {
		got, err := stk.Pop()
		if err != nil {
			t.Fatalf("Unexpected error from Pop: %v", err)
		}
		if *got != 7 {
			t.Errorf("Expected 7, got %v", *got)
		}
	}
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after popping all duplicates")
	}
}

func TestLIFOOrder(t *testing.T) {
	var stk Stack[Int]
	for i := 0; i < 10; i++ {
		v := Int(i)
		stk.Push(&v)
	}
	// Pop must return elements in exact reverse push order.
	for i := 9; i >= 0; i-- {
		got, err := stk.Pop()
		if err != nil {
			t.Fatalf("Unexpected error from Pop: %v", err)
		}
		if int(*got) != i {
			t.Errorf("Expected %d, got %d", i, *got)
		}
	}
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after draining")
	}
}

func TestTruncate(t *testing.T) {
	var stk Stack[Int]
	for i := 0; i < 5; i++ {
		v := Int(i)
		stk.Push(&v)
	}
	stk.Truncate()
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after Truncate")
	}
	if got := stk.Length(); got != 0 {
		t.Errorf("Expected length 0 after Truncate, got %d", got)
	}
	if _, err := stk.Pop(); !errors.Is(err, sll_ts.ErrEmptySll) {
		t.Errorf("Expected sll_ts.ErrEmptySll after Truncate, got %v", err)
	}
	// The stack must be fully usable again after Truncate.
	v := Int(99)
	stk.Push(&v)
	got, err := stk.Pop()
	if err != nil {
		t.Fatalf("Unexpected error from Pop after Truncate: %v", err)
	}
	if *got != 99 {
		t.Errorf("Expected 99, got %v", *got)
	}
}

func TestIteratorsEarlyBreak(t *testing.T) {
	var stk Stack[Int]
	for i := 1; i <= 5; i++ {
		v := Int(i)
		stk.Push(&v)
	}

	// All early break: top element is 5.
	n := 0
	for i, v := range stk.All() {
		if i != 0 || int(*v) != 5 {
			t.Errorf("All: expected (0, 5), got (%d, %d)", i, *v)
		}
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected All early exit after 1 element, got %d", n)
	}

	// Backward early break: bottom element is 1.
	n = 0
	for i, v := range stk.Backward() {
		if i != 0 || int(*v) != 1 {
			t.Errorf("Backward: expected (0, 1), got (%d, %d)", i, *v)
		}
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected Backward early exit after 1 element, got %d", n)
	}

	// Breaking part-way through Backward must stop exactly there.
	n = 0
	for range stk.Backward() {
		n++
		if n == 3 {
			break
		}
	}
	if n != 3 {
		t.Errorf("Expected Backward to stop after 3 elements, got %d", n)
	}

	// Early break must not disturb the stack contents.
	if got := stk.Length(); got != 5 {
		t.Errorf("Expected length 5 after iterator breaks, got %d", got)
	}
}

func TestBackwardSnapshot(t *testing.T) {
	var stk Stack[Int]
	for i := 1; i <= 3; i++ {
		v := Int(i)
		stk.Push(&v)
	}

	// Backward works on a snapshot: a Pop that happens after the snapshot
	// is taken (here simulated by popping between the collection pass and
	// consumption via a second iteration) must not affect a fresh call.
	var first []int
	for _, v := range stk.Backward() {
		first = append(first, int(*v))
	}
	if want := []int{1, 2, 3}; !slices.Equal(first, want) {
		t.Errorf("Backward: expected %v, got %v", want, first)
	}
	if _, err := stk.Pop(); err != nil {
		t.Fatalf("Unexpected error from Pop: %v", err)
	}
	var second []int
	for _, v := range stk.Backward() {
		second = append(second, int(*v))
	}
	if want := []int{1, 2}; !slices.Equal(second, want) {
		t.Errorf("Backward after Pop: expected %v, got %v", want, second)
	}
}

// TestRandomizedAgainstModel runs hundreds of mixed operations with a fixed
// seed and cross-checks the stack against a plain slice used as a reference
// LIFO model (model[0] is the bottom).
func TestRandomizedAgainstModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	var stk Stack[Int]
	var model []Int

	for op := 0; op < 2000; op++ {
		switch rng.Intn(6) {
		case 0, 1, 2: // Push (weighted toward growth)
			v := Int(rng.Intn(100))
			stk.Push(&v)
			model = append(model, v)
		case 3, 4: // Pop
			got, err := stk.Pop()
			if len(model) == 0 {
				if !errors.Is(err, sll_ts.ErrEmptySll) {
					t.Fatalf("op %d: expected sll_ts.ErrEmptySll, got %v", op, err)
				}
			} else {
				if err != nil {
					t.Fatalf("op %d: unexpected error from Pop: %v", op, err)
				}
				if *got != model[len(model)-1] {
					t.Fatalf("op %d: Pop expected %v, got %v", op, model[len(model)-1], *got)
				}
				model = model[:len(model)-1]
			}
		case 5: // Peek / Length / IsEmpty checks
			if len(model) == 0 {
				if _, err := stk.Peek(); !errors.Is(err, sll_ts.ErrEmptySll) {
					t.Fatalf("op %d: expected sll_ts.ErrEmptySll from Peek, got %v", op, err)
				}
			} else {
				p, err := stk.Peek()
				if err != nil {
					t.Fatalf("op %d: unexpected error from Peek: %v", op, err)
				}
				if *p != model[len(model)-1] {
					t.Fatalf("op %d: Peek expected %v, got %v", op, model[len(model)-1], *p)
				}
			}
		}

		if got, want := stk.Length(), len(model); got != want {
			t.Fatalf("op %d: Length expected %d, got %d", op, want, got)
		}
		if got, want := stk.IsEmpty(), len(model) == 0; got != want {
			t.Fatalf("op %d: IsEmpty expected %v, got %v", op, want, got)
		}
	}

	// Occasionally exercise Truncate, then re-verify.
	stk.Truncate()
	model = model[:0]
	checkInvariant(t, &stk, model)
	v := Int(1)
	stk.Push(&v)
	model = append(model, v)
	checkInvariant(t, &stk, model)
}

// TestConcurrentReadersWriters exercises concurrent pushes, pops, peeks,
// length checks and iterator walks.  It is primarily useful under
// `go test -race`.
func TestConcurrentReadersWriters(t *testing.T) {
	var stk Stack[Int]
	const perWriter = 200
	var wg sync.WaitGroup

	// Writers.
	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				v := Int(i)
				stk.Push(&v)
			}
		}()
	}

	// Readers: Peek/Length/IsEmpty must never fail or observe corruption.
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				if _, err := stk.Peek(); err != nil && !errors.Is(err, sll_ts.ErrEmptySll) {
					t.Errorf("Unexpected error from Peek: %v", err)
					return
				}
				if n := stk.Length(); n < 0 {
					t.Errorf("Negative length %d", n)
					return
				}
				_ = stk.IsEmpty()
			}
		}()
	}

	wg.Wait()
	if got, want := stk.Length(), 3*perWriter; got != want {
		t.Fatalf("Expected length %d, got %d", want, got)
	}

	// After the writers finish, concurrent iteration plus reads must be
	// consistent with the final state.
	var sums sync.Map
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			count := 0
			for range stk.All() {
				count++
			}
			sums.Store(id, count)
		}(w)
	}
	wg.Wait()
	sums.Range(func(_, val any) bool {
		if val.(int) != 3*perWriter {
			t.Errorf("Iterator saw %d elements, expected %d", val.(int), 3*perWriter)
		}
		return true
	})
}
