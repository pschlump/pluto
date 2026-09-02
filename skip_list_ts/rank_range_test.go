package skip_list_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"iter"
	"math/rand/v2"
	"sort"
	"testing"
)

// newOrderedList returns a test list loaded with the given keys in the given
// order.
func newOrderedList(keys ...string) *SkipList[TestSkipListNode] {
	list := newTestList()
	for _, k := range keys {
		list.Insert(TestSkipListNode{S: k})
	}
	return list
}

// TestRankBasics verifies Rank against hand-checked positions, including the
// not-found contract for missing keys.
func TestRankBasics(t *testing.T) {
	List1 := newOrderedList("05", "02", "09", "00", "03", "07")

	want := []string{"00", "02", "03", "05", "07", "09"}
	for i, k := range want {
		rank, found := List1.Rank(TestSkipListNode{S: k})
		if !found || rank != i {
			t.Errorf("Rank(%s) = %d found=%v, expected %d found=true", k, rank, found, i)
		}
	}

	// Missing keys report not-found — including keys that fall between, before
	// and after every element.
	for _, k := range []string{"", "01", "04", "06", "08", "10", "99"} {
		if _, found := List1.Rank(TestSkipListNode{S: k}); found {
			t.Errorf("Rank(%s) on missing key reported found", k)
		}
	}

	if r, found := List1.Rank(TestSkipListNode{S: "00"}); !found || r != 0 {
		t.Errorf("Rank of the minimum should be 0, got %d found=%v", r, found)
	}
}

// TestAtIndex verifies AtIndex over a known list plus the out-of-range
// contract.
func TestAtIndex(t *testing.T) {
	List1 := newOrderedList("05", "02", "09", "00", "03", "07")

	want := []string{"00", "02", "03", "05", "07", "09"}
	for i, k := range want {
		v, found := List1.AtIndex(i)
		if !found || v.S != k {
			t.Errorf("AtIndex(%d) = %s found=%v, expected %s found=true", i, v.S, found, k)
		}
	}

	for _, i := range []int{-1, -100, 6, 7, 100} {
		if _, found := List1.AtIndex(i); found {
			t.Errorf("AtIndex(%d) out of range reported found", i)
		}
	}

	// An empty list has no positions at all.
	empty := newTestList()
	for _, i := range []int{-1, 0, 1} {
		if _, found := empty.AtIndex(i); found {
			t.Errorf("AtIndex(%d) on empty list reported found", i)
		}
	}
}

// TestCeilFloor verifies the neighbor queries from both sides of every
// element.
func TestCeilFloor(t *testing.T) {
	List1 := newOrderedList("05", "02", "09", "00", "03", "07")

	cases := []struct {
		probe       string
		ceil, floor string // "" = not found
	}{
		{probe: "00", ceil: "00", floor: "00"}, // exact hits
		{probe: "09", ceil: "09", floor: "09"},
		{probe: "01", ceil: "02", floor: "00"}, // between elements
		{probe: "04", ceil: "05", floor: "03"},
		{probe: "06", ceil: "07", floor: "05"},
		{probe: "08", ceil: "09", floor: "07"},
		{probe: "99", ceil: "", floor: "09"}, // beyond the ends
		{probe: "", ceil: "00", floor: ""},
	}
	for _, tc := range cases {
		probe := TestSkipListNode{S: tc.probe}
		if v, found := List1.Ceil(probe); (found != (tc.ceil != "")) || (found && v.S != tc.ceil) {
			t.Errorf("Ceil(%q) = %q found=%v, expected %q", tc.probe, v.S, found, tc.ceil)
		}
		if v, found := List1.Floor(probe); (found != (tc.floor != "")) || (found && v.S != tc.floor) {
			t.Errorf("Floor(%q) = %q found=%v, expected %q", tc.probe, v.S, found, tc.floor)
		}
	}

	// An empty list has no ceiling and no floor.
	empty := newTestList()
	if _, found := empty.Ceil(TestSkipListNode{S: "00"}); found {
		t.Errorf("Ceil on empty list reported found")
	}
	if _, found := empty.Floor(TestSkipListNode{S: "00"}); found {
		t.Errorf("Floor on empty list reported found")
	}
}

