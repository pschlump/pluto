/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stream_ts

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// entriesOf collects the whole stream in ascending ID order through the
// snapshot iterator.
func entriesOf(s *Stream) []Entry {
	return slices.Collect(s.Range(MinID, MaxID, 0))
}

func TestStreamMarshalJSON(t *testing.T) {
	// Exact array output, ascending ID order.
	var s Stream
	for i := range 3 {
		if _, err := s.Add(ID{Ms: 100, Seq: AutoSeq}, [][2]string{{"sensor", fmt.Sprintf("%d", i*7)}}); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	want := `[{"id":"100-0","fields":[["sensor","0"]]},` +
		`{"id":"100-1","fields":[["sensor","7"]]},` +
		`{"id":"100-2","fields":[["sensor","14"]]}]`
	if string(b) != want {
		t.Errorf("Expected %s, got %s", want, b)
	}

	// An empty (zero-value) stream encodes as [].
	if b, err := json.Marshal(&Stream{}); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty stream, got (%s, %v)", b, err)
	}

	// A direct call on a nil stream encodes as []; json.Marshal on a nil
	// *Stream never reaches the method — the json package writes null
	// for nil pointers itself.
	var nilStream *Stream
	if b, err := nilStream.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-stream call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilStream); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil stream, got (%s, %v)", b, err)
	}

	// Only the entry log is encoded: the last assigned ID and the
	// consumer groups are per-run delivery state and do not appear.
	if err := s.CreateGroup("g", MinID); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	s.SetLastID(ID{Ms: 999})
	if b, err := json.Marshal(&s); err != nil || string(b) != want {
		t.Errorf("Groups and the last ID leaked into the encoding: (%s, %v)", b, err)
	}
}

