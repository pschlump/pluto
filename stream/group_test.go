/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stream

import (
	"errors"
	"testing"
	"time"
)

// fill adds n entries 1-0 .. 1-(n-1) with one field each and returns the
// stream.
func fill(t *testing.T, n int) *Stream {
	t.Helper()
	s := &Stream{}
	for i := range n {
		mustAdd(t, s, ID{Ms: 1, Seq: uint64(i)}, [2]string{"f", "v"})
	}
	return s
}

func TestCreateGroup(t *testing.T) {
	s := fill(t, 3)
	if err := s.CreateGroup("a", MinID); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := s.CreateGroup("b", MaxID); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := s.CreateGroup("a", MinID); !errors.Is(err, ErrGroupExists) {
		t.Errorf("duplicate CreateGroup: got %v, want ErrGroupExists", err)
	}
	if names := s.GroupNames(); len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("GroupNames = %v, want [a b] sorted", names)
	}
	if n := s.DestroyGroup("a"); n != 0 {
		t.Errorf("DestroyGroup dropped %d pending, want 0", n)
	}
	if names := s.GroupNames(); len(names) != 1 || names[0] != "b" {
		t.Errorf("GroupNames after destroy = %v", names)
	}
	if s.DestroyGroup("missing") != 0 {
		t.Error("DestroyGroup of a missing group should return 0")
	}
	// A zero-value stream can create groups directly.
	var s2 Stream
	if err := s2.CreateGroup("g", MinID); err != nil {
		t.Errorf("CreateGroup on zero-value stream: %v", err)
	}
}

func TestReadGroupDelivery(t *testing.T) {
	s := fill(t, 5)
	if err := s.CreateGroup("g", MinID); err != nil {
		t.Fatal(err)
	}

	// ">" form (after == MinID): delivers above the group's
	// last-delivered ID, advancing it.
	got := idsOf(s.ReadGroup("g", "c1", MinID, 2))
	if len(got) != 2 || got[0] != (ID{Ms: 1}) || got[1] != (ID{Ms: 1, Seq: 1}) {
		t.Fatalf("first read = %v", got)
	}
	if last, ok := s.GroupLastID("g"); !ok || last != (ID{Ms: 1, Seq: 1}) {
		t.Errorf("last-delivered after first read = (%v, %v)", last, ok)
	}
	// Consumers are created implicitly by delivery.
	if cons := s.GroupConsumers("g"); len(cons) != 1 || cons[0] != "c1" {
		t.Errorf("consumers = %v", cons)
	}

	got = idsOf(s.ReadGroup("g", "c2", MinID, 0))
	if len(got) != 3 || got[0] != (ID{Ms: 1, Seq: 2}) || got[2] != (ID{Ms: 1, Seq: 4}) {
		t.Fatalf("second read = %v", got)
	}
	if count, min, max, per := s.Pending("g"); count != 5 || min != (ID{Ms: 1}) || max != (ID{Ms: 1, Seq: 4}) ||
		per["c1"] != 2 || per["c2"] != 3 {
		t.Errorf("Pending = (%d, %v, %v, %v)", count, min, max, per)
	}

	// Everything is pending now: further ">" reads deliver nothing.
	if got := s.ReadGroup("g", "c1", MinID, 10); len(got) != 0 {
		t.Errorf("read on fully-pending stream = %v", got)
	}
	// A missing group returns nil.
	if got := s.ReadGroup("missing", "c1", MinID, 10); got != nil {
		t.Errorf("ReadGroup on missing group = %v", got)
	}

	// New entries are delivered to whoever asks next.
	mustAdd(t, s, ID{Ms: 2})
	got = idsOf(s.ReadGroup("g", "c1", MinID, 10))
	if len(got) != 1 || got[0] != (ID{Ms: 2}) {
		t.Errorf("read after new add = %v", got)
	}
	if last, _ := s.GroupLastID("g"); last != (ID{Ms: 2}) {
		t.Errorf("last-delivered = %v", last)
	}
}