// TestCountRange verifies the inclusive count from both sides of every
// element, plus the empty lo > hi range.
func TestCountRange(t *testing.T) {
	List1 := newOrderedList("05", "02", "09", "00", "03", "07")

	cases := []struct {
		lo, hi string
		want   int
	}{
		{lo: "00", hi: "09", want: 6}, // whole list
		{lo: "00", hi: "00", want: 1}, // single elements
		{lo: "09", hi: "09", want: 1},
		{lo: "03", hi: "07", want: 3},
		{lo: "01", hi: "06", want: 3}, // bounds between elements
		{lo: "06", hi: "08", want: 1},
		{lo: "10", hi: "99", want: 0}, // outside the list
		{lo: "", hi: "00", want: 1},
		{lo: "09", hi: "00", want: 0}, // lo > hi is empty
	}
	for _, tc := range cases {
		lo, hi := TestSkipListNode{S: tc.lo}, TestSkipListNode{S: tc.hi}
		if got := List1.CountRange(lo, hi); got != tc.want {
			t.Errorf("CountRange(%q,%q) = %d, expected %d", tc.lo, tc.hi, got, tc.want)
		}
	}

	if got := newTestList().CountRange(TestSkipListNode{S: "a"}, TestSkipListNode{S: "z"}); got != 0 {
		t.Errorf("CountRange on empty list = %d, expected 0", got)
	}
}

// TestRangeIterators verifies Range and RangeBackward contents, the global
// rank indexes they yield, the empty-range contracts and early exit.
func TestRangeIterators(t *testing.T) {
	List1 := newOrderedList("05", "02", "09", "00", "03", "07")
	// Ascending: 00 02 03 05 07 09 (ranks 0..5).

	collect := func(seq iter.Seq2[int, TestSkipListNode]) (idx []int, vals []string) {
		for i, v := range seq {
			idx = append(idx, i)
			vals = append(vals, v.S)
		}
		return
	}
	K := func(s string) TestSkipListNode { return TestSkipListNode{S: s} }

	// Interior range: 02..07 covers 02 03 05 07 at ranks 1..4.
	idx, vals := collect(List1.Range(K("02"), K("07")))
	if fmt.Sprint(vals) != "[02 03 05 07]" || fmt.Sprint(idx) != "[1 2 3 4]" {
		t.Errorf("Range(02,07) = %v %v, expected [02 03 05 07] [1 2 3 4]", vals, idx)
	}

	// Whole list; bounds that fall between elements; bounds outside.
	idx, vals = collect(List1.Range(K("00"), K("09")))
	if len(vals) != 6 || fmt.Sprint(idx) != "[0 1 2 3 4 5]" {
		t.Errorf("Range(00,09) = %v %v, expected all 6 with ranks 0..5", vals, idx)
	}
	idx, vals = collect(List1.Range(K("01"), K("06")))
	if fmt.Sprint(vals) != "[02 03 05]" || fmt.Sprint(idx) != "[1 2 3]" {
		t.Errorf("Range(01,06) = %v %v, expected [02 03 05] [1 2 3]", vals, idx)
	}
	if _, vals = collect(List1.Range(K("10"), K("99"))); len(vals) != 0 {
		t.Errorf("Range beyond the list yielded %v", vals)
	}

	// lo > hi iterates as empty.
	if _, vals = collect(List1.Range(K("09"), K("00"))); len(vals) != 0 {
		t.Errorf("Range(09,00) yielded %v, expected empty", vals)
	}

	// Backward over the same interior range: descending values, indexes
	// counting down from the same global ranks.
	idx, vals = collect(List1.RangeBackward(K("02"), K("07")))
	if fmt.Sprint(vals) != "[07 05 03 02]" || fmt.Sprint(idx) != "[4 3 2 1]" {
		t.Errorf("RangeBackward(02,07) = %v %v, expected [07 05 03 02] [4 3 2 1]", vals, idx)
	}
	if _, vals = collect(List1.RangeBackward(K("09"), K("00"))); len(vals) != 0 {
		t.Errorf("RangeBackward(09,00) yielded %v, expected empty", vals)
	}

	// Early break stops after the yielded element without corrupting the
	// list: the spans must still hold.
	n := 0
	for range List1.Range(K("00"), K("09")) {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Early break from Range yielded %d items, expected 1", n)
	}
	n = 0
	for range List1.RangeBackward(K("00"), K("09")) {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Early break from RangeBackward yielded %d items, expected 1", n)
	}
	checkInvariant(t, List1, "after range iteration with early break")

	// Empty list ranges iterate as empty.
	empty := newTestList()
	for range empty.Range(K("a"), K("z")) {
		t.Errorf("Range on empty list yielded an item")
	}
	for range empty.RangeBackward(K("a"), K("z")) {
		t.Errorf("RangeBackward on empty list yielded an item")
	}
}

