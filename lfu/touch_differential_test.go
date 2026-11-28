/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"testing"
	"time"

	"github.com/pschlump/pluto/hash_grow"
)

// Differential test for Touch's single-probe form (hash_grow UpsertFunc)
// against the former two-probe form (Search then Insert), kept here
// verbatim as the oracle.  Both are driven by the same seeded random
// draws and the same minute sequence; every returned counter, every
// Counter/IdleMinutes read and the final per-key state must be identical.

// oldTouchOracle is the pre-UpsertFunc Touch body, verbatim apart from
// the injected minute and random draw (Touch draws exactly one clock
// reading, then exactly one rand.Float64, in that order — both forms).
type oldTouchOracle struct {
	tab       *hash_grow.HashTab[lfuEntry[int]]
	logFactor int
	decayTime int
}

func newOldTouchOracle(logFactor, decayTime int) *oldTouchOracle {
	return &oldTouchOracle{
		tab: hash_grow.NewHashTabFunc(
			func(a, b lfuEntry[int]) bool { return a.key == b.key },
			func(e lfuEntry[int]) uint64 { return uint64(e.key)*0x9E3779B97F4A7C15 + 1 },
			16, 0,
		),
		logFactor: logFactor,
		decayTime: decayTime,
	}
}

func (o *oldTouchOracle) touch(key int, now uint16, r float64) uint8 {
	counter := InitVal
	if e, ok := o.tab.Search(lfuEntry[int]{key: key}); ok {
		counter = e.counter
		if o.decayTime > 0 {
			counter = decayed(counter, elapsedMinutes(now, e.lastMin)/o.decayTime)
		}
	}
	counter = incrWithR(counter, o.logFactor, r)
	o.tab.Insert(lfuEntry[int]{key: key, counter: counter, lastMin: now})
	return counter
}

func sortedEntries(tab *hash_grow.HashTab[lfuEntry[int]]) []lfuEntry[int] {
	s := tab.Slice()
	sort.Slice(s, func(i, j int) bool { return s[i].key < s[j].key })
	return s
}

func TestTouchDifferentialVsSearchInsert(t *testing.T) {
	for _, logFactor := range []int{0, 1, 10, 100} {
		for _, decayTime := range []int{0, 1, 3} {
			t.Run(fmt.Sprintf("lf=%d/decay=%d", logFactor, decayTime), func(t *testing.T) {
				rng := rand.New(rand.NewPCG(42, 7))
				minute := int64(65536 - 3000) // crosses the 16-bit minute wrap mid-run
				l := NewLfuWithClock[int](logFactor, decayTime, func() time.Time { return time.Unix(minute*60, 0) })
				o := newOldTouchOracle(logFactor, decayTime)
				const universe = 300
				for step := range 20000 {
					switch p := rng.IntN(100); {
					case p < 70: // advance the clock by 0, 1, a few or many minutes
						switch rng.IntN(10) {
						case 0:
							minute += int64(rng.IntN(200))
						case 1, 2:
							minute += int64(rng.IntN(5))
						}
					case p < 72:
						minute += int64(rng.IntN(70000)) // past a full 16-bit wrap
					}
					key := rng.IntN(universe)
					switch op := rng.IntN(20); {
					case op < 16:
						now := l.minutesNow()
						r := rng.Float64()
						got, want := l.touchAt(key, now, r), o.touch(key, now, r)
						if got != want {
							t.Fatalf("step %d: Touch(%d) = %d, old Search+Insert form = %d", step, key, got, want)
						}
					case op < 18:
						l.Add(key)
						o.tab.Insert(lfuEntry[int]{key: key, counter: InitVal, lastMin: l.minutesNow()})
					default:
						if g, w := l.Delete(key), o.tab.Delete(lfuEntry[int]{key: key}); g != w {
							t.Fatalf("step %d: Delete(%d) = %v, oracle %v", step, key, g, w)
						}
					}
					if step%50 == 0 {
						k := rng.IntN(universe)
						c, ok := l.Counter(k)
						e, eok := o.tab.Search(lfuEntry[int]{key: k})
						if ok != eok {
							t.Fatalf("step %d: Counter(%d) found=%v, oracle %v", step, k, ok, eok)
						}
						if ok {
							wantC := e.counter
							if decayTime > 0 {
								wantC = decayed(wantC, elapsedMinutes(l.minutesNow(), e.lastMin)/decayTime)
							}
							idle, _ := l.IdleMinutes(k)
							if c != wantC || idle != elapsedMinutes(l.minutesNow(), e.lastMin) {
								t.Fatalf("step %d: key %d Counter/IdleMinutes = %d/%d, oracle %d/%d", step, k, c, idle, wantC, elapsedMinutes(l.minutesNow(), e.lastMin))
							}
						}
					}
				}
				got, want := sortedEntries(l.tab), sortedEntries(o.tab)
				if len(got) != len(want) {
					t.Fatalf("final Len %d, oracle %d", len(got), len(want))
				}
				for i := range got {
					if got[i] != want[i] {
						t.Fatalf("final state differs at %d: %+v vs oracle %+v", i, got[i], want[i])
					}
				}
			})
		}
	}
}

// TestTouchAllocs: Touch of a present key and the first-ever Touch of an
// absent key (no growth) allocate nothing — the UpsertFunc closure does
// not escape.
func TestTouchAllocs(t *testing.T) {
	l := NewLfu[int](DefaultLogFactor, DefaultDecayTime)
	for i := range 100 {
		l.Touch(i)
	}
	if a := testing.AllocsPerRun(1000, func() { l.Touch(7) }); a != 0 {
		t.Errorf("Touch of a present key: %v allocs/op, want 0", a)
	}
	if a := testing.AllocsPerRun(1000, func() { l.Touch(1000); l.Delete(1000) }); a != 0 {
		t.Errorf("Touch of an absent key (+Delete): %v allocs/op, want 0", a)
	}
}