func TestReadGroupStartIDs(t *testing.T) {
	// A group created at MinID sees the whole stream; one created at the
	// current tail (Redis "$") only sees later entries.
	s := fill(t, 3)
	_ = s.CreateGroup("all", MinID)
	_ = s.CreateGroup("new", s.LastID())

	if got := idsOf(s.ReadGroup("all", "c", MinID, 10)); len(got) != 3 {
		t.Errorf("group at 0-0 delivered %v", got)
	}
	if got := s.ReadGroup("new", "c", MinID, 10); len(got) != 0 {
		t.Errorf("group at $ delivered %v, want nothing", got)
	}
	mustAdd(t, s, ID{Ms: 2})
	if got := idsOf(s.ReadGroup("new", "c", MinID, 10)); len(got) != 1 || got[0] != (ID{Ms: 2}) {
		t.Errorf("group at $ delivered after add = %v", got)
	}

	// GroupSetID repositions the next ">" read.
	s2 := fill(t, 5)
	_ = s2.CreateGroup("g", MinID)
	s2.GroupSetID("g", ID{Ms: 1, Seq: 2})
	if got := idsOf(s2.ReadGroup("g", "c", MinID, 10)); len(got) != 2 || got[0] != (ID{Ms: 1, Seq: 3}) {
		t.Errorf("read after GroupSetID = %v", got)
	}
	s2.GroupSetID("missing", MinID) // no-op
}

func TestReadGroupReplayForm(t *testing.T) {
	s := fill(t, 4)
	_ = s.CreateGroup("g", MinID)
	if got := idsOf(s.ReadGroup("g", "c1", MinID, 2)); len(got) != 2 {
		t.Fatalf("setup read = %v", got)
	}

	// The explicit-after form replays above that ID without advancing
	// the group's last-delivered ID, and skips live pending entries:
	// 1-1 is pending to c1 (skipped), 1-2 and 1-3 are re-delivered to c2.
	got := idsOf(s.ReadGroup("g", "c2", ID{Ms: 1}, 0))
	if len(got) != 2 || got[0] != (ID{Ms: 1, Seq: 2}) || got[1] != (ID{Ms: 1, Seq: 3}) {
		t.Errorf("replay read = %v, want [1-2 1-3]", got)
	}
	if last, _ := s.GroupLastID("g"); last != (ID{Ms: 1, Seq: 1}) {
		t.Errorf("replay moved last-delivered to %v, want it unchanged", last)
	}
	// The re-delivered entries changed ownership in the PEL.
	if count, _, _, per := s.Pending("g"); count != 4 || per["c1"] != 2 || per["c2"] != 2 {
		t.Errorf("Pending after replay = (%d, %v)", count, per)
	}

	// A replay floor of MaxID delivers nothing (nothing sorts above it).
	if got := s.ReadGroup("g", "c2", MaxID, 10); len(got) != 0 {
		t.Errorf("replay from MaxID = %v", got)
	}

	// A literal replay from the very beginning is GroupSetID + the ">"
	// form (after == MinID is reserved for ">" semantics).
	s2 := fill(t, 3)
	_ = s2.CreateGroup("g", MaxID)
	s2.GroupSetID("g", MinID)
	if got := idsOf(s2.ReadGroup("g", "c", MinID, 10)); len(got) != 3 {
		t.Errorf("GroupSetID + > read = %v", got)
	}
}