// TestRangeAndBackwardAgree verifies that RangeBackward yields exactly the
// reverse of Range, indexes included.
func TestRangeAndBackwardAgree(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 11))
	List1 := newTestList()
	keys := make([]string, 0, 200)
	for range 200 {
		k := fmt.Sprintf("%04d", rng.IntN(500))
		List1.Insert(TestSkipListNode{S: k})
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for trial := range 50 {
		lo := fmt.Sprintf("%04d", rng.IntN(500))
		hi := fmt.Sprintf("%04d", rng.IntN(500))
		var fwdIdx, bwdIdx []int
		var fwd, bwd []string
		for i, v := range List1.Range(TestSkipListNode{S: lo}, TestSkipListNode{S: hi}) {
			fwdIdx, fwd = append(fwdIdx, i), append(fwd, v.S)
		}
		for i, v := range List1.RangeBackward(TestSkipListNode{S: lo}, TestSkipListNode{S: hi}) {
			bwdIdx, bwd = append(bwdIdx, i), append(bwd, v.S)
		}
		if len(fwd) != len(bwd) {
			t.Fatalf("trial %d: Range yielded %d, RangeBackward %d", trial, len(fwd), len(bwd))
		}
		for i := range fwd {
			if fwd[i] != bwd[len(fwd)-1-i] || fwdIdx[i] != bwdIdx[len(fwd)-1-i] {
				t.Fatalf("trial %d: Range and RangeBackward disagree at %d: %v/%v vs %v/%v",
					trial, i, fwd, fwdIdx, bwd, bwdIdx)
			}
		}
	}
}

// TestDeleteRange verifies the inclusive bulk delete and everything it must
// leave intact.
func TestDeleteRange(t *testing.T) {
	List1 := newOrderedList("05", "02", "09", "00", "03", "07")

	if n := List1.DeleteRange(TestSkipListNode{S: "09"}, TestSkipListNode{S: "00"}); n != 0 {
		t.Errorf("DeleteRange with lo > hi removed %d, expected 0", n)
	}
	if n := List1.DeleteRange(TestSkipListNode{S: "10"}, TestSkipListNode{S: "99"}); n != 0 {
		t.Errorf("DeleteRange beyond the list removed %d, expected 0", n)
	}
	if n := newTestList().DeleteRange(TestSkipListNode{S: "a"}, TestSkipListNode{S: "z"}); n != 0 {
		t.Errorf("DeleteRange on empty list removed %d, expected 0", n)
	}

	// Interior delete: 02..05 removes 02 03 05.
	if n := List1.DeleteRange(TestSkipListNode{S: "02"}, TestSkipListNode{S: "05"}); n != 3 {
		t.Errorf("DeleteRange(02,05) removed %d, expected 3", n)
	}
	if List1.Length() != 3 {
		t.Errorf("Length after DeleteRange = %d, expected 3", List1.Length())
	}
	checkInvariant(t, List1, "after interior DeleteRange")
	var rest []string
	for v := range List1.All() {
		rest = append(rest, v.S)
	}
	if fmt.Sprint(rest) != "[00 07 09]" {
		t.Errorf("Remaining elements = %v, expected [00 07 09]", rest)
	}

	// Ranks shift after the delete.
	if r, found := List1.Rank(TestSkipListNode{S: "09"}); !found || r != 2 {
		t.Errorf("Rank(09) after delete = %d found=%v, expected 2", r, found)
	}

	// Whole-list delete empties it.
	if n := List1.DeleteRange(TestSkipListNode{S: ""}, TestSkipListNode{S: "zz"}); n != 3 {
		t.Errorf("Whole-list DeleteRange removed %d, expected 3", n)
	}
	if !List1.IsEmpty() || List1.level != 0 {
		t.Errorf("List not clean after whole-list DeleteRange: empty=%v level=%d", List1.IsEmpty(), List1.level)
	}
}

