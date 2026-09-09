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
// call's result.  Probing is attempt-capped, so a sparse stripe can
// return FEWER than n elements even when it holds n or more — callers
// needing exactly n live elements must retry.  A nil or empty stripe
// yields nil.
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
	for attempt := 0; len(out) < n && attempt < 4*n+8; attempt++ {
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