func TestAck(t *testing.T) {
	s := fill(t, 4)
	_ = s.CreateGroup("g", MinID)
	_ = s.ReadGroup("g", "c1", MinID, 4)

	if n := s.Ack("g", ID{Ms: 1}, ID{Ms: 1, Seq: 2}, ID{Ms: 9}); n != 2 {
		t.Errorf("Ack returned %d, want 2 (one ID was not pending)", n)
	}
	if n := s.Ack("g", ID{Ms: 1}); n != 0 {
		t.Errorf("re-Ack returned %d, want 0", n)
	}
	if count, _, _, _ := s.Pending("g"); count != 2 {
		t.Errorf("Pending after ack = %d, want 2", count)
	}
	// Acking with no IDs, and on a missing group, is 0.
	if s.Ack("g") != 0 || s.Ack("missing", ID{Ms: 1}) != 0 {
		t.Error("Ack edge cases should return 0")
	}
	// Acked entries can be re-delivered by a replay read (floor 0-1, so
	// the acked 1-0 is above it; the still-pending 1-1 and 1-3 are not).
	if got := idsOf(s.ReadGroup("g", "c2", ID{Seq: 1}, 0)); len(got) != 2 || got[0] != (ID{Ms: 1}) || got[1] != (ID{Ms: 1, Seq: 2}) {
		t.Errorf("re-delivery after ack = %v", got)
	}
}

func TestPendingSummary(t *testing.T) {
	s := fill(t, 3)
	_ = s.CreateGroup("g", MinID)
	// A group with an empty PEL reports zeros and an empty per-consumer
	// map (never a nil one).
	count, min, max, per := s.Pending("g")
	if count != 0 || min != MinID || max != MinID || per == nil || len(per) != 0 {
		t.Errorf("empty Pending = (%d, %v, %v, %v)", count, min, max, per)
	}
	if count, _, _, per := s.Pending("missing"); count != 0 || per == nil {
		t.Errorf("missing-group Pending = %d, %v", count, per)
	}

	_ = s.ReadGroup("g", "c1", MinID, 2)
	_ = s.ReadGroup("g", "c2", MinID, 1)
	count, min, max, per = s.Pending("g")
	if count != 3 || min != (ID{Ms: 1}) || max != (ID{Ms: 1, Seq: 2}) ||
		per["c1"] != 2 || per["c2"] != 1 || len(per) != 2 {
		t.Errorf("Pending = (%d, %v, %v, %v)", count, min, max, per)
	}
}

func TestPendingRange(t *testing.T) {
	s := fill(t, 6)
	_ = s.CreateGroup("g", MinID)
	_ = s.ReadGroup("g", "c1", MinID, 4)
	_ = s.ReadGroup("g", "c2", MinID, 2)

	pes := s.PendingRange("g", "", MinID, MaxID, 0)
	if len(pes) != 6 {
		t.Fatalf("full PendingRange len = %d", len(pes))
	}
	if pes[0].ID != (ID{Ms: 1}) || pes[5].ID != (ID{Ms: 1, Seq: 5}) {
		t.Errorf("PendingRange IDs = %v .. %v", pes[0].ID, pes[5].ID)
	}
	if pes[0].Consumer != "c1" || pes[0].DeliveryCount != 1 {
		t.Errorf("PendingRange[0] = %+v", pes[0])
	}
	if pes[0].DeliveryTime.IsZero() {
		t.Error("DeliveryTime should be set")
	}
	if pes[4].Consumer != "c2" || pes[5].Consumer != "c2" {
		t.Errorf("c2 entries = %v %v", pes[4], pes[5])
	}

	// Window, consumer filter and count.
	pes = s.PendingRange("g", "c1", ID{Ms: 1, Seq: 1}, ID{Ms: 1, Seq: 3}, 0)
	if len(pes) != 3 || pes[0].ID != (ID{Ms: 1, Seq: 1}) || pes[2].ID != (ID{Ms: 1, Seq: 3}) {
		t.Errorf("filtered PendingRange = %v", pes)
	}
	if pes := s.PendingRange("g", "nobody", MinID, MaxID, 0); len(pes) != 0 {
		t.Errorf("unknown consumer PendingRange = %v", pes)
	}
	if pes := s.PendingRange("g", "", MinID, MaxID, 2); len(pes) != 2 {
		t.Errorf("counted PendingRange len = %d", len(pes))
	}
	if pes := s.PendingRange("missing", "", MinID, MaxID, 0); pes != nil {
		t.Errorf("missing-group PendingRange = %v", pes)
	}
}