// TestDeleteByRank verifies the rank-window bulk delete, its clamping rules
// and its effect on the remaining ranks.
func TestDeleteByRank(t *testing.T) {
	build := func() *SkipList[TestSkipListNode] {
		return newOrderedList("05", "02", "09", "00", "03", "07")
	}
	K := func(s string) TestSkipListNode { return TestSkipListNode{S: s} }

	// Degenerate windows remove nothing.
	for _, tc := range []struct{ start, stop int }{{-1, 2}, {3, 2}, {6, 9}, {100, 200}} {
		List1 := build()
		if n := List1.DeleteByRank(tc.start, tc.stop); n != 0 {
			t.Errorf("DeleteByRank(%d,%d) removed %d, expected 0", tc.start, tc.stop, n)
		}
		if List1.Length() != 6 {
			t.Errorf("List changed after no-op DeleteByRank(%d,%d)", tc.start, tc.stop)
		}
	}
	if n := newTestList().DeleteByRank(0, 5); n != 0 {
		t.Errorf("DeleteByRank on empty list removed %d, expected 0", n)
	}

	// Head window: ranks 0..1 are 00 02.
	List1 := build()
	if n := List1.DeleteByRank(0, 1); n != 2 {
		t.Errorf("DeleteByRank(0,1) removed %d, expected 2", n)
	}
	if v, found := List1.FindMin(); !found || v.S != "03" {
		t.Errorf("FindMin after DeleteByRank(0,1) = %s, expected 03", v.S)
	}
	checkInvariant(t, List1, "after DeleteByRank(0,1)")

	// stop is clamped to Len()-1.
	List1 = build()
	if n := List1.DeleteByRank(4, 99); n != 2 {
		t.Errorf("DeleteByRank(4,99) removed %d, expected 2 (clamped)", n)
	}
	if v, found := List1.FindMax(); !found || v.S != "05" {
		t.Errorf("FindMax after clamped DeleteByRank = %s, expected 05", v.S)
	}

	// Interior window: ranks 2..3 are 03 05.
	List1 = build()
	if n := List1.DeleteByRank(2, 3); n != 2 {
		t.Errorf("DeleteByRank(2,3) removed %d, expected 2", n)
	}
	var rest []string
	for v := range List1.All() {
		rest = append(rest, v.S)
	}
	if fmt.Sprint(rest) != "[00 02 07 09]" {
		t.Errorf("Remaining after DeleteByRank(2,3) = %v, expected [00 02 07 09]", rest)
	}
	if r, found := List1.Rank(K("09")); !found || r != 3 {
		t.Errorf("Rank(09) after window delete = %d found=%v, expected 3", r, found)
	}

	// Whole-list window; the list must come back clean.
	if n := List1.DeleteByRank(0, List1.Length()-1); n != 4 {
		t.Errorf("Whole-list DeleteByRank removed %d, expected 4", n)
	}
	if !List1.IsEmpty() || List1.level != 0 {
		t.Errorf("List not clean after whole-list DeleteByRank")
	}
	// ...and it is immediately reusable.
	if !List1.Insert(K("42")) || List1.Length() != 1 {
		t.Errorf("List not reusable after whole-list DeleteByRank")
	}
	if r, found := List1.Rank(K("42")); !found || r != 0 {
		t.Errorf("Rank after rebuild = %d found=%v, expected 0", r, found)
	}
}

