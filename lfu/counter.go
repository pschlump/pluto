/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu

import "math/rand/v2"

// InitVal is the counter value a fresh key starts at (Redis LFU_INIT_VAL
// = 5): new keys collect a few accesses at near-certain increment
// probability before the logarithmic ramp begins, so they are not
// evicted as coldest before they have had a chance to heat up.
const InitVal uint8 = 5

// MaxVal is the saturation point of the counter (the 8-bit register
// width).  Incr at MaxVal stays at MaxVal.
const MaxVal uint8 = 255

// MorrisCounter is an 8-bit logarithmic probabilistic counter — the
// Morris technique (Morris 1978, "Counting large numbers of events in
// small registers") in the exact form Redis uses for LFU eviction
// (LFULogIncr in note/redis/src/evict.c): the stored value v approximates
// the logarithm of the number of events, and each event increments v
// only with probability 1/((v-InitVal)·logFactor+1) — so the first
// Increment after InitVal is certain, and each further one is
// logFactor times less likely than the one before it.  With logFactor 0
// every event increments (a plain saturating count); with Redis's
// default 10, one million accesses land near 145.
//
// The zero value is a usable counter holding 0 (a legal but
// under-counted state — Redis keys start at InitVal, which is what
// NewMorrisCounter builds).  Incr and Decay mutate and panic on a nil
// *MorrisCounter; Value on a nil reports 0.
//
// Incr draws from the global math/rand/v2 source (the skip_list
// convention — no per-counter RNG state to allocate or lock), so
// outputs that depend on the draws are nondeterministic and must not
// appear as fixed assertions; incrWithR is the deterministic core the
// tests drive with a seeded source.
type MorrisCounter struct {
	v uint8
}

// NewMorrisCounter returns a counter at InitVal — the state a fresh
// Redis key's LFU counter starts in.
// Complexity is O(1).
func NewMorrisCounter() *MorrisCounter {
	return &MorrisCounter{v: InitVal}
}

// Incr records one event: with probability 1.0/((v-InitVal)·logFactor+1)
// the stored value gains 1, saturating at MaxVal.  A logFactor ≤ 0 makes
// every increment certain (p = 1, the Redis math at factor 0).  It
// returns the new value.  Incr on a nil *MorrisCounter panics.
// Complexity is O(1).
func (c *MorrisCounter) Incr(logFactor int) uint8 {
	if c == nil {
		panic("lfu: Incr on a nil *MorrisCounter — create one with NewMorrisCounter() or use a zero MorrisCounter value")
	}
	c.v = incrWithR(c.v, logFactor, rand.Float64())
	return c.v
}

// Decay decrements the counter by periods, the number of elapsed decay
// periods the caller computed — the Lfu table derives it from its clock;
// a standalone counter owner does the same.  A period count below 1 is
// a no-op; a count above the stored value saturates at 0 (Redis
// LFUDecrAndReturn).  It returns the new value.  Decay on a nil
// *MorrisCounter panics.
// Complexity is O(1).
func (c *MorrisCounter) Decay(periods int) uint8 {
	if c == nil {
		panic("lfu: Decay on a nil *MorrisCounter — create one with NewMorrisCounter() or use a zero MorrisCounter value")
	}
	c.v = decayed(c.v, periods)
	return c.v
}

// Value returns the current counter value.  A nil *MorrisCounter
// reports 0.
// Complexity is O(1).
func (c *MorrisCounter) Value() uint8 {
	if c == nil {
		return 0
	}
	return c.v
}

// incrWithR is LFULogIncr with the random draw passed in — the
// deterministic core of Incr.  r is uniform in [0,1); an r of 0 must
// increment whenever p > 0, which the r < p comparison guarantees.
func incrWithR(counter uint8, logFactor int, r float64) uint8 {
	if counter == MaxVal {
		return MaxVal
	}
	baseval := int(counter) - int(InitVal)
	if baseval < 0 {
		baseval = 0
	}
	if logFactor < 0 {
		logFactor = 0
	}
	p := 1.0 / (float64(baseval)*float64(logFactor) + 1.0)
	if r < p {
		counter++
	}
	return counter
}

// decayed subtracts periods from counter, saturating at 0 — Redis
// LFUDecrAndReturn's clamp.  A period count below 1 changes nothing.
func decayed(counter uint8, periods int) uint8 {
	if periods <= 0 {
		return counter
	}
	if periods > int(counter) { // periods > counter: everything decays away
		return 0
	}
	return counter - uint8(periods)
}

// elapsedMinutes returns the minutes between two readings of the 16-bit
// minute clock, lastMin (stored) and now — LFUTimeElapsed: when now has
// not wrapped past lastMin the difference is direct; otherwise the time
// is treated as wrapping exactly once.  now == lastMin is 0 elapsed;
// the boundary pair lastMin = 65535, now = 0 reads as 0 elapsed (the
// Redis arithmetic 65535-ldt+now, mirrored exactly — do not "fix" it).
// The clock wraps every 65536 minutes (~45.5 days), so an idle stretch
// longer than that is indistinguishable from a short one — an inherent
// property of the 16-bit LDT, documented in the README.
func elapsedMinutes(now, lastMin uint16) int {
	if now >= lastMin {
		return int(now) - int(lastMin)
	}
	return 65535 - int(lastMin) + int(now)
}