func TestClaim(t *testing.T) {
	s := fill(t, 4)
	_ = s.CreateGroup("g", MinID)
	_ = s.ReadGroup("g", "c1", MinID, 4)

	// minIdle 0 claims unconditionally: transfer 1-0 and 1-2 to c2.
	got := idsOf(s.Claim("g", "c2", 0, ID{Ms: 1}, ID{Ms: 1, Seq: 2}, ID{Ms: 9}))
	if len(got) != 2 || got[0] != (ID{Ms: 1}) || got[1] != (ID{Ms: 1, Seq: 2}) {
		t.Fatalf("Claim = %v", got)
	}
	pes := s.PendingRange("g", "", MinID, MaxID, 0)
	if pes[0].Consumer != "c2" || pes[0].DeliveryCount != 2 {
		t.Errorf("claimed entry state = %+v, want c2 count 2", pes[0])
	}
	if pes[1].Consumer != "c1" || pes[1].DeliveryCount != 1 {
		t.Errorf("unclaimed entry state = %+v", pes[1])
	}
	if count, _, _, per := s.Pending("g"); count != 4 || per["c1"] != 2 || per["c2"] != 2 {
		t.Errorf("Pending after claim = (%d, %v)", count, per)
	}
	// The claiming consumer was created implicitly.
	if cons := s.GroupConsumers("g"); len(cons) != 2 || cons[0] != "c1" || cons[1] != "c2" {
		t.Errorf("consumers = %v", cons)
	}

	// A large minIdle claims nothing (real idle time is ~0).
	if got := s.Claim("g", "c1", time.Hour, ID{Ms: 1}); len(got) != 0 {
		t.Errorf("Claim under minIdle = %v", got)
	}
	// Missing group and no IDs are nil.
	if s.Claim("missing", "c", 0, ID{Ms: 1}) != nil || s.Claim("g", "c", 0) != nil {
		t.Error("Claim edge cases should return nil")
	}

	// A pending entry whose stream entry was deleted is dropped from the
	// PEL silently and not returned.
	s.Delete(ID{Ms: 1, Seq: 1})
	if got := s.Claim("g", "c2", 0, ID{Ms: 1, Seq: 1}); len(got) != 0 {
		t.Errorf("Claim of deleted entry = %v", got)
	}
	if count, _, _, _ := s.Pending("g"); count != 3 {
		t.Errorf("Pending after claiming a deleted entry = %d, want 3", count)
	}
}

func TestAutoClaim(t *testing.T) {
	s := fill(t, 8)
	_ = s.CreateGroup("g", MinID)
	_ = s.ReadGroup("g", "c1", MinID, 8)

	// Entries 1-1 and 1-5 lose their stream entries: AutoClaim must
	// report them as deleted and clear them from the PEL.
	s.Delete(ID{Ms: 1, Seq: 1})
	s.Delete(ID{Ms: 1, Seq: 5})

	entries, next, deleted := s.AutoClaim("g", "c2", 0, MinID, 3)
	if got := idsOf(entries); len(got) != 3 || got[0] != (ID{Ms: 1}) || got[1] != (ID{Ms: 1, Seq: 2}) || got[2] != (ID{Ms: 1, Seq: 3}) {
		t.Fatalf("first AutoClaim = %v", got)
	}
	if len(deleted) != 1 || deleted[0] != (ID{Ms: 1, Seq: 1}) {
		t.Errorf("first deleted = %v, want [1-1]", deleted)
	}
	if next != (ID{Ms: 1, Seq: 4}) {
		t.Errorf("first nextStart = %v, want 1-4", next)
	}

	entries, next, deleted = s.AutoClaim("g", "c2", 0, next, 3)
	if got := idsOf(entries); len(got) != 3 || got[0] != (ID{Ms: 1, Seq: 4}) || got[1] != (ID{Ms: 1, Seq: 6}) || got[2] != (ID{Ms: 1, Seq: 7}) {
		t.Fatalf("second AutoClaim = %v", got)
	}
	if len(deleted) != 1 || deleted[0] != (ID{Ms: 1, Seq: 5}) {
		t.Errorf("second deleted = %v, want [1-5]", deleted)
	}
	// 1-7 was the last examined entry and the greatest left in the PEL:
	// the scan is exhausted and nextStart is MinID.
	if next != MinID {
		t.Errorf("second nextStart = %v, want 0-0 (exhausted)", next)
	}
	// Everything surviving is now owned by c2 with delivery count 2.
	if count, _, _, per := s.Pending("g"); count != 6 || per["c2"] != 6 {
		t.Errorf("Pending after AutoClaim sweep = (%d, %v)", count, per)
	}
	for _, pe := range s.PendingRange("g", "", MinID, MaxID, 0) {
		if pe.Consumer != "c2" || pe.DeliveryCount != 2 {
			t.Errorf("post-AutoClaim state = %+v", pe)
		}
	}
	// A missing group is (nil, MinID, nil).
	if e, n, d := s.AutoClaim("missing", "c", 0, MinID, 3); e != nil || n != MinID || d != nil {
		t.Errorf("AutoClaim on missing group = (%v, %v, %v)", e, n, d)
	}
}