// TestRankRangeRandomizedOracle is the required oracle test: N=10000 random
// inserts and deletes, cross-checking Rank, AtIndex, Ceil, Floor, Range,
// RangeBackward, CountRange, DeleteRange and DeleteByRank against a
// sorted-slice oracle after every batch.
func TestRankRangeRandomizedOracle(t *testing.T) {
	const Batches = 20
	const OpsPerBatch = 500
	const KeySpace = 4000

	rng := rand.New(rand.NewPCG(42, 7))
	List1 := newTestList()
	var oracle []string // always kept sorted

	K := func(s string) TestSkipListNode { return TestSkipListNode{S: s} }

	// oracleRange returns the oracle slice for lo <= x <= hi.
	oracleRange := func(lo, hi string) []string {
		loAt := sort.SearchStrings(oracle, lo)
		hiAt := sort.SearchStrings(oracle, hi)
		for hiAt < len(oracle) && oracle[hiAt] <= hi {
			hiAt++
		}
		if loAt >= hiAt {
			return nil
		}
		return oracle[loAt:hiAt]
	}

	check := func(step string) {
		t.Helper()
		if List1.Length() != len(oracle) {
			t.Fatalf("%s: Length=%d, oracle has %d", step, List1.Length(), len(oracle))
		}
		checkInvariant(t, List1, step)

		// Rank of every present key; AtIndex of a sample of positions.
		for i, k := range oracle {
			if r, found := List1.Rank(K(k)); !found || r != i {
				t.Fatalf("%s: Rank(%s) = %d found=%v, expected %d", step, k, r, found, i)
			}
		}
		for _, i := range []int{0, 1, len(oracle) / 2, len(oracle) - 1} {
			if i < 0 || i >= len(oracle) {
				continue
			}
			if v, found := List1.AtIndex(i); !found || v.S != oracle[i] {
				t.Fatalf("%s: AtIndex(%d) = %s found=%v, expected %s", step, i, v.S, found, oracle[i])
			}
		}
		if _, found := List1.AtIndex(len(oracle)); found {
			t.Fatalf("%s: AtIndex(len) reported found", step)
		}
		if _, found := List1.AtIndex(-1); found {
			t.Fatalf("%s: AtIndex(-1) reported found", step)
		}

		// Ceil/Floor, CountRange, Range and RangeBackward over a fixed probe
		// set: exact members, gaps between members, and the far ends.
		probes := []string{"", "0000", "2000", "3999", "4000", "9999"}
		if len(oracle) > 0 {
			probes = append(probes, oracle[0], oracle[len(oracle)-1])
		}
		for _, p := range probes {
			wantCeil, ceilOK := "", false
			wantFloor, floorOK := "", false
			at := sort.SearchStrings(oracle, p)
			if at < len(oracle) && oracle[at] == p {
				// The probe is present: it is its own ceiling and floor.
				wantCeil, ceilOK = p, true
				wantFloor, floorOK = p, true
			} else {
				if at < len(oracle) {
					wantCeil, ceilOK = oracle[at], true
				}
				if at > 0 {
					wantFloor, floorOK = oracle[at-1], true
				}
			}
			if v, found := List1.Ceil(K(p)); found != ceilOK || (found && v.S != wantCeil) {
				t.Fatalf("%s: Ceil(%s) = %s found=%v, expected %s found=%v", step, p, v.S, found, wantCeil, ceilOK)
			}
			if v, found := List1.Floor(K(p)); found != floorOK || (found && v.S != wantFloor) {
				t.Fatalf("%s: Floor(%s) = %s found=%v, expected %s found=%v", step, p, v.S, found, wantFloor, floorOK)
			}

			want := oracleRange("0000", p)
			if got := List1.CountRange(K("0000"), K(p)); got != len(want) {
				t.Fatalf("%s: CountRange(0000,%s) = %d, expected %d", step, p, got, len(want))
			}
			var gotVals []string
			var gotIdx []int
			for i, v := range List1.Range(K("0000"), K(p)) {
				gotVals, gotIdx = append(gotVals, v.S), append(gotIdx, i)
			}
			if fmt.Sprint(gotVals) != fmt.Sprint(want) {
				t.Fatalf("%s: Range(0000,%s) = %v, expected %v", step, p, gotVals, want)
			}
			for i := range want {
				if gotIdx[i] != i {
					t.Fatalf("%s: Range(0000,%s) index %d = %d", step, p, i, gotIdx[i])
				}
			}
			bwdVals, bwdIdx := []string{}, []int{}
			for i, v := range List1.RangeBackward(K("0000"), K(p)) {
				bwdVals, bwdIdx = append(bwdVals, v.S), append(bwdIdx, i)
			}
			if len(bwdVals) != len(want) {
				t.Fatalf("%s: RangeBackward(0000,%s) len %d, expected %d", step, p, len(bwdVals), len(want))
			}
			for i := range want {
				if bwdVals[i] != want[len(want)-1-i] || bwdIdx[i] != len(want)-1-i {
					t.Fatalf("%s: RangeBackward(0000,%s) disagrees at %d", step, p, i)
				}
			}
		}
	}

	for batch := range Batches {
		for range OpsPerBatch {
			k := fmt.Sprintf("%04d", rng.IntN(KeySpace))
			switch rng.IntN(4) {
			case 0, 1, 2:
				List1.Insert(K(k))
				at := sort.SearchStrings(oracle, k)
				if at == len(oracle) || oracle[at] != k {
					oracle = append(oracle, "")
					copy(oracle[at+1:], oracle[at:])
					oracle[at] = k
				}
			case 3:
				List1.Delete(K(k))
				at := sort.SearchStrings(oracle, k)
				if at < len(oracle) && oracle[at] == k {
					oracle = append(oracle[:at], oracle[at+1:]...)
				}
			}
		}
		check(fmt.Sprintf("batch %d", batch))

		// Every other batch also exercises a bulk removal against the oracle.
		if batch%2 == 1 && len(oracle) > 4 {
			switch rng.IntN(2) {
			case 0: // DeleteRange over a random window
				lo := fmt.Sprintf("%04d", rng.IntN(KeySpace))
				hi := fmt.Sprintf("%04d", rng.IntN(KeySpace))
				if lo > hi {
					lo, hi = hi, lo
				}
				want := oracleRange(lo, hi)
				n := List1.DeleteRange(K(lo), K(hi))
				if n != len(want) {
					t.Fatalf("batch %d: DeleteRange(%s,%s) removed %d, expected %d",
						batch, lo, hi, n, len(want))
				}
				kept := oracle[:0]
				for _, k := range oracle {
					if k < lo || k > hi {
						kept = append(kept, k)
					}
				}
				oracle = kept
			case 1: // DeleteByRank over a random window
				start := rng.IntN(len(oracle) - 2)
				stop := start + rng.IntN(len(oracle)-start)
				want := oracle[start : stop+1]
				n := List1.DeleteByRank(start, stop)
				if n != len(want) {
					t.Fatalf("batch %d: DeleteByRank(%d,%d) removed %d, expected %d",
						batch, start, stop, n, len(want))
				}
				oracle = append(oracle[:start], oracle[stop+1:]...)
			}
			check(fmt.Sprintf("batch %d after bulk delete", batch))
		}
	}

	// Drain via DeleteByRank windows and verify the list comes back clean.
	for !List1.IsEmpty() {
		List1.DeleteByRank(0, List1.Length()/2)
		checkInvariant(t, List1, "drain")
	}
	if List1.level != 0 {
		t.Errorf("level = %d after full drain, expected 0", List1.level)
	}
	if !List1.Insert(K("0000")) {
		t.Errorf("list not reusable after drain")
	}
	if r, found := List1.Rank(K("0000")); !found || r != 0 {
		t.Errorf("Rank after drain-and-rebuild = %d found=%v, expected 0 true", r, found)
	}
}

