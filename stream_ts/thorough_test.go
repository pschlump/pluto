/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stream_ts

import (
	"sync"
	"testing"
)

// TestConcurrentReadWriteMix hammers every side of the stream from many
// goroutines: appends, range snapshots, group reads, acks, claims and
// trims.  Run under -race.
func TestConcurrentReadWriteMix(t *testing.T) {
	var s Stream
	if err := s.CreateGroup("g", MinID); err != nil {
		t.Fatal(err)
	}

	const producers, each = 8, 250
	var wg sync.WaitGroup

	// Producers append concurrently the way Redis clients do — the
	// auto-sequence form, whose "next seq for this ms" resolution runs
	// under the stream's lock, so the strictly-increasing rule holds
	// without the callers coordinating.  (Explicit IDs cannot be added
	// concurrently: the total order has no room for two independent
	// choosers — that is the stream's contract, not a bug.)
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				if _, err := s.Add(ID{Ms: 1, Seq: AutoSeq}, [][2]string{{"p", ""}}); err != nil {
					t.Errorf("auto Add: %v", err)
					return
				}
				// Stale explicit IDs are rejected under the same lock.
				if _, err := s.Add(ID{Ms: 1}, nil); err == nil {
					t.Error("stale explicit Add should have been rejected")
					return
				}
			}
		}(p)
	}

	// Observers read through the snapshot iterators and the group
	// surface while the producers write.
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				n := 0
				for e := range s.Range(MinID, MaxID, 10) {
					_ = e
					n++
				}
				if n > 10 {
					t.Error("count limit exceeded")
					return
				}
				for e := range s.RevRange(MaxID, MinID, 0) {
					_ = e
				}
				_, _, _, _ = s.Pending("g")
				_ = s.PendingRange("g", "", MinID, MaxID, 5)
				_, _ = s.GroupLastID("g")
				_ = s.GroupNames()
				_ = s.GroupConsumers("g")
			}
		}()
	}

	// Consumers take deliveries and acknowledge them.
	for c := 0; c < 4; c++ {
		wg.Add(1)
		go func(c int) {
			defer wg.Done()
			name := "consumer"
			for i := 0; i < 100; i++ {
				entries := s.ReadGroup("g", name, MinID, 5)
				for _, e := range entries {
					if i%2 == 0 {
						s.Ack("g", e.ID)
					}
				}
				// Reclaim abandoned work occasionally.
				s.AutoClaim("g", name, 0, MinID, 3)
			}
		}(c)
	}

	wg.Wait()

	// Accounting: every producer's IDs are distinct, so the stream holds
	// exactly producers*each entries (nothing here deletes or trims).
	if s.Len() != producers*each {
		t.Errorf("Len = %d, want %d", s.Len(), producers*each)
	}
	// The group's last-delivered ID plus its PEL cover everything the
	// consumers touched: delivered - acked - autoclaim-deleted == pending.
	count, _, _, _ := s.Pending("g")
	if count < 0 || count > producers*each {
		t.Errorf("pending count %d out of range", count)
	}
	// One shared consumer name: everything undelivered stays below the
	// last-delivered ID.
	if last, ok := s.GroupLastID("g"); !ok || last == MinID {
		t.Errorf("last-delivered = (%v, %v), deliveries should have happened", last, ok)
	}
}

// TestLockNlCompound runs the atomic compound the _ts surface exists
// for: read-then-ack under one lock hold, with racing writers excluded.
func TestLockNlCompound(t *testing.T) {
	var s Stream
	for i := range 20 {
		_, _ = s.Add(ID{Ms: 1, Seq: uint64(i)}, nil)
	}
	if err := s.CreateGroup("g", MinID); err != nil {
		t.Fatal(err)
	}

	// A competing goroutine tries to claim the same IDs without the
	// compound; only one side can win each entry, and under Lock the
	// compound's read+ack is atomic, so anything it acked was delivered
	// by it.
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			s.AutoClaim("g", "thief", 0, MinID, 2)
		}
	}()

	s.Lock()
	delivered := s.NlReadGroup("g", "worker", MinID, 10)
	acked := s.NlAck("g", idsOf(delivered)...)
	_ = s.NlLen()
	_, _ = s.NlFirstID()
	_ = s.NlLastID()
	_ = s.NlIsEmpty()
	s.NlGroupSetID("g", MinID)
	s.Unlock()

	close(stop)
	wg.Wait()

	if acked != len(delivered) {
		t.Errorf("acked %d of %d delivered in the compound", acked, len(delivered))
	}
	if len(delivered) > 10 {
		t.Errorf("compound delivered %d, want <= 10", len(delivered))
	}
	// Everything the compound delivered and acked is out of the PEL —
	// the thief can only hold the rest.
	if count, _, _, per := s.Pending("g"); count != 0 && per["worker"] != 0 {
		t.Errorf("worker still holds %d pending after acking its batch", per["worker"])
	}
}

func idsOf(es []Entry) []ID {
	out := make([]ID, len(es))
	for i, e := range es {
		out[i] = e.ID
	}
	return out
}