func TestAutoClaimAttemptCap(t *testing.T) {
	// A large minIdle makes every entry unclaimable; one call must not
	// scan the whole PEL but stop at autoClaimAttempts x count examined
	// entries, resuming from the cursor on the next call.
	s := fill(t, 40)
	_ = s.CreateGroup("g", MinID)
	_ = s.ReadGroup("g", "c1", MinID, 40)

	entries, next, _ := s.AutoClaim("g", "c2", time.Hour, MinID, 3)
	if len(entries) != 0 {
		t.Fatalf("nothing should be claimable under a 1h minIdle, got %v", idsOf(entries))
	}
	if next != (ID{Ms: 1, Seq: 30}) { // 10 x 3 examined (1-0 .. 1-29), resume at 1-30
		t.Errorf("nextStart after capped scan = %v, want 1-30", next)
	}
	// Walking the cursor to exhaustion takes ceil(40/30) more calls and
	// claims nothing.
	for next != MinID {
		entries, next, _ = s.AutoClaim("g", "c2", time.Hour, next, 3)
		if len(entries) != 0 {
			t.Fatalf("unexpected claims: %v", idsOf(entries))
		}
	}
	if count, _, _, _ := s.Pending("g"); count != 40 {
		t.Errorf("Pending after capped sweeps = %d, want 40", count)
	}

	// count <= 0 means no limit and no cap: one call sweeps everything
	// (claiming it, with minIdle 0).
	entries, next, _ = s.AutoClaim("g", "c2", 0, MinID, 0)
	if len(entries) != 40 {
		t.Errorf("uncapped AutoClaim claimed %d, want 40", len(entries))
	}
	if next != MinID {
		t.Errorf("uncapped nextStart = %v, want 0-0", next)
	}
}