// TestRankNilAndZeroTolerated verifies the empty-list contracts of every new
// operation on nil and zero-value lists.
func TestRankNilAndZeroTolerated(t *testing.T) {
	key := TestSkipListNode{S: "12"}

	var nilList *SkipList[TestSkipListNode]
	if r, found := nilList.Rank(key); found || r != 0 {
		t.Errorf("Rank on nil list = %d found=%v, expected 0 false", r, found)
	}
	if _, found := nilList.AtIndex(0); found {
		t.Errorf("AtIndex on nil list reported found")
	}
	if _, found := nilList.Ceil(key); found {
		t.Errorf("Ceil on nil list reported found")
	}
	if _, found := nilList.Floor(key); found {
		t.Errorf("Floor on nil list reported found")
	}
	if n := nilList.CountRange(key, key); n != 0 {
		t.Errorf("CountRange on nil list = %d, expected 0", n)
	}
	if n := nilList.DeleteRange(key, key); n != 0 {
		t.Errorf("DeleteRange on nil list = %d, expected 0", n)
	}
	if n := nilList.DeleteByRank(0, 9); n != 0 {
		t.Errorf("DeleteByRank on nil list = %d, expected 0", n)
	}
	for range nilList.Range(key, key) {
		t.Errorf("Range on nil list yielded an item")
	}
	for range nilList.RangeBackward(key, key) {
		t.Errorf("RangeBackward on nil list yielded an item")
	}

	var zero SkipList[TestSkipListNode]
	if _, found := zero.Rank(key); found {
		t.Errorf("Rank on zero-value list reported found")
	}
	if _, found := zero.AtIndex(0); found {
		t.Errorf("AtIndex on zero-value list reported found")
	}
	if _, found := zero.Ceil(key); found {
		t.Errorf("Ceil on zero-value list reported found")
	}
	if _, found := zero.Floor(key); found {
		t.Errorf("Floor on zero-value list reported found")
	}
	if n := zero.CountRange(key, key); n != 0 {
		t.Errorf("CountRange on zero-value list = %d, expected 0", n)
	}
	if n := zero.DeleteRange(key, key); n != 0 {
		t.Errorf("DeleteRange on zero-value list = %d, expected 0", n)
	}
	if n := zero.DeleteByRank(0, 9); n != 0 {
		t.Errorf("DeleteByRank on zero-value list = %d, expected 0", n)
	}
	for range zero.Range(key, key) {
		t.Errorf("Range on zero-value list yielded an item")
	}
	for range zero.RangeBackward(key, key) {
		t.Errorf("RangeBackward on zero-value list yielded an item")
	}

	// After Truncate the new structures reset cleanly (spans included).
	List1 := newOrderedList("05", "02", "09")
	List1.Truncate()
	checkInvariant(t, List1, "after Truncate")
	if n := List1.DeleteRange(TestSkipListNode{S: "00"}, TestSkipListNode{S: "99"}); n != 0 {
		t.Errorf("DeleteRange after Truncate = %d, expected 0", n)
	}
	List1.Insert(TestSkipListNode{S: "42"})
	if r, found := List1.Rank(TestSkipListNode{S: "42"}); !found || r != 0 {
		t.Errorf("Rank after rebuild = %d found=%v, expected 0 true", r, found)
	}
	checkInvariant(t, List1, "after rebuild")
}

