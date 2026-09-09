/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package sharded_hash_ts

import "math/rand"

// SampleStripe returns up to n pseudo-randomly chosen elements from the
// one stripe `stripeIdx` — the eviction-sampling primitive (Redis's
// maxmemory-samples: allkeys-random / volatile-random pick victims, and
// the LFU policies pick the coldest of a random candidate set, from a
// sample of the keyspace).  Only stripe `stripeIdx` is read-locked, and
// every returned element comes from that stripe.
//
// Sampling is by random bucket-head probes with a per-chain reservoir
// draw, so any live element can be returned and elements in longer
// chains are not starved; the distribution is approximate, not uniform
// (bucket lengths differ), and the same element may appear twice in one
// call's result.  The attempt budget scales with the stripe's sparsity
// (heads/live): a stripe right after a growth doubling is half-empty, and
// deletes never shrink heads, so a fixed cap starves sparse stripes.
// A nil or empty stripe yields nil.
//
// rng may be nil, in which case the global math/rand source is used.
// It panics if stripeIdx is out of range (a programmer error: the
// stripe count is fixed at construction and StripeCount reports it).
// Complexity is O(min(n, attempts cap)) chain walks.
func (tt *ShardedHash[T]) SampleStripe(stripeIdx, n int, rng *rand.Rand) []T {
	if tt == nil || len(tt.stripes) == 0 || n < 1 {
		return nil
	}
	if stripeIdx < 0 || stripeIdx >= len(tt.stripes) {
		panic("sharded_hash_ts: SampleStripe called with an out-of-range stripe index")
	}
	intn := rand.Intn
	if rng != nil {
		intn = rng.Intn
	}
	s := tt.stripes[stripeIdx]
	s.lock.RLock()
	defer s.lock.RUnlock()
	tab := s.tab
	if tab.live == 0 {
		return nil
	}
	heads := tab.heads
	out := make([]T, 0, n)
	// Sparsity-aware attempt budget: a hit needs a non-nil bucket, and the
	// non-nil fraction can be as low as live/heads, so scale the cap by
	// heads/live (a dense stripe keeps the small 4n+32 budget).
	sparse := 1
	if tab.live > 0 && len(heads)/tab.live > 1 {
		sparse = len(heads) / tab.live
	}
	maxAttempts := 4*n*sparse + 32
	for attempt := 0; len(out) < n && attempt < maxAttempts; attempt++ {
		node := heads[intn(len(heads))]
		if node == nil {
			continue
		}
		// Reservoir of one over the chain: uniform within the bucket.
		pick := node
		k := 1
		for m := node.next; m != nil; m = m.next {
			k++
			if intn(k) == 0 {
				pick = m
			}
		}
		out = append(out, pick.data)
	}
	return out
}