func TestTrimDeleteLeavePEL(t *testing.T) {
	// The spec's edge case: evicted/deleted entries keep their PEL
	// entries; AutoClaim reports them as deleted.
	s := fill(t, 5)
	_ = s.CreateGroup("g", MinID)
	_ = s.ReadGroup("g", "c1", MinID, 5)

	s.Delete(ID{Ms: 1})
	if n := s.TrimMaxLen(2); n != 2 { // evicts 1-1 and 1-2
		t.Fatalf("TrimMaxLen evicted %d, want 2", n)
	}
	if s.Len() != 2 {
		t.Fatalf("Len after trim = %d", s.Len())
	}
	if count, min, max, _ := s.Pending("g"); count != 5 || min != (ID{Ms: 1}) || max != (ID{Ms: 1, Seq: 4}) {
		t.Errorf("PEL after trim = (%d, %v, %v), want all 5 kept", count, min, max)
	}
	// Ack still works for evicted IDs.
	if n := s.Ack("g", ID{Ms: 1, Seq: 4}); n != 1 {
		t.Errorf("Ack of trimmed pending = %d, want 1", n)
	}

	// Of the four still-pending IDs, only 1-3 survives in the stream:
	// 1-0 was deleted and 1-1/1-2 were trimmed (1-4 was acked above).
	entries, next, deleted := s.AutoClaim("g", "c2", 0, MinID, 10)
	if len(entries) != 1 || entries[0].ID != (ID{Ms: 1, Seq: 3}) {
		t.Errorf("surviving claims = %v", idsOf(entries))
	}
	if len(deleted) != 3 || deleted[0] != (ID{Ms: 1}) || deleted[1] != (ID{Ms: 1, Seq: 1}) || deleted[2] != (ID{Ms: 1, Seq: 2}) {
		t.Errorf("deleted report = %v, want [1-0 1-1 1-2]", deleted)
	}
	if next != MinID {
		t.Errorf("nextStart = %v", next)
	}
	if count, _, _, _ := s.Pending("g"); count != 1 {
		t.Errorf("PEL after AutoClaim sweep = %d, want 1", count)
	}

	// TrimMaxLen(0) empties the stream; the PEL survives intact.
	if n := s.TrimMaxLen(0); n != 2 {
		t.Fatalf("TrimMaxLen(0) evicted %d, want 2", n)
	}
	if s.Len() != 0 || s.LastID() != (ID{Ms: 1, Seq: 4}) {
		t.Errorf("stream after empty trim: Len %d LastID %v", s.Len(), s.LastID())
	}
	if count, _, _, _ := s.Pending("g"); count != 1 {
		t.Errorf("PEL after empty trim = %d, want 1", count)
	}
}

func TestGroupConsumersLifecycle(t *testing.T) {
	s := fill(t, 4)
	_ = s.CreateGroup("g", MinID)

	if !s.GroupCreateConsumer("g", "alice") {
		t.Error("GroupCreateConsumer on a new name should return true")
	}
	if s.GroupCreateConsumer("g", "alice") {
		t.Error("GroupCreateConsumer on an existing name should return false")
	}
	if s.GroupCreateConsumer("missing", "alice") {
		t.Error("GroupCreateConsumer on a missing group should return false")
	}
	if cons := s.GroupConsumers("g"); len(cons) != 1 || cons[0] != "alice" {
		t.Errorf("consumers = %v", cons)
	}
	// An empty consumer set is an empty slice, missing group is nil.
	if cons := s.GroupConsumers("missing"); cons != nil {
		t.Errorf("GroupConsumers on missing group = %v", cons)
	}

	// Deleting a consumer drops only its pending entries.
	_ = s.ReadGroup("g", "alice", MinID, 3)
	_ = s.ReadGroup("g", "bob", MinID, 1)
	if n := s.GroupDeleteConsumer("g", "alice"); n != 3 {
		t.Errorf("GroupDeleteConsumer dropped %d, want 3", n)
	}
	if count, _, _, per := s.Pending("g"); count != 1 || per["bob"] != 1 {
		t.Errorf("Pending after consumer delete = (%d, %v)", count, per)
	}
	if cons := s.GroupConsumers("g"); len(cons) != 1 || cons[0] != "bob" {
		t.Errorf("consumers after delete = %v", cons)
	}
	if s.GroupDeleteConsumer("g", "alice") != 0 {
		t.Error("re-deleting a consumer should return 0")
	}
	if s.GroupDeleteConsumer("missing", "alice") != 0 {
		t.Error("deleting from a missing group should return 0")
	}

	// DestroyGroup reports the pending count it dropped.
	if n := s.DestroyGroup("g"); n != 1 {
		t.Errorf("DestroyGroup dropped %d, want 1", n)
	}
}