// TestRangeSnapshotSemantics verifies that the Range/RangeBackward
// iterators operate on a snapshot taken when they are called: later
// modifications — even truncating the whole list — are not observed, and
// mutating the list from inside the loop is safe (the inverted contract
// from the plain package's live-walk Range).
func TestRangeSnapshotSemantics(t *testing.T) {
	list := newOrderedList("05", "02", "09", "00", "03", "07")
	K := func(s string) TestSkipListNode { return TestSkipListNode{S: s} }

	rng := list.Range(K("00"), K("09"))
	rbw := list.RangeBackward(K("02"), K("07"))

	list.Truncate() // the iterators above must not observe this

	var fwd []string
	for _, v := range rng {
		fwd = append(fwd, v.S)
	}
	if fmt.Sprint(fwd) != "[00 02 03 05 07 09]" {
		t.Errorf("Range after Truncate = %v, expected the snapshot [00 02 03 05 07 09]", fwd)
	}
	var bwd []string
	for _, v := range rbw {
		bwd = append(bwd, v.S)
	}
	if fmt.Sprint(bwd) != "[07 05 03 02]" {
		t.Errorf("RangeBackward after Truncate = %v, expected the snapshot [07 05 03 02]", bwd)
	}

	// Mutating from inside the loop is safe: the loop sees the snapshot.
	list = newOrderedList("05", "02", "09")
	visited := 0
	for _, v := range list.Range(K("00"), K("99")) {
		visited++
		list.Delete(v)
	}
	if visited != 3 {
		t.Errorf("Expected 3 visits while deleting during Range iteration, got %d", visited)
	}
	if !list.IsEmpty() {
		t.Errorf("Expected empty list after deleting every visited element.")
	}
}