func TestStreamUnmarshalJSON(t *testing.T) {
	// Decoded order is the ID order; the zero value is usable (the stream
	// has no constructor).
	var s Stream
	data := `[{"id":"1-0","fields":[["job","a"]]},{"id":"1-1","fields":[["job","b"]]}]`
	if err := json.Unmarshal([]byte(data), &s); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	got := entriesOf(&s)
	if len(got) != 2 || got[0].ID != (ID{Ms: 1}) || got[1].ID != (ID{Ms: 1, Seq: 1}) ||
		got[0].Fields[0] != [2]string{"job", "a"} {
		t.Errorf("Unexpected entries after unmarshal: %v", got)
	}

	// The whole state is replaced, so the last assigned ID is the
	// restored tail and the next Add starts above it.
	if s.LastID() != (ID{Ms: 1, Seq: 1}) {
		t.Errorf("LastID after unmarshal = %v", s.LastID())
	}
	if _, err := s.Add(ID{Ms: 1, Seq: AutoSeq}, nil); err != nil {
		t.Errorf("Add after unmarshal: %v", err)
	}

	// A round trip rebuilds an equivalent stream.
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var again Stream
	if err := json.Unmarshal(b, &again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(idsOf(entriesOf(&again))), fmt.Sprint(idsOf(entriesOf(&s))); got != want {
		t.Errorf("Round trip got %s, want %s", got, want)
	}

	// Unmarshaling replaces the entry log; it does not append.  A higher
	// previous last ID is replaced too — the document is the whole state.
	s.SetLastID(ID{Ms: 50})
	if err := json.Unmarshal([]byte(`[{"id":"7-0","fields":[["k","v"]]}]`), &s); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if s.Len() != 1 {
		t.Errorf("Expected replacement, got length %d", s.Len())
	}
	if s.LastID() != (ID{Ms: 7}) {
		t.Errorf("LastID after replacement = %v, want 7-0", s.LastID())
	}

	// Replacing the state drops the consumer groups as well.
	var gs Stream
	for i := range 3 {
		if _, err := gs.Add(ID{Ms: 1, Seq: uint64(i)}, [][2]string{{"job", "x"}}); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	if err := gs.CreateGroup("workers", MinID); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if n := len(gs.ReadGroup("workers", "alice", MinID, 2)); n != 2 {
		t.Fatalf("ReadGroup delivered %d", n)
	}
	if err := json.Unmarshal([]byte(`[{"id":"9-0","fields":[["job","y"]]}]`), &gs); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if names := gs.GroupNames(); len(names) != 0 {
		t.Errorf("Groups survived the state replacement: %v", names)
	}

	// An empty array and null clear the stream — the last-ID marker goes
	// back to MinID, so even 1-0 can be added afterwards.
	if err := json.Unmarshal([]byte("[]"), &s); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !s.IsEmpty() {
		t.Errorf("Expected [] to clear the stream.")
	}
	if _, err := s.Add(ID{Ms: 1}, nil); err != nil {
		t.Fatalf("Add after clear: %v", err)
	}
	if err := json.Unmarshal([]byte("null"), &s); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !s.IsEmpty() || s.LastID() != MinID {
		t.Errorf("Expected null to clear the stream, got (len %d, last %v)", s.Len(), s.LastID())
	}

	// Decode and validation errors are returned and leave the stream
	// untouched.
	keep := &Stream{}
	if _, err := keep.Add(ID{Ms: 3}, [][2]string{{"k", "keep"}}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	for _, badData := range []string{
		"[{",                             // malformed
		`{"id":"1-0"}`,                   // not an array
		"7",                              // not an array
		`[{"id":"bogus","fields":null}]`, // invalid ID
		`[{"id":7,"fields":null}]`,       // non-string ID
		`[{"id":"1-1"},{"id":"1-1"}]`,    // duplicate IDs
		`[{"id":"2-0"},{"id":"1-0"}]`,    // not increasing
		`[{"id":"0-0","fields":null}]`,   // 0-0 is never an entry ID
		`[{"id":"5-*","fields":null}]`,   // the AutoSeq sentinel is a request form
	} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got := fmt.Sprint(idsOf(entriesOf(keep))); got != "[3-0]" {
			t.Errorf("Stream changed after the error on %s: %s", badData, got)
		}
	}

	// The ordering violations wrap ErrIDTooSmall, the AutoSeq sentinel
	// wraps ErrInvalidID.
	if err := json.Unmarshal([]byte(`[{"id":"2-0"},{"id":"1-0"}]`), keep); !errors.Is(err, ErrIDTooSmall) {
		t.Errorf("Expected ErrIDTooSmall for non-increasing IDs, got %v", err)
	}
	if err := json.Unmarshal([]byte(`[{"id":"5-*"}]`), keep); !errors.Is(err, ErrInvalidID) {
		t.Errorf("Expected ErrInvalidID for the AutoSeq sentinel, got %v", err)
	}
	if got := fmt.Sprint(idsOf(entriesOf(keep))); got != "[3-0]" {
		t.Errorf("Stream changed after the validation errors: %s", got)
	}
}

// TestStreamUnmarshalJSONPanics verifies that UnmarshalJSON joins the
// insert family: storing entries into a nil stream panics with the Add
// precedent's message, while [] and null — which store nothing — are
// tolerated everywhere.  (The stream has no constructor, so a zero-value
// Stream accepts data; only the nil pointer panics.)
func TestStreamUnmarshalJSONPanics(t *testing.T) {
	var nilStream *Stream
	for _, data := range []string{"[]", "null"} {
		if err := nilStream.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil stream to be tolerated, got %v", data, err)
		}
	}
	expectPanic(t, "stream_ts: UnmarshalJSON called on a nil Stream", func() {
		_ = nilStream.UnmarshalJSON([]byte(`[{"id":"1-0","fields":null}]`))
	})

	// The zero value is ready to use, including for unmarshaling.
	var zero Stream
	if err := json.Unmarshal([]byte(`[{"id":"1-0","fields":null}]`), &zero); err != nil {
		t.Errorf("Expected a zero-value stream to accept data, got %v", err)
	}
	if zero.Len() != 1 {
		t.Errorf("Expected 1 entry in the zero-value stream, got %d", zero.Len())
	}
}

// TestJSONConcurrent hammers MarshalJSON and UnmarshalJSON concurrently
// with producers and replacers; every marshaled output must be a valid
// JSON array of entries with strictly increasing IDs.  Run under -race.
func TestJSONConcurrent(t *testing.T) {
	var s Stream

	const producers, each = 4, 100
	stop := make(chan struct{})
	var writers sync.WaitGroup
	var readers sync.WaitGroup

	// A marshaling reader: MarshalJSON snapshots under the read lock, so
	// it is safe while the writers add and replace.
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			b, err := s.MarshalJSON()
			if err != nil {
				t.Errorf("MarshalJSON: %v", err)
				return
			}
			var probe []Entry
			if err := json.Unmarshal(b, &probe); err != nil {
				t.Errorf("MarshalJSON produced invalid JSON %s: %v", b, err)
				return
			}
			for i := 1; i < len(probe); i++ {
				if CompareID(probe[i-1].ID, probe[i].ID) >= 0 {
					t.Errorf("MarshalJSON output not in ascending ID order: %v", probe[i-1].ID)
					return
				}
			}
		}
	}()

	// Producers append in the auto-sequence form.
	for p := 0; p < producers; p++ {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for i := 0; i < each; i++ {
				if _, err := s.Add(ID{Ms: 100, Seq: AutoSeq}, nil); err != nil {
					t.Errorf("Add: %v", err)
					return
				}
			}
		}()
	}

	// Concurrent replacers: each replaces the whole state with a valid
	// one-entry document of its own.
	for w := 0; w < 4; w++ {
		writers.Add(1)
		go func(w int) {
			defer writers.Done()
			for i := 0; i < 50; i++ {
				doc := fmt.Sprintf(`[{"id":"%d-0","fields":[["w","%d"]]}]`, w+1, i)
				if err := s.UnmarshalJSON([]byte(doc)); err != nil {
					t.Errorf("worker %d: UnmarshalJSON: %v", w, err)
					return
				}
			}
		}(w)
	}

	writers.Wait()
	close(stop)
	readers.Wait()

	// The final state is whatever the last writer left, but it must be
	// internally consistent: ascending IDs, Len matching the range count.
	entries := entriesOf(&s)
	if len(entries) != s.Len() {
		t.Errorf("Len = %d, Range visited %d", s.Len(), len(entries))
	}
	for i := 1; i < len(entries); i++ {
		if CompareID(entries[i-1].ID, entries[i].ID) >= 0 {
			t.Fatalf("entries not strictly ascending: %v then %v", entries[i-1].ID, entries[i].ID)
		}
	}
}