// TestLockNlCompound verifies the atomic rank-then-delete compound: under a
// client-held Lock, NlRank + NlDeleteByRank remove exactly the ranked
// element, and the Nl* reads behave like their locked counterparts.
func TestLockNlCompound(t *testing.T) {
	list := newOrderedList("05", "02", "09", "00", "03", "07")
	K := func(s string) TestSkipListNode { return TestSkipListNode{S: s} }

	list.Lock()
	if n := list.NlLen(); n != 6 {
		t.Errorf("NlLen = %d, expected 6", n)
	}
	if list.NlIsEmpty() { // a 6-element list is not empty
		t.Errorf("NlIsEmpty reported true on a 6-element list")
	}
	if r, found := list.NlRank(K("05")); !found || r != 3 {
		t.Errorf("NlRank(05) = %d found=%v, expected 3", r, found)
	}
	if v, found := list.NlAtIndex(4); !found || v.S != "07" {
		t.Errorf("NlAtIndex(4) = %s found=%v, expected 07", v.S, found)
	}
	if v, found := list.NlCeil(K("04")); !found || v.S != "05" {
		t.Errorf("NlCeil(04) = %s found=%v, expected 05", v.S, found)
	}
	if v, found := list.NlFloor(K("04")); !found || v.S != "03" {
		t.Errorf("NlFloor(04) = %s found=%v, expected 03", v.S, found)
	}
	if n := list.NlCountRange(K("02"), K("07")); n != 4 {
		t.Errorf("NlCountRange(02,07) = %d, expected 4", n)
	}

	// Rank-then-delete: remove "05" atomically.
	r, found := list.NlRank(K("05"))
	if !found {
		t.Fatalf("NlRank(05) not found")
	}
	if n := list.NlDeleteByRank(r, r); n != 1 {
		t.Errorf("NlDeleteByRank(%d,%d) removed %d, expected 1", r, r, n)
	}
	// And a ranked window via the Nl bulk delete.
	if n := list.NlDeleteRange(K("00"), K("02")); n != 2 {
		t.Errorf("NlDeleteRange(00,02) removed %d, expected 2", n)
	}
	list.Unlock()

	if list.Length() != 3 {
		t.Errorf("Length after compound = %d, expected 3", list.Length())
	}
	checkInvariant(t, list, "after Lock/Nl compound")
	var rest []string
	for v := range list.All() {
		rest = append(rest, v.S)
	}
	if fmt.Sprint(rest) != "[03 07 09]" {
		t.Errorf("Remaining after compound = %v, expected [03 07 09]", rest)
	}

	// A nil list tolerates the Nl* surface too (its Lock is a no-op).
	var nilList *SkipList[TestSkipListNode]
	nilList.Lock()
	if _, found := nilList.NlRank(K("x")); found {
		t.Errorf("NlRank on nil list reported found")
	}
	if n := nilList.NlDeleteRange(K("a"), K("z")); n != 0 {
		t.Errorf("NlDeleteRange on nil list = %d, expected 0", n)
	}
	nilList.Unlock()
}
